/*
 * like_dto.go
 * 功能：点赞应用服务相关的请求、响应类型定义
 * 时间戳：2026-04-26
 */

package application

// LikeRequest 点赞/取消点赞请求
// comment_id 仅评论点赞使用，视频点赞时由 URL 参数决定目标，因此不设 required
type LikeRequest struct {
	CommentID int64 `json:"comment_id,omitempty"`
	Action    int8  `json:"action" binding:"required,oneof=1 2"` // 1点赞 2取消
}

// LikeResponse 点赞响应
type LikeResponse struct {
	LikeCount int64 `json:"like_count"`
}
