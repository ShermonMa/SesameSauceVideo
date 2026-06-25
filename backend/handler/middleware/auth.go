/*
 * auth.go
 * 功能：JWT 认证中间件，从 Authorization 头提取 token 并注入 user_id
 *      2026-06-21 新增 OptionalJWTAuth：支持需要识别登录态但允许匿名的接口（如视频详情）
 * 时间戳：2026-04-22
 */

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/infra/pkg"
	"github.com/shermon/SesameSauce/pkg/response"
)

// JWTAuth 校验 Bearer Token，将 user_id 写入 gin.Context
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Fail(c, 1001, "缺少 Authorization 头")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Fail(c, 1001, "Authorization 格式错误")
			c.Abort()
			return
		}

		claims, err := pkg.ParseToken(parts[1])
		if err != nil {
			response.Fail(c, 1001, "Token 无效或已过期")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

// OptionalJWTAuth 可选 JWT 认证：携带有效 Token 时注入 user_id，未携带或无效时继续放行
// 用于需要区分登录/匿名态的公开接口（如视频详情需返回当前用户是否点赞）
func OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		claims, err := pkg.ParseToken(parts[1])
		if err == nil {
			c.Set("user_id", claims.UserID)
		}
		c.Next()
	}
}
