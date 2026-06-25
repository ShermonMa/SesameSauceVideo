/*
 * progress_cache.go
 * 功能：播放进度 Redis 缓存层，封装 Hash 读写、容量淘汰、TTL 刷新与分布式锁
 * 时间戳：2026-04-27
 */

package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shermon/SesameSauce/config"
)

// progressValue 是 Hash 中存储的 JSON 结构体
// 字段精简为 p(progress) 与 t(timestamp)
type progressValue struct {
	P int   `json:"p"`
	T int64 `json:"t"`
}

// ProgressRecord 聚合任务使用的进度记录
type ProgressRecord struct {
	UserID   uint64
	VideoID  uint64
	Progress int
}

// SetProgress 将用户某视频的观看进度写入 Redis Hash
// Key: video:progress:{user_id}, Field: {video_id}, Value: JSON
// 若该用户缓存数 >= max_cache_per_user，淘汰最旧的一条
// 每次写入后重置 Key 的 TTL
func SetProgress(userID, videoID uint64, progress int) error {
	if Client == nil {
		return fmt.Errorf("redis client 未初始化")
	}

	ctx := context.Background()
	key := fmt.Sprintf("video:progress:%d", userID)
	field := strconv.FormatUint(videoID, 10)
	val, _ := json.Marshal(progressValue{P: progress, T: time.Now().Unix()})

	// 容量淘汰：若 HLEN >= max_cache_per_user，删除最旧的一条
	maxCache := config.ParamViper.GetInt("progress.max_cache_per_user")
	if maxCache <= 0 {
		maxCache = 10
	}
	count, err := Client.HLen(ctx, key).Result()
	if err != nil {
		log.Printf("[progress_cache] HLen 失败 key=%s: %v", key, err)
	}
	if count >= int64(maxCache) {
		if err := evictOldest(ctx, key); err != nil {
			log.Printf("[progress_cache] 淘汰最旧进度失败 key=%s: %v", key, err)
		}
	}

	// 写入 Hash 并刷新 TTL
	if err := Client.HSet(ctx, key, field, val).Err(); err != nil {
		return err
	}

	ttl := config.ParamViper.GetInt("progress.ttl_seconds")
	if ttl <= 0 {
		ttl = 3600
	}
	return Client.Expire(ctx, key, time.Duration(ttl)*time.Second).Err()
}

// GetProgress 从 Redis 读取用户某视频的近期进度
// 若 key 或 field 不存在，返回 (0, nil)
func GetProgress(userID, videoID uint64) (int, error) {
	if Client == nil {
		return 0, fmt.Errorf("redis client 未初始化")
	}

	ctx := context.Background()
	key := fmt.Sprintf("video:progress:%d", userID)
	field := strconv.FormatUint(videoID, 10)

	val, err := Client.HGet(ctx, key, field).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var pv progressValue
	if err := json.Unmarshal([]byte(val), &pv); err != nil {
		return 0, err
	}
	return pv.P, nil
}

// ScanAllProgress 扫描所有 video:progress:* 的 Key，返回全部进度记录
// 生产环境使用 SCAN 避免阻塞
func ScanAllProgress() ([]ProgressRecord, error) {
	if Client == nil {
		return nil, fmt.Errorf("redis client 未初始化")
	}

	ctx := context.Background()
	var records []ProgressRecord

	var cursor uint64
	for {
		keys, nextCursor, err := Client.Scan(ctx, cursor, "video:progress:*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			userID, ok := parseUserIDFromKey(key)
			if !ok {
				continue
			}

			fields, err := Client.HGetAll(ctx, key).Result()
			if err != nil {
				log.Printf("[progress_cache] HGetAll 失败 key=%s: %v", key, err)
				continue
			}

			for field, val := range fields {
				videoID, err := strconv.ParseUint(field, 10, 64)
				if err != nil {
					continue
				}
				var pv progressValue
				if err := json.Unmarshal([]byte(val), &pv); err != nil {
					continue
				}
				records = append(records, ProgressRecord{
					UserID:   userID,
					VideoID:  videoID,
					Progress: pv.P,
				})
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return records, nil
}

// AcquireAggregateLock 尝试获取聚合任务的分布式锁
// 使用 SET NX EX 实现，成功返回 true
func AcquireAggregateLock(lockID string, expireSec int) (bool, error) {
	if Client == nil {
		return false, fmt.Errorf("redis client 未初始化")
	}
	ctx := context.Background()
	ok, err := Client.SetNX(ctx, "lock:aggregate", lockID, time.Duration(expireSec)*time.Second).Result()
	return ok, err
}

// ReleaseAggregateLock 使用 Lua 脚本原子释放分布式锁
// 仅当锁的值与 lockID 匹配时才删除，防止误释放其他实例的锁
func ReleaseAggregateLock(lockID string) error {
	if Client == nil {
		return fmt.Errorf("redis client 未初始化")
	}
	ctx := context.Background()
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return Client.Eval(ctx, script, []string{"lock:aggregate"}, lockID).Err()
}

// evictOldest 删除指定 Hash 中时间戳最旧的一个 field
func evictOldest(ctx context.Context, key string) error {
	fields, err := Client.HGetAll(ctx, key).Result()
	if err != nil {
		return err
	}

	var oldestField string
	var oldestTs int64 = -1
	for field, val := range fields {
		var pv progressValue
		if err := json.Unmarshal([]byte(val), &pv); err != nil {
			continue
		}
		if oldestTs == -1 || pv.T < oldestTs {
			oldestTs = pv.T
			oldestField = field
		}
	}

	if oldestField != "" {
		return Client.HDel(ctx, key, oldestField).Err()
	}
	return nil
}

// parseUserIDFromKey 从 video:progress:{user_id} 中解析 user_id
func parseUserIDFromKey(key string) (uint64, bool) {
	prefix := "video:progress:"
	if !strings.HasPrefix(key, prefix) {
		return 0, false
	}
	uidStr := strings.TrimPrefix(key, prefix)
	uid, err := strconv.ParseUint(uidStr, 10, 64)
	if err != nil {
		return 0, false
	}
	return uid, true
}
