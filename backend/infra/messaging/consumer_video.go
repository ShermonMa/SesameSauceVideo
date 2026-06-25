/*
 * consumer_video.go
 * 功能：视频相关 Kafka 消费者实现，包含 video_basic（基础信息入库+ES索引）与 video_complex（视频转码）；
 *      processVideoBasic 改用 VideoDAO.UpdateBasicInfo 仅更新 title/description/cover_url 列，避免 Save 全字段与 ConfirmUpload 写竞态；
 *      新增并发改造：processVideoBasic 中 DB 写入与 ES 索引并发执行，通过 timing.Tracker 输出加速比报告，
 *      可缩短消费者单条消息处理时间，提升消费吞吐
 *      2026-06-21 转码完成后删除 MinIO 原始视频文件，释放存储空间
 *      2026-06-22 processVideoBasic 索引增加 Description/PublishTime；processVideoComplex 发布后更新 ES duration/publish_time
 */

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/elasticsearch"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/infra/media"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/infra/storage"
	"github.com/shermon/SesameSauce/infra/workspace"
	"github.com/shermon/SesameSauce/pkg/timing"
)

// StartVideoBasicConsumer 启动 video_basic Topic 消费者
func StartVideoBasicConsumer() {
	go runVideoBasicConsumer()
}

// StartVideoComplexConsumer 启动 video_complex Topic 消费者
func StartVideoComplexConsumer() {
	go runVideoComplexConsumer()
}

func runVideoBasicConsumer() {
	reader := config.NewKafkaReader(TopicVideoBasic, "sesame-sauce-video-basic-consumer")
	defer reader.Close()

	videoDAO := persistence.NewVideoDAO()
	videoCategoryDAO := persistence.NewVideoCategoryDAO()
	categoryDAO := persistence.NewCategoryDAO()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				log.Printf("[kafka-consumer-video-basic] 读取消息失败(网络错误): temporary=%v timeout=%v err=%v", netErr.Temporary(), netErr.Timeout(), err)
			} else {
				log.Printf("[kafka-consumer-video-basic] 读取消息失败: 类型=%T err=%v", err, err)
			}
			time.Sleep(time.Second)
			continue
		}

		var event VideoBasicEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("[kafka-consumer-video-basic] 消息反序列化失败: %v", err)
			continue
		}

		if err := processVideoBasic(videoDAO, videoCategoryDAO, categoryDAO, event); err != nil {
			log.Printf("[kafka-consumer-video-basic] 处理失败 video=%d: %v", event.VideoID, err)
		}
	}
}

