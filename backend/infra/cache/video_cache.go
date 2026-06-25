/*
 * video_cache.go
 * 功能：视频详情 Cache-Aside 缓存层，支持互斥锁回源、空值防穿透、TTL 抖动防雪崩
 * 时间戳：2026-05-22
 */

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shermon/SesameSauce/config"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// VideoCacheValue 缓存中存储的视频详情结构
// 不包含计数（由 counter cache 维护），只存元数据 + 关联 ID
type VideoCacheValue struct {
	ID           uint64   `json:"id"`
	HashID       string   `json:"hashid"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	CoverURL     string   `json:"cover_url"` // 存储对象键（相对路径），非完整URL
	PlayURL      string   `json:"play_url"`  // 存储对象键（相对路径），非完整URL
	Duration     int      `json:"duration"`
	Width        int      `json:"width,omitempty"`
	Height       int      `json:"height,omitempty"`
	FileSize     int64    `json:"file_size,omitempty"`
	PublishTime  int64    `json:"publish_time"`
	Status       int8     `json:"status"`
	AuthorID     uint64   `json:"author_id"`
	CategoryIDs  []uint64 `json:"category_ids"`
}

// VideoCacheGetResult 视频缓存读取结果
type VideoCacheGetResult struct {
	Value    VideoCacheValue
	IsNull   bool
	Hit      bool
}

// GetVideoDetail 读取视频详情缓存
// 命中 JSON → 返回值；命中 "null" → IsNull=true；miss → Hit=false
func GetVideoDetail(videoID uint64) (VideoCacheGetResult, error) {
	ctx := context.Background()
	key := fmt.Sprintf("ss:video:detail:%d", videoID)

	val, err := redisc.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return VideoCacheGetResult{Hit: false}, nil
	}
	if err != nil {
		return VideoCacheGetResult{Hit: false}, err
	}

	if val == "null" {
		return VideoCacheGetResult{Hit: true, IsNull: true}, nil
	}

	var cv VideoCacheValue
	if err := json.Unmarshal([]byte(val), &cv); err != nil {
		// 格式损坏视为 miss，并删除脏 key
		redisc.Client.Del(ctx, key)
		return VideoCacheGetResult{Hit: false}, nil
	}

	return VideoCacheGetResult{Hit: true, Value: cv}, nil
}

// SetVideoDetail 回填视频详情缓存，TTL 带抖动
func SetVideoDetail(videoID uint64, val VideoCacheValue) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:video:detail:%d", videoID)

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ttl := config.ParamViper.GetInt("cache.video_detail.ttl_seconds")
	if ttl <= 0 {
		ttl = 3600
	}
	jitter := config.ParamViper.GetInt("cache.video_detail.jitter_seconds")
	if jitter > 0 {
		ttl += rand.Intn(jitter)
	}

	return redisc.Client.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

// SetVideoDetailNull 回填空值缓存（防穿透）
func SetVideoDetailNull(videoID uint64) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:video:detail:%d", videoID)
	ttl := config.ParamViper.GetInt("cache.video_detail.null_ttl_seconds")
	if ttl <= 0 {
		ttl = 60
	}
	return redisc.Client.Set(ctx, key, "null", time.Duration(ttl)*time.Second).Err()
}

// DelVideoDetail 删除视频详情缓存（写后失效）
func DelVideoDetail(videoID uint64) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:video:detail:%d", videoID)
	return redisc.Client.Del(ctx, key).Err()
}

// AcquireVideoDetailLock 尝试获取视频详情回源互斥锁
// 成功返回 (lockValue, true)；失败返回 ("", false)
func AcquireVideoDetailLock(videoID uint64) (string, bool, error) {
	ctx := context.Background()
	lockKey := fmt.Sprintf("ss:video:detail:lock:%d", videoID)
	lockVal := uuid.NewString()
	ttl := config.ParamViper.GetInt("cache.video_detail.mutex_ttl_seconds")
	if ttl <= 0 {
		ttl = 3
	}

	ok, err := redisc.Client.SetNX(ctx, lockKey, lockVal, time.Duration(ttl)*time.Second).Result()
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return lockVal, true, nil
}

// ReleaseVideoDetailLock 原子释放视频详情回源互斥锁
func ReleaseVideoDetailLock(videoID uint64, lockVal string) error {
	ctx := context.Background()
	lockKey := fmt.Sprintf("ss:video:detail:lock:%d", videoID)
	return redisc.Client.Eval(ctx, LuaReleaseLock, []string{lockKey}, lockVal).Err()
}
