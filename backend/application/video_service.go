/*
 * video_service.go
 * 功能：视频应用服务（写操作 + 异步编排），职责退化为纯流程编排——所有"怎么下载、怎么分析、怎么生成、怎么上传"
 *      均通过注入的领域端口（VideoStorage/VideoAnalyzer/CoverGenerator/Workspace）委托 infra 实现，本层不再 import 任何技术栈；
 *      ConfirmUpload 通过 VideoStorage.CoverURL 把封面对外访问 URL 写入 Kafka，并改用 UpdateConfirmed 仅写 is_confirmed 列，
 *      避免与 video.basic 消费者写 cover_url 时发生 Save 全字段覆写竞态；同时在拼接 cover_url、投递 Kafka、回写状态等节点输出日志；
 *      新增并发改造：CreateVideo 中 raw/cover 两次 MinIO 预签名 URL 并发请求，ConfirmUpload 中 raw/cover 两次 HEAD 并发执行，
 *      通过 timing.Tracker 输出加速比报告
 *      2026-06-22 注入 SearchIndexer 端口，委托 ES 搜索与局部更新能力
 */

package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/elasticsearch"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/infra/media"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/infra/redis"
	"github.com/shermon/SesameSauce/infra/storage"
	"github.com/shermon/SesameSauce/infra/workspace"
	"github.com/shermon/SesameSauce/pkg/timing"
)

// VideoService 处理视频相关的应用层业务逻辑
// 通过端口字段持有 infrastructure 能力，本结构体不感知 ffmpeg、MinIO、本地文件系统等技术细节
type VideoService struct {
	videoDAO         *persistence.VideoDAO
	categoryDAO      *persistence.CategoryDAO
	videoCategoryDAO *persistence.VideoCategoryDAO
	userDAO          *persistence.UserDAO
	playHistoryDAO   *persistence.PlayHistoryDAO
	likeService      *LikeService

	videoStorage   domain.VideoStorage
	videoAnalyzer  domain.VideoAnalyzer
	coverGenerator domain.CoverGenerator
	workspace      domain.Workspace
	searchIndexer  domain.SearchIndexer
}

// NewVideoService 创建 VideoService 实例并装配所需端口实现
func NewVideoService() *VideoService {
	return &VideoService{
		videoDAO:         persistence.NewVideoDAO(),
		categoryDAO:      persistence.NewCategoryDAO(),
		videoCategoryDAO: persistence.NewVideoCategoryDAO(),
		userDAO:          persistence.NewUserDAO(),
		playHistoryDAO:   persistence.NewPlayHistoryDAO(),
		likeService:      NewLikeService(),

		videoStorage:   storage.NewMinIOVideoStorage(),
		videoAnalyzer:  media.NewFFprobeAnalyzer(),
		coverGenerator: media.NewFFmpegCoverGenerator(),
		workspace:      workspace.NewTempWorkspace(),
		searchIndexer:  elasticsearch.ESIndexer{},
	}
}

