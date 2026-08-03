package handler

import (
	"hospital-qc-wework/internal/app"
	"hospital-qc-wework/internal/middleware"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// RegisterRoutes 注册全部路由
// app 为应用装配上下文；数据库不可用时（app.DB == nil）仅健康检查可用
func RegisterRoutes(r *gin.Engine, appCtx *app.App) {
	dbReady := appCtx.DB != nil
	weWorkReady := appCtx.TokenMgr != nil

	// 初始化 Handler
	reportHandler := NewReportHandler(appCtx.CaseDAO, appCtx.ResultDAO, appCtx.RuleDAO, appCtx.ConfirmDAO, appCtx.AuthSvc)
	doctorHandler := NewDoctorHandler(appCtx.CaseDAO)
	deptHandler := NewDeptHandler(appCtx.CaseDAO, appCtx.ResultDAO, appCtx.DoctorDAO, appCtx.ConfirmDAO)
	adminHandler := NewAdminHandler(appCtx.RuleDAO, appCtx.DoctorDAO, appCtx.PushLogDAO, appCtx.SyncLogDAO, appCtx.QCEngine, appCtx.SyncSvc, appCtx.PushSvc)

	// JWT 鉴权中间件
	authMw := middleware.JWTAuth(appCtx.AuthSvc)

	// ============ 公开接口 ============

	// 健康检查 — 返回基础设施状态
	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":     "ok",
			"version":    "1.0.0",
			"database":   dbReady,
			"wework":     weWorkReady,
			"serverTime": gin.H{"mode": appCtx.Cfg.Server.Mode},
		})
	})

	// ============ H5 接口（需要 JWT 鉴权 + 数据库） ============
	if dbReady {
		apiV1 := r.Group("/api/v1")
		{
			reportHandler.RegisterRoutes(apiV1, authMw)
			doctorHandler.RegisterRoutes(apiV1, authMw)
			deptHandler.RegisterRoutes(apiV1, authMw)
		}

		// ============ 管理接口 ============
		adminHandler.RegisterRoutes(apiV1)
	} else {
		log.Warn().Msg("数据库未连接，H5/管理接口未注册（仅健康检查可用）")
	}
}
