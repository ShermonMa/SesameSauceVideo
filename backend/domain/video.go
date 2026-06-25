/*
 * video.go
 * 功能：视频领域实体，对应 videos 表；扩充 Status 状态机语义（0/1/2/3），定义 PreparePublish/MarkFailed/MarkCancelled 状态机方法
 *      与 VideoMetadata 值对象；CoverURL 由 video.basic 消费者负责写入，PreparePublish 不再触碰，避免与封面写入产生覆写竞态
 * 时间戳：2026-05-01
 */

package domain

import (
	"errors"
	"time"
)

// 视频状态机：0=已发布，1=处理中，2=处理失败，3=已取消
const (
	VideoStatusPublished  int8 = 0
	VideoStatusProcessing int8 = 1
	VideoStatusFailed     int8 = 2
	VideoStatusCancelled  int8 = 3
)

// Video 定义视频元数据结构
type Video struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	HashID        string     `gorm:"column:hashid;size:32" json:"hashid"`
	CoverHashID   string     `gorm:"column:cover_hashid;size:32;not null;default:''" json:"cover_hashid"`
	AuthorID      uint64     `gorm:"column:author_id;not null" json:"author_id"`
	PlayURL       string     `gorm:"column:play_url;size:512;not null;default:''" json:"play_url"` // 存储对象键（相对路径），非完整URL
	CoverURL      string     `gorm:"column:cover_url;size:512;not null;default:''" json:"cover_url"` // 存储对象键（相对路径），非完整URL
	Title         *string    `gorm:"column:title;size:128" json:"title,omitempty"`
	Description   *string    `gorm:"column:description" json:"description,omitempty"`
	Duration      int        `gorm:"column:duration;not null;default:0" json:"duration"`
	Width         *int       `gorm:"column:width" json:"width,omitempty"`
	Height        *int       `gorm:"column:height" json:"height,omitempty"`
	FileSize      *int64     `gorm:"column:file_size" json:"file_size,omitempty"`
	Progress      int8       `gorm:"column:progress;not null;default:0" json:"progress"`
	IsConfirmed   int8       `gorm:"column:is_confirmed;not null;default:0" json:"is_confirmed"`
	FavoriteCount int64      `gorm:"column:favorite_count;not null;default:0" json:"favorite_count"`
	CommentCount  int64      `gorm:"column:comment_count;not null;default:0" json:"comment_count"`
	PlayCount     int64      `gorm:"column:play_count;not null;default:0" json:"play_count"`
	Status        int8       `gorm:"column:status;not null;default:1" json:"status"` // 见上方 VideoStatus* 常量
	PublishTime   time.Time  `gorm:"column:publish_time;not null;default:CURRENT_TIMESTAMP" json:"publish_time"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at" json:"-"`
}

// TableName 指定表名
func (Video) TableName() string {
	return "videos"
}

// VideoMetadata 视频处理完成后用于推进发布状态的元数据值对象；不含 cover_url，封面由 video.basic 单独写入
type VideoMetadata struct {
	Duration int
	Width    int
	Height   int
	FileSize int64
	PlayURL  string
}

// PreparePublish 推进视频从"处理中"到"已发布"，校验状态合法性后填充元数据
// 仅允许当前状态为 VideoStatusProcessing 时调用，避免重复发布或越权状态变更；
// 不修改 CoverURL，避免覆盖 video.basic 消费者写入的封面 URL
func (v *Video) PreparePublish(meta VideoMetadata) error {
	if v.Status != VideoStatusProcessing {
		return errors.New("视频状态不允许发布")
	}
	width := meta.Width
	height := meta.Height
	fileSize := meta.FileSize
	v.Duration = meta.Duration
	v.Width = &width
	v.Height = &height
	v.FileSize = &fileSize
	v.PlayURL = meta.PlayURL
	v.Status = VideoStatusPublished
	return nil
}

// MarkFailed 将视频状态推进到"处理失败"，幂等，无状态校验，便于异步任务任意阶段兜底
func (v *Video) MarkFailed() {
	v.Status = VideoStatusFailed
}

// MarkCancelled 将视频状态推进到"已取消"，仅允许从"处理中"转换
func (v *Video) MarkCancelled() error {
	if v.Status != VideoStatusProcessing {
		return errors.New("仅处理中的视频可取消")
	}
	v.Status = VideoStatusCancelled
	return nil
}

// UpdateProgress 更新转码进度，仅处理中状态允许更新
func (v *Video) UpdateProgress(p int8) error {
	if v.Status != VideoStatusProcessing {
		return errors.New("仅处理中的视频可更新进度")
	}
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	v.Progress = p
	return nil
}
