/*
 * progress_aggregate_task.go
 * 功能：播放进度聚合任务，每固定周期扫描 Redis 中的进度缓存并批量刷入 MySQL
 *        支持分布式锁避免多实例重复刷盘
 * 时间戳：2026-04-27
 */

package persistence

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/redis"
)

var (
	aggregateOnce sync.Once
	playHistoryDAO *PlayHistoryDAO
)

// StartProgressAggregateTask 启动聚合任务，全局仅执行一次
func StartProgressAggregateTask() {
	aggregateOnce.Do(func() {
		playHistoryDAO = NewPlayHistoryDAO()
		go progressAggregateWorker()
	})
}

// progressAggregateWorker 周期性执行聚合刷盘
func progressAggregateWorker() {
	interval := config.ParamViper.GetInt("progress.aggregate_interval_minutes")
	if interval <= 0 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	defer ticker.Stop()

	// 首次立即执行一次
	flushProgressToDB()

	for {
		select {
		case <-ticker.C:
			flushProgressToDB()
		}
	}
}

// flushProgressToDB 获取分布式锁，扫描 Redis 并批量写入 MySQL
func flushProgressToDB() {
	lockID := uuid.New().String()
	// 锁过期时间设为间隔的 2 倍，防止任务卡死导致锁不释放
	lockExpire := config.ParamViper.GetInt("progress.aggregate_interval_minutes")
	if lockExpire <= 0 {
		lockExpire = 5
	}
	lockExpire *= 120 // 转换为秒并留 2 倍余量

	acquired, err := redis.AcquireAggregateLock(lockID, lockExpire)
	if err != nil {
		log.Printf("[progress_aggregate] 获取分布式锁失败: %v", err)
		return
	}
	if !acquired {
		log.Println("[progress_aggregate] 未获取到分布式锁，跳过本次聚合")
		return
	}
	defer func() {
		if err := redis.ReleaseAggregateLock(lockID); err != nil {
			log.Printf("[progress_aggregate] 释放分布式锁失败: %v", err)
		}
	}()

	records, err := redis.ScanAllProgress()
	if err != nil {
		log.Printf("[progress_aggregate] 扫描 Redis 失败: %v", err)
		return
	}
	if len(records) == 0 {
		return
	}

	histories := make([]domain.PlayHistory, 0, len(records))
	for _, r := range records {
		histories = append(histories, domain.PlayHistory{
			UserID:   r.UserID,
			VideoID:  r.VideoID,
			Progress: r.Progress,
			PlayedAt: time.Now(),
		})
	}

	if err := playHistoryDAO.BatchUpsert(histories); err != nil {
		log.Printf("[progress_aggregate] 批量写入 MySQL 失败: %v", err)
		return
	}

	log.Printf("[progress_aggregate] 成功刷盘 %d 条记录", len(histories))
}
