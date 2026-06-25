/*
 * follow.go
 * 功能：关注领域实体，对应 follows 表
 * 时间戳：2026-05-22
 */

package domain

import "time"

// Follow 关注领域实体
type Follow struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     uint64    `gorm:"column:user_id;not null" json:"user_id"`
	FollowerID uint64    `gorm:"column:follower_id;not null" json:"follower_id"`
	Cancel     int8      `gorm:"column:cancel;not null;default:0" json:"cancel"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (Follow) TableName() string {
	return "follows"
}

// IsActive 判断当前关注是否有效
func (f *Follow) IsActive() bool {
	return f.Cancel == 0
}
