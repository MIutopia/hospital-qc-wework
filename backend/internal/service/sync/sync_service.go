package sync

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"

	"github.com/rs/zerolog/log"
)

// SyncService 数据同步服务
// 从 HIS 数据仓库增量同步住院病例到业务库 inpatient_case 表。
type SyncService struct {
	hisDAO     *dao.HISDAO
	caseDAO    *dao.CaseDAO
	doctorDAO  *dao.DoctorDAO
	syncLogDAO *dao.SyncLogDAO
	batchSize  int
}

// NewSyncService 创建同步服务（hisDAO 可空：未配置 HIS 时 CSV 导入仍可用）
func NewSyncService(hisDAO *dao.HISDAO, caseDAO *dao.CaseDAO, doctorDAO *dao.DoctorDAO, syncLogDAO *dao.SyncLogDAO, batchSize int) *SyncService {
	return &SyncService{
		hisDAO:     hisDAO,
		caseDAO:    caseDAO,
		doctorDAO:  doctorDAO,
		syncLogDAO: syncLogDAO,
		batchSize:  batchSize,
	}
}

// SyncResult 同步结果
type SyncResult struct {
	TotalSynced int    `json:"totalSynced"`
	NewCases    int    `json:"newCases"`
	Updated     int    `json:"updated"`
	Elapsed     string `json:"elapsed"`
}

// RunSync 执行数据同步
// 增量断点 = 业务库 inpatient_case 中最大的 sync_time；
// 首次同步（无记录）取 HIS 最近 30 天数据。
func (s *SyncService) RunSync() (*SyncResult, error) {
	start := time.Now()
	log.Info().Msg("数据同步开始")
	tracker := s.newSyncLogTracker(model.SyncTypeHIS)

	// 未配置 HIS 连接时给出明确提示（CSV 导入仍可用）
	if s.hisDAO == nil {
		err := fmt.Errorf("HIS 数据库未配置连接（请设置 HIS_DB_USER / HIS_DB_PASS 环境变量，或信息科填写连接信息后重试）")
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}

	// 1. 计算增量断点
	since, err := s.caseDAO.GetMaxSyncTime()
	if err != nil {
		err = fmt.Errorf("获取同步断点失败: %w", err)
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}
	log.Info().Str("since", since).Msg("增量断点")

	// 2. 拉取 HIS 增量病案数据
	hisCases, err := s.hisDAO.QueryNewCases(&since, s.batchSize)
	if err != nil {
		err = fmt.Errorf("查询 HIS 病案数据失败: %w", err)
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}
	log.Info().Int("count", len(hisCases)).Msg("HIS 增量数据拉取完成")

	// 3. 拉取入院记录（补充主诉/现病史）
	admissionRecords, err := s.hisDAO.QueryAdmissionRecords(&since, s.batchSize)
	if err != nil {
		err = fmt.Errorf("查询 HIS 入院记录失败: %w", err)
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}
	log.Info().Int("count", len(admissionRecords)).Msg("HIS 入院记录拉取完成")

	// 4. 索引入院记录：姓名+入院时间 → 记录
	admissionIndex := make(map[string]model.HISAdmissionRecord)
	for _, r := range admissionRecords {
		if r.PatientName == "" || r.AdmitTime == nil {
			continue
		}
		key := admissionKey(r.PatientName, *r.AdmitTime)
		admissionIndex[key] = r
	}

	// 5. 转换并写入业务库
	newCases := 0
	updated := 0
	for _, hc := range hisCases {
		ic := s.toInpatientCase(hc)

		// 补充入院记录内容到 raw_data
		if hc.Name != "" && hc.AdmitTime != nil {
			if rec, ok := admissionIndex[admissionKey(hc.Name, *hc.AdmitTime)]; ok {
				s.enrichRawData(ic, &rec)
			}
		}

		isNew, err := s.caseDAO.UpsertByCaseNo(ic)
		if err != nil {
			log.Warn().Err(err).Str("caseNo", ic.CaseNo).Msg("病例写入失败")
			continue
		}
		if isNew {
			newCases++
		} else {
			updated++
		}
	}

	elapsed := time.Since(start)
	result := &SyncResult{
		TotalSynced: len(hisCases),
		NewCases:    newCases,
		Updated:     updated,
		Elapsed:     elapsed.Round(time.Millisecond).String(),
	}

	log.Info().
		Int("totalSynced", result.TotalSynced).
		Int("newCases", result.NewCases).
		Int("updated", result.Updated).
		Str("elapsed", result.Elapsed).
		Msg("数据同步完成")

	tracker.Finish(model.SyncStatusSuccess, result, nil)
	return result, nil
}

