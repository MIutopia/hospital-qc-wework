package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hospital-qc-wework/internal/config"
	"hospital-qc-wework/internal/dao"
	"hospital-qc-wework/internal/handler"
	"hospital-qc-wework/internal/middleware"
	"hospital-qc-wework/internal/service/auth"

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

	// 初始化数据库连接
	db, err := dao.InitDB(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("数据库连接失败")
	}
	defer db.Close()

	// 初始化 JWT 鉴权服务
	authSvc := auth.NewJWTService(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 初始化 Gin 引擎
	r := gin.New()
	r.Use(
		middleware.Recovery(),
		middleware.Logger(),
	)

	// 注册路由
	handler.RegisterRoutes(r, db, authSvc, cfg)

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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatal().Err(err).Msg("服务关闭失败")
		}
	}()

	log.Info().Int("port", cfg.Server.Port).Msg("服务启动")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("服务启动失败")
	}
	log.Info().Msg("服务已关闭")
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
