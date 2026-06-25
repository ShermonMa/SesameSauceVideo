/*
 * user_handler.go
 * 功能：用户接口层，处理 HTTP 请求并调用应用服务
 * 时间戳：2026-04-20
 */

package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/pkg/response"
)

// UserHandler 提供用户相关的 HTTP 接口
type UserHandler struct {
	userService *application.UserService
}

// NewUserHandler 创建 UserHandler 实例
func NewUserHandler() *UserHandler {
	return &UserHandler{
		userService: application.NewUserService(),
	}
}

// Register 处理用户注册请求 POST /api/v1/user/register
func (h *UserHandler) Register(c *gin.Context) {
	var req application.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.userService.Register(req)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, resp)
}

// Login 处理用户登录请求 POST /api/v1/user/login
func (h *UserHandler) Login(c *gin.Context) {
	var req application.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	resp, err := h.userService.Login(req)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, resp)
}

// GetUserProfile 处理用户资料查询请求 GET /api/v1/users/:id
// 支持 OptionalJWTAuth：登录时返回 is_following 状态
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || userID == 0 {
		response.Fail(c, 1001, "用户 ID 格式错误")
		return
	}

	// 可选登录态：提取当前访客 ID
	var viewerID uint64
	if uid, ok := c.Get("user_id"); ok {
		viewerID = uid.(uint64)
	}

	resp, err := h.userService.GetUserProfile(userID, viewerID)
	if err != nil {
		response.Fail(c, 1002, err.Error())
		return
	}

	response.Success(c, resp)
}
