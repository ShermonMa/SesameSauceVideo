/*
 * ports.go
 * 功能：领域端口契约，声明视频处理流程依赖的对外能力（对象存储、视频分析、封面生成、临时工作空间、视频转码、搜索索引）；
 *      新增 CoverURL 端口，供确认上传后回写 MySQL cover_url 使用
 * 时间戳：2026-06-22 扩展 SearchIndexer 接口与 VideoSearchDoc，支持搜索查询与文档局部更新
 */

package domain

import (
	"context"
	"time"
)

// VideoStorage 业务语义化的对象存储端口
// 屏蔽 bucket、key、endpoint 等存储方案细节，调用方只通过 videoID 操作业务对象
type VideoStorage interface {
	// PresignedRawUploadURL 为指定视频生成原始视频的预签名上传 URL（私有桶，使用数字 ID）
	PresignedRawUploadURL(videoID uint64, ext string, expires time.Duration) (string, error)

	// PresignedCoverUploadURL 为指定视频生成封面的预签名上传 URL（公共桶，使用 cover_hashid）
	PresignedCoverUploadURL(coverHashID string, ext string, expires time.Duration) (string, error)

	// RawExists 检查指定视频的原始文件是否已上传到对象存储（私有桶，使用数字 ID）
	RawExists(ctx context.Context, videoID uint64, ext string) error

	// CoverExists 检查指定视频的封面是否已上传到对象存储（公共桶，使用 cover_hashid）
	CoverExists(ctx context.Context, coverHashID string, ext string) error

	// DownloadRaw 将指定视频的原始文件下载到本地路径（私有桶，使用数字 ID）
	DownloadRaw(ctx context.Context, videoID uint64, ext string, localPath string) error

	// UploadM3U8Dir 将本地转码后的 m3u8 目录上传到对象存储（公共桶，使用 video_hashid）
	UploadM3U8Dir(ctx context.Context, videoHashID string, localDir string) error

	// UploadCover 将本地封面文件上传，返回可对外访问的封面 URL（公共桶，使用 cover_hashid）
	UploadCover(ctx context.Context, coverHashID string, localPath string) (string, error)

	// CoverURL 根据 cover_hashid 与扩展名返回封面对外访问 URL，不上传文件，仅做 URL 拼接
	CoverURL(coverHashID string, ext string) string

	// MasterPlayURL 返回 master.m3u8 的播放访问 URL（公共桶，使用 video_hashid）
	MasterPlayURL(videoHashID string) string

	// DeleteRaw 删除原始视频文件（私有桶，使用数字 ID）
	DeleteRaw(ctx context.Context, videoID uint64) error

	// DeleteCover 删除封面文件（公共桶，使用 cover_hashid）
	DeleteCover(ctx context.Context, coverHashID string) error
}

// VideoAnalyzer 视频元数据分析端口，封装 ffprobe 与文件系统探测等技术细节
type VideoAnalyzer interface {
	// Analyze 探测本地视频文件，返回时长（秒）、宽、高、文件大小（字节）
	Analyze(localPath string) (duration int, width int, height int, fileSize int64, err error)
}

// CoverGenerator 封面生成端口，封装 ffmpeg 截帧等技术细节
type CoverGenerator interface {
	// Generate 从源视频生成封面图片到目标路径
	Generate(srcVideoPath, dstCoverPath string) error
}

// Workspace 临时工作空间端口，封装临时目录的申请与释放
type Workspace interface {
	// Acquire 申请一个临时工作目录，返回目录路径与释放函数；调用方应在 defer 中执行 release
	Acquire(prefix string) (dir string, release func(), err error)
}

// VideoTranscoder 视频转码端口，封装 ffmpeg m3u8 切片与多码率生成
type VideoTranscoder interface {
	// TranscodeToM3U8 将本地源视频转码为 m3u8 多码率输出到目标目录，返回时长、宽、高、文件大小
	TranscodeToM3U8(ctx context.Context, srcPath, dstDir string, videoID uint64) (duration, width, height int, fileSize int64, err error)
}

// SearchIndexer 搜索索引端口，封装 Elasticsearch 文档增删查改
type SearchIndexer interface {
	// IndexVideo 将视频文档索引到搜索引擎
	IndexVideo(ctx context.Context, doc VideoSearchDoc) error

	// DeleteVideo 从搜索引擎删除视频文档
	DeleteVideo(ctx context.Context, videoID string) error

	// SearchVideos 根据关键词与分类搜索视频，返回匹配的视频 ID 列表
	SearchVideos(ctx context.Context, req VideoSearchRequest) (*VideoSearchResult, error)

	// UpdateVideoFields 局部更新 ES 文档指定字段（如 duration、play_count）
	UpdateVideoFields(ctx context.Context, videoID string, fields map[string]interface{}) error
}

// VideoSearchDoc 视频搜索索引文档值对象
type VideoSearchDoc struct {
	VideoID       string    `json:"video_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CategoryIDs   []string  `json:"category_ids"`
	CategoryNames []string  `json:"category_names"`
	AuthorID      string    `json:"author_id"`
	Duration      int       `json:"duration"`
	PlayCount     int64     `json:"play_count"`
	PublishTime   time.Time `json:"publish_time"`
	CreatedAt     time.Time `json:"created_at"`
}

// VideoSearchRequest ES 搜索请求参数
type VideoSearchRequest struct {
	Keyword     string   // 搜索关键词
	CategoryIDs []string // hashed 分类 ID 列表（空=不过滤）
	Page        int      // 页码，从 1 开始
	PageSize    int      // 每页条数
}

// VideoSearchResult ES 搜索结果
type VideoSearchResult struct {
	VideoIDs []string // 匹配的视频 hashid 列表
	Total    int64    // 匹配总数
}
