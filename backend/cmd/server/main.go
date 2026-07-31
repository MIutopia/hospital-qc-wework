package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/handler"
	"hospital-qc-wework/internal/middleware"
	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/internal/service/push"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// 命令行参数
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	initLogger(cfg.Log)

	// 设置 Gin 运行模式
	initGinMode(cfg.Server.Mode)

	// 初始化数据库连接（开发环境无 SQL Server 时不阻塞启动）
	db, err := dao.InitDB(cfg.Database)
	if err != nil {
		log.Warn().Err(err).Msg("数据库连接失败，服务将以降级模式启动（健康检查/日志正常，数据相关接口不可用）")
	} else {
		defer db.Close()
		log.Info().Str("server", cfg.Database.Server).Int("port", cfg.Database.Port).Str("name", cfg.Database.Name).Msg("数据库连接成功")
	}

	// 初始化 JWT 鉴权服务
	authSvc := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 初始化企业微信 Token 管理器（无凭证时仅记录警告，不阻塞启动）
	var tokenMgr *push.TokenManager
	if cfg.WeWork.CorpID != "" && cfg.WeWork.AgentSecret != "" && cfg.WeWork.AgentID != 0 {
		tokenMgr = push.NewTokenManager(&cfg.WeWork)
		go tokenMgr.RefreshLoop(cfg.WeWork.TokenRefreshInterval)
		log.Info().Msg("企业微信 Token 管理器已启动")
	} else {
		log.Warn().Msg("企业微信凭证未配置（CorpID/AgentID/Secret），消息推送功能暂不可用。请设置环境变量 WEWORK_CORP_ID、WEWORK_AGENT_ID、WEWORK_AGENT_SECRET")
	}

	// 初始化 Gin 引擎
	r := gin.New()
	r.Use(
		middleware.Recovery(),
		middleware.Logger(),
	)

	// 注册路由（tokenMgr 可能为 nil，handler 内部做降级处理）
	handler.RegisterRoutes(r, db, authSvc, cfg, tokenMgr)

	// 初始化应用内定时任务调度器
	// TODO: 网络恢复后替换为 robfig/cron（PM 技术决策已确认）
	sched := newSimpleScheduler()
	setupScheduledTasks(sched, cfg)
	sched.Start()

	// HTTP 服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Info().Msg("正在关闭服务...")

		// 停止定时任务
		sched.Stop()
		log.Info().Msg("定时任务已停止")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("服务关闭失败")
		}
	}()

	log.Info().Int("port", cfg.Server.Port).Str("mode", cfg.Server.Mode).Msg("服务启动")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("服务启动失败")
	}
	log.Info().Msg("服务已关闭")
}

// initGinMode 设置 Gin 运行模式
func initGinMode(mode string) {
	switch mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
}

// simpleScheduler 简易定时调度器（标准库实现）
// TODO: 网络恢复后替换为 robfig/cron
type simpleScheduler struct {
	mu      sync.Mutex
	tasks   []scheduledTask
	stopCh  chan struct{}
	running bool
}

type scheduledTask struct {
	name     string
	cronExpr string // "min hour * * *" 格式
	fn       func()
}

func newSimpleScheduler() *simpleScheduler {
	return &simpleScheduler{stopCh: make(chan struct{})}
}

// Add 添加定时任务（cronExpr 格式："HH:MM" 即 "时:分"）
func (s *simpleScheduler) Add(name, cronExpr string, fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, scheduledTask{name: name, cronExpr: cronExpr, fn: fn})
}

// Start 启动调度器（每分钟检查一次）
func (s *simpleScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			case t := <-ticker.C:
				s.checkAndRun(t)
			}
		}
	}()
}

// Stop 停止调度器
func (s *simpleScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// checkAndRun 检查并执行到达时间的任务
func (s *simpleScheduler) checkAndRun(now time.Time) {
	s.mu.Lock()
	tasks := make([]scheduledTask, len(s.tasks))
	copy(tasks, s.tasks)
	s.mu.Unlock()

	nowHM := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	for _, task := range tasks {
		if task.cronExpr == nowHM {
			log.Info().Str("task", task.name).Str("time", nowHM).Msg("[Scheduler] 定时任务触发")
			go task.fn() // 异步执行，避免阻塞调度器
		}
	}
}

// parseCronToHM 将 "min hour * * *" 格式的 cron 转换为 "HH:MM"
func parseCronToHM(cronExpr string) string {
	parts := strings.Fields(cronExpr)
	if len(parts) < 2 {
		return ""
	}
	return fmt.Sprintf("%02s:%02s", parts[1], parts[0])
}

// setupScheduledTasks 配置定时任务
// 注意：当前 cron 调度为占位实现，M3/M4 阶段联调时接入实际的 QC 引擎和推送服务
func setupScheduledTasks(s *simpleScheduler, cfg *config.Config) {
	// 每日质控任务
	if cfg.QC.Cron != "" {
		hm := parseCronToHM(cfg.QC.Cron)
		if hm != "" {
			s.Add("QC Batch", hm, func() {
				log.Info().Str("cron", cfg.QC.Cron).Str("time", hm).Msg("[Scheduler] 质控定时任务触发 — 待 M3/M4 联调接入")
			})
			log.Info().Str("cron", cfg.QC.Cron).Str("triggerAt", hm).Msg("质控定时任务已注册")
		}
	}

	// 每日推送任务
	if cfg.QC.PushCron != "" {
		hm := parseCronToHM(cfg.QC.PushCron)
		if hm != "" {
			s.Add("Push Notify", hm, func() {
				log.Info().Str("cron", cfg.QC.PushCron).Str("time", hm).Msg("[Scheduler] 推送定时任务触发 — 待 M4 联调接入")
			})
			log.Info().Str("cron", cfg.QC.PushCron).Str("triggerAt", hm).Msg("推送定时任务已注册")
		}
	}
}

// initLogger 初始化 zerolog
func initLogger(cfg config.LogConfig) {
	// 日志级别
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// 日志格式
	if cfg.Format == "text" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	} else {
		// JSON 格式，添加时间字段
		log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	// 文件输出（可选）
	if cfg.Output == "file" && cfg.FilePath != "" {
		file, err := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Warn().Err(err).Msg("无法打开日志文件，使用标准输出")
		} else {
			log.Logger = zerolog.New(file).With().Timestamp().Logger()
		}
	}
}
