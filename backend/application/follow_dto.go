/*
 * follow_dto.go
 * 功能：关注应用服务相关的请求、响应类型定义
 * 时间戳：2026-05-22
 */

package application

// FollowRequest 关注/取消关注请求
type FollowRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Action int8   `json:"action" binding:"required,oneof=1 2"` // 1关注 2取消
}

// FollowResponse 关注响应
type FollowResponse struct {
	FollowerCount  int64 `json:"follower_count"`
	FollowingCount int64 `json:"following_count"`
}
