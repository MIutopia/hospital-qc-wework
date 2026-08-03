// Package app 应用装配层：集中创建 DAO / 服务 / 引擎实例，
// 供 main 入口、路由注册与定时调度器共用，避免各层重复初始化。
package app

import (
	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/internal/service/push"
	"hospital-qc-wework/internal/service/qc"
	"hospital-qc-wework/internal/service/sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// App 应用运行时上下文
type App struct {
	Cfg      *config.Config
	DB       *sqlx.DB // 业务库连接（数据库不可用时为 nil，降级模式）
	AuthSvc  *auth.JWTService
	TokenMgr *push.TokenManager

	CaseDAO    *dao.CaseDAO
	RuleDAO    *dao.RuleDAO
	ResultDAO  *dao.ResultDAO
	DoctorDAO  *dao.DoctorDAO
	PushLogDAO *dao.PushLogDAO
	SyncLogDAO *dao.SyncLogDAO
	ConfirmDAO *dao.ConfirmDAO

	QCEngine *qc.Engine
	SyncSvc  *sync.SyncService
	Pusher   *push.Pusher
	PushSvc  *push.PushService
}

// New 装配应用上下文（数据库连接失败时降级，返回的 App.DB 为 nil）
func New(cfg *config.Config) *App {
	a := &App{Cfg: cfg}

	// JWT 鉴权服务
	a.AuthSvc = auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 企业微信 Token 管理器（凭证缺失时为 nil，推送功能降级）
	if cfg.WeWork.CorpID != "" && cfg.WeWork.AgentSecret != "" && cfg.WeWork.AgentID != 0 {
		a.TokenMgr = push.NewTokenManager(&cfg.WeWork)
	} else {
		log.Warn().Msg("企业微信凭证未配置（CorpID/AgentID/Secret），消息推送功能暂不可用")
	}

	// 业务数据库
	db, err := dao.InitDB(cfg.Database)
	if err != nil {
		log.Warn().Err(err).Msg("数据库连接失败，服务将以降级模式启动（健康检查/日志正常，数据相关接口不可用）")
		return a
	}
	a.DB = db
	log.Info().Str("server", cfg.Database.Server).Int("port", cfg.Database.Port).Str("name", cfg.Database.Name).Msg("数据库连接成功")

	// DAO 层
	a.CaseDAO = dao.NewCaseDAO(db)
	a.RuleDAO = dao.NewRuleDAO(db)
	a.ResultDAO = dao.NewResultDAO(db)
	a.DoctorDAO = dao.NewDoctorDAO(db)
	a.PushLogDAO = dao.NewPushLogDAO(db)
	a.SyncLogDAO = dao.NewSyncLogDAO(db)
	a.ConfirmDAO = dao.NewConfirmDAO(db)

	// 质控引擎
	a.QCEngine = qc.NewEngine(a.RuleDAO, a.CaseDAO, a.ResultDAO, cfg.QC.Concurrency)

	// 数据同步服务（HIS 连接可选：未配置/连接失败时 CSV 导入仍可用）
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
	a.SyncSvc = sync.NewSyncService(hisDAO, a.CaseDAO, a.DoctorDAO, a.SyncLogDAO, cfg.QC.BatchSize)

	// 企业微信推送服务
	if a.TokenMgr != nil {
		a.Pusher = push.NewPusher(&cfg.WeWork, a.TokenMgr)
		a.PushSvc = push.NewPushService(cfg, a.CaseDAO, a.ResultDAO, a.RuleDAO, a.DoctorDAO, a.PushLogDAO, a.Pusher, a.AuthSvc)
		log.Info().Msg("企业微信推送服务已就绪")
	}

	return a
}

// Close 释放资源
func (a *App) Close() {
	if a.DB != nil {
		_ = a.DB.Close()
	}
}
