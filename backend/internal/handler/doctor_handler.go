package handler

import (
	"strconv"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
)

// DoctorHandler 医生相关处理器
type DoctorHandler struct {
	caseDAO *dao.CaseDAO
}

func NewDoctorHandler(caseDAO *dao.CaseDAO) *DoctorHandler {
	return &DoctorHandler{caseDAO: caseDAO}
}

// RegisterRoutes 注册路由
func (h *DoctorHandler) RegisterRoutes(r *gin.RouterGroup, authMw gin.HandlerFunc) {
	r.GET("/doctor/tasks", authMw, h.GetTasks)
}

// GetTasks 获取医生的待整改列表
// GET /api/v1/doctor/tasks?page=1&pageSize=20&status=ISSUED
func (h *DoctorHandler) GetTasks(c *gin.Context) {
	// 从 JWT 获取 doctor_id
	doctorID, exists := c.Get("doctor_id")
	if !exists {
		response.Unauthorized(c, "未获取到用户信息")
		return
	}

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "20")
	status := c.DefaultQuery("status", "ISSUED")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	cases, total, err := h.caseDAO.GetDoctorCases(doctorID.(int64), status, page, pageSize)
	if err != nil {
		response.ServerError(c, "查询失败")
		return
	}

	response.SuccessPage(c, cases, total, page, pageSize)
}
