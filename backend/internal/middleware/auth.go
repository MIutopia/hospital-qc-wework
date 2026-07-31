package middleware

import (
	"net/http"
	"strings"

	"hospital-qc-wework/internal/service/auth"
	"hospital-qc-wework/pkg/response"

	"github.com/gin-gonic/gin"
)

// JWTAuth JWT 鉴权中间件
func JWTAuth(authSvc *auth.JWTService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, 40101, "缺少 Authorization 头")
			c.Abort()
			return
		}

		// 格式: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(c, http.StatusUnauthorized, 40102, "Authorization 格式错误，应为 Bearer <token>")
			c.Abort()
			return
		}

		tokenStr := parts[1]

		// 验证 JWT
		claims, err := authSvc.ValidateToken(tokenStr)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, 40103, "Token 无效或已过期")
			c.Abort()
			return
		}

		// 将用户信息写入上下文
		c.Set("doctor_id", claims.DoctorID)
		c.Set("case_id", claims.CaseID)
		c.Set("token_id", claims.ID)

		c.Next()
	}
}