// SyncFromCSV 从 CSV 文件导入病例数据（阶段一兜底方案）
// 支持中文表头：住院号,姓名,性别,年龄,入院时间,出院时间,入院科室,住院医师,西医初步诊断
func (s *SyncService) SyncFromCSV(filePath string) (*SyncResult, error) {
	start := time.Now()
	tracker := s.newSyncLogTracker(model.SyncTypeCSV)

	f, err := os.Open(filePath)
	if err != nil {
		err = fmt.Errorf("打开 CSV 文件失败: %w", err)
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true

	// 读取表头
	header, err := reader.Read()
	if err != nil {
		err = fmt.Errorf("读取 CSV 表头失败: %w", err)
		tracker.Finish(model.SyncStatusFailed, nil, err)
		return nil, err
	}
	colIndex := make(map[string]int, len(header))
	for i, name := range header {
		colIndex[strings.TrimSpace(name)] = i
	}

	newCases := 0
	updated := 0
	rows := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Warn().Err(err).Msg("CSV 行解析失败，跳过")
			continue
		}
		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		ic := s.parseCSVRow(record, colIndex)
		if ic.CaseNo == "" {
			log.Warn().Msg("CSV 行缺少住院号，跳过")
			continue
		}

		isNew, err := s.caseDAO.UpsertByCaseNo(ic)
		if err != nil {
			log.Warn().Err(err).Str("caseNo", ic.CaseNo).Msg("CSV 病例写入失败")
			continue
		}
		if isNew {
			newCases++
		} else {
			updated++
		}
		rows++
	}

	elapsed := time.Since(start)
	result := &SyncResult{
		TotalSynced: rows,
		NewCases:    newCases,
		Updated:     updated,
		Elapsed:     elapsed.Round(time.Millisecond).String(),
	}
	log.Info().
		Int("totalSynced", result.TotalSynced).
		Int("newCases", result.NewCases).
		Int("updated", result.Updated).
		Str("elapsed", result.Elapsed).
		Msg("CSV 导入完成")

	tracker.Finish(model.SyncStatusSuccess, result, nil)
	return result, nil
}

