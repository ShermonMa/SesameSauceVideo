/*
 * system_notification.go
 * 功能：系统通知领域实体，对应 system_notifications 表
 * 时间戳：2026-04-26
 */

package domain

import (
	"time"
)

// 系统通知目标类型常量
const (
	TargetTypeAll  int8 = 1 // 全体用户
	TargetTypeSome int8 = 2 // 指定用户
)

// SystemNotification 系统通知实体
type SystemNotification struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Title         string    `gorm:"column:title;size:128;not null" json:"title"`
	Content       string    `gorm:"column:content;type:text;not null" json:"content"`
	TargetType    int8      `gorm:"column:target_type;not null;default:1" json:"target_type"`
	TargetUserIDs *string   `gorm:"column:target_user_ids;type:json" json:"target_user_ids,omitempty"`
	PublishTime   time.Time `gorm:"column:publish_time;not null" json:"publish_time"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (SystemNotification) TableName() string {
	return "system_notifications"
}
