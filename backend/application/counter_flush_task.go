/*
 * counter_flush_task.go
 * 功能：计数器异步刷盘守护任务，5 个 field 各启一个 FlushDaemon
 *      混合触发（定时 60s ‖ 脏列表 ≥1000），GETDEL 原子清零并累加到 MySQL
 * 时间戳：2026-05-22
 */

package application

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/persistence"
)

var flushOnce sync.Once

// StartCounterFlushTasks 启动全部 5 个计数器刷盘守护任务
func StartCounterFlushTasks() {
	flushOnce.Do(func() {
		fields := []string{
			cache.CounterFieldPlay,
			cache.CounterFieldLike,
			cache.CounterFieldComment,
			cache.CounterFieldFollower,
			cache.CounterFieldFollowing,
		}
		for _, f := range fields {
			go FlushDaemon(context.Background(), f)
		}
		log.Println("[counter-flush] 全部 5 个刷盘守护任务已启动")
	})
}

// FlushDaemon 计数器异步刷盘守护任务
// 触发条件：距上次刷盘 ≥ 60s 或 脏列表大小 ≥ 1000
func FlushDaemon(ctx context.Context, field string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastFlush := time.Now()
	interval := time.Duration(config.ParamViper.GetInt("counter.flush.interval_seconds")) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	threshold := config.ParamViper.GetInt("counter.flush.dirty_threshold")
	if threshold <= 0 {
		threshold = 1000
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := cache.GetDirtyCount(field)
			if err != nil {
				log.Printf("[flush-daemon] 读取脏列表失败 field=%s: %v", field, err)
				continue
			}
			timeUp := time.Since(lastFlush) >= interval
			sizeUp := count >= int64(threshold)

			if timeUp || sizeUp {
				if doFlush(field) {
					lastFlush = time.Now()
				}
			}
		}
	}
}

// doFlush 执行一次刷盘，返回是否成功
func doFlush(field string) bool {
	lockVal := uuid.NewString()
	ok, err := cache.AcquireFlushLock(field, lockVal)
	if err != nil {
		log.Printf("[flush] 获取锁失败 field=%s: %v", field, err)
		return false
	}
	if !ok {
		return false // 其他实例正在刷
	}
	defer cache.ReleaseFlushLock(field, lockVal)

	members, err := cache.GetDirtyMembers(field)
	if err != nil {
		log.Printf("[flush] 读取脏列表失败 field=%s: %v", field, err)
		return false
	}
	if len(members) == 0 {
		return true
	}

	batchSize := config.ParamViper.GetInt("counter.flush.batch_size")
	if batchSize <= 0 {
		batchSize = 500
	}
	retryMax := config.ParamViper.GetInt("counter.flush.retry_max")
	if retryMax <= 0 {
		retryMax = 3
	}

	counterDAO := persistence.NewCounterDAO()
	successIDs := make([]uint64, 0)
	retryMap := make(map[uint64]int)

	for i := 0; i < len(members); i += batchSize {
		end := i + batchSize
		if end > len(members) {
			end = len(members)
		}
		batch := members[i:end]

		for _, id := range batch {
			// like 字段使用 total 直接 SET，避免 base+delta+live 长期漂移
			if field == cache.CounterFieldLike {
				total, err := cache.GetTotalCounter(field, id)
				if err != nil {
					log.Printf("[flush] 读取 total 失败 field=%s id=%d: %v", field, id, err)
					continue
				}
				if err := counterDAO.SetVideoCounter(id, field, total); err != nil {
					log.Printf("[flush] SET 失败 field=%s id=%d total=%d: %v", field, id, total, err)
					rollbackDelta(field, id, 1, retryMap, retryMax) // 轻量重试标记
					continue
				}
				successIDs = append(successIDs, id)
				continue
			}

			delta, err := cache.GetAndClearDelta(field, id)
			if err != nil {
				log.Printf("[flush] GETDEL 失败 field=%s id=%d: %v", field, id, err)
				continue
			}
			if delta == 0 {
				successIDs = append(successIDs, id)
				continue
			}

			// 区分视频维度 vs 用户维度
			var updateErr error
			switch field {
			case cache.CounterFieldFollower, cache.CounterFieldFollowing:
				updateErr = counterDAO.IncrementUserCounter(id, field, delta)
			default:
				updateErr = counterDAO.IncrementVideoCounter(id, field, delta)
			}

			if updateErr != nil {
				log.Printf("[flush] UPDATE 失败 field=%s id=%d delta=%d: %v", field, id, delta, updateErr)
				// 精确路线补救：回滚 delta
				if field != cache.CounterFieldPlay {
					rollbackDelta(field, id, delta, retryMap, retryMax)
				}
			} else {
				successIDs = append(successIDs, id)
				// 刷盘成功后删除 base 缓存与实时计数器
				if err := cache.DelBase(field, id); err != nil {
					log.Printf("[flush] DEL base 失败 field=%s id=%d: %v", field, id, err)
				}
				if err := cache.DelLiveCounter(field, id); err != nil {
					log.Printf("[flush] DEL live counter 失败 field=%s id=%d: %v", field, id, err)
				}
			}
		}
	}

	// 清理脏列表中已成功的 ID
	if len(successIDs) > 0 {
		if err := cache.RemoveDirty(field, successIDs...); err != nil {
			log.Printf("[flush] SREM dirty 失败 field=%s: %v", field, err)
		}
	}

	log.Printf("[flush] 完成 field=%s success=%d total=%d", field, len(successIDs), len(members))
	return true
}

// rollbackDelta 刷盘失败时回滚 delta
func rollbackDelta(field string, id uint64, delta int64, retryMap map[uint64]int, retryMax int) {
	retryMap[id]++
	if retryMap[id] >= retryMax {
		// 死信
		if err := cache.SetDeadLetter(field, id, delta); err != nil {
			log.Printf("[flush] 写入死信失败 field=%s id=%d: %v", field, id, err)
		}
		log.Printf("[flush] 进入死信 field=%s id=%d delta=%d", field, id, delta)
		return
	}

	if err := cache.IncrByDelta(field, id, delta); err != nil {
		log.Printf("[flush] 回滚 delta 失败 field=%s id=%d: %v", field, id, err)
	}
}
