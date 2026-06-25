/*
 * comment_dto.go
 * 功能：评论应用服务相关的请求、响应及 DTO 类型定义；video_id 改为 hashid 字符串以与对外 API 保持一致
 * 时间戳：2026-05-01
 */

package application

import "time"

// PublishCommentRequest 发布评论请求；VideoID 为 hashid 字符串（对外 ID）
type PublishCommentRequest struct {
	VideoID  string `json:"video_id" binding:"required"`
	Content  string `json:"content" binding:"required"`
	ParentID *int64 `json:"parent_id"`
}

// PublishCommentResponse 发布评论响应
type PublishCommentResponse struct {
	CommentID uint64    `json:"comment_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CommentListRequest 获取评论列表请求
type CommentListRequest struct {
	VideoID  uint64
	Page     int
	PageSize int
	UserID   uint64 // 当前登录用户ID，用于拉黑过滤
}

// CommentListResponse 评论列表响应
type CommentListResponse struct {
	Comments []CommentItemDTO `json:"comments"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

// ReplyListRequest 获取楼中楼列表请求
type ReplyListRequest struct {
	RootID   uint64
	Page     int
	PageSize int
}

// ReplyListResponse 楼中楼列表响应
type ReplyListResponse struct {
	Replies  []ReplyItemDTO `json:"replies"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}

// CommentItemDTO 一级评论列表项
type CommentItemDTO struct {
	ID          uint64         `json:"id"`
	UserID      uint64         `json:"user_id"`
	UserName    string         `json:"user_name"`
	UserAvatar  string         `json:"user_avatar"`
	Content     string         `json:"content"`
	LikeCount   int64          `json:"like_count"`
	IsLiked     bool           `json:"is_liked"`
	CreatedAt   time.Time      `json:"created_at"`
	Replies     []ReplyItemDTO `json:"replies"`
	ReplyCount  int64          `json:"reply_count"`
	IsDeleted   bool           `json:"is_deleted"`
}

// ReplyItemDTO 二级评论（楼中楼）列表项
type ReplyItemDTO struct {
	ID         uint64    `json:"id"`
	UserID     uint64    `json:"user_id"`
	UserName   string    `json:"user_name"`
	UserAvatar string    `json:"user_avatar"`
	Content    string    `json:"content"`
	LikeCount  int64     `json:"like_count"`
	CreatedAt  time.Time `json:"created_at"`
	IsDeleted  bool      `json:"is_deleted"`
}
