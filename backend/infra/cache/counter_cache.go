/*
 * counter_cache.go
 * 功能：计数器双缓存层（base + delta），支持 Pipeline 读写、脏列表追踪、原子刷盘
 *      新增 total 计数器统一初始化与 like 专用调整接口，保证 Redis 计数一致性
 * 时间戳：2026-05-22
 */

package cache

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shermon/SesameSauce/config"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// CounterField 支持的计数字段类型
const (
	CounterFieldPlay      = "play"
	CounterFieldLike      = "like"
	CounterFieldComment   = "comment"
	CounterFieldFollower  = "follower"
	CounterFieldFollowing = "following"
)

// IncrByDelta 对指定 field + id 的 delta 执行 INCRBY value
func IncrByDelta(field string, id uint64, value int64) error {
	ctx := context.Background()
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)

	pipe := redisc.Client.Pipeline()
	pipe.IncrBy(ctx, deltaKey, value)
	pipe.SAdd(ctx, dirtyKey, id)
	_, err := pipe.Exec(ctx)
	return err
}

// IncrDelta 对指定 field + id 的 delta 执行 INCR，并加入脏列表
func IncrDelta(field string, id uint64) error {
	ctx := context.Background()
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)

	pipe := redisc.Client.Pipeline()
	pipe.Incr(ctx, deltaKey)
	pipe.SAdd(ctx, dirtyKey, id)
	_, err := pipe.Exec(ctx)
	return err
}

// DecrDelta 对指定 field + id 的 delta 执行 DECR，并加入脏列表
func DecrDelta(field string, id uint64) error {
	ctx := context.Background()
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)

	pipe := redisc.Client.Pipeline()
	pipe.Decr(ctx, deltaKey)
	pipe.SAdd(ctx, dirtyKey, id)
	_, err := pipe.Exec(ctx)
	return err
}

// GetDelta 读取当前 delta 值（未命中视为 0）
func GetDelta(field string, id uint64) (int64, error) {
	ctx := context.Background()
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)
	val, err := redisc.Client.Get(ctx, deltaKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// GetBase 读取 base 缓存值
func GetBase(field string, id uint64) (int64, error) {
	ctx := context.Background()
	baseKey := fmt.Sprintf("ss:video:counter:base:%s:%d", field, id)
	val, err := redisc.Client.Get(ctx, baseKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetBase 回填 base 缓存，TTL 带抖动
func SetBase(field string, id uint64, value int64) error {
	ctx := context.Background()
	baseKey := fmt.Sprintf("ss:video:counter:base:%s:%d", field, id)

	ttl := config.ParamViper.GetInt("counter.base_cache.ttl_seconds")
	if ttl <= 0 {
		ttl = 60
	}
	jitter := config.ParamViper.GetInt("counter.base_cache.jitter_seconds")
	if jitter > 0 {
		ttl += rand.Intn(jitter)
	}

	return redisc.Client.Set(ctx, baseKey, value, time.Duration(ttl)*time.Second).Err()
}

// DelBase 删除 base 缓存（刷盘后失效）
func DelBase(field string, id uint64) error {
	ctx := context.Background()
	baseKey := fmt.Sprintf("ss:video:counter:base:%s:%d", field, id)
	return redisc.Client.Del(ctx, baseKey).Err()
}

// GetCounter 读取计数器值：base + delta
// base miss → 返回 0（由调用方加载）
func GetCounter(field string, id uint64) (base, delta int64, err error) {
	ctx := context.Background()
	baseKey := fmt.Sprintf("ss:video:counter:base:%s:%d", field, id)
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)

	pipe := redisc.Client.Pipeline()
	baseCmd := pipe.Get(ctx, baseKey)
	deltaCmd := pipe.Get(ctx, deltaKey)
	_, err = pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return 0, 0, err
	}

	baseVal, _ := baseCmd.Int64()
	deltaVal, _ := deltaCmd.Int64()
	return baseVal, deltaVal, nil
}

// GetAndClearDelta 原子读取并清零 delta（GETDEL）
func GetAndClearDelta(field string, id uint64) (int64, error) {
	ctx := context.Background()
	deltaKey := fmt.Sprintf("ss:video:counter:delta:%s:%d", field, id)
	val, err := redisc.Client.GetDel(ctx, deltaKey).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// AddDirty 将 id 加入脏列表（用于补救回滚）
func AddDirty(field string, ids ...uint64) error {
	ctx := context.Background()
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)
	members := make([]interface{}, len(ids))
	for i, id := range ids {
		members[i] = id
	}
	return redisc.Client.SAdd(ctx, dirtyKey, members...).Err()
}

// RemoveDirty 从脏列表移除 id（刷盘成功后）
func RemoveDirty(field string, ids ...uint64) error {
	ctx := context.Background()
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)
	members := make([]interface{}, len(ids))
	for i, id := range ids {
		members[i] = id
	}
	return redisc.Client.SRem(ctx, dirtyKey, members...).Err()
}

