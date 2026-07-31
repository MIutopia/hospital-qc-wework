package middleware

import (
	"net/http"
	"runtime/debug"

	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// Recovery panic 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Error().
					Interface("panic", err).
					Str("stack", string(debug.Stack())).
					Str("path", c.Request.URL.Path).
					Msg("服务器发生 panic")
				response.Error(c, http.StatusInternalServerError, 50001, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}
