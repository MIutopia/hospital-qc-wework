package qc

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"

	"github.com/rs/zerolog/log"
)

// Engine 质控引擎
type Engine struct {
	ruleDAO    *dao.RuleDAO
	caseDAO    *dao.CaseDAO
	resultDAO  *dao.ResultDAO
	parser     *DSLParser
	concurrency int
}

// NewEngine 创建质控引擎
func NewEngine(ruleDAO *dao.RuleDAO, caseDAO *dao.CaseDAO, resultDAO *dao.ResultDAO, concurrency int) *Engine {
	return &Engine{
		ruleDAO:     ruleDAO,
		caseDAO:     caseDAO,
		resultDAO:   resultDAO,
		parser:      NewDSLParser(),
		concurrency: concurrency,
	}
}

// BatchResult 批次执行结果
type BatchResult struct {
	BatchID      string `json:"batchId"`
	TotalCases   int    `json:"totalCases"`
	DefectCases  int    `json:"defectCases"`
	TotalDefects int    `json:"totalDefects"`
	PassedCases  int    `json:"passedCases"`
	Elapsed      string `json:"elapsed"`
}

// RunBatch 执行一批质控
func (e *Engine) RunBatch() (*BatchResult, error) {
	start := time.Now()
	batchID := fmt.Sprintf("QC_%s_%03d", start.Format("20060102"), start.UnixMilli()%1000)

	log.Info().Str("batchId", batchID).Msg("质控批次开始")

	// 1. 加载启用规则
	rules, err := e.ruleDAO.GetEnabledRules()
	if err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}
	if len(rules) == 0 {
		log.Warn().Msg("没有启用的质控规则")
		return &BatchResult{BatchID: batchID, Elapsed: time.Since(start).String()}, nil
	}
	log.Info().Int("ruleCount", len(rules)).Msg("规则加载完成")

	// 2. 查询待质控病例
	cases, err := e.caseDAO.GetPendingCases(0)
	if err != nil {
		return nil, fmt.Errorf("查询待质控病例失败: %w", err)
	}
	if len(cases) == 0 {
		log.Info().Msg("没有待质控的病例")
		return &BatchResult{BatchID: batchID, Elapsed: time.Since(start).String()}, nil
	}
	log.Info().Int("caseCount", len(cases)).Msg("待质控病例加载完成")

	// 3. 并发执行质控
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		allResults []model.QCResult
		defectCases int
		passedCases int
	)

	sem := make(chan struct{}, e.concurrency)

	for _, c := range cases {
		wg.Add(1)
		sem <- struct{}{} // 获取信号量

		go func(caseItem model.InpatientCase) {
			defer wg.Done()
			defer func() { <-sem }() // 释放信号量

			results := e.executeForCase(caseItem, rules, batchID)

			mu.Lock()
			allResults = append(allResults, results...)
			hasDefect := false
			for _, r := range results {
				if r.IsDefect == 1 {
					hasDefect = true
				}
			}
			if hasDefect {
				defectCases++
			} else {
				passedCases++
			}
			mu.Unlock()
		}(c)
	}

	wg.Wait()

	// 4. 批量写入质控结果
	if len(allResults) > 0 {
		if err := e.resultDAO.BatchCreate(allResults); err != nil {
			return nil, fmt.Errorf("写入质控结果失败: %w", err)
		}
	}

	// 5. 更新病例状态
	for _, c := range cases {
		hasDefect := false
		for _, r := range allResults {
			if r.CaseID == c.ID && r.IsDefect == 1 {
				hasDefect = true
				break
			}
		}
		status := model.QCStatusPassed
		if hasDefect {
			status = model.QCStatusIssued
		}
		if err := e.caseDAO.UpdateQCStatus(c.ID, status); err != nil {
			log.Warn().Err(err).Int64("caseId", c.ID).Msg("更新病例状态失败")
		}
	}

	elapsed := time.Since(start)
	result := &BatchResult{
		BatchID:      batchID,
		TotalCases:   len(cases),
		DefectCases:  defectCases,
		TotalDefects: len(allResults),
		PassedCases:  passedCases,
		Elapsed:      elapsed.Round(time.Millisecond).String(),
	}

	log.Info().
		Str("batchId", batchID).
		Int("totalCases", result.TotalCases).
		Int("defectCases", result.DefectCases).
		Int("totalDefects", result.TotalDefects).
		Int("passedCases", result.PassedCases).
		Str("elapsed", result.Elapsed).
		Msg("质控批次完成")

	return result, nil
}

// executeForCase 对单个病例执行全部规则
func (e *Engine) executeForCase(caseItem model.InpatientCase, rules []model.QCRule, batchID string) []model.QCResult {
	var results []model.QCResult

	// 构建执行环境（含 raw_data JSON 解析）
	env := buildEnv(caseItem)

	for _, rule := range rules {
		// 解析 DSL
		dsl, err := e.parser.Parse(rule.RuleExpression)
		if err != nil {
			log.Warn().Err(err).Int64("ruleId", rule.ID).Msg("规则 DSL 解析失败")
			continue
		}

		// 编译表达式
		program, err := e.parser.Compile(dsl)
		if err != nil {
			log.Warn().Err(err).Int64("ruleId", rule.ID).Str("ruleCode", rule.RuleCode).Msg("规则编译失败")
			continue
		}

		// 执行求值
		isDefect, err := e.parser.Eval(program, env)
		if err != nil {
			log.Warn().Err(err).Int64("ruleId", rule.ID).Str("ruleCode", rule.RuleCode).Msg("规则执行失败")
			continue
		}

		// 构建结果
		result := model.QCResult{
			CaseID:   caseItem.ID,
			RuleID:   rule.ID,
			QCBatchID: &batchID,
		}

		if isDefect {
			result.IsDefect = 1
			detail := dsl.DefectTemplate
			result.DefectDetail = &detail
			if dsl.Suggestion != "" {
				result.Suggestion = &dsl.Suggestion
			}
		} else {
			result.IsDefect = 0
		}

		results = append(results, result)
	}

	return results
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// buildEnv 从病例记录构造规则执行环境
// raw_data 为 JSON 字符串（病历文书内容），解析失败时按空 map 处理
func buildEnv(caseItem model.InpatientCase) *Env {
	var gender int
	if caseItem.PatientGender != nil {
		gender = *caseItem.PatientGender
	}
	var age int
	if caseItem.PatientAge != nil {
		age = *caseItem.PatientAge
	}

	rawMap := make(map[string]interface{})
	if caseItem.RawData != nil && strings.TrimSpace(*caseItem.RawData) != "" {
		if err := json.Unmarshal([]byte(*caseItem.RawData), &rawMap); err != nil {
			log.Warn().Err(err).Int64("caseId", caseItem.ID).Msg("raw_data JSON 解析失败，按空对象处理")
		}
	}

	return &Env{
		AdmitTime:     caseItem.AdmitTime,
		DischargeTime: caseItem.DischargeTime,
		Diagnosis:     safeString(caseItem.Diagnosis),
		CaseStatus:    caseItem.CaseStatus,
		RawData:       rawMap,
		DeptName:      safeString(caseItem.DeptName),
		DoctorName:    safeString(caseItem.DoctorName),
		PatientGender: gender,
		PatientAge:    age,
	}
}
