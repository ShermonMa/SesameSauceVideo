/*
 * blacklist_dto.go
 * 功能：拉黑应用服务相关的请求、响应类型定义
 * 时间戳：2026-04-26
 */

package application

// BlacklistRequest 拉黑/取消拉黑请求
type BlacklistRequest struct {
	BlockedUserID int64 `json:"blocked_user_id" binding:"required"`
}
