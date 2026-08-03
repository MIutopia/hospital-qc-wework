package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"
	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
)

// ReportHandler 质控报告处理器
type ReportHandler struct {
	caseDAO    *dao.CaseDAO
	resultDAO  *dao.ResultDAO
	ruleDAO    *dao.RuleDAO
	confirmDAO *dao.ConfirmDAO
	authSvc    *auth.JWTService
}

func NewReportHandler(caseDAO *dao.CaseDAO, resultDAO *dao.ResultDAO, ruleDAO *dao.RuleDAO, confirmDAO *dao.ConfirmDAO, authSvc *auth.JWTService) *ReportHandler {
	return &ReportHandler{
		caseDAO:    caseDAO,
		resultDAO:  resultDAO,
		ruleDAO:    ruleDAO,
		confirmDAO: confirmDAO,
		authSvc:    authSvc,
	}
}

// RegisterRoutes 注册报告相关路由
func (h *ReportHandler) RegisterRoutes(r *gin.RouterGroup, authMw gin.HandlerFunc) {
	r.GET("/report/detail", authMw, h.GetDetail)
	r.POST("/report/confirm", authMw, h.Confirm)
}

// GetDetail 获取质控报告详情
// GET /api/v1/report/detail?caseId=12345
func (h *ReportHandler) GetDetail(c *gin.Context) {
	caseIDStr := c.Query("caseId")
	if caseIDStr == "" {
		response.BadRequest(c, "缺少 caseId 参数")
		return
	}

	caseID, err := strconv.ParseInt(caseIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "caseId 格式错误")
		return
	}

	// 获取病例信息
	caseItem, err := h.caseDAO.GetByID(caseID)
	if err != nil {
		response.Error(c, http.StatusNotFound, 40401, "病例不存在")
		return
	}

	// 获取质控结果
	results, err := h.resultDAO.GetByCaseID(caseID)
	if err != nil {
		response.ServerError(c, "查询质控结果失败")
		return
	}

	// 获取缺陷汇总
	summary, err := h.resultDAO.GetDefectSummary(caseID)
	if err != nil {
		summary = &model.DefectSummary{}
	}

	// 构建缺陷列表
	var defects []model.DefectItem
	for _, r := range results {
		if r.IsDefect == 1 {
			rule, ruleErr := h.ruleDAO.GetByID(r.RuleID)
			if ruleErr != nil {
				continue
			}
			defectDetail := safeDeref(r.DefectDetail)
			defectLocation := safeDeref(r.DefectLocation)
			suggestion := safeDeref(r.Suggestion)

			defects = append(defects, model.DefectItem{
				ID:             r.ID,
				RuleName:       rule.RuleName,
				RuleLevel:      rule.RuleLevel,
				DefectDetail:   defectDetail,
				DefectLocation: defectLocation,
				Suggestion:     suggestion,
			})
		}
	}

	// 是否已确认整改
	isConfirmed := false
	if confirm, err := h.confirmDAO.GetByCase(caseID); err == nil && confirm.ConfirmStatus == model.ConfirmStatusConfirmed {
		isConfirmed = true
	}

	// 响应数据结构
	type ReportDetail struct {
		CaseID        int64                `json:"caseId"`
		CaseNo        string               `json:"caseNo"`
		PatientName   string               `json:"patientName"`
		PatientGender *int                 `json:"patientGender"`
		PatientAge    *int                 `json:"patientAge"`
		DeptName      string               `json:"deptName"`
		DoctorName    string               `json:"doctorName"`
		AdmitTime     string               `json:"admitTime"`
		Diagnosis     string               `json:"diagnosis"`
		QCStatus      string               `json:"qcStatus"`
		DefectSummary *model.DefectSummary `json:"defectSummary"`
		Defects       []model.DefectItem   `json:"defects"`
		IsConfirmed   bool                 `json:"isConfirmed"`
	}

	deptName := safeDeref(caseItem.DeptName)
	doctorName := safeDeref(caseItem.DoctorName)
	diagnosis := safeDeref(caseItem.Diagnosis)

	resp := ReportDetail{
		CaseID:        caseItem.ID,
		CaseNo:        caseItem.CaseNo,
		PatientName:   caseItem.PatientName,
		PatientGender: caseItem.PatientGender,
		PatientAge:    caseItem.PatientAge,
		DeptName:      deptName,
		DoctorName:    doctorName,
		AdmitTime:     caseItem.AdmitTime.Format("2006-01-02T15:04:05"),
		Diagnosis:     diagnosis,
		QCStatus:      caseItem.QCStatus,
		DefectSummary: summary,
		Defects:       defects,
		IsConfirmed:   isConfirmed,
	}

	response.Success(c, resp)
}

// Confirm 确认整改
// POST /api/v1/report/confirm body: {"caseId": 12345}
// 校验：病例存在 + JWT 医生与责任医生一致（防止越权）
func (h *ReportHandler) Confirm(c *gin.Context) {
	var req struct {
		CaseID int64  `json:"caseId" binding:"required"`
		Note   string `json:"note,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 验证病例存在
	caseItem, err := h.caseDAO.GetByID(req.CaseID)
	if err != nil {
		response.Error(c, http.StatusNotFound, 40401, "病例不存在")
		return
	}

	// 越权校验：病例已关联责任医生时，仅责任医生可确认
	doctorID, exists := c.Get("doctor_id")
	if exists {
		tokenDoctorID := doctorID.(int64)
		if caseItem.DoctorID != nil && *caseItem.DoctorID != tokenDoctorID {
			response.Error(c, http.StatusForbidden, 40301, "无权确认该病例（非责任医生）")
			return
		}
	}

	// 确认的缺陷 ID 列表（当前全部缺陷）
	var defectIDs []int64
	if results, err := h.resultDAO.GetByCaseID(req.CaseID); err == nil {
		for _, r := range results {
			if r.IsDefect == 1 {
				defectIDs = append(defectIDs, r.ID)
			}
		}
	}
	idsJSON := ""
	if b, err := json.Marshal(defectIDs); err == nil {
		idsJSON = string(b)
	}

	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	var idsPtr *string
	if idsJSON != "" {
		idsPtr = &idsJSON
	}

	if err := h.confirmDAO.Upsert(req.CaseID, doctorID.(int64), idsPtr, note); err != nil {
		response.ServerError(c, "确认整改失败: "+err.Error())
		return
	}

	response.Success(c, gin.H{"caseId": req.CaseID, "confirmed": true})
}

func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