// GetDirtyMembers 读取脏列表全部成员
func GetDirtyMembers(field string) ([]uint64, error) {
	ctx := context.Background()
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)
	members, err := redisc.Client.SMembers(ctx, dirtyKey).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(members))
	for _, m := range members {
		var id uint64
		// go-redis SMembers 对整数成员返回整数，但 Result() 返回 []string
		// 这里用 fmt.Sscanf 兼容
		fmt.Sscanf(m, "%d", &id)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// GetDirtyCount 读取脏列表大小
func GetDirtyCount(field string) (int64, error) {
	ctx := context.Background()
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", field)
	return redisc.Client.SCard(ctx, dirtyKey).Result()
}

// liveCounterKey 实时计数器 key，用于弥补 base+delta 在异步刷盘窗口内的读数延迟
func liveCounterKey(field string, id uint64) string {
	return fmt.Sprintf("ss:video:counter:live:%s:%d", field, id)
}

// IncrLiveCounter 实时计数器 +1
func IncrLiveCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Incr(ctx, liveCounterKey(field, id)).Err()
}

// DecrLiveCounter 实时计数器 -1
func DecrLiveCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Decr(ctx, liveCounterKey(field, id)).Err()
}

// GetLiveCounter 读取实时计数器，未命中视为 0
func GetLiveCounter(field string, id uint64) (int64, error) {
	ctx := context.Background()
	val, err := redisc.Client.Get(ctx, liveCounterKey(field, id)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// LiveCounterExists 判断实时计数器 key 是否存在
func LiveCounterExists(field string, id uint64) (bool, error) {
	ctx := context.Background()
	n, err := redisc.Client.Exists(ctx, liveCounterKey(field, id)).Result()
	return n > 0, err
}

// DelLiveCounter 删除实时计数器（刷盘成功后调用）
func DelLiveCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Del(ctx, liveCounterKey(field, id)).Err()
}

// totalCounterKey 视频维度总赞数 key，直接维护实时总量
func totalCounterKey(field string, id uint64) string {
	return fmt.Sprintf("ss:video:counter:total:%s:%d", field, id)
}

// totalCounterTTL 返回 total 计数器缓存 TTL（带抖动）
func totalCounterTTL() time.Duration {
	ttl := config.ParamViper.GetInt("counter.total_cache.ttl_seconds")
	if ttl <= 0 {
		ttl = 86400 // 默认 24h
	}
	jitter := config.ParamViper.GetInt("counter.total_cache.jitter_seconds")
	if jitter > 0 {
		ttl += rand.Intn(jitter)
	}
	return time.Duration(ttl) * time.Second
}

// IncrTotalCounter 总数计数器 +1
func IncrTotalCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Incr(ctx, totalCounterKey(field, id)).Err()
}

// DecrTotalCounter 总数计数器 -1
func DecrTotalCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Decr(ctx, totalCounterKey(field, id)).Err()
}

// GetTotalCounter 读取总数计数器，未命中返回 redis.Nil 错误
func GetTotalCounter(field string, id uint64) (int64, error) {
	ctx := context.Background()
	return redisc.Client.Get(ctx, totalCounterKey(field, id)).Int64()
}

// SetTotalCounter 回填总数计数器，带 TTL
func SetTotalCounter(field string, id uint64, value int64) error {
	ctx := context.Background()
	return redisc.Client.Set(ctx, totalCounterKey(field, id), value, totalCounterTTL()).Err()
}

// MGetTotalCounters 批量读取 total 计数器。
// 返回已命中的值 map，以及未命中的 id 列表。
func MGetTotalCounters(field string, ids []uint64) (map[uint64]int64, []uint64, error) {
	ctx := context.Background()
	if len(ids) == 0 {
		return map[uint64]int64{}, nil, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = totalCounterKey(field, id)
	}
	vals, err := redisc.Client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, nil, err
	}
	res := make(map[uint64]int64, len(ids))
	var missing []uint64
	for i, id := range ids {
		if vals[i] == nil {
			missing = append(missing, id)
			continue
		}
		s, err := strconv.ParseInt(fmt.Sprintf("%v", vals[i]), 10, 64)
		if err != nil {
			missing = append(missing, id)
			continue
		}
		res[id] = s
	}
	return res, missing, nil
}

