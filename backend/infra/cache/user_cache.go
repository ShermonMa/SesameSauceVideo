/*
 * user_cache.go
 * 功能：用户资料 Cache-Aside 缓存层，支持 TTL 抖动防雪崩
 * 时间戳：2026-05-22
 */

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shermon/SesameSauce/config"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// UserCacheValue 缓存中存储的用户资料结构
// 2026-06-22 扩展统计字段，支撑个人主页美化
type UserCacheValue struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Avatar          string `json:"avatar,omitempty"`
	BackgroundImage string `json:"background_image,omitempty"`
	Signature       string `json:"signature,omitempty"`
	FollowCount     int64  `json:"follow_count"`
	FollowerCount   int64  `json:"follower_count"`
	TotalFavorited  int64  `json:"total_favorited"`
	WorkCount       int64  `json:"work_count"`
	FavoriteCount   int64  `json:"favorite_count"`
	CreatedAt       int64  `json:"created_at"` // Unix 时间戳
}

// GetUserProfile 读取用户资料缓存
// 命中 → (value, true)；miss → (zero, false)
func GetUserProfile(userID uint64) (UserCacheValue, bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("ss:user:profile:%d", userID)

	val, err := redisc.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return UserCacheValue{}, false, nil
	}
	if err != nil {
		return UserCacheValue{}, false, err
	}

	var cv UserCacheValue
	if err := json.Unmarshal([]byte(val), &cv); err != nil {
		redisc.Client.Del(ctx, key)
		return UserCacheValue{}, false, nil
	}
	return cv, true, nil
}

// SetUserProfile 回填用户资料缓存，TTL 带抖动
func SetUserProfile(userID uint64, val UserCacheValue) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:user:profile:%d", userID)

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ttl := config.ParamViper.GetInt("cache.user_profile.ttl_seconds")
	if ttl <= 0 {
		ttl = 3600
	}
	jitter := config.ParamViper.GetInt("cache.user_profile.jitter_seconds")
	if jitter > 0 {
		ttl += rand.Intn(jitter)
	}

	return redisc.Client.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

// DelUserProfile 删除用户资料缓存（用户改资料后失效）
func DelUserProfile(userID uint64) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:user:profile:%d", userID)
	return redisc.Client.Del(ctx, key).Err()
}
