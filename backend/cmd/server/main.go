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

	"hospital-qc-wework/internal/app"
	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/handler"
	"hospital-qc-wework/internal/middleware"

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

	// 装配应用上下文（数据库/企微不可用时代降级启动）
	appCtx := app.New(cfg)
	defer appCtx.Close()

	// 企业微信 Token 管理器后台刷新（凭证有效时）
	if appCtx.TokenMgr != nil {
		go appCtx.TokenMgr.RefreshLoop(cfg.WeWork.TokenRefreshInterval)
		log.Info().Msg("企业微信 Token 管理器已启动")
	}

	// 初始化 Gin 引擎
	r := gin.New()
	r.Use(
		middleware.Recovery(),
		middleware.Logger(),
	)

	// 注册路由
	handler.RegisterRoutes(r, appCtx)

	// 初始化应用内定时任务调度器
	// TODO: 网络恢复后替换为 robfig/cron（PM 技术决策已确认）
	sched := newSimpleScheduler()
	setupScheduledTasks(sched, appCtx)
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

// setupScheduledTasks 配置定时任务（M2 同步 / M3 质控 / M4 推送）
func setupScheduledTasks(s *simpleScheduler, appCtx *app.App) {
	cfg := appCtx.Cfg

	// 1. HIS 数据同步（his_database.sync_time，如 23:00）
	if appCtx.SyncSvc != nil && cfg.HISDatabase.SyncTime != "" {
		s.Add("HIS Sync", cfg.HISDatabase.SyncTime, func() {
			log.Info().Msg("[Scheduler] 开始定时 HIS 数据同步")
			res, err := appCtx.SyncSvc.RunSync()
			if err != nil {
				log.Error().Err(err).Msg("[Scheduler] 定时同步失败")
				return
			}
			log.Info().Int("total", res.TotalSynced).Int("new", res.NewCases).Int("updated", res.Updated).Msg("[Scheduler] 定时同步完成")
		})
		log.Info().Str("triggerAt", cfg.HISDatabase.SyncTime).Msg("HIS 数据同步定时任务已注册")
	}

	// 2. 每日质控（qc.cron，如 06:10）
	if appCtx.QCEngine != nil && cfg.QC.Cron != "" {
		hm := parseCronToHM(cfg.QC.Cron)
		if hm != "" {
			s.Add("QC Batch", hm, func() {
				log.Info().Msg("[Scheduler] 开始定时质控")
				res, err := appCtx.QCEngine.RunBatch()
				if err != nil {
					log.Error().Err(err).Msg("[Scheduler] 定时质控失败")
					return
				}
				log.Info().Str("batchId", res.BatchID).Int("defectCases", res.DefectCases).Msg("[Scheduler] 定时质控完成")

				// 有缺陷病例 → 自动推送（M4）
				if res.DefectCases > 0 && appCtx.PushSvc != nil {
					if _, pushErr := appCtx.PushSvc.PushIssuedCases(); pushErr != nil {
						log.Error().Err(pushErr).Msg("[Scheduler] 质控后自动推送失败")
					}
				}
			})
			log.Info().Str("cron", cfg.QC.Cron).Str("triggerAt", hm).Msg("质控定时任务已注册")
		}
	}

	// 3. 每日推送（qc.push_cron，如 06:30）
	if appCtx.PushSvc != nil && cfg.QC.PushCron != "" {
		hm := parseCronToHM(cfg.QC.PushCron)
		if hm != "" {
			s.Add("Push Notify", hm, func() {
				log.Info().Msg("[Scheduler] 开始定时推送")
				res, err := appCtx.PushSvc.PushIssuedCases()
				if err != nil {
					log.Error().Err(err).Msg("[Scheduler] 定时推送失败")
					return
				}
				log.Info().Int("success", res.Success).Int("failed", res.Failed).Int("deferred", res.Deferred).Msg("[Scheduler] 定时推送完成")
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
