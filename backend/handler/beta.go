/*
 * beta.go
 * 功能：内测准入接口层，提供密钥校验与 Beta Token 检查接口
 * 时间戳：2026-05-29
 */

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/infra/pkg"
	"github.com/shermon/SesameSauce/pkg/response"
)

// BetaVerifyRequest 密钥校验请求
 type BetaVerifyRequest struct {
	Key string `json:"key" binding:"required,min=1,max=64"`
}

// BetaHandler 提供内测准入相关的 HTTP 接口
 type BetaHandler struct{}

// NewBetaHandler 创建 BetaHandler 实例
func NewBetaHandler() *BetaHandler {
	return &BetaHandler{}
}

// Verify 校验内测密钥并签发 Beta Token  POST /api/v1/beta/verify
func (h *BetaHandler) Verify(c *gin.Context) {
	var req BetaVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	// 查询 beta_keys 表
	var count int64
	if err := config.DB.Table("beta_keys").Where("`key` = ?", req.Key).Count(&count).Error; err != nil {
		response.Error(c, "数据库查询失败")
		return
	}
	if count == 0 {
		response.Fail(c, 1004, "内测密钥错误")
		return
	}

	// 签发 Beta Token
	token, exp, err := pkg.GenerateBetaToken()
	if err != nil {
		response.Error(c, "Token 签发失败")
		return
	}

	response.Success(c, gin.H{
		"beta_token": token,
		"expires_at": exp.Format("2006-01-02T15:04:05-07:00"),
	})
}

// Check 检查当前 Beta Token 是否有效  GET /api/v1/beta/check
func (h *BetaHandler) Check(c *gin.Context) {
	token := c.GetHeader("X-Beta-Token")
	if token == "" {
		response.Fail(c, 1005, "缺少内测准入凭证")
		return
	}

	claims, err := pkg.ParseBetaToken(token)
	if err != nil {
		response.Fail(c, 1005, "内测权限已过期")
		return
	}

	response.Success(c, gin.H{
		"valid":      true,
		"expires_at": claims.ExpiresAt.Time.Format("2006-01-02T15:04:05-07:00"),
	})
}
