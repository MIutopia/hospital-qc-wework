package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageData 分页数据
type PageData struct {
	List     interface{} `json:"list"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"pageSize"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    data,
	})
}

// SuccessPage 成功分页响应
func SuccessPage(c *gin.Context, list interface{}, total, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: PageData{
			List:     list,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// Error 错误响应
func Error(c *gin.Context, httpStatus, code int, message string) {
	c.JSON(httpStatus, Response{
		Code:    code,
		Message: message,
	})
}

// BadRequest 参数错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, 40001, message)
}

// Unauthorized 鉴权失败
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, 40101, message)
}

// Forbidden 权限不足
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, 40301, message)
}

// ServerError 服务器错误
func ServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, 50001, message)
}
