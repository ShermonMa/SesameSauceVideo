/*
 * like_cache.go
 * 功能：点赞关系异步化所需的 Redis 中间状态缓存
 *      维护 pending Hash（待落库状态）、dirty ZSet（待刷盘集合）、分布式锁、死信
 * 时间戳：2026-06-18
 */

package cache

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	redisc "github.com/shermon/SesameSauce/infra/redis"
)

// LikePending 存储单条点赞关系的待落库状态
type LikePending struct {
	State int8  `redis:"state"` // 1=点赞 0=取消
	Ts    int64 `redis:"ts"`    // 操作时间戳（毫秒）
}

// LikeDirtyItem 待刷盘的点赞关系项
type LikeDirtyItem struct {
	BizType  int8
	UserID   uint64
	TargetID uint64
	Ts       int64
}

// likePendingKey 单个点赞关系 pending key
func likePendingKey(bizType int8, userID, targetID uint64) string {
	return fmt.Sprintf("ss:like:pending:%d:%d:%d", bizType, userID, targetID)
}

// likeDirtyKey 某业务类型的脏列表 key
func likeDirtyKey(bizType int8) string {
	return fmt.Sprintf("ss:like:dirty:%d", bizType)
}

// likeFlushLockKey 某业务类型的刷盘锁 key
func likeFlushLockKey(bizType int8) string {
	return fmt.Sprintf("ss:like:flush:lock:%d", bizType)
}

// likeDeadLetterKey 某业务类型的死信 key
func likeDeadLetterKey(bizType int8) string {
	return fmt.Sprintf("ss:like:deadletter:%d", bizType)
}

// likeUserLockKey 单用户单资源的操作锁 key，防止并发点赞/取消点赞
func likeUserLockKey(bizType int8, userID, targetID uint64) string {
	return fmt.Sprintf("ss:like:lock:%d:%d:%d", bizType, userID, targetID)
}

// AcquireLikeUserLock 获取单用户单资源点赞操作锁
func AcquireLikeUserLock(bizType int8, userID, targetID uint64, lockVal string) (bool, error) {
	ctx := context.Background()
	key := likeUserLockKey(bizType, userID, targetID)
	return redisc.Client.SetNX(ctx, key, lockVal, 5*time.Second).Result()
}

// ReleaseLikeUserLock 释放单用户单资源点赞操作锁
func ReleaseLikeUserLock(bizType int8, userID, targetID uint64, lockVal string) error {
	ctx := context.Background()
	key := likeUserLockKey(bizType, userID, targetID)
	return redisc.Client.Eval(ctx, LuaReleaseLock, []string{key}, lockVal).Err()
}

// VideoLikeCuckooKey 返回视频点赞 Cuckoo Filter 的 Redis key
func VideoLikeCuckooKey(videoID uint64) string {
	return fmt.Sprintf("ss:video:like:cuckoo:%d", videoID)
}

// SetLikePending 写入待落库状态
func SetLikePending(bizType int8, userID, targetID uint64, state int8, ts int64) error {
	ctx := context.Background()
	key := likePendingKey(bizType, userID, targetID)
	return redisc.Client.HSet(ctx, key, map[string]interface{}{
		"state": state,
		"ts":    ts,
	}).Err()
}

// GetLikePending 读取待落库状态；不存在返回 nil
func GetLikePending(bizType int8, userID, targetID uint64) (*LikePending, error) {
	ctx := context.Background()
	key := likePendingKey(bizType, userID, targetID)
	m, err := redisc.Client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, nil
	}
	state, _ := strconv.ParseInt(m["state"], 10, 8)
	ts, _ := strconv.ParseInt(m["ts"], 10, 64)
	return &LikePending{State: int8(state), Ts: ts}, nil
}

// DelLikePending 删除待落库状态
func DelLikePending(bizType int8, userID, targetID uint64) error {
	ctx := context.Background()
	return redisc.Client.Del(ctx, likePendingKey(bizType, userID, targetID)).Err()
}

// AddLikeDirty 将点赞关系加入脏列表
func AddLikeDirty(bizType int8, userID, targetID uint64, ts int64) error {
	ctx := context.Background()
	key := likeDirtyKey(bizType)
	member := fmt.Sprintf("%d:%d", userID, targetID)
	return redisc.Client.ZAdd(ctx, key, redis.Z{Score: float64(ts), Member: member}).Err()
}

// RemLikeDirty 从脏列表移除
func RemLikeDirty(bizType int8, userID, targetID uint64) error {
	ctx := context.Background()
	key := likeDirtyKey(bizType)
	member := fmt.Sprintf("%d:%d", userID, targetID)
	return redisc.Client.ZRem(ctx, key, member).Err()
}

// GetDirtyLikes 读取指定时间之前的脏列表项
// beforeTs 为 0 表示读取全部
func GetDirtyLikes(bizType int8, beforeTs int64, limit int64) ([]LikeDirtyItem, error) {
	ctx := context.Background()
	key := likeDirtyKey(bizType)
	max := "+inf"
	if beforeTs > 0 {
		max = strconv.FormatInt(beforeTs, 10)
	}
	members, err := redisc.Client.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    max,
		Offset: 0,
		Count:  limit,
	}).Result()
	if err != nil {
		return nil, err
	}

	items := make([]LikeDirtyItem, 0, len(members))
	for _, z := range members {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		var userID, targetID uint64
		if _, err := fmt.Sscanf(member, "%d:%d", &userID, &targetID); err != nil {
			continue
		}
		items = append(items, LikeDirtyItem{
			BizType:  bizType,
			UserID:   userID,
			TargetID: targetID,
			Ts:       int64(z.Score),
		})
	}
	return items, nil
}

// GetDirtyLikeCount 读取脏列表大小
func GetDirtyLikeCount(bizType int8) (int64, error) {
	ctx := context.Background()
	return redisc.Client.ZCard(ctx, likeDirtyKey(bizType)).Result()
}

// AcquireLikeFlushLock 获取点赞关系刷盘分布式锁
func AcquireLikeFlushLock(bizType int8, lockVal string) (bool, error) {
	ctx := context.Background()
	key := likeFlushLockKey(bizType)
	ok, err := redisc.Client.SetNX(ctx, key, lockVal, 5*time.Minute).Result()
	return ok, err
}

// ReleaseLikeFlushLock 释放点赞关系刷盘分布式锁
func ReleaseLikeFlushLock(bizType int8, lockVal string) error {
	ctx := context.Background()
	key := likeFlushLockKey(bizType)
	return redisc.Client.Eval(ctx, LuaReleaseLock, []string{key}, lockVal).Err()
}

// SetLikeDeadLetter 写入死信
func SetLikeDeadLetter(bizType int8, userID, targetID uint64, state int8, ts int64) error {
	ctx := context.Background()
	key := likeDeadLetterKey(bizType)
	field := fmt.Sprintf("%d:%d", userID, targetID)
	value := fmt.Sprintf("%d|%d", state, ts)
	return redisc.Client.HSet(ctx, key, field, value).Err()
}

// LogLikeFlushError 记录刷盘错误日志（保持与 counter flush 一致的日志风格）
func LogLikeFlushError(bizType int8, userID, targetID uint64, err error) {
	log.Printf("[like-flush] 刷盘失败 bizType=%d user=%d target=%d: %v", bizType, userID, targetID, err)
}
