/*
 * category_cache.go
 * 功能：分类元数据 Cache-Aside 缓存层
 * 时间戳：2026-05-22
 */

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shermon/SesameSauce/config"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// CategoryCacheValue 缓存中存储的分类元数据
type CategoryCacheValue struct {
	ID          uint64 `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

// GetCategory 读取分类缓存
// 命中 → (value, true)；miss → (zero, false)
func GetCategory(categoryID uint64) (CategoryCacheValue, bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("ss:category:detail:%d", categoryID)

	val, err := redisc.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return CategoryCacheValue{}, false, nil
	}
	if err != nil {
		return CategoryCacheValue{}, false, err
	}

	var cv CategoryCacheValue
	if err := json.Unmarshal([]byte(val), &cv); err != nil {
		redisc.Client.Del(ctx, key)
		return CategoryCacheValue{}, false, nil
	}
	return cv, true, nil
}

// SetCategory 回填分类缓存
func SetCategory(categoryID uint64, val CategoryCacheValue) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:category:detail:%d", categoryID)

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}

	ttl := config.ParamViper.GetInt("cache.category.ttl_seconds")
	if ttl <= 0 {
		ttl = 7200
	}

	return redisc.Client.Set(ctx, key, data, time.Duration(ttl)*time.Second).Err()
}

// DelCategory 删除分类缓存（分类后台更新后失效）
func DelCategory(categoryID uint64) error {
	ctx := context.Background()
	key := fmt.Sprintf("ss:category:detail:%d", categoryID)
	return redisc.Client.Del(ctx, key).Err()
}
