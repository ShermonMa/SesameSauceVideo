/*
 * play_token.go
 * 功能：domain 层播放令牌生成策略，封装播放地址生成的业务语义
 * 时间戳：2026-05-02
 * 变更：
 *   1. 转码产物迁移到公共 bucket，master.m3u8 及其引用的子 playlist/ts 切片均依赖 bucket anonymous read，
 *      因此不再使用预签名 URL（预签名只对单个 objectKey 有效，无法覆盖 hls.js 后续请求的子文件）
 *   2. 对象键从数字 videoID 改为 videoHashID，防止攻击者遍历扫站
 */

package domain

import (
	"fmt"

	"github.com/shermon/SesameSauce/infra/minio"
)

// GeneratePlayToken 生成视频 master.m3u8 的公开访问 URL
// 返回 (url, expiresSeconds, err)；expiresSeconds=0 表示不过期（依赖 bucket 公开读策略）
func GeneratePlayToken(videoHashID string) (string, int, error) {
	bucket := minio.GetPublicBucket()
	baseURL := minio.BuildVideoBaseURL(bucket)
	objectKey := fmt.Sprintf("videos/%s/master.m3u8", videoHashID)
	url := fmt.Sprintf("%s/%s", baseURL, objectKey)
	return url, 0, nil
}
