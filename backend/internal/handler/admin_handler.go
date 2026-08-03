package handler

import (
	"strconv"

	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/model"
	"hospital-qc-wework/internal/service/push"
	"hospital-qc-wework/internal/service/qc"
	"hospital-qc-wework/internal/service/sync"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// AdminHandler 管理后台处理器
type AdminHandler struct {
	ruleDAO    *dao.RuleDAO
	doctorDAO  *dao.DoctorDAO
	pushLogDAO *dao.PushLogDAO
	syncLogDAO *dao.SyncLogDAO
	qcEngine   *qc.Engine
	syncSvc    *sync.SyncService
	pushSvc    *push.PushService
}

func NewAdminHandler(ruleDAO *dao.RuleDAO, doctorDAO *dao.DoctorDAO, pushLogDAO *dao.PushLogDAO, syncLogDAO *dao.SyncLogDAO, qcEngine *qc.Engine, syncSvc *sync.SyncService, pushSvc *push.PushService) *AdminHandler {
	return &AdminHandler{
		ruleDAO:    ruleDAO,
		doctorDAO:  doctorDAO,
		pushLogDAO: pushLogDAO,
		syncLogDAO: syncLogDAO,
		qcEngine:   qcEngine,
		syncSvc:    syncSvc,
		pushSvc:    pushSvc,
	}
}

// RegisterRoutes 注册管理路由
func (h *AdminHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/admin/rules", h.ListRules)
	r.POST("/admin/rules", h.CreateRule)
	r.PUT("/admin/rules/:id", h.UpdateRule)
	r.DELETE("/admin/rules/:id", h.DeleteRule)

	r.GET("/admin/doctors", h.ListDoctors)
	r.POST("/admin/doctors", h.UpsertDoctor)

	r.POST("/admin/sync", h.RunSync)        // 手动触发 HIS 增量同步
	r.POST("/admin/sync/csv", h.RunSyncCSV) // CSV 手工导入（阶段一兜底）
	r.GET("/admin/sync/logs", h.ListSyncLogs) // 同步日志查询

	r.POST("/admin/qc/run", h.RunQC)

	r.GET("/admin/push/logs", h.ListPushLogs)
}

// RunSync 手动触发 HIS 增量同步
func (h *AdminHandler) RunSync(c *gin.Context) {
	if h.syncSvc == nil {
		response.ServerError(c, "HIS 同步服务未初始化（请先配置 HIS_DB_USER/HIS_DB_PASS 环境变量）")
		return
	}
	result, err := h.syncSvc.RunSync()
	if err != nil {
		response.ServerError(c, "数据同步失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// RunSyncCSV CSV 手工导入（阶段一兜底）
func (h *AdminHandler) RunSyncCSV(c *gin.Context) {
	if h.syncSvc == nil {
		response.ServerError(c, "同步服务未初始化")
		return
	}
	path := c.PostForm("path")
	if path == "" {
		response.BadRequest(c, "缺少 path 参数（CSV 文件路径）")
		return
	}
	result, err := h.syncSvc.SyncFromCSV(path)
	if err != nil {
		response.ServerError(c, "CSV 导入失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// ListRules 规则列表
func (h *AdminHandler) ListRules(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	rules, total, err := h.ruleDAO.List(page, pageSize)
	if err != nil {
		response.ServerError(c, "查询规则失败")
		return
	}
	response.SuccessPage(c, rules, total, page, pageSize)
}

// CreateRule 创建规则
func (h *AdminHandler) CreateRule(c *gin.Context) {
	var rule model.QCRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	id, err := h.ruleDAO.Create(&rule)
	if err != nil {
		response.ServerError(c, "创建规则失败")
		return
	}
	response.Success(c, gin.H{"id": id})
}

// UpdateRule 更新规则
func (h *AdminHandler) UpdateRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 格式错误")
		return
	}

	var rule model.QCRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	rule.ID = id

	if err := h.ruleDAO.Update(&rule); err != nil {
		response.ServerError(c, "更新规则失败")
		return
	}
	response.Success(c, nil)
}

// DeleteRule 删除规则
func (h *AdminHandler) DeleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "id 格式错误")
		return
	}

	if err := h.ruleDAO.Delete(id); err != nil {
		response.ServerError(c, "删除规则失败")
		return
	}
	response.Success(c, nil)
}

// ListDoctors 医生映射列表
func (h *AdminHandler) ListDoctors(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	doctors, total, err := h.doctorDAO.List(page, pageSize)
	if err != nil {
		response.ServerError(c, "查询医生列表失败")
		return
	}
	response.SuccessPage(c, doctors, total, page, pageSize)
}

// UpsertDoctor 创建/更新医生映射
func (h *AdminHandler) UpsertDoctor(c *gin.Context) {
	var doc model.DoctorWeWork
	if err := c.ShouldBindJSON(&doc); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if err := h.doctorDAO.Upsert(&doc); err != nil {
		response.ServerError(c, "保存医生映射失败")
		return
	}
	response.Success(c, nil)
}

// RunQC 手动触发质控（执行完成后自动推送 ISSUED 病例）
func (h *AdminHandler) RunQC(c *gin.Context) {
	result, err := h.qcEngine.RunBatch()
	if err != nil {
		response.ServerError(c, "质控执行失败: "+err.Error())
		return
	}

	// 存在缺陷病例 → 触发推送（M4）
	if result.DefectCases > 0 && h.pushSvc != nil {
		pushResult, pushErr := h.pushSvc.PushIssuedCases()
		if pushErr != nil {
			log.Warn().Err(pushErr).Msg("质控后自动推送失败")
		} else {
			log.Info().Int("pushed", pushResult.Success).Int("deferred", pushResult.Deferred).Msg("质控后自动推送完成")
		}
		response.Success(c, gin.H{
			"batchId":      result.BatchID,
			"totalCases":   result.TotalCases,
			"defectCases":  result.DefectCases,
			"totalDefects": result.TotalDefects,
			"passedCases":  result.PassedCases,
			"elapsed":      result.Elapsed,
			"push":         pushResult,
		})
		return
	}
	response.Success(c, result)
}

// ListSyncLogs 同步日志列表
func (h *AdminHandler) ListSyncLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

	logs, total, err := h.syncLogDAO.List(page, pageSize)
	if err != nil {
		response.ServerError(c, "查询同步日志失败")
		return
	}
	response.SuccessPage(c, logs, total, page, pageSize)
}

// ListPushLogs 推送日志
func (h *AdminHandler) ListPushLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := c.DefaultQuery("status", "")

	logs, total, err := h.pushLogDAO.List(page, pageSize, status)
	if err != nil {
		response.ServerError(c, "查询推送日志失败")
		return
	}
	response.SuccessPage(c, logs, total, page, pageSize)
}
