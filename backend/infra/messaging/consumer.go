/*
 * consumer.go
 * 功能：Kafka 统一消费者，按 ActionType 路由写入 messages 表，支持有限重试
 * 时间戳：2026-04-26
 */

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// StartMessageConsumer 启动统一消息消费者
func StartMessageConsumer() {
	go runMessageConsumer()
}

func runMessageConsumer() {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     config.KafkaBroker,
		GroupID:     "sesame-sauce-message-consumer",
		GroupTopics: []string{TopicReply, TopicLike, TopicFollow, TopicSystem},
		MinBytes:    1,
		MaxBytes:    10e6,
	})
	defer reader.Close()

	msgDAO := persistence.NewMessageDAO()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) {
				log.Printf("[kafka-consumer] 读取消息失败(网络错误): temporary=%v timeout=%v err=%v", netErr.Temporary(), netErr.Timeout(), err)
			} else {
				log.Printf("[kafka-consumer] 读取消息失败: 类型=%T err=%v", err, err)
			}
			time.Sleep(time.Second)
			continue
		}

		var event MessageEvent
		if err := json.Unmarshal(m.Value, &event); err != nil {
			log.Printf("[kafka-consumer] 消息反序列化失败: %v", err)
			continue
		}

		// 有限重试：最多 3 次
		var lastErr error
		for i := 0; i < 3; i++ {
			lastErr = persistMessage(msgDAO, event)
			if lastErr == nil {
				break
			}
			log.Printf("[kafka-consumer] 持久化消息重试 %d/3: %v", i+1, lastErr)
			time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
		}
		if lastErr != nil {
			log.Printf("[kafka-consumer] 持久化消息最终失败，丢弃: event=%+v err=%v", event, lastErr)
		}
	}
}

func persistMessage(msgDAO *persistence.MessageDAO, event MessageEvent) error {
	msg := &domain.Message{
		FromUserID: uint64(event.FromUserID),
		ToUserID:   uint64(event.ToUserID),
		Content:    event.Content,
		ActionType: event.ActionType,
		IsRead:     0,
		IsDeleted:  0,
	}
	if event.BizID > 0 {
		bid := uint64(event.BizID)
		msg.BizID = &bid
	}
	if event.BizType > 0 {
		bt := event.BizType
		msg.BizType = &bt
	}

	return msgDAO.Create(msg)
}
