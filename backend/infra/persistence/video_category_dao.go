/*
 * video_category_dao.go
 * 功能：视频-分类关联数据访问对象
 * 时间戳：2026-04-22
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
)

// VideoCategoryDAO 提供 video_categories 表的数据库操作
type VideoCategoryDAO struct{}

// NewVideoCategoryDAO 创建 VideoCategoryDAO 实例
func NewVideoCategoryDAO() *VideoCategoryDAO {
	return &VideoCategoryDAO{}
}

// BatchCreate 批量创建视频与分类的关联记录
func (dao *VideoCategoryDAO) BatchCreate(videoID uint64, categoryIDs []uint64) error {
	if len(categoryIDs) == 0 {
		return nil
	}
	records := make([]domain.VideoCategory, 0, len(categoryIDs))
	for _, cid := range categoryIDs {
		records = append(records, domain.VideoCategory{
			VideoID:    videoID,
			CategoryID: cid,
		})
	}
	return config.DB.Create(&records).Error
}

// GetCategoryIDsByVideoID 查询视频关联的分类 ID 列表
func (dao *VideoCategoryDAO) GetCategoryIDsByVideoID(videoID uint64) ([]uint64, error) {
	var list []domain.VideoCategory
	if err := config.DB.Where("video_id = ?", videoID).Find(&list).Error; err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(list))
	for _, v := range list {
		ids = append(ids, v.CategoryID)
	}
	return ids, nil
}

// DeleteByVideoID 删除视频的所有分类关联（用于幂等重入）
func (dao *VideoCategoryDAO) DeleteByVideoID(videoID uint64) error {
	return config.DB.Where("video_id = ?", videoID).Delete(&domain.VideoCategory{}).Error
}
