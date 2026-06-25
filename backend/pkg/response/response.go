/*
 * response.go
 * 功能：统一 JSON 响应格式封装
 * 时间戳：2026-04-20
 * 变更：新增内测相关状态码常量
 */

package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 统一响应状态码常量
const (
	CodeSuccess         = 0
	CodeInvalidParam    = 1001
	CodeBizError        = 1002
	CodeBetaKeyInvalid  = 1004
	CodeBetaTokenExpire = 1005
	CodeInternalError   = 5000
)

// Response 是前后端统一的 JSON 响应结构
// Code: 0=成功, 1xxx=客户端错误, 5xxx=服务端错误
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	})
}

// Fail 返回业务失败响应
func Fail(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// Error 返回服务端内部错误
func Error(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    CodeInternalError,
		Message: message,
		Data:    nil,
	})
}
