/*
 * category.go
 * 功能：视频分类领域实体，对应 categories 表
 * 时间戳：2026-04-22
 */

package domain

import "time"

// Category 定义视频分类结构
type Category struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;size:64;not null;uniqueIndex:uk_name" json:"name"`
	Description *string   `gorm:"column:description;size:255" json:"description,omitempty"`
	SortOrder   int       `gorm:"column:sort_order;not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

// TableName 指定表名
func (Category) TableName() string {
	return "categories"
}
