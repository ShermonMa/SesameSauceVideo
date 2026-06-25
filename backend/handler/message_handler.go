/*
 * message_handler.go
 * 功能：消息/信箱接口层，处理私信、未读数、消息列表、系统通知
 * 时间戳：2026-04-26
 */

package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/pkg/response"
)

// MessageHandler 提供消息相关的 HTTP 接口
type MessageHandler struct {
	messageService *application.MessageService
}

// NewMessageHandler 创建 MessageHandler 实例
func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		messageService: application.NewMessageService(),
	}
}

// SendPrivate 发送私信 POST /api/v1/message/private
func (h *MessageHandler) SendPrivate(c *gin.Context) {
	var req application.SendPrivateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.messageService.SendPrivate(uid, req); err != nil {
		switch err.Error() {
		case "对方暂不接受消息":
			response.Fail(c, 1001, err.Error())
		case "不能给自己发送私信":
			response.Fail(c, 1001, err.Error())
		default:
			response.Fail(c, 1002, err.Error())
		}
		return
	}

	response.Success(c, nil)
}

// GetUnreadCount 获取未读气泡数 GET /api/v1/message/unread-count
func (h *MessageHandler) GetUnreadCount(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	resp, err := h.messageService.GetUnreadCount(uid)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// ListMessages 获取某 Tab 消息列表 GET /api/v1/message/list
func (h *MessageHandler) ListMessages(c *gin.Context) {
	msgType, err := strconv.ParseInt(c.Query("type"), 10, 64)
	if err != nil {
		response.Fail(c, 1001, "无效的消息类型")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	req := application.MessageListRequest{
		Type:     int8(msgType),
		Page:     page,
		PageSize: pageSize,
		UserID:   uid,
	}

	resp, err := h.messageService.ListMessages(req)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// MarkRead 标记已读 POST /api/v1/message/read
func (h *MessageHandler) MarkRead(c *gin.Context) {
	var req application.MarkReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, 1001, "请求参数错误: "+err.Error())
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.messageService.MarkRead(uid, req); err != nil {
		response.Fail(c, 1001, err.Error())
		return
	}

	response.Success(c, nil)
}

// ListSystemNotifications 获取系统通知列表 GET /api/v1/message/system-list
func (h *MessageHandler) ListSystemNotifications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("size", "20"))

	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	resp, err := h.messageService.ListSystemNotifications(uid, page, pageSize)
	if err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, resp)
}

// MarkSystemRead 标记系统通知已读 POST /api/v1/message/system-read
func (h *MessageHandler) MarkSystemRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid := userID.(uint64)

	if err := h.messageService.MarkSystemRead(uid); err != nil {
		response.Fail(c, 5000, err.Error())
		return
	}

	response.Success(c, nil)
}
