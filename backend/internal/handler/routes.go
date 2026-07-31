package handler

import (
	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/middleware"
	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/internal/service/qc"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// RegisterRoutes 注册全部路由
func RegisterRoutes(r *gin.Engine, db *sqlx.DB, authSvc *auth.JWTService, cfg *config.Config) {
	// 初始化 DAO
	caseDAO := dao.NewCaseDAO(db)
	ruleDAO := dao.NewRuleDAO(db)
	resultDAO := dao.NewResultDAO(db)
	doctorDAO := dao.NewDoctorDAO(db)
	pushLogDAO := dao.NewPushLogDAO(db)

	// 初始化服务
	qcEngine := qc.NewEngine(ruleDAO, caseDAO, resultDAO, cfg.QC.Concurrency)

	// 初始化 Handler
	reportHandler := NewReportHandler(caseDAO, resultDAO, ruleDAO, authSvc)
	doctorHandler := NewDoctorHandler(caseDAO)
	deptHandler := NewDeptHandler(caseDAO)
	adminHandler := NewAdminHandler(ruleDAO, doctorDAO, pushLogDAO, qcEngine)

	// JWT 鉴权中间件
	authMw := middleware.JWTAuth(authSvc)

	// ============ 公开接口 ============

	// 健康检查
	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// ============ H5 接口（需要 JWT 鉴权） ============
	apiV1 := r.Group("/api/v1")
	{
		reportHandler.RegisterRoutes(apiV1, authMw)
		doctorHandler.RegisterRoutes(apiV1, authMw)
		deptHandler.RegisterRoutes(apiV1, authMw)
	}

	// ============ 管理接口 ============
	adminHandler.RegisterRoutes(apiV1)
}