// CreateVideo 校验参数、插入记录、生成原始视频与封面的预签名上传 URL
func (s *VideoService) CreateVideo(authorID uint64, req CreateVideoRequest) (*CreateVideoResponse, error) {
	if len(req.Title) == 0 || len(req.Title) > MaxTitleLength {
		return nil, errors.New("标题长度必须在 1-128 个字符之间")
	}
	if len(req.Description) > MaxDescriptionLength {
		return nil, errors.New("描述最多 3000 个字符")
	}
	if len(req.CategoryIDs) > MaxCategoryCount {
		return nil, errors.New("分类数量最多 3 个")
	}

	if len(req.CategoryIDs) > 0 {
		cats, err := s.categoryDAO.GetByIDs(req.CategoryIDs)
		if err != nil {
			return nil, errors.New("分类查询失败")
		}
		if len(cats) != len(req.CategoryIDs) {
			return nil, errors.New("存在无效的分类 ID")
		}
	}

	maxConcurrency := MaxUploadConcurrency
	if config.ParamViper != nil {
		maxConcurrency = config.ParamViper.GetInt("upload.max_concurrency")
		if maxConcurrency <= 0 {
			maxConcurrency = MaxUploadConcurrency
		}
	}
	pendingCount, err := s.videoDAO.CountByAuthorAndStatus(authorID, domain.VideoStatusProcessing)
	if err != nil {
		return nil, errors.New("查询并发数失败")
	}
	if pendingCount >= int64(maxConcurrency) {
		return nil, errors.New("上传并发数超限")
	}

	// 创建时不显式设置 publish_time，交由数据库 DEFAULT CURRENT_TIMESTAMP 写入，
	// 避免 GORM 与 default 标签交互导致该列被写入零值。
	video := &domain.Video{
		AuthorID: authorID,
		Status:   domain.VideoStatusProcessing,
	}
	video.Title = &req.Title
	if req.Description != "" {
		desc := req.Description
		video.Description = &desc
	}

	if err := s.videoDAO.Create(video); err != nil {
		return nil, errors.New("视频记录创建失败")
	}

	// 生成 HashID 与 CoverHashID 并回写
	video.HashID = hashids.Encode(video.ID)
	video.CoverHashID = storage.GenerateCoverHashID()
	if err := s.videoDAO.Update(video); err != nil {
		return nil, errors.New("HashID 写入失败")
	}
	log.Printf("[CreateVideo] hashid 已生成 video=%d hashid=%s cover_hashid=%s", video.ID, video.HashID, video.CoverHashID)

	if err := s.videoCategoryDAO.BatchCreate(video.ID, req.CategoryIDs); err != nil {
		return nil, errors.New("分类关联失败")
	}

	expires := DefaultPresignedPutExpiresSeconds * time.Second

	// 从 filename 提取视频扩展名
	videoExt := "mp4"
	if req.Filename != "" {
		if ext := strings.TrimPrefix(filepath.Ext(req.Filename), "."); ext != "" {
			videoExt = ext
		}
	}
	coverExt := "jpg"
	if req.CoverExt != "" {
		coverExt = strings.TrimPrefix(req.CoverExt, ".")
	}

	// 并发：raw 与 cover 两次 MinIO 预签名 URL 调用相互独立，并行可让接口 RT 减半
	var (
		uploadURL, coverUploadURL string
		uploadErr, coverErr       error
		wg                        sync.WaitGroup
	)
	tracker := timing.NewTracker()

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("raw_presign", time.Now())
		uploadURL, uploadErr = s.videoStorage.PresignedRawUploadURL(video.ID, videoExt, expires)
	}()
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("cover_presign", time.Now())
		coverUploadURL, coverErr = s.videoStorage.PresignedCoverUploadURL(video.CoverHashID, coverExt, expires)
	}()
	wg.Wait()

	tracker.Report(fmt.Sprintf("CreateVideo(video=%d)", video.ID))

	if uploadErr != nil {
		return nil, errors.New("生成上传地址失败")
	}
	if coverErr != nil {
		return nil, errors.New("生成封面上传地址失败")
	}

	resp := &CreateVideoResponse{
		VideoID:        video.HashID,
		VideoUploadURL: uploadURL,
		Expires:        DefaultPresignedPutExpiresSeconds,
		CoverUploadURL: coverUploadURL,
	}

	return resp, nil
}

// ConfirmUpload 校验所有权与对象存储中文件是否到位，向 Kafka 投递两 Topic 消息
func (s *VideoService) ConfirmUpload(authorID uint64, hashid string, req ConfirmUploadRequest) (*ConfirmUploadResponse, error) {
	videoID, err := hashids.Decode(hashid)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return nil, errors.New("视频不存在")
	}
	if err := domain.CheckVideoOwnership(video, authorID); err != nil {
		return nil, err
	}

	// 幂等：status 不为处理中时直接返回当前状态
	if video.Status != domain.VideoStatusProcessing {
		return &ConfirmUploadResponse{VideoID: video.HashID, Status: video.Status}, nil
	}

	// 已 confirm 则幂等返回
	if video.IsConfirmed == 1 {
		return &ConfirmUploadResponse{VideoID: video.HashID, Status: video.Status}, nil
	}

	videoExt := "mp4"
	coverExt := "jpg"
	if req.CoverExt != "" {
		coverExt = strings.TrimPrefix(req.CoverExt, ".")
	}

	// 并发：raw 与 cover 两次 MinIO HEAD 调用相互独立，并行可让 ConfirmUpload 校验阶段 RT 减半
	var (
		rawErr, coverHeadErr error
		headWG               sync.WaitGroup
	)
	headTracker := timing.NewTracker()
	headCtx := context.Background()

	headWG.Add(2)
	go func() {
		defer headWG.Done()
		defer headTracker.RecordSince("raw_head", time.Now())
		rawErr = s.videoStorage.RawExists(headCtx, videoID, videoExt)
	}()
	go func() {
		defer headWG.Done()
		defer headTracker.RecordSince("cover_head", time.Now())
		coverHeadErr = s.videoStorage.CoverExists(headCtx, video.CoverHashID, coverExt)
	}()
	headWG.Wait()

	headTracker.Report(fmt.Sprintf("ConfirmUpload.HEAD(video=%d)", videoID))

	if rawErr != nil {
		return nil, errors.New("视频文件未找到，请完成上传")
	}
	if coverHeadErr != nil {
		return nil, errors.New("封面文件未找到，请完成上传")
	}

	basicEvent := messaging.VideoBasicEvent{
		VideoID:     videoID,
		HashID:      video.HashID,
		CoverHashID: video.CoverHashID,
		Title:       "",
		Description: "",
		AuthorID:    authorID,
		CategoryIDs: []uint64{},
		CoverURL:    storage.CoverKey(video.CoverHashID, coverExt),
		CreatedAt:   time.Now(),
	}
	log.Printf("[ConfirmUpload] 拼接 cover_url video=%d hashid=%s cover_hashid=%s coverExt=%s cover_url=%s", videoID, video.HashID, video.CoverHashID, coverExt, basicEvent.CoverURL)

	if video.Title != nil {
		basicEvent.Title = *video.Title
	}
	if video.Description != nil {
		basicEvent.Description = *video.Description
	}
	// 从 DB 查询分类 IDs
	catIDs, err := s.videoCategoryDAO.GetCategoryIDsByVideoID(videoID)
	if err == nil {
		basicEvent.CategoryIDs = catIDs
	}

	complexEvent := messaging.VideoComplexEvent{
		VideoID:      videoID,
		HashID:       video.HashID,
		RawVideoURL:  rawKeyURL(videoID, videoExt),
		SourceFormat: videoExt,
	}

	log.Printf("[ConfirmUpload] 投递 video.basic video=%d hashid=%s cover_hashid=%s title=%q cover_url=%s", videoID, video.HashID, video.CoverHashID, basicEvent.Title, basicEvent.CoverURL)
	if err := messaging.SendVideoBasic(basicEvent); err != nil {
		log.Printf("[ConfirmUpload] video.basic 投递失败 video=%d err=%v", videoID, err)
		return nil, errors.New("投递基础信息消息失败，请重试")
	}
	log.Printf("[ConfirmUpload] video.basic 投递成功 video=%d", videoID)

	if err := messaging.SendVideoComplex(complexEvent); err != nil {
		log.Printf("[ConfirmUpload] video.complex 投递失败 video=%d err=%v", videoID, err)
		return nil, errors.New("投递转码消息失败，请重试")
	}

	// 仅更新 is_confirmed 列，避免 Save 全字段把后续消费者刚写入的 cover_url 覆盖回空串
	if err := s.videoDAO.UpdateConfirmed(videoID, 1); err != nil {
		log.Printf("[ConfirmUpload] 更新 is_confirmed 失败 video=%d err=%v", videoID, err)
		return nil, errors.New("更新确认状态失败")
	}
	log.Printf("[ConfirmUpload] is_confirmed 已置 1 video=%d", videoID)

	return &ConfirmUploadResponse{
		VideoID: video.HashID,
		Status:  domain.VideoStatusProcessing,
	}, nil
}