// MSetTotalCounters 批量回填 total 计数器，统一 TTL
func MSetTotalCounters(field string, values map[uint64]int64) error {
	ctx := context.Background()
	if len(values) == 0 {
		return nil
	}
	ttl := totalCounterTTL()
	pipe := redisc.Client.Pipeline()
	for id, val := range values {
		pipe.Set(ctx, totalCounterKey(field, id), val, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// DelTotalCounter 删除总数计数器
func DelTotalCounter(field string, id uint64) error {
	ctx := context.Background()
	return redisc.Client.Del(ctx, totalCounterKey(field, id)).Err()
}

// luaInitTotalCounter 原子初始化 total 计数器：不存在则写入 value，最后返回当前值
const luaInitTotalCounter = `
local key = KEYS[1]
local value = ARGV[1]
local ttl = ARGV[2]
if redis.call('EXISTS', key) == 0 then
    if ttl == "0" then
        redis.call('SET', key, value)
    else
        redis.call('SET', key, value, 'EX', ttl)
    end
end
return redis.call('GET', key)
`

// InitTotalCounter 原子初始化 total 计数器。
// 若 key 已存在则直接返回已有值；若不存在则使用 dbLoader 回源并写入，保证并发下只回源一次。
func InitTotalCounter(field string, id uint64, dbLoader func() (int64, error)) (int64, error) {
	ctx := context.Background()
	key := totalCounterKey(field, id)

	// 先快速路径读取，避免不必要的回源
	if val, err := redisc.Client.Get(ctx, key).Int64(); err == nil {
		return val, nil
	}

	count, err := dbLoader()
	if err != nil {
		return 0, err
	}

	res, err := redisc.Client.Eval(ctx, luaInitTotalCounter, []string{key}, count, int64(totalCounterTTL().Seconds())).Result()
	if err != nil {
		return 0, err
	}
	s, err := strconv.ParseInt(fmt.Sprintf("%v", res), 10, 64)
	if err != nil {
		return count, nil
	}
	return s, nil
}

// luaAdjustLikeTotal 原子调整 like total 并标记脏
const luaAdjustLikeTotal = `
local totalKey = KEYS[1]
local dirtyKey = KEYS[2]
local delta = tonumber(ARGV[1])
local member = ARGV[2]
local ttl = tonumber(ARGV[3])
local newVal = redis.call('INCRBY', totalKey, delta)
redis.call('SADD', dirtyKey, member)
if ttl > 0 then
    redis.call('EXPIRE', totalKey, ttl)
end
return newVal
`

// AdjustLikeTotal 原子调整视频点赞 total 计数，并把视频 ID 加入通用脏集合供 flush 任务刷盘。
// 调用前应先通过 InitTotalCounter 保证 total 已初始化，否则 INCRBY 会从 0 开始。
func AdjustLikeTotal(videoID uint64, delta int64) (int64, error) {
	ctx := context.Background()
	totalKey := totalCounterKey(CounterFieldLike, videoID)
	dirtyKey := fmt.Sprintf("ss:counter:dirty:%s", CounterFieldLike)

	res, err := redisc.Client.Eval(ctx, luaAdjustLikeTotal, []string{totalKey, dirtyKey}, delta, videoID, int64(totalCounterTTL().Seconds())).Result()
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(fmt.Sprintf("%v", res), 10, 64)
}

// AcquireFlushLock 获取刷盘分布式锁
func AcquireFlushLock(field string, lockVal string) (bool, error) {
	ctx := context.Background()
	lockKey := fmt.Sprintf("ss:counter:flush:lock:%s", field)
	ttl := config.ParamViper.GetInt("counter.flush.lock_ttl_seconds")
	if ttl <= 0 {
		ttl = 600
	}
	ok, err := redisc.Client.SetNX(ctx, lockKey, lockVal, time.Duration(ttl)*time.Second).Result()
	return ok, err
}

// ReleaseFlushLock 释放刷盘分布式锁
func ReleaseFlushLock(field string, lockVal string) error {
	ctx := context.Background()
	lockKey := fmt.Sprintf("ss:counter:flush:lock:%s", field)
	return redisc.Client.Eval(ctx, LuaReleaseLock, []string{lockKey}, lockVal).Err()
}

// SetDeadLetter 写入死信 Hash
func SetDeadLetter(field string, id uint64, delta int64) error {
	ctx := context.Background()
	deadKey := fmt.Sprintf("ss:counter:deadletter:%s", field)
	value := fmt.Sprintf("%d|%d", delta, time.Now().Unix())
	return redisc.Client.HSet(ctx, deadKey, fmt.Sprintf("%d", id), value).Err()
}
