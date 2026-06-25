/*
 * object_key.go
 * 功能：集中所有 MinIO 对象路径生成规则与 cover_hashid 随机生成器，避免硬编码散落于 video_storage.go
 * 时间戳：2026-05-02
 */

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RawKey 拼接原始视频对象键（私有桶，保持数字 ID）
func RawKey(videoID uint64, ext string) string {
	return fmt.Sprintf("videos/%d/raw.%s", videoID, ext)
}

// RawPrefix 拼接原始视频对象前缀，用于列举/删除
func RawPrefix(videoID uint64) string {
	return fmt.Sprintf("videos/%d/raw.", videoID)
}

// VideoHLSPrefix 拼接 HLS 产物目录前缀（公共桶，使用 video hashid）
func VideoHLSPrefix(videoHashID string) string {
	return fmt.Sprintf("videos/%s/", videoHashID)
}

// MasterPlaylistKey 拼接 master.m3u8 对象键
func MasterPlaylistKey(videoHashID string) string {
	return fmt.Sprintf("videos/%s/master.m3u8", videoHashID)
}

// MasterPlaylistURL 拼接 master.m3u8 完整对外访问 URL
func MasterPlaylistURL(baseURL, videoHashID string) string {
	return fmt.Sprintf("%s/%s", baseURL, MasterPlaylistKey(videoHashID))
}

// CoverKey 拼接封面对象键（公共桶，使用 cover hashid）
func CoverKey(coverHashID, ext string) string {
	return fmt.Sprintf("covers/%s/cover.%s", coverHashID, ext)
}

// CoverPrefix 拼接封面对象前缀，用于列举/删除
func CoverPrefix(coverHashID string) string {
	return fmt.Sprintf("covers/%s/cover.", coverHashID)
}

// CoverURL 拼接封面完整对外访问 URL
func CoverURL(baseURL, coverHashID, ext string) string {
	return fmt.Sprintf("%s/%s", baseURL, CoverKey(coverHashID, ext))
}

// GenerateCoverHashID 生成独立的封面随机 hashid（6 字节 → hex → 12 字符）
// 与 video hashid 同源不可逆，攻击者拿到 cover URL 后无法反推 video URL
func GenerateCoverHashID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}
