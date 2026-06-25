/*
 * beta.go
 * 功能：全局 Beta 准入中间件，校验 X-Beta-Token 头；排除 /api/v1/beta/verify 自身
 * 时间戳：2026-05-29
 */

package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/infra/pkg"
	"github.com/shermon/SesameSauce/pkg/response"
)

// BetaAuth 全局内测准入校验中间件
func BetaAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// OPTIONS 预检请求直接放行
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// 排除 beta/verify 和 beta/check 自身，防止死循环
		path := c.Request.URL.Path
		if strings.HasSuffix(path, "/beta/verify") || strings.HasSuffix(path, "/beta/check") {
			c.Next()
			return
		}

		token := c.GetHeader("X-Beta-Token")
		if token == "" {
			response.Fail(c, 1005, "缺少内测准入凭证")
			c.Abort()
			return
		}

		_, err := pkg.ParseBetaToken(token)
		if err != nil {
			response.Fail(c, 1005, "内测权限已过期")
			c.Abort()
			return
		}

		c.Next()
	}
}
