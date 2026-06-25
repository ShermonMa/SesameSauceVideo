/*
 * category_dao.go
 * 功能：分类数据访问对象，封装 categories 表操作
 * 时间戳：2026-04-22
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
)

// CategoryDAO 提供分类表的数据库操作
type CategoryDAO struct{}

// NewCategoryDAO 创建 CategoryDAO 实例
func NewCategoryDAO() *CategoryDAO {
	return &CategoryDAO{}
}

// Create 创建单个分类
func (dao *CategoryDAO) Create(category *domain.Category) error {
	return config.DB.Create(category).Error
}

// GetByIDs 根据 ID 列表查询分类
func (dao *CategoryDAO) GetByIDs(ids []uint64) ([]domain.Category, error) {
	var categories []domain.Category
	err := config.DB.Where("id IN ?", ids).Find(&categories).Error
	return categories, err
}

// ListAll 查询所有分类，按 sort_order 降序排列
func (dao *CategoryDAO) ListAll() ([]domain.Category, error) {
	var categories []domain.Category
	err := config.DB.Order("sort_order DESC").Find(&categories).Error
	return categories, err
}

// GetNamesByVideoID 根据视频 ID 查询关联的分类名称列表
func (dao *CategoryDAO) GetNamesByVideoID(videoID uint64) ([]string, error) {
	var names []string
	err := config.DB.Model(&domain.Category{}).
		Select("categories.name").
		Joins("JOIN video_categories ON video_categories.category_id = categories.id").
		Where("video_categories.video_id = ?", videoID).
		Pluck("categories.name", &names).Error
	return names, err
}
