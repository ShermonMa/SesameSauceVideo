/*
 * video_storage.go
 * 功能：基于 MinIO 的视频对象存储实现，封装 bucket 选择与业务编排；
 *      对象键命名规则已抽取到 object_key.go，本文件仅负责调用 minio 端口完成上传/下载/删除/预签名；
 *      UploadM3U8Dir 改造为 worker pool 并发上传切片，并使用 timing.Tracker 输出加速比报告
 * 时间戳：2026-05-03
 */

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/minio"
	"github.com/shermon/SesameSauce/pkg/timing"
	"github.com/spf13/viper"
)

// 默认 m3u8 切片并发上传数，可被 parameters.upload.m3u8_concurrency 覆盖
const defaultM3U8UploadConcurrency = 8

// MinIOVideoStorage 基于 MinIO 的 VideoStorage 端口实现
type MinIOVideoStorage struct{}

// NewMinIOVideoStorage 创建实例
func NewMinIOVideoStorage() *MinIOVideoStorage {
	return &MinIOVideoStorage{}
}

// 编译期校验接口实现
var _ domain.VideoStorage = (*MinIOVideoStorage)(nil)

// bucket 读取配置中的私有桶名（存视频），未配置时回落默认值
func bucket() string {
	b := viper.GetString("minio.bucket")
	if b == "" {
		b = "sesame-sauce"
	}
	return b
}

// publicBucket 读取配置中的公有桶名（存封面与转码产物），未配置时回落默认值
func publicBucket() string {
	b := viper.GetString("minio.public_bucket")
	if b == "" {
		b = "sesame-sauce-public"
	}
	return b
}

// PresignedRawUploadURL 生成原始视频上传的预签名 URL，落到私有 bucket
func (s *MinIOVideoStorage) PresignedRawUploadURL(videoID uint64, ext string, expires time.Duration) (string, error) {
	return minio.GeneratePresignedPutURL(bucket(), RawKey(videoID, ext), expires)
}

// PresignedCoverUploadURL 生成封面上传的预签名 URL，落到公有 bucket，需与 CoverExists 校验一致
func (s *MinIOVideoStorage) PresignedCoverUploadURL(coverHashID string, ext string, expires time.Duration) (string, error) {
	return minio.GeneratePresignedPutURL(publicBucket(), CoverKey(coverHashID, ext), expires)
}

// RawExists 检查原始视频是否已成功上传
func (s *MinIOVideoStorage) RawExists(ctx context.Context, videoID uint64, ext string) error {
	return minio.StatObject(ctx, bucket(), RawKey(videoID, ext))
}

// CoverExists 检查封面是否已成功上传
func (s *MinIOVideoStorage) CoverExists(ctx context.Context, coverHashID string, ext string) error {
	return minio.StatObject(ctx, publicBucket(), CoverKey(coverHashID, ext))
}

// DownloadRaw 下载原始视频到本地路径
func (s *MinIOVideoStorage) DownloadRaw(ctx context.Context, videoID uint64, ext string, localPath string) error {
	return minio.DownloadObject(ctx, bucket(), RawKey(videoID, ext), localPath)
}

