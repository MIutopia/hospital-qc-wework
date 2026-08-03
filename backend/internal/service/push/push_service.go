package push

import (
	"encoding/json"
	"fmt"
	"time"

	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"
	"hospital-qc-wework/internal/service/auth"

	"github.com/rs/zerolog/log"
)

// PushService 消息推送编排服务（M4）
// 职责：
//  1. 从质控结果聚合按医生+病例的缺陷清单
//  2. 免打扰时段（22:00-08:00）过滤 → DEFERRED
//  3. 调用 Pusher 发送模板卡片（内部令牌桶限频 10次/秒）
//  4. 记录 push_log（SUCCESS/FAILED/DEFERRED）
type PushService struct {
	cfg        *config.Config
	caseDAO    *dao.CaseDAO
	resultDAO  *dao.ResultDAO
	ruleDAO    *dao.RuleDAO
	doctorDAO  *dao.DoctorDAO
	pushLogDAO *dao.PushLogDAO
	pusher     *Pusher
	authSvc    *auth.JWTService
}

// NewPushService 创建推送编排服务
func NewPushService(cfg *config.Config, caseDAO *dao.CaseDAO, resultDAO *dao.ResultDAO,
	ruleDAO *dao.RuleDAO, doctorDAO *dao.DoctorDAO, pushLogDAO *dao.PushLogDAO,
	pusher *Pusher, authSvc *auth.JWTService) *PushService {
	return &PushService{
		cfg:        cfg,
		caseDAO:    caseDAO,
		resultDAO:  resultDAO,
		ruleDAO:    ruleDAO,
		doctorDAO:  doctorDAO,
		pushLogDAO: pushLogDAO,
		pusher:     pusher,
		authSvc:    authSvc,
	}
}

// PushResult 批量推送结果
type PushResult struct {
	Total    int    `json:"total"`
	Success  int    `json:"success"`
	Failed   int    `json:"failed"`
	Deferred int    `json:"deferred"`
	Elapsed  string `json:"elapsed"`
}

// PushIssuedCases 推送全部「已发出待整改且未成功推送」的病例
// 供定时任务（每日 06:30）与质控完成后自动触发调用。
func (s *PushService) PushIssuedCases() (*PushResult, error) {
	start := time.Now()
	cases, err := s.caseDAO.GetIssuedUnpushed()
	if err != nil {
		return nil, fmt.Errorf("查询待推送病例失败: %w", err)
	}
	log.Info().Int("count", len(cases)).Msg("待推送病例加载完成")

	result := &PushResult{Total: len(cases)}
	for i := range cases {
		switch err := s.PushCase(&cases[i]); {
		case err == nil:
			result.Success++
		case isDeferredError(err):
			result.Deferred++
		default:
			result.Failed++
			log.Warn().Err(err).Int64("caseId", cases[i].ID).Msg("推送失败")
		}
	}

	result.Elapsed = time.Since(start).Round(time.Millisecond).String()
	log.Info().
		Int("total", result.Total).
		Int("success", result.Success).
		Int("failed", result.Failed).
		Int("deferred", result.Deferred).
		Str("elapsed", result.Elapsed).
		Msg("批量推送完成")
	return result, nil
}

