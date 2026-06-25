/*
 * cuckoo_filter.go
 * 功能：RedisBloom Cuckoo Filter 封装，支持判重/添加/删除/初始化
 *      2026-06-21 修复 CF.EXISTS 返回值类型解析：RedisBloom 返回 bool，使用 Bool() 避免 go-redis Int() 解析失败
 * 时间戳：2026-05-22
 */

package cache

import (
	"context"
	"log"
	"strings"

	"github.com/shermon/SesameSauce/config"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// CuckooExists 检查元素是否可能存在于 Cuckoo Filter 中
// 返回 true = 可能存在（含误判）；false = 确定不存在
// 2026-06-21 修复：CF.EXISTS 在 RedisBloom 中返回 bool，改为 Bool() 解析，避免 unexpected type=bool for Int
func CuckooExists(key string, userID uint64) (bool, error) {
	ctx := context.Background()
	return redisc.Client.Do(ctx, "CF.EXISTS", key, userID).Bool()
}

// CuckooAdd 将元素添加到 Cuckoo Filter
func CuckooAdd(key string, userID uint64) error {
	ctx := context.Background()
	_, err := redisc.Client.Do(ctx, "CF.ADD", key, userID).Result()
	return err
}

// CuckooDel 从 Cuckoo Filter 删除元素
func CuckooDel(key string, userID uint64) error {
	ctx := context.Background()
	_, err := redisc.Client.Do(ctx, "CF.DEL", key, userID).Int()
	return err
}

// CuckooReserve 初始化 Cuckoo Filter（幂等：已存在则忽略错误）
func CuckooReserve(key string) error {
	ctx := context.Background()
	capacity := config.ParamViper.GetInt("counter.cuckoo.initial_capacity")
	if capacity <= 0 {
		capacity = 10000
	}
	expansion := config.ParamViper.GetInt("counter.cuckoo.expansion")
	if expansion <= 0 {
		expansion = 2
	}

	_, err := redisc.Client.Do(ctx, "CF.RESERVE", key, capacity, "EXPANSION", expansion).Result()
	if err != nil {
		if strings.Contains(err.Error(), "item exists") {
			return nil
		}
		return err
	}
	return nil
}

// EnsureCuckooReserved 确保 Cuckoo Filter 已初始化，失败时仅记日志
func EnsureCuckooReserved(key string) {
	if err := CuckooReserve(key); err != nil {
		log.Printf("[cuckoo] 初始化失败 key=%s: %v", key, err)
	}
}

// DelCuckooFilter 删除 Cuckoo Filter（视频/用户删除时清理）
func DelCuckooFilter(key string) error {
	ctx := context.Background()
	return redisc.Client.Del(ctx, key).Err()
}