// UploadM3U8Dir 将本地转码后的 m3u8 目录批量上传到 MinIO 公共 bucket
// 之所以落公共 bucket：hls.js 加载 master.m3u8 后会按相对路径请求子 playlist 与 ts 切片，
// MinIO 预签名 URL 只对单个 objectKey 生效无法覆盖这些子请求，故转码产物需公开可读；
// 切片数量通常在数十到上百，串行上传是转码完成→可播放的最大瓶颈，此处用 worker pool 并发上传，
// 并通过 timing.Tracker 输出累加耗时与墙钟耗时的对比，方便评估并发效果
func (s *MinIOVideoStorage) UploadM3U8Dir(ctx context.Context, videoHashID string, localDir string) error {
	b := publicBucket()
	prefix := VideoHLSPrefix(videoHashID)

	// 1) 先 walk 收集所有待上传任务，再提交给 worker pool；避免 walk 与上传交错导致并发难以控制
	type fileTask struct {
		path        string
		objectKey   string
		contentType string
	}
	var tasks []fileTask
	if err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		objectKey := prefix + filepath.ToSlash(rel)
		contentType := "application/octet-stream"
		if strings.HasSuffix(path, ".m3u8") {
			contentType = "application/vnd.apple.mpegurl"
		} else if strings.HasSuffix(path, ".ts") {
			contentType = "video/mp2t"
		}
		tasks = append(tasks, fileTask{path: path, objectKey: objectKey, contentType: contentType})
		return nil
	}); err != nil {
		return err
	}
	if len(tasks) == 0 {
		return nil
	}

	// 2) 读取并发度配置，未配置时回落默认值
	concurrency := defaultM3U8UploadConcurrency
	if config.ParamViper != nil {
		if v := config.ParamViper.GetInt("upload.m3u8_concurrency"); v > 0 {
			concurrency = v
		}
	}
	if concurrency > len(tasks) {
		concurrency = len(tasks)
	}

	// 3) worker pool：任一切片失败立刻 cancel，让其他 worker 尽快退出
	tracker := timing.NewTracker()
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	taskCh := make(chan fileTask, len(tasks))
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	var wg sync.WaitGroup
	errCh := make(chan error, concurrency)

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		workerID := w
		go func() {
			defer wg.Done()
			workerStart := time.Now()
			processed := 0
			defer func() {
				tracker.RecordSince(fmt.Sprintf("worker-%d(files=%d)", workerID, processed), workerStart)
			}()
			for task := range taskCh {
				select {
				case <-cctx.Done():
					return
				default:
				}
				if err := minio.UploadObject(cctx, b, task.objectKey, task.path, task.contentType); err != nil {
					select {
					case errCh <- err:
					default:
					}
					cancel()
					return
				}
				processed++
			}
		}()
	}

	wg.Wait()
	close(errCh)

	tracker.Report(fmt.Sprintf("UploadM3U8Dir(hash=%s,files=%d,workers=%d)", videoHashID, len(tasks), concurrency))

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// UploadCover 上传封面文件到公有 bucket 并返回可对外访问的封面 URL
func (s *MinIOVideoStorage) UploadCover(ctx context.Context, coverHashID string, localPath string) (string, error) {
	// 从本地路径推断扩展名
	ext := "jpg"
	if strings.HasSuffix(strings.ToLower(localPath), ".png") {
		ext = "png"
	}
	key := CoverKey(coverHashID, ext)
	b := publicBucket()
	if err := minio.UploadObject(ctx, b, key, localPath, "image/"+ext); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", minio.BuildVideoBaseURL(b), key), nil
}

// CoverURL 仅做 URL 拼接，不做上传，供 ConfirmUpload 阶段把已经预签名上传完成的封面地址回写到 MySQL
func (s *MinIOVideoStorage) CoverURL(coverHashID string, ext string) string {
	return CoverURL(minio.BuildVideoBaseURL(publicBucket()), coverHashID, ext)
}

// MasterPlayURL 返回 master.m3u8 的播放访问 URL，落公共 bucket 与 UploadM3U8Dir 保持一致
func (s *MinIOVideoStorage) MasterPlayURL(videoHashID string) string {
	return MasterPlaylistURL(minio.BuildVideoBaseURL(publicBucket()), videoHashID)
}

// DeleteRaw 删除原始视频文件
func (s *MinIOVideoStorage) DeleteRaw(ctx context.Context, videoID uint64) error {
	// 由于扩展名不确定，列举前缀并删除匹配 raw.* 的对象
	b := bucket()
	prefix := RawPrefix(videoID)
	objects, err := minio.ListObjects(ctx, b, prefix)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if err := minio.DeleteObject(ctx, b, obj.Key); err != nil {
			return err
		}
	}
	return nil
}

// DeleteCover 删除封面文件
func (s *MinIOVideoStorage) DeleteCover(ctx context.Context, coverHashID string) error {
	b := publicBucket()
	prefix := CoverPrefix(coverHashID)
	objects, err := minio.ListObjects(ctx, b, prefix)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if err := minio.DeleteObject(ctx, b, obj.Key); err != nil {
			return err
		}
	}
	return nil
}
