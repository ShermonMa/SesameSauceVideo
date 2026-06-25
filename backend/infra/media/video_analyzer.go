/*
 * video_analyzer.go
 * 功能：基于 ffprobe + os.Stat 的视频元数据分析器，封装时长/宽高/文件大小一次性返回，实现 domain.VideoAnalyzer 端口
 * 时间戳：2026-04-26
 */

package media

import (
	"os"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/tools"
)

// FFprobeAnalyzer 基于 ffprobe 命令行工具的视频分析器实现
// 同时承担文件大小探测，避免 application 层为拿一个 fileSize 直接 os.Stat 暴露 IO 细节
type FFprobeAnalyzer struct{}

// NewFFprobeAnalyzer 创建实例
func NewFFprobeAnalyzer() *FFprobeAnalyzer {
	return &FFprobeAnalyzer{}
}

// 编译期校验接口实现
var _ domain.VideoAnalyzer = (*FFprobeAnalyzer)(nil)

// Analyze 探测本地视频文件，返回时长（秒）、宽、高、文件大小（字节）
func (a *FFprobeAnalyzer) Analyze(localPath string) (int, int, int, int64, error) {
	duration, width, height, err := tools.ProbeVideo(localPath)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	stat, err := os.Stat(localPath)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return duration, width, height, stat.Size(), nil
}
