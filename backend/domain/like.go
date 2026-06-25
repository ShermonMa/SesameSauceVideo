/*
 * like.go
 * 功能：点赞领域实体，对应 likes 表；支持视频点赞与评论点赞
 * 时间戳：2026-04-26
 */

package domain

import (
	"time"
)

// 点赞业务类型常量
const (
	BizTypeVideo   int8 = 1
	BizTypeComment int8 = 2
)

// Like 点赞领域实体
type Like struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"column:user_id;not null" json:"user_id"`
	TargetID  uint64    `gorm:"column:target_id;not null" json:"target_id"`
	BizType   int8      `gorm:"column:biz_type;not null;default:1" json:"biz_type"`
	VideoID   uint64    `gorm:"column:video_id;not null" json:"video_id"` // 旧字段保留，视频点赞时=TargetID，评论点赞时=0
	Cancel    int8      `gorm:"column:cancel;not null;default:0" json:"cancel"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名
func (Like) TableName() string {
	return "likes"
}

// IsActive 判断当前点赞是否有效
func (l *Like) IsActive() bool {
	return l.Cancel == 0
}
