/*
 * video_transcoder.go
 * 功能：基于 ffmpeg 的 VideoTranscoder 端口实现，支持 m3u8 多码率转码与 Context 取消
 * 时间戳：2026-05-01
 */

package media

import (
	"context"
	"fmt"
	"os"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/tools"
)

// FFmpegTranscoder 实现 domain.VideoTranscoder
type FFmpegTranscoder struct{}

// NewFFmpegTranscoder 创建实例
func NewFFmpegTranscoder() *FFmpegTranscoder {
	return &FFmpegTranscoder{}
}

var _ domain.VideoTranscoder = (*FFmpegTranscoder)(nil)

// TranscodeToM3U8 将源视频转码为 m3u8（1080p + 360p），生成 master playlist
func (t *FFmpegTranscoder) TranscodeToM3U8(ctx context.Context, srcPath, dstDir string, videoID uint64) (int, int, int, int64, error) {
	duration, origWidth, origHeight, fileSize, err := tools.ProbeVideoFull(srcPath)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("视频探测失败: %w", err)
	}

	// 若原视频低于 1080p，则按原视频最大分辨率输出，不强制放大
	width1080, height1080 := 1920, 1080
	if origWidth < 1920 || origHeight < 1080 {
		width1080, height1080 = origWidth, origHeight
	}

	resolutions := []tools.Resolution{
		{Name: "1080p", Width: width1080, Height: height1080, Bitrate: "5000k"},
		{Name: "360p", Width: 640, Height: 360, Bitrate: "800k"},
	}

	if err := tools.TranscodeM3U8(ctx, srcPath, dstDir, resolutions); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("m3u8 转码失败: %w", err)
	}

	if err := tools.GenerateMasterPlaylist(dstDir, resolutions); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("生成 master playlist 失败: %w", err)
	}

	return duration, origWidth, origHeight, fileSize, nil
}

// CleanupM3U8Dir 清理转码输出目录
func CleanupM3U8Dir(dir string) {
	_ = os.RemoveAll(dir)
}
