package handler

import (
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
	caseDAO   *dao.CaseDAO
	resultDAO *dao.ResultDAO
	ruleDAO   *dao.RuleDAO
	authSvc   *auth.JWTService
}

func NewReportHandler(caseDAO *dao.CaseDAO, resultDAO *dao.ResultDAO, ruleDAO *dao.RuleDAO, authSvc *auth.JWTService) *ReportHandler {
	return &ReportHandler{
		caseDAO:   caseDAO,
		resultDAO: resultDAO,
		ruleDAO:   ruleDAO,
		authSvc:   authSvc,
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

	// 响应数据结构
	type ReportDetail struct {
		CaseID        int64               `json:"caseId"`
		CaseNo        string              `json:"caseNo"`
		PatientName   string              `json:"patientName"`
		PatientGender *int                `json:"patientGender"`
		PatientAge    *int                `json:"patientAge"`
		DeptName      string              `json:"deptName"`
		DoctorName    string              `json:"doctorName"`
		AdmitTime     string              `json:"admitTime"`
		Diagnosis     string              `json:"diagnosis"`
		QCStatus      string              `json:"qcStatus"`
		DefectSummary *model.DefectSummary `json:"defectSummary"`
		Defects       []model.DefectItem  `json:"defects"`
		IsConfirmed   bool                `json:"isConfirmed"`
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
	}

	response.Success(c, resp)
}

// Confirm 确认整改
// POST /api/v1/report/confirm body: {"caseId": 12345}
func (h *ReportHandler) Confirm(c *gin.Context) {
	var req struct {
		CaseID int64 `json:"caseId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	// 验证病例存在
	if _, err := h.caseDAO.GetByID(req.CaseID); err != nil {
		response.Error(c, http.StatusNotFound, 40401, "病例不存在")
		return
	}

	// TODO: 更新确认状态（M5 阶段实现确认表后补充）
	response.Success(c, gin.H{"caseId": req.CaseID, "confirmed": true})
}

func safeDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