// toInpatientCase 将 HIS 病案首页转换为业务库 InpatientCase
func (s *SyncService) toInpatientCase(hc model.HISCaseman) *model.InpatientCase {
	ic := &model.InpatientCase{
		CaseNo:     strings.TrimSpace(hc.HospitalNo),
		PatientName: strings.TrimSpace(hc.Name),
		CaseStatus: model.CaseStatusActive,
		QCStatus:   model.QCStatusPending,
	}

	// 性别：男→1 女→2
	if g := genderToInt(hc.Gender); g != nil {
		ic.PatientGender = g
	}

	// 年龄
	if hc.Age > 0 {
		age := hc.Age
		ic.PatientAge = &age
	}

	// 时间字段
	if hc.AdmitTime != nil {
		ic.AdmitTime = *hc.AdmitTime
	}
	if hc.DischargeTime != nil {
		ic.DischargeTime = hc.DischargeTime
	}

	// 科室：优先入院科室，其次当前科室
	deptName := ""
	if hc.AdmitDept != nil && *hc.AdmitDept != "" {
		deptName = *hc.AdmitDept
	} else if hc.CurrentDept != nil {
		deptName = *hc.CurrentDept
	}
	if deptName != "" {
		ic.DeptName = &deptName
		// dept_id 通过科室名匹配本地科室表，找不到留空
		if did := s.caseDAO.FindDeptIDByName(deptName); did != 0 {
			ic.DeptID = did
		}
	}

	// 责任医生：优先住院医师，其次主治/主任
	doctorName := ""
	if hc.ResidentDoctor != nil && *hc.ResidentDoctor != "" {
		doctorName = *hc.ResidentDoctor
	} else if hc.AttendingDoctor != nil && *hc.AttendingDoctor != "" {
		doctorName = *hc.AttendingDoctor
	} else if hc.ChiefDoctor != nil && *hc.ChiefDoctor != "" {
		doctorName = *hc.ChiefDoctor
	}
	if doctorName != "" {
		ic.DoctorName = &doctorName
		if did := s.doctorDAO.FindIDByName(doctorName); did != nil {
			ic.DoctorID = did
		}
	}

	// 诊断：优先入院记录西医初步诊断（由 enrichRawData 补充），此处用病案首页的门急诊西医诊断兜底
	if hc.OutpatientWDiag != nil && *hc.OutpatientWDiag != "" {
		d := *hc.OutpatientWDiag
		ic.Diagnosis = &d
	}

	// 初始化 raw_data（含手术信息），入院记录内容由 enrichRawData 合并补充
	raw := map[string]interface{}{
		"has_surgery": hc.SurgeryCode != nil && *hc.SurgeryCode != "",
	}
	if hc.SurgeryCode != nil {
		raw["surgery"] = map[string]interface{}{
			"code":     *hc.SurgeryCode,
			"name":     strPtrOrEmpty(hc.SurgeryName),
			"surgeon":  strPtrOrEmpty(hc.Surgeon),
			"anesthesia": strPtrOrEmpty(hc.Anesthesia),
		}
	}
	if hc.OutpatientWDiag != nil {
		raw["outpatient_diagnosis"] = *hc.OutpatientWDiag
	}
	if b, err := json.Marshal(raw); err == nil {
		rawStr := string(b)
		ic.RawData = &rawStr
	}

	return ic
}

// enrichRawData 将入院记录的病历文书内容合并到 raw_data JSON
// 键名与规则 DSL 保持一致（英文，如 complaint / hpi）
func (s *SyncService) enrichRawData(ic *model.InpatientCase, rec *model.HISAdmissionRecord) {
	// 解析已有 raw_data，合并入院记录内容
	raw := make(map[string]interface{})
	if ic.RawData != nil {
		_ = json.Unmarshal([]byte(*ic.RawData), &raw)
	}

	raw["admission_record"] = map[string]interface{}{
		"complaint":        strPtrOrEmpty(rec.ChiefComplaint),
		"hpi":              strPtrOrEmpty(rec.PresentHistory),
		"past_history":     strPtrOrEmpty(rec.PastHistory),
		"personal_history": strPtrOrEmpty(rec.PersonalHistory),
		"marital_history":  strPtrOrEmpty(rec.MaritalHistory),
		"menstrual_history": strPtrOrEmpty(rec.MenstrualHistory),
		"family_history":   strPtrOrEmpty(rec.FamilyHistory),
		"physical_exam":    strPtrOrEmpty(rec.PhysicalExam),
		"specialty_exam":   strPtrOrEmpty(rec.SpecialtyExam),
		"auxiliary_exam":   strPtrOrEmpty(rec.AuxiliaryExam),
		"tcm_diagnosis":    strPtrOrEmpty(rec.TCMInitialDiag),
		"western_diagnosis": strPtrOrEmpty(rec.WesternInitDiag),
	}
	if rec.RecordTime != nil {
		raw["admission_record"].(map[string]interface{})["create_time"] = rec.RecordTime.Format("2006-01-02 15:04:05")
	}

	// 若病案首页诊断为空，用入院记录西医初步诊断补充
	if (ic.Diagnosis == nil || *ic.Diagnosis == "") && rec.WesternInitDiag != nil {
		d := *rec.WesternInitDiag
		ic.Diagnosis = &d
	}

	if b, err := json.Marshal(raw); err == nil {
		rawStr := string(b)
		ic.RawData = &rawStr
	}
}

