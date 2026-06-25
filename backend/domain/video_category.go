/*
 * video_category.go
 * 功能：视频-分类关联实体，对应 video_categories 表
 * 时间戳：2026-04-22
 */

package domain

// VideoCategory 定义视频与分类的多对多关联
type VideoCategory struct {
	ID         uint64 `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	VideoID    uint64 `gorm:"column:video_id;not null" json:"video_id"`
	CategoryID uint64 `gorm:"column:category_id;not null" json:"category_id"`
}

// TableName 指定表名
func (VideoCategory) TableName() string {
	return "video_categories"
}
