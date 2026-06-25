/*
 * follow_handler.go
 * 功能：关注接口层，处理关注/取消关注 HTTP 请求
 * 时间戳：2026-05-22
 */

package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/pkg/response"
)

// FollowHandler 提供关注相关的 HTTP 接口
type FollowHandler struct {
	followService *application.FollowService
}

// NewFollowHandler 创建 FollowHandler 实例
func NewFollowHandler() *FollowHandler {
	return &FollowHandler{
		followService: application.NewFollowService(),
	}
}

// FollowUser 关注/取消关注 POST /api/v1/follow
func (h *FollowHandler) FollowUser(c *gin.Context) {
	var req application.FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	var err error
	if req.Action == 1 {
		err = h.followService.FollowUser(uid, req.UserID)
	} else {
		err = h.followService.UnfollowUser(uid, req.UserID)
	}

	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	resp := &application.FollowResponse{
		FollowerCount:  h.followService.GetFollowerCount(req.UserID),
		FollowingCount: h.followService.GetFollowingCount(uid),
	}
	response.Success(c, resp)
}
