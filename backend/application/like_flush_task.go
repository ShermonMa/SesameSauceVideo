/*
 * like_flush_task.go
 * 功能：点赞关系异步落库的兜底刷盘任务
 *      定期扫描 Redis dirty ZSet，对 Kafka consumer 未及时处理的 pending 状态直接写入 DB
 *      2026-06-22 重构：兜底 flush 直接写 DB 后，通过 InitTotalCounter + AdjustLikeTotal 更新 Redis total；
 *                  不再维护 delta/live 计数器；只扫描视频类型（评论点赞同步）。
 * 时间戳：2026-06-18
 */

package application

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/persistence"
)

var likeFlushOnce sync.Once

// StartLikeFlushTasks 启动全部点赞关系兜底刷盘任务
func StartLikeFlushTasks() {
	likeFlushOnce.Do(func() {
		// 评论点赞是同步落库，不会产生 dirty；只兜底视频点赞
		go likeFlushDaemon(context.Background(), domain.BizTypeVideo)
		log.Println("[like-flush] 点赞关系兜底刷盘任务已启动")
	})
}

// likeFlushDaemon 单个业务类型的兜底刷盘守护任务
func likeFlushDaemon(ctx context.Context, bizType int8) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 默认只处理 60 秒前产生的脏数据，给 Kafka consumer 留足时间
	pendingSeconds := 60

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			beforeTs := time.Now().Add(-time.Duration(pendingSeconds) * time.Second).UnixMilli()
			if err := doLikeFlush(bizType, beforeTs); err != nil {
				log.Printf("[like-flush] 刷盘异常 bizType=%d: %v", bizType, err)
			}
		}
	}
}

// doLikeFlush 执行一次兜底刷盘
func doLikeFlush(bizType int8, beforeTs int64) error {
	lockVal := uuid.NewString()
	ok, err := cache.AcquireLikeFlushLock(bizType, lockVal)
	if err != nil {
		return err
	}
	if !ok {
		return nil // 其他实例正在刷
	}
	defer cache.ReleaseLikeFlushLock(bizType, lockVal)

	items, err := cache.GetDirtyLikes(bizType, beforeTs, 500)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	likeDAO := persistence.NewLikeDAO()
	success := 0
	failed := 0

	for _, item := range items {
		pending, err := cache.GetLikePending(bizType, item.UserID, item.TargetID)
		if err != nil {
			cache.LogLikeFlushError(bizType, item.UserID, item.TargetID, err)
			failed++
			continue
		}
		if pending == nil {
			// 已被 consumer 处理，仅清理 dirty
			if err := cache.RemLikeDirty(bizType, item.UserID, item.TargetID); err != nil {
				log.Printf("[like-flush] 清理 dirty 失败 bizType=%d user=%d target=%d: %v", bizType, item.UserID, item.TargetID, err)
			}
			continue
		}

		// 直接落库
		var changed bool
		var dbErr error
		if pending.State == 1 {
			like := &domain.Like{
				UserID:   item.UserID,
				TargetID: item.TargetID,
				BizType:  bizType,
				VideoID:  item.TargetID, // 视频点赞关联视频ID
			}
			changed, dbErr = likeDAO.CreateOrRestore(like)
		} else {
			changed, dbErr = likeDAO.Cancel(item.UserID, item.TargetID, bizType)
		}

		if dbErr != nil {
			cache.LogLikeFlushError(bizType, item.UserID, item.TargetID, dbErr)
			_ = cache.SetLikeDeadLetter(bizType, item.UserID, item.TargetID, pending.State, pending.Ts)
			failed++
			continue
		}

		// DB 状态确实发生变化时同步更新 Redis 业务状态
		if changed && bizType == domain.BizTypeVideo {
			cuckooKey := cache.VideoLikeCuckooKey(item.TargetID)
			if pending.State == 1 {
				if err := cache.CuckooAdd(cuckooKey, item.UserID); err != nil {
					log.Printf("[like-flush] Cuckoo 添加失败 user=%d target=%d: %v", item.UserID, item.TargetID, err)
				}
				if _, err := cache.InitTotalCounter(cache.CounterFieldLike, item.TargetID, func() (int64, error) {
					return likeDAO.CountByTarget(item.TargetID, domain.BizTypeVideo)
				}); err != nil {
					log.Printf("[like-flush] 初始化 total 失败 target=%d: %v", item.TargetID, err)
				} else if _, err := cache.AdjustLikeTotal(item.TargetID, 1); err != nil {
					log.Printf("[like-flush] 递增 total 失败 target=%d: %v", item.TargetID, err)
				}
			} else {
				if err := cache.CuckooDel(cuckooKey, item.UserID); err != nil {
					log.Printf("[like-flush] 删除 Cuckoo 失败 user=%d target=%d: %v", item.UserID, item.TargetID, err)
				}
				if _, err := cache.InitTotalCounter(cache.CounterFieldLike, item.TargetID, func() (int64, error) {
					return likeDAO.CountByTarget(item.TargetID, domain.BizTypeVideo)
				}); err != nil {
					log.Printf("[like-flush] 初始化 total 失败 target=%d: %v", item.TargetID, err)
				} else if _, err := cache.AdjustLikeTotal(item.TargetID, -1); err != nil {
					log.Printf("[like-flush] 递减 total 失败 target=%d: %v", item.TargetID, err)
				}
			}
		}

		// 成功后清理 Redis
		if err := cache.DelLikePending(bizType, item.UserID, item.TargetID); err != nil {
			log.Printf("[like-flush] 删除 pending 失败 bizType=%d user=%d target=%d: %v", bizType, item.UserID, item.TargetID, err)
		}
		if err := cache.RemLikeDirty(bizType, item.UserID, item.TargetID); err != nil {
			log.Printf("[like-flush] 移除 dirty 失败 bizType=%d user=%d target=%d: %v", bizType, item.UserID, item.TargetID, err)
		}
		success++
	}

	log.Printf("[like-flush] 完成 bizType=%d success=%d failed=%d total=%d", bizType, success, failed, len(items))
	return nil
}
