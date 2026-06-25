/*
 * message.go
 * 功能：消息通知领域实体，对应 messages 表；定义六 Tab 常量
 * 时间戳：2026-04-26
 */

package domain

import (
	"time"
)

// 消息 action_type 常量
const (
	ActionTypePrivate int8 = 1 // 私信
	ActionTypeFollow  int8 = 2 // 关注通知
	ActionTypeLike    int8 = 3 // 点赞通知
	ActionTypeComment int8 = 4 // 评论通知
	ActionTypeReply   int8 = 5 // 回复通知
	ActionTypeMention int8 = 6 // @通知（预留）
	ActionTypeSystem  int8 = 7 // 系统通知
)

// Message 消息通知领域实体
type Message struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FromUserID uint64    `gorm:"column:from_user_id;not null" json:"from_user_id"`
	ToUserID   uint64    `gorm:"column:to_user_id;not null" json:"to_user_id"`
	Content    string    `gorm:"column:content;size:512;not null" json:"content"`
	ActionType int8      `gorm:"column:action_type;not null;default:1" json:"action_type"`
	IsRead     int8      `gorm:"column:is_read;not null;default:0" json:"is_read"`
	BizID      *uint64   `gorm:"column:biz_id" json:"biz_id,omitempty"`
	BizType    *int8     `gorm:"column:biz_type" json:"biz_type,omitempty"`
	IsDeleted  int8      `gorm:"column:is_deleted;not null;default:0" json:"is_deleted"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (Message) TableName() string {
	return "messages"
}
