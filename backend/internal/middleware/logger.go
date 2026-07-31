package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// 处理请求
		c.Next()

		// 计算耗时
		latency := time.Since(start)

		// 不记录健康检查接口
		if path == "/api/v1/health" {
			return
		}

		log.Info().
			Int("status", c.Writer.Status()).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", c.Request.URL.RawQuery).
			Str("ip", c.ClientIP()).
			Dur("latency", latency).
			Int("body_size", c.Writer.Size()).
			Msg("请求日志")
	}
}