func processVideoBasic(videoDAO *persistence.VideoDAO, videoCategoryDAO *persistence.VideoCategoryDAO, categoryDAO *persistence.CategoryDAO, event VideoBasicEvent) error {
	log.Printf("[video-basic] 收到事件 video=%d hash=%s title=%q cover_url=%s", event.VideoID, event.HashID, event.Title, event.CoverURL)

	// 视频不存在时跳过（防御幂等）
	if _, err := videoDAO.GetByID(event.VideoID); err != nil {
		log.Printf("[video-basic] 视频不存在跳过 video=%d err=%v", event.VideoID, err)
		return err
	}

	// DB 写入与 ES 索引相互独立，并发执行可缩短消费者单条消息处理时长，提升消费吞吐
	tracker := timing.NewTracker()
	var wg sync.WaitGroup
	wg.Add(2)

	// goroutine A：DB 写入 —— 分类关联（删旧+批量插）+ 基础字段更新（title/description/cover_url）
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("db_write", time.Now())

		// 幂等：先删除旧关联，再写入新关联
		_ = videoCategoryDAO.DeleteByVideoID(event.VideoID)
		if len(event.CategoryIDs) > 0 {
			if err := videoCategoryDAO.BatchCreate(event.VideoID, event.CategoryIDs); err != nil {
				log.Printf("[video-basic] 分类关联写入失败 video=%d: %v", event.VideoID, err)
			}
		}

		// 仅更新 title/description/cover_url 列，避免 Save 全字段把 ConfirmUpload 后续置位的字段或并发字段覆写
		if err := videoDAO.UpdateBasicInfo(event.VideoID, event.Title, event.Description, event.CoverURL); err != nil {
			log.Printf("[video-basic] 更新视频基础字段失败 video=%d cover_url=%s err=%v", event.VideoID, event.CoverURL, err)
		} else {
			log.Printf("[video-basic] 更新视频基础字段成功 video=%d cover_url=%s", event.VideoID, event.CoverURL)
		}

		// 视频元数据更新后失效详情缓存
		if err := cache.DelVideoDetail(event.VideoID); err != nil {
			log.Printf("[video-basic] 删除视频缓存失败 video=%d: %v", event.VideoID, err)
		}
	}()

	// goroutine B：ES 索引 —— 反查分类名后写入 ES（失败降级，不阻塞）
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("es_index", time.Now())

		catNames := []string{}
		if len(event.CategoryIDs) > 0 {
			cats, err := categoryDAO.GetByIDs(event.CategoryIDs)
			if err == nil {
				for _, c := range cats {
					catNames = append(catNames, c.Name)
				}
			}
		}

		catIDStrs := make([]string, 0, len(event.CategoryIDs))
		for _, id := range event.CategoryIDs {
			catIDStrs = append(catIDStrs, hashids.Encode(id))
		}
		doc := domain.VideoSearchDoc{
			VideoID:       event.HashID,
			Title:         event.Title,
			Description:   event.Description,
			CategoryIDs:   catIDStrs,
			CategoryNames: catNames,
			AuthorID:      hashids.Encode(event.AuthorID),
			PublishTime:   event.CreatedAt,
			CreatedAt:     event.CreatedAt,
		}
		if err := elasticsearch.IndexVideo(context.Background(), doc); err != nil {
			log.Printf("[video-basic] ES 索引失败 video=%d: %v", event.VideoID, err)
		}
	}()

	wg.Wait()
	tracker.Report(fmt.Sprintf("processVideoBasic(video=%d)", event.VideoID))

	return nil
}

func runVideoComplexConsumer() {
	reader := config.NewKafkaReader(TopicVideoComplex, "sesame-sauce-video-complex-consumer")
	defer reader.Close()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				log.Printf("[kafka-consumer-video-complex] 读取消息失败(网络错误): temporary=%v timeout=%v err=%v", netErr.Temporary(), netErr.Timeout(), err)
			} else {
				log.Printf("[kafka-consumer-video-complex] 读取消息失败: 类型=%T err=%v", err, err)
			}
			time.Sleep(time.Second)
			continue
		}

		var event VideoComplexEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("[kafka-consumer-video-complex] 消息反序列化失败: %v", err)
			continue
		}

		if err := processVideoComplex(event); err != nil {
			log.Printf("[kafka-consumer-video-complex] 处理失败 video=%d: %v", event.VideoID, err)
		}
	}
}

