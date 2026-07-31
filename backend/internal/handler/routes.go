package handler

import (
	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/middleware"
	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/internal/service/push"
	"hospital-qc-wework/internal/service/qc"
	"hospital-qc-wework/internal/service/sync"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/jmoiron/sqlx"
)

// RegisterRoutes 注册全部路由
func RegisterRoutes(r *gin.Engine, db *sqlx.DB, authSvc *auth.JWTService, cfg *config.Config, tokenMgr *push.TokenManager) {
	dbReady := db != nil
	weWorkReady := tokenMgr != nil && cfg.WeWork.CorpID != ""

	// 初始化 DAO（db 可能为 nil，降级模式下仅健康检查可用）
	var (
		caseDAO    *dao.CaseDAO
		ruleDAO    *dao.RuleDAO
		resultDAO  *dao.ResultDAO
		doctorDAO  *dao.DoctorDAO
		pushLogDAO *dao.PushLogDAO
		syncLogDAO *dao.SyncLogDAO
		qcEngine   *qc.Engine
		syncSvc    *sync.SyncService
	)
	if dbReady {
		caseDAO = dao.NewCaseDAO(db)
		ruleDAO = dao.NewRuleDAO(db)
		resultDAO = dao.NewResultDAO(db)
		doctorDAO = dao.NewDoctorDAO(db)
		pushLogDAO = dao.NewPushLogDAO(db)
		syncLogDAO = dao.NewSyncLogDAO(db)

		// 初始化质控引擎
		qcEngine = qc.NewEngine(ruleDAO, caseDAO, resultDAO, cfg.QC.Concurrency)

		// 企业微信推送服务状态日志（M4 阶段正式接入 Pusher）
		if weWorkReady {
			log.Info().Msg("企业微信凭证已配置，M4 阶段将接入推送服务")
		} else {
			log.Warn().Msg("企业微信凭证未配置，消息推送功能暂不可用")
		}

		// 初始化同步服务（HIS 连接可选：未配置/连接失败时 CSV 导入仍可用）
		var hisDAO *dao.HISDAO
		if cfg.HISDatabase.User != "" {
			hisDB, err := dao.InitHISDB(cfg.HISDatabase)
			if err != nil {
				log.Warn().Err(err).Msg("HIS 数据库连接失败，自动同步不可用（可改用 CSV 导入）")
			} else {
				hisDAO = dao.NewHISDAO(hisDB)
				log.Info().Str("db", cfg.HISDatabase.Name).Msg("HIS 数据仓库连接成功")
			}
		}
		syncSvc = sync.NewSyncService(hisDAO, caseDAO, doctorDAO, syncLogDAO, cfg.QC.BatchSize)
	}

	// 初始化 Handler
	reportHandler := NewReportHandler(caseDAO, resultDAO, ruleDAO, authSvc)
	doctorHandler := NewDoctorHandler(caseDAO)
	deptHandler := NewDeptHandler(caseDAO)
	adminHandler := NewAdminHandler(ruleDAO, doctorDAO, pushLogDAO, syncLogDAO, qcEngine, syncSvc)

	// JWT 鉴权中间件
	authMw := middleware.JWTAuth(authSvc)

	// ============ 公开接口 ============

	// 健康检查 — 返回基础设施状态
	r.GET("/api/v1/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":    "ok",
			"version":   "1.0.0",
			"database":  dbReady,
			"wework":    weWorkReady,
			"serverTime": gin.H{"mode": cfg.Server.Mode},
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
	}
}
