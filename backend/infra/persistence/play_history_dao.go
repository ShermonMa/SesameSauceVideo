/*
 * play_history_dao.go
 * 功能：播放历史数据访问对象，封装 play_histories 表操作
 * 时间戳：2026-04-22
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm/clause"
)

// PlayHistoryDAO 提供播放历史表的数据库操作
type PlayHistoryDAO struct{}

// NewPlayHistoryDAO 创建 PlayHistoryDAO 实例
func NewPlayHistoryDAO() *PlayHistoryDAO {
	return &PlayHistoryDAO{}
}

// Upsert 插入或更新播放进度（存在则更新 progress，不存在则插入）
func (dao *PlayHistoryDAO) Upsert(userID, videoID uint64, progress int) error {
	h := domain.PlayHistory{
		UserID:   userID,
		VideoID:  videoID,
		Progress: progress,
	}
	return config.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"progress"}),
	}).Create(&h).Error
}

// GetByUserAndVideo 查询指定用户与视频的播放记录
func (dao *PlayHistoryDAO) GetByUserAndVideo(userID, videoID uint64) (*domain.PlayHistory, error) {
	var h domain.PlayHistory
	err := config.DB.Where("user_id = ? AND video_id = ?", userID, videoID).First(&h).Error
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// BatchUpsert 批量插入或更新播放进度
// 使用 INSERT ... ON DUPLICATE KEY UPDATE，更新 progress 与 played_at
func (dao *PlayHistoryDAO) BatchUpsert(records []domain.PlayHistory) error {
	if len(records) == 0 {
		return nil
	}
	return config.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"progress", "played_at"}),
	}).Create(&records).Error
}
