package handler

import (
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
)

// DeptHandler 科室统计处理器
type DeptHandler struct {
	caseDAO *dao.CaseDAO
}

func NewDeptHandler(caseDAO *dao.CaseDAO) *DeptHandler {
	return &DeptHandler{caseDAO: caseDAO}
}

// RegisterRoutes 注册路由
func (h *DeptHandler) RegisterRoutes(r *gin.RouterGroup, authMw gin.HandlerFunc) {
	r.GET("/dept/stats", authMw, h.GetStats)
}

// GetStats 获取科室质控统计
// GET /api/v1/dept/stats?date=2026-07-31
func (h *DeptHandler) GetStats(c *gin.Context) {
	// 从 JWT 获取医生信息
	doctorID, exists := c.Get("doctor_id")
	if !exists {
		response.Unauthorized(c, "未获取到用户信息")
		return
	}

	_ = doctorID // 后续从 doctor_wework 获取 dept_id

	// TODO: M5 阶段实现完整统计逻辑
	response.Success(c, gin.H{
		"deptName":    "-",
		"totalCases":  0,
		"defectCases": 0,
		"defectRate":  "0%",
		"message":     "统计功能开发中（M5阶段）",
	})
}
