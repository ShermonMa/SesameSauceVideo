/*
 * cover_generator.go
 * 功能：基于 ffmpeg 随机帧截取的封面生成器，实现 domain.CoverGenerator 端口
 * 时间戳：2026-04-26
 */

package media

import (
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/tools"
)

// FFmpegCoverGenerator 基于 ffmpeg 命令行工具的封面生成器实现
// 业务策略：从视频时长内随机抽取一帧作为封面，避开第 0 秒以防黑屏
type FFmpegCoverGenerator struct{}

// NewFFmpegCoverGenerator 创建实例
func NewFFmpegCoverGenerator() *FFmpegCoverGenerator {
	return &FFmpegCoverGenerator{}
}

// 编译期校验接口实现
var _ domain.CoverGenerator = (*FFmpegCoverGenerator)(nil)

// Generate 从源视频提取一帧到目标路径作为封面
func (g *FFmpegCoverGenerator) Generate(srcVideoPath, dstCoverPath string) error {
	return tools.ExtractRandomFrame(srcVideoPath, dstCoverPath)
}
