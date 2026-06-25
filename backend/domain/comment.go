/*
 * comment.go
 * 功能：评论领域实体，对应 comments 表；支持楼中楼（二级评论）
 * 时间戳：2026-04-26
 */

package domain

import (
	"errors"
	"time"
)

// MaxCommentLength 评论内容最大字符数
const MaxCommentLength = 512

// ErrCommentTooDeep 评论层级超限错误
var ErrCommentTooDeep = errors.New("评论层级超限，仅支持二级评论")

// ErrEmptyCommentContent 评论内容为空错误
var ErrEmptyCommentContent = errors.New("评论内容不能为空")

// ErrCommentTooLong 评论内容过长错误
var ErrCommentTooLong = errors.New("评论内容超过最大长度限制")

// Comment 评论领域实体
type Comment struct {
	ID         uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     uint64     `gorm:"column:user_id;not null" json:"user_id"`
	VideoID    uint64     `gorm:"column:video_id;not null" json:"video_id"`
	Content    string     `gorm:"column:content;size:512;not null" json:"content"`
	LikeCount  int64      `gorm:"column:like_count;not null;default:0" json:"like_count"`
	ParentID   *uint64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
	RootID     *uint64    `gorm:"column:root_id" json:"root_id,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt  *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName 指定表名
func (Comment) TableName() string {
	return "comments"
}

// IsRootComment 判断是否为一级评论（parent_id IS NULL）
func (c *Comment) IsRootComment() bool {
	return c.ParentID == nil
}

// IsDeleted 判断评论是否已软删除
func (c *Comment) IsDeleted() bool {
	return c.DeletedAt != nil
}

// ValidateContent 校验评论内容合法性
func ValidateContent(content string) error {
	if len(content) == 0 {
		return ErrEmptyCommentContent
	}
	if len(content) > MaxCommentLength {
		return ErrCommentTooLong
	}
	return nil
}
