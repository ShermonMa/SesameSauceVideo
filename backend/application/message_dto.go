/*
 * message_dto.go
 * 功能：消息/信箱应用服务相关的请求、响应及 DTO 类型定义
 * 时间戳：2026-04-26
 */

package application

import "time"

// SendPrivateRequest 发送私信请求
type SendPrivateRequest struct {
	ToUserID int64  `json:"to_user_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
}

// UnreadCountResponse 未读气泡数聚合响应
type UnreadCountResponse struct {
	Private int64 `json:"private"`
	Reply   int64 `json:"reply"`
	Mention int64 `json:"mention"`
	Like    int64 `json:"like"`
	Follow  int64 `json:"follow"`
	System  int64 `json:"system"`
}

// MessageListRequest 获取消息列表请求
type MessageListRequest struct {
	Type     int8
	Page     int
	PageSize int
	UserID   uint64
}

// MessageListResponse 消息列表响应
type MessageListResponse struct {
	Messages []MessageItemDTO `json:"messages"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

// MessageItemDTO 消息列表项
type MessageItemDTO struct {
	ID           uint64    `json:"id"`
	FromUserID   uint64    `json:"from_user_id"`
	FromUserName string    `json:"from_user_name"`
	FromUserAvatar string  `json:"from_user_avatar"`
	Content      string    `json:"content"`
	BizID        *uint64   `json:"biz_id,omitempty"`
	BizType      *int8     `json:"biz_type,omitempty"`
	IsRead       int8      `json:"is_read"`
	CreatedAt    time.Time `json:"created_at"`
}

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	IDs  []int64 `json:"ids"`
	Type int8    `json:"type"`
}

// SystemListResponse 系统通知列表响应
type SystemListResponse struct {
	Notifications []SystemNotificationDTO `json:"notifications"`
	HasNew        bool                    `json:"has_new"`
	Page          int                     `json:"page"`
	PageSize      int                     `json:"page_size"`
	Total         int64                   `json:"total"`
}

// SystemNotificationDTO 系统通知列表项
type SystemNotificationDTO struct {
	ID          uint64    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	PublishTime time.Time `json:"publish_time"`
}