// PushCase 推送单个病例的质控报告卡片
func (s *PushService) PushCase(c *model.InpatientCase) error {
	// 1. 责任医生映射（无映射则无法推送）
	if c.DoctorID == nil {
		return fmt.Errorf("病例 %s 未关联责任医生，跳过推送", c.CaseNo)
	}
	doctor, err := s.doctorDAO.GetByDoctorID(*c.DoctorID)
	if err != nil {
		return fmt.Errorf("医生映射缺失 doctor_id=%d: %w", *c.DoctorID, err)
	}

	// 2. 缺陷汇总 + 缺陷明细
	summary, err := s.resultDAO.GetDefectSummary(c.ID)
	if err != nil {
		summary = &model.DefectSummary{}
	}
	defects, defectIDs, err := s.buildDefects(c.ID)
	if err != nil {
		return err
	}

	// 3. H5 访问令牌（7 天有效期）
	token, err := s.authSvc.GenerateH5Token(doctor.DoctorID, c.ID)
	if err != nil {
		return fmt.Errorf("生成 H5 令牌失败: %w", err)
	}

	// 4. 免打扰时段判断 → DEFERRED
	now := time.Now()
	if inQuietWindow(now, s.cfg.QC.QuietStart, s.cfg.QC.QuietEnd) {
		log.Info().Int64("caseId", c.ID).Msg("当前处于免打扰时段，推送状态置为 DEFERRED")
		_, logErr := s.pushLogDAO.Create(&model.PushLog{
			CaseID:         c.ID,
			QCResultIDs:    defectIDs,
			ReceiverUserID: doctor.WeWorkUserID,
			PushType:       model.PushTypeCard,
			PushStatus:     model.PushStatusDeferred,
		})
		if logErr != nil {
			log.Warn().Err(logErr).Msg("DEFERRED 推送日志写入失败")
		}
		return errDeferred
	}

	// 5. 发送模板卡片（限频在 Pusher 内部处理）
	respBody, err := s.pusher.SendCard(c, summary, defects, doctor, s.cfg.H5.BaseURL, token)

	// 6. 记录推送日志
	status := model.PushStatusSuccess
	if err != nil {
		status = model.PushStatusFailed
	}
	pushedAt := now
	respStr := respBody
	if err != nil {
		respStr = err.Error()
	}
	_, logErr := s.pushLogDAO.Create(&model.PushLog{
		CaseID:         c.ID,
		QCResultIDs:    defectIDs,
		ReceiverUserID: doctor.WeWorkUserID,
		PushType:       model.PushTypeCard,
		PushStatus:     status,
		PushResponse:   &respStr,
		PushedAt:       &pushedAt,
	})
	if logErr != nil {
		log.Warn().Err(logErr).Msg("推送日志写入失败")
	}
	if err != nil {
		return fmt.Errorf("发送卡片失败: %w", err)
	}
	return nil
}

// buildDefects 构建缺陷明细与缺陷结果 ID 列表（JSON 数组字符串）
func (s *PushService) buildDefects(caseID int64) ([]model.DefectItem, string, error) {
	results, err := s.resultDAO.GetByCaseID(caseID)
	if err != nil {
		return nil, "", fmt.Errorf("查询质控结果失败: %w", err)
	}

	var defects []model.DefectItem
	var ids []int64
	for _, r := range results {
		if r.IsDefect != 1 {
			continue
		}
		ids = append(ids, r.ID)
		rule, ruleErr := s.ruleDAO.GetByID(r.RuleID)
		if ruleErr != nil {
			continue
		}
		defects = append(defects, model.DefectItem{
			ID:             r.ID,
			RuleName:       rule.RuleName,
			RuleLevel:      rule.RuleLevel,
			DefectDetail:   strOrDash(r.DefectDetail),
			DefectLocation: strOrDash(r.DefectLocation),
			Suggestion:     strOrDash(r.Suggestion),
		})
	}

	idBytes, _ := json.Marshal(ids)
	return defects, string(idBytes), nil
}

func strOrDash(p *string) string {
	if p == nil || *p == "" {
		return ""
	}
	return *p
}

// errDeferred 免打扰延迟标记
var errDeferred = fmt.Errorf("推送已延迟（免打扰时段）")

func isDeferredError(err error) bool {
	return err == errDeferred
}

// inQuietWindow 判断当前时间是否在免打扰窗口内（跨午夜，如 22:00-08:00）
// 配置格式："22:00" / "08:00"；非法配置按不限制处理
func inQuietWindow(now time.Time, quietStart, quietEnd string) bool {
	start, ok1 := parseHM(quietStart)
	end, ok2 := parseHM(quietEnd)
	if !ok1 || !ok2 || start == end {
		return false
	}

	cur := now.Hour()*60 + now.Minute()
	if start < end {
		// 同日窗口（如 01:00-06:00）
		return cur >= start && cur < end
	}
	// 跨午夜窗口（如 22:00-08:00）
	return cur >= start || cur < end
}

// parseHM 解析 "HH:MM" 为当日分钟数
func parseHM(s string) (int, bool) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}
