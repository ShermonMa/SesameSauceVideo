/*
 * play_service.go
 * 功能：播放应用服务，轻量路线：仅 INCR Redis delta，无判重、无关系表
 * 时间戳：2026-05-22
 */

package application

import (
	"github.com/shermon/SesameSauce/infra/cache"
)

// PlayService 处理播放相关的应用层业务逻辑
type PlayService struct{}

// NewPlayService 创建 PlayService 实例
func NewPlayService() *PlayService {
	return &PlayService{}
}

// RecordPlay 记录一次播放（轻量路线）
// 仅 INCR Redis delta + SADD dirty，不查 DB、不判重
func (s *PlayService) RecordPlay(videoID uint64) error {
	return cache.IncrDelta(cache.CounterFieldPlay, videoID)
}
