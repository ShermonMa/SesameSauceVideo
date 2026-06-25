/*
 * video_dto.go
 * 功能：视频应用服务相关的请求、响应及 DTO 类型定义；适配 Hashids 字符串 video_id 与 v2 需求
 * 时间戳：2026-05-01
 */

package application

import "time"

// CreateVideoRequest 定义创建视频请求参数
type CreateVideoRequest struct {
	Title       string   `json:"title" binding:"required"`
	Description string   `json:"description"`
	CategoryIDs []uint64 `json:"category_ids"`
	Filename    string   `json:"filename"`
	CoverExt    string   `json:"cover_ext,omitempty"`
}

// CreateVideoResponse 定义创建视频成功后的响应数据
type CreateVideoResponse struct {
	VideoID        string `json:"video_id"`
	VideoUploadURL string `json:"video_upload_url"`
	CoverUploadURL string `json:"cover_upload_url"`
	Expires        int    `json:"expires"`
}

// ConfirmUploadRequest 定义确认上传请求参数
type ConfirmUploadRequest struct {
	CoverExt string `json:"cover_ext,omitempty"`
}

// ConfirmUploadResponse 定义确认上传成功后的响应数据
type ConfirmUploadResponse struct {
	VideoID string `json:"video_id"`
	Status  int8   `json:"status"`
}

// CancelUploadResponse 定义取消上传成功后的响应数据
type CancelUploadResponse struct {
	VideoID string `json:"video_id"`
	Status  int8   `json:"status"`
}

// VideoStatusResponse 定义视频处理状态查询响应
type VideoStatusResponse struct {
	VideoID    string `json:"video_id"`
	Status     int8   `json:"status"`
	StatusText string `json:"status_text"`
	Progress   int8   `json:"progress,omitempty"`
}

// PlayURLResponse 定义播放地址响应数据
type PlayURLResponse struct {
	PlayURL string `json:"play_url"`
	Expires int    `json:"expires"`
}

// ReportProgressRequest 定义上报播放进度请求参数；不使用 required，因 progress=0 是合法值（视频从头开始）
type ReportProgressRequest struct {
	Progress int `json:"progress"`
}

// AuthorDTO 定义作者信息 DTO
type AuthorDTO struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	FollowCount   int64  `json:"follow_count"`
	FollowerCount int64  `json:"follower_count"`
}

// VideoDetailResponse 定义视频详情响应数据
type VideoDetailResponse struct {
	ID            string    `json:"id"`
	Author        AuthorDTO `json:"author"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CoverURL      string    `json:"cover_url"`
	Duration      int       `json:"duration"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	FavoriteCount int64     `json:"favorite_count"`
	CommentCount  int64     `json:"comment_count"`
	PlayCount     int64     `json:"play_count"`
	PublishTime   time.Time `json:"publish_time"`
	Categories    []string  `json:"categories"`
	Progress      int       `json:"progress"`
	IsLiked       bool      `json:"is_liked"`
}

// CategoryDTO 定义分类列表项 DTO
type CategoryDTO struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

// AuthorListItemDTO 列表场景下的作者精简信息
type AuthorListItemDTO struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// VideoListItemDTO 列表项 DTO（精简版，不含描述、宽高等详情字段）
type VideoListItemDTO struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	CoverURL      string            `json:"cover_url"`
	Duration      int               `json:"duration"`
	PlayCount     int64             `json:"play_count"`
	FavoriteCount int64             `json:"favorite_count"`
	PublishTime   time.Time         `json:"publish_time"`
	Author        AuthorListItemDTO `json:"author"`
}

// ListVideosRequest 列表请求参数
type ListVideosRequest struct {
	Page       int
	PageSize   int
	CategoryID uint64
	Keyword    string
	OrderBy    string
}

// ListVideosResponse 列表响应
type ListVideosResponse struct {
	List     []VideoListItemDTO `json:"list"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

// MyVideoListItemDTO 我的视频列表项，关键字段是 Status 让前端区分"处理中/已发布/失败/已取消"
type MyVideoListItemDTO struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	CoverURL      string    `json:"cover_url"`
	Duration      int       `json:"duration"`
	Status        int8      `json:"status"`
	Progress      int8      `json:"progress,omitempty"`
	PlayCount     int64     `json:"play_count"`
	FavoriteCount int64     `json:"favorite_count"`
	CommentCount  int64     `json:"comment_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// MyVideosResponse 我的视频列表分页响应
type MyVideosResponse struct {
	List     []MyVideoListItemDTO `json:"list"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Total    int64                `json:"total"`
}
