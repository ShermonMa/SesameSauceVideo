/*
 * blacklist_handler.go
 * 功能：拉黑接口层，处理拉黑/取消拉黑用户
 * 时间戳：2026-04-26
 */

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/pkg/response"
)

// BlacklistHandler 提供拉黑相关的 HTTP 接口
type BlacklistHandler struct {
	blacklistService *application.BlacklistService
}

// NewBlacklistHandler 创建 BlacklistHandler 实例
func NewBlacklistHandler() *BlacklistHandler {
	return &BlacklistHandler{
		blacklistService: application.NewBlacklistService(),
	}
}

// BlockUser 拉黑用户 POST /api/v1/blacklist/block
func (h *BlacklistHandler) BlockUser(c *gin.Context) {
	var req application.BlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.blacklistService.BlockUser(uid, req); err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, nil)
}

// UnblockUser 取消拉黑 POST /api/v1/blacklist/unblock
func (h *BlacklistHandler) UnblockUser(c *gin.Context) {
	var req application.BlacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.blacklistService.UnblockUser(uid, req); err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, nil)
}
