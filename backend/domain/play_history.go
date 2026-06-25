/*
 * play_history.go
 * 功能：定义播放历史实体，对应 play_histories 表
 * 时间戳：2026-04-22
 */

package domain

import "time"

// PlayHistory 记录用户观看视频的进度，用于“继续观看”
type PlayHistory struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"column:user_id;not null;uniqueIndex:uk_user_video" json:"user_id"`
	VideoID   uint64    `gorm:"column:video_id;not null;uniqueIndex:uk_user_video" json:"video_id"`
	Progress  int       `gorm:"column:progress;not null;default:0" json:"progress"`
	PlayedAt  time.Time `gorm:"column:played_at;not null;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"played_at"`
}
