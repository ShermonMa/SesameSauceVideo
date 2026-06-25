/*
 * ffmpeg.go
 * 功能：封装 ffmpeg/ffprobe 外部命令调用，提供视频探测、封面帧提取与 m3u8 多码率转码能力
 * 时间戳：2026-05-01
 */

package tools

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ProbeVideo 使用 ffprobe 获取视频时长、宽、高
func ProbeVideo(videoPath string) (int, int, int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}
	durSec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, 0, 0, err
	}
	duration := int(durSec)

	cmd = exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	out, err = cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}
	width, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, 0, 0, err
	}

	cmd = exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=height", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	out, err = cmd.Output()
	if err != nil {
		return 0, 0, 0, err
	}
	height, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, 0, 0, err
	}

	return duration, width, height, nil
}

// ProbeVideoFull 使用 ffprobe 获取视频时长、宽、高、文件大小
func ProbeVideoFull(videoPath string) (int, int, int, int64, error) {
	duration, width, height, err := ProbeVideo(videoPath)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return duration, width, height, info.Size(), nil
}

// ExtractRandomFrame 从视频中提取随机一帧作为封面
func ExtractRandomFrame(videoPath, coverPath string) error {
	duration, _, _, err := ProbeVideo(videoPath)
	if err != nil {
		return err
	}

	maxSec := duration
	if maxSec < 1 {
		maxSec = 1
	}
	rand.Seed(time.Now().UnixNano())
	seconds := rand.Intn(maxSec)
	if seconds < 1 {
		seconds = 1
	}

	timestamp := strconv.Itoa(seconds)
	return ExtractFrameAt(videoPath, coverPath, timestamp)
}

// ExtractFrameAt 在指定秒数提取单帧封面
func ExtractFrameAt(videoPath, coverPath, timestamp string) error {
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-ss", timestamp+".000", "-vframes", "1", coverPath)
	return cmd.Run()
}

// Resolution 定义转码输出分辨率配置
type Resolution struct {
	Name   string
	Width  int
	Height int
	Bitrate string
}

// TranscodeM3U8 使用 ffmpeg 将源视频转码为指定分辨率的 m3u8 切片
func TranscodeM3U8(ctx context.Context, srcPath, dstDir string, resolutions []Resolution) error {
	for _, res := range resolutions {
		resDir := filepath.Join(dstDir, res.Name)
		if err := os.MkdirAll(resDir, 0755); err != nil {
			return fmt.Errorf("创建输出目录失败 %s: %w", res.Name, err)
		}
		playlistPath := filepath.Join(resDir, "playlist.m3u8")

		scaleFilter := fmt.Sprintf("scale=w=%d:h=%d:force_original_aspect_ratio=decrease", res.Width, res.Height)
		args := []string{
			"-i", srcPath,
			"-c:v", "libx264",
			"-c:a", "aac",
			"-vf", scaleFilter,
			"-b:v", res.Bitrate,
			"-hls_time", "10",
			"-hls_list_size", "0",
			"-f", "hls",
			"-y",
			playlistPath,
		}
		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg 转码失败 %s: %w", res.Name, err)
		}
	}
	return nil
}

// GenerateMasterPlaylist 生成 master.m3u8，包含各码率的子 playlist 引用
func GenerateMasterPlaylist(dstDir string, resolutions []Resolution) error {
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	for _, res := range resolutions {
		bandwidth := "5000000"
		if res.Name == "360p" {
			bandwidth = "800000"
		}
		sb.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%s,RESOLUTION=%dx%d\n", bandwidth, res.Width, res.Height))
		sb.WriteString(fmt.Sprintf("%s/playlist.m3u8\n", res.Name))
	}
	masterPath := filepath.Join(dstDir, "master.m3u8")
	return os.WriteFile(masterPath, []byte(sb.String()), 0644)
}
