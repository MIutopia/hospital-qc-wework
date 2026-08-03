package handler

import (
	"fmt"
	"time"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// DeptHandler 科室统计处理器
type DeptHandler struct {
	caseDAO    *dao.CaseDAO
	resultDAO  *dao.ResultDAO
	doctorDAO  *dao.DoctorDAO
	confirmDAO *dao.ConfirmDAO
}

func NewDeptHandler(caseDAO *dao.CaseDAO, resultDAO *dao.ResultDAO, doctorDAO *dao.DoctorDAO, confirmDAO *dao.ConfirmDAO) *DeptHandler {
	return &DeptHandler{
		caseDAO:    caseDAO,
		resultDAO:  resultDAO,
		doctorDAO:  doctorDAO,
		confirmDAO: confirmDAO,
	}
}

// RegisterRoutes 注册路由
func (h *DeptHandler) RegisterRoutes(r *gin.RouterGroup, authMw gin.HandlerFunc) {
	r.GET("/dept/stats", authMw, h.GetStats)
}

// GetStats 获取科室质控统计
// GET /api/v1/dept/stats?date=2026-07-31（默认今日，取 qc_time 所在日期）
func (h *DeptHandler) GetStats(c *gin.Context) {
	doctorID, exists := c.Get("doctor_id")
	if !exists {
		response.Unauthorized(c, "未获取到用户信息")
		return
	}

	// 统计日期
	dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		response.BadRequest(c, "date 格式错误，应为 YYYY-MM-DD")
		return
	}

	// 从医生映射获取科室
	doctor, err := h.doctorDAO.GetByDoctorID(doctorID.(int64))
	if err != nil || doctor.DeptID == nil {
		response.Success(c, gin.H{
			"deptName":    "-",
			"totalCases":  0,
			"defectCases": 0,
			"defectRate":  "0.00%",
			"message":     "当前医生未关联科室",
		})
		return
	}
	deptID := *doctor.DeptID

	// 科室名称
	deptName := h.doctorDAO.GetDeptNameByID(deptID)

	// 基础统计
	totalCases, err := h.caseDAO.CountQCCasesByDeptDate(deptID, dateStr)
	if err != nil {
		log.Warn().Err(err).Int64("deptId", deptID).Msg("统计总病例数失败")
		response.ServerError(c, "统计失败")
		return
	}
	defectCases, err := h.caseDAO.CountDefectCasesByDeptDate(deptID, dateStr)
	if err != nil {
		log.Warn().Err(err).Int64("deptId", deptID).Msg("统计缺陷病例数失败")
		response.ServerError(c, "统计失败")
		return
	}

	// 缺陷等级统计
	levelA, levelB, err := h.resultDAO.CountDefectLevelsByDeptDate(deptID, dateStr)
	if err != nil {
		log.Warn().Err(err).Msg("统计缺陷等级失败")
		levelA, levelB = 0, 0
	}

	// 确认整改率
	confirmed, err := h.confirmDAO.CountConfirmedByDeptDate(deptID, dateStr)
	if err != nil {
		confirmed = 0
	}

	defectRate := 0.0
	if totalCases > 0 {
		defectRate = float64(defectCases) / float64(totalCases) * 100
	}
	confirmedRate := 0.0
	if defectCases > 0 {
		confirmedRate = float64(confirmed) / float64(defectCases) * 100
	}

	response.Success(c, gin.H{
		"deptName":      deptName,
		"totalCases":    totalCases,
		"defectCases":   defectCases,
		"defectRate":    fmt.Sprintf("%.2f%%", defectRate),
		"levelADefects": levelA,
		"levelBDefects": levelB,
		"confirmedRate": fmt.Sprintf("%.2f%%", confirmedRate),
	})
}