func processVideoComplex(event VideoComplexEvent) error {
	videoDAO := persistence.NewVideoDAO()
	videoStorage := storage.NewMinIOVideoStorage()
	transcoder := media.NewFFmpegTranscoder()
	ws := workspace.NewTempWorkspace()

	video, err := videoDAO.GetByID(event.VideoID)
	if err != nil {
		return err
	}

	// 若视频已取消，直接跳过
	if video.Status == domain.VideoStatusCancelled {
		log.Printf("[video-complex] 视频已取消，跳过转码 video=%d", event.VideoID)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 DB 状态轮询 goroutine，若 status 变为 3 则取消转码
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				v, err := videoDAO.GetByID(event.VideoID)
				if err == nil && v.Status == domain.VideoStatusCancelled {
					cancel()
					return
				}
			}
		}
	}()

	tmpDir, release, err := ws.Acquire("video-transcode-")
	if err != nil {
		markFailed(videoDAO, event.VideoID, "申请工作空间", err)
		return err
	}
	defer release()

	localVideoPath := filepath.Join(tmpDir, "raw."+event.SourceFormat)
	localOutputDir := filepath.Join(tmpDir, "output")

	if err := videoStorage.DownloadRaw(ctx, event.VideoID, event.SourceFormat, localVideoPath); err != nil {
		markFailed(videoDAO, event.VideoID, "下载视频", err)
		return err
	}

	// 进度回调：每 10% 更新一次 DB
	progressUpdater := func(p int8) {
		_ = videoDAO.UpdateProgress(event.VideoID, p)
	}
	_ = progressUpdater

	duration, width, height, fileSize, err := transcoder.TranscodeToM3U8(ctx, localVideoPath, localOutputDir, event.VideoID)
	if err != nil {
		markFailed(videoDAO, event.VideoID, "m3u8 转码", err)
		return err
	}

	// 上传转码结果到 MinIO
	if err := videoStorage.UploadM3U8Dir(ctx, event.HashID, localOutputDir); err != nil {
		markFailed(videoDAO, event.VideoID, "上传 m3u8", err)
		return err
	}

	// 更新视频为已发布状态
	video, err = videoDAO.GetByID(event.VideoID)
	if err != nil {
		markFailed(videoDAO, event.VideoID, "查询视频", err)
		return err
	}

	meta := domain.VideoMetadata{
		Duration: duration,
		Width:    width,
		Height:   height,
		FileSize: fileSize,
		PlayURL:  storage.MasterPlaylistKey(event.HashID),
	}
	if err := video.PreparePublish(meta); err != nil {
		markFailed(videoDAO, event.VideoID, "状态推进", err)
		return err
	}

	// 仅更新 duration/width/height/file_size/play_url/status，避免 Save 全字段把并发写入的 cover_url 等覆盖
	// 发布成功时统一将 publish_time 置为当前时间；此前依赖 DB DEFAULT/GORM 零值判断均导致 0000-00-00 写入
	publishTime := time.Now()
	if err := videoDAO.UpdatePublishMeta(event.VideoID, meta.Duration, meta.Width, meta.Height, meta.FileSize, meta.PlayURL, domain.VideoStatusPublished, publishTime); err != nil {
		markFailed(videoDAO, event.VideoID, "更新视频记录", err)
		return err
	}

	// 转码产物已可用，删除 MinIO 原始视频文件释放存储；删除失败仅记录日志，不阻塞发布流程
	if err := videoStorage.DeleteRaw(ctx, event.VideoID); err != nil {
		log.Printf("[video-complex] 删除原始视频失败 video=%d: %v", event.VideoID, err)
	} else {
		log.Printf("[video-complex] 原始视频已删除 video=%d", event.VideoID)
	}

	// 视频发布后更新 ES 文档的 duration 和 publish_time（失败不阻塞）
	if elasticsearch.IsAvailable() {
		if err := elasticsearch.UpdateVideoFields(context.Background(), event.HashID, map[string]interface{}{
			"duration":     meta.Duration,
			"publish_time": publishTime,
		}); err != nil {
			log.Printf("[video-complex] ES 更新 duration 失败 video=%d: %v", event.VideoID, err)
		}
	}

	// 视频状态变为已发布，失效详情缓存
	if err := cache.DelVideoDetail(event.VideoID); err != nil {
		log.Printf("[video-complex] 删除视频缓存失败 video=%d: %v", event.VideoID, err)
	}

	log.Printf("[video-complex] 视频转码完成 video=%d duration=%d size=%dx%d play_url=%s", event.VideoID, meta.Duration, meta.Width, meta.Height, meta.PlayURL)
	return nil
}

func markFailed(videoDAO *persistence.VideoDAO, videoID uint64, stage string, err error) {
	log.Printf("[video-complex] %s 失败 video=%d: %v", stage, videoID, err)
	if e := videoDAO.UpdateStatus(videoID, domain.VideoStatusFailed); e != nil {
		log.Printf("[video-complex] 标记失败状态失败 video=%d: %v", videoID, e)
	}
}

// consumerVideoReader 独立构建 reader，避免与 config.NewKafkaReader 冲突时的 topic 问题
func consumerVideoReader(topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     config.KafkaBroker,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
	})
}
