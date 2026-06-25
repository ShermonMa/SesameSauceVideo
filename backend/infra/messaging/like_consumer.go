/*
 * like_consumer.go
 * 功能：Kafka like.state Topic 消费者，消费点赞关系事件后同步更新 MySQL likes 表与 Redis total 缓存
 *      通过 Redis pending 状态实现幂等与去重
 *      2026-06-22 重构：服务层只投递事件，消费侧统一修改 DB + Redis；
 *                  使用 InitTotalCounter + AdjustLikeTotal 保证 total 计数器一致性；
 *                  删除 RecreateLike hack，依赖 CreateOrRestore 返回的 changed 做幂等。
 * 时间戳：2026-06-18
 */

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// StartLikeStateConsumer 启动点赞关系状态消费者
func StartLikeStateConsumer() {
	go runLikeStateConsumer()
}

func runLikeStateConsumer() {
	reader := config.NewKafkaReader(TopicLikeState, "sesame-sauce-like-state-consumer")
	defer reader.Close()

	likeDAO := persistence.NewLikeDAO()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				log.Printf("[like-consumer] 读取消息失败(网络错误): temporary=%v timeout=%v err=%v", netErr.Temporary(), netErr.Timeout(), err)
			} else {
				log.Printf("[like-consumer] 读取消息失败: err=%v", err)
			}
			time.Sleep(time.Second)
			continue
		}

		var event LikeStateEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("[like-consumer] 消息反序列化失败: %v", err)
			continue
		}

		if err := persistLikeState(likeDAO, event); err != nil {
			log.Printf("[like-consumer] 处理 like.state 失败 user=%d target=%d action=%d: %v", event.UserID, event.TargetID, event.Action, err)
			// 不提交 offset，等待 Kafka 重试
			continue
		}
	}
}

// persistLikeState 将单个 like.state 事件持久化到 DB，并在 DB 状态确实发生变化时同步更新 Redis total
func persistLikeState(likeDAO *persistence.LikeDAO, event LikeStateEvent) error {
	pending, err := cache.GetLikePending(event.BizType, event.UserID, event.TargetID)
	if err != nil {
		log.Printf("[like-consumer] 读取 pending 失败 user=%d target=%d: %v", event.UserID, event.TargetID, err)
	}
	matched := pending != nil && pending.Ts == event.Ts && pending.State == event.Action

	var changed bool
	if event.Action == 1 {
		like := &domain.Like{
			UserID:   event.UserID,
			TargetID: event.TargetID,
			BizType:  event.BizType,
				VideoID:  event.TargetID, // 视频点赞关联视频ID
		}
		changed, err = likeDAO.CreateOrRestore(like)
	} else {
		changed, err = likeDAO.Cancel(event.UserID, event.TargetID, event.BizType)
	}
	if err != nil {
		return err
	}

	// DB 状态确实发生变化时才更新 Redis 业务状态，保证幂等性
	if event.BizType == domain.BizTypeVideo {
		cuckooKey := cache.VideoLikeCuckooKey(event.TargetID)
		if event.Action == 1 {
			if err := cache.CuckooAdd(cuckooKey, event.UserID); err != nil {
				log.Printf("[like-consumer] Cuckoo 添加失败 user=%d target=%d: %v", event.UserID, event.TargetID, err)
			}
			if changed {
				if _, err := cache.InitTotalCounter(cache.CounterFieldLike, event.TargetID, func() (int64, error) {
					return likeDAO.CountByTarget(event.TargetID, domain.BizTypeVideo)
				}); err != nil {
					log.Printf("[like-consumer] 初始化 total 失败 target=%d: %v", event.TargetID, err)
				} else if _, err := cache.AdjustLikeTotal(event.TargetID, 1); err != nil {
					log.Printf("[like-consumer] 递增 total 失败 target=%d: %v", event.TargetID, err)
				}
			}
		} else if changed {
			if err := cache.CuckooDel(cuckooKey, event.UserID); err != nil {
				log.Printf("[like-consumer] 删除 Cuckoo 失败 user=%d target=%d: %v", event.UserID, event.TargetID, err)
			}
			if _, err := cache.InitTotalCounter(cache.CounterFieldLike, event.TargetID, func() (int64, error) {
				return likeDAO.CountByTarget(event.TargetID, domain.BizTypeVideo)
			}); err != nil {
				log.Printf("[like-consumer] 初始化 total 失败 target=%d: %v", event.TargetID, err)
			} else if _, err := cache.AdjustLikeTotal(event.TargetID, -1); err != nil {
				log.Printf("[like-consumer] 递减 total 失败 target=%d: %v", event.TargetID, err)
			}
		}
	}

	// 只有 pending 与当前事件匹配时才清理，避免误删新事件的 pending
	if matched {
		if err := cache.DelLikePending(event.BizType, event.UserID, event.TargetID); err != nil {
			log.Printf("[like-consumer] 删除 pending 失败 user=%d target=%d: %v", event.UserID, event.TargetID, err)
		}
		if err := cache.RemLikeDirty(event.BizType, event.UserID, event.TargetID); err != nil {
			log.Printf("[like-consumer] 移除 dirty 失败 user=%d target=%d: %v", event.UserID, event.TargetID, err)
		}
	}
	return nil
}