// CancelUpload 取消视频处理
func (s *VideoService) CancelUpload(authorID uint64, hashid string) (*CancelUploadResponse, error) {
	videoID, err := hashids.Decode(hashid)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return nil, errors.New("视频不存在")
	}
	if err := domain.CheckVideoOwnership(video, authorID); err != nil {
		return nil, err
	}

	// 幂等：非处理中状态直接返回当前状态
	if video.Status != domain.VideoStatusProcessing {
		return &CancelUploadResponse{VideoID: video.HashID, Status: video.Status}, nil
	}

	if err := video.MarkCancelled(); err != nil {
		return nil, err
	}
	if err := s.videoDAO.UpdateStatus(videoID, domain.VideoStatusCancelled); err != nil {
		return nil, errors.New("取消处理失败")
	}

	// 状态变更后失效视频详情缓存
	if err := cache.DelVideoDetail(videoID); err != nil {
		log.Printf("[CancelUpload] 删除视频缓存失败 video=%d: %v", videoID, err)
	}

	return &CancelUploadResponse{
		VideoID: video.HashID,
		Status:  domain.VideoStatusCancelled,
	}, nil
}

// GetVideoStatus 查询视频处理状态
func (s *VideoService) GetVideoStatus(hashid string) (*VideoStatusResponse, error) {
	videoID, err := hashids.Decode(hashid)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return nil, errors.New("视频不存在")
	}

	statusText := "未知"
	switch video.Status {
	case domain.VideoStatusPublished:
		statusText = "已发布"
	case domain.VideoStatusProcessing:
		statusText = "处理中"
	case domain.VideoStatusFailed:
		statusText = "处理失败"
	case domain.VideoStatusCancelled:
		statusText = "已取消"
	}

	resp := &VideoStatusResponse{
		VideoID:    video.HashID,
		Status:     video.Status,
		StatusText: statusText,
	}
	if video.Status == domain.VideoStatusProcessing {
		resp.Progress = video.Progress
	}
	return resp, nil
}

// ReportProgress 校验参数并将进度写入 Redis 缓存
// Redis 写入失败时静默处理，不阻塞播放流程
func (s *VideoService) ReportProgress(userID, videoID uint64, progress int) error {
	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return errors.New("视频不存在")
	}
	if err := domain.CheckVisibility(video.Status); err != nil {
		return err
	}

	if progress < 0 {
		progress = 0
	}
	if video.Duration > 0 && progress > video.Duration {
		progress = video.Duration
	}

	if err := redis.SetProgress(userID, videoID, progress); err != nil {
		log.Printf("[ReportProgress] Redis 写入失败 user=%d video=%d: %v", userID, videoID, err)
		// 容错：Redis 不可用时不阻塞播放流程，进度丢失可接受
	}
	return nil
}

// rawKeyURL 拼接原始视频对象访问 URL（供 consumer 下载使用）
func rawKeyURL(videoID uint64, ext string) string {
	return fmt.Sprintf("videos/%d/raw.%s", videoID, ext)
}
