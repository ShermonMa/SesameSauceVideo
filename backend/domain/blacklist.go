/*
 * blacklist.go
 * 功能：用户拉黑关系领域实体，对应 blacklists 表
 * 时间戳：2026-04-26
 */

package domain

import (
	"time"
)

// Blacklist 用户拉黑关系实体
type Blacklist struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID        uint64    `gorm:"column:user_id;not null" json:"user_id"`
	BlockedUserID uint64    `gorm:"column:blocked_user_id;not null" json:"blocked_user_id"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (Blacklist) TableName() string {
	return "blacklists"
}