// parseCSVRow 解析 CSV 行为 InpatientCase
func (s *SyncService) parseCSVRow(record []string, col map[string]int) *model.InpatientCase {
	get := func(name string) string {
		if i, ok := col[name]; ok && i < len(record) {
			return strings.TrimSpace(record[i])
		}
		return ""
	}

	ic := &model.InpatientCase{
		CaseNo:     get("住院号"),
		PatientName: get("姓名"),
		CaseStatus: model.CaseStatusActive,
		QCStatus:   model.QCStatusPending,
	}

	if g := genderToInt(get("性别")); g != nil {
		ic.PatientGender = g
	}
	if v := parseIntPtr(get("年龄")); v != nil {
		ic.PatientAge = v
	}
	if t := parseTime(get("入院时间")); t != nil {
		ic.AdmitTime = *t
	}
	if t := parseTime(get("出院时间")); t != nil {
		ic.DischargeTime = t
	}
	if v := get("入院科室"); v != "" {
		ic.DeptName = &v
		if did := s.caseDAO.FindDeptIDByName(v); did != 0 {
			ic.DeptID = did
		}
	}
	if v := get("住院医师"); v != "" {
		ic.DoctorName = &v
		if did := s.doctorDAO.FindIDByName(v); did != nil {
			ic.DoctorID = did
		}
	}
	if v := get("西医初步诊断"); v != "" {
		ic.Diagnosis = &v
	}
	if v := get("主诉"); v != "" {
		raw := map[string]interface{}{"admission_record": map[string]interface{}{"complaint": v}}
		if b, err := json.Marshal(raw); err == nil {
			rawStr := string(b)
			ic.RawData = &rawStr
		}
	}

	return ic
}

// syncLogTracker 同步日志跟踪器：开始创建 RUNNING 记录，结束更新为 SUCCESS/FAILED
type syncLogTracker struct {
	dao     *dao.SyncLogDAO
	logID   int64
	started time.Time
}

// newSyncLogTracker 创建跟踪器并写入 RUNNING 日志（DAO 为空或写库失败时静默降级，不影响同步主流程）
func (s *SyncService) newSyncLogTracker(syncType string) *syncLogTracker {
	t := &syncLogTracker{dao: s.syncLogDAO, started: time.Now()}
	if s.syncLogDAO == nil {
		return t
	}
	id, err := s.syncLogDAO.Create(&model.SyncLog{
		SyncType:  syncType,
		Status:    model.SyncStatusRunning,
		StartedAt: t.started,
	})
	if err != nil {
		log.Warn().Err(err).Msg("同步日志创建失败")
		return t
	}
	t.logID = id
	return t
}

// Finish 更新同步日志终态
func (t *syncLogTracker) Finish(status string, res *SyncResult, syncErr error) {
	if t.dao == nil || t.logID == 0 {
		return
	}
	elapsedMS := time.Since(t.started).Milliseconds()
	now := time.Now()
	var errMsg *string
	if syncErr != nil {
		m := syncErr.Error()
		errMsg = &m
	}
	var total, newC, upd int
	if res != nil {
		total, newC, upd = res.TotalSynced, res.NewCases, res.Updated
	}
	if err := t.dao.Update(t.logID, &model.SyncLog{
		Status:      status,
		TotalSynced: total,
		NewCases:    newC,
		Updated:     upd,
		ErrorMsg:    errMsg,
		FinishedAt:  &now,
		ElapsedMS:   &elapsedMS,
	}); err != nil {
		log.Warn().Err(err).Msg("同步日志更新失败")
	}
}

// ----- 工具函数 -----

// admissionKey 入院记录关联键：姓名 + 入院时间（yyyy-MM-dd HH:mm:ss）
func admissionKey(name string, t time.Time) string {
	return name + "|" + t.Format("2006-01-02 15:04:05")
}

func genderToInt(s string) *int {
	switch strings.TrimSpace(s) {
	case "男", "M", "1":
		v := 1
		return &v
	case "女", "F", "2":
		v := 2
		return &v
	default:
		return nil
	}
}

func parseIntPtr(s string) *int {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	// 支持多种时间格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func strPtrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
