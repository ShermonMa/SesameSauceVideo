/*
 * producer.go
 * 功能：Kafka 消息生产者封装，提供按类型发送消息的能力
 *      2026-06-20 like.state 事件增加 Message Key（userID:targetID），保证同一用户同一视频的事件顺序消费
 * 时间戳：2026-04-26
 */

package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/shermon/SesameSauce/config"
)

// actionTypeToTopic 将 action_type 映射到对应 Kafka Topic
func actionTypeToTopic(actionType int8) string {
	switch actionType {
	case 1:
		return TopicPrivate
	case 2:
		return TopicFollow
	case 3:
		return TopicLike
	case 5:
		return TopicReply
	case 7:
		return TopicSystem
	case 6:
		return TopicMention
	default:
		return TopicPrivate
	}
}

// SendMessageEvent 发送消息事件到 Kafka
func SendMessageEvent(event MessageEvent) error {
	if config.KafkaWriter == nil {
		log.Println("[kafka-producer] KafkaWriter 未初始化，跳过发送")
		return nil
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	topic := actionTypeToTopic(event.ActionType)
	msg := kafka.Message{
		Topic: topic,
		Value: value,
		Time:  time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := config.KafkaWriter.WriteMessages(ctx, msg); err != nil {
		log.Printf("[kafka-producer] 发送消息失败 topic=%s err=%v", topic, err)
		return err
	}

	return nil
}

// SendVideoBasic 发送 video_basic 事件到 Kafka
func SendVideoBasic(event VideoBasicEvent) error {
	if config.KafkaWriter == nil {
		return errors.New("KafkaWriter 未初始化")
	}
	value, err := json.Marshal(event)
	if err != nil {
		return err
	}
	msg := kafka.Message{
		Topic: TopicVideoBasic,
		Value: value,
		Time:  time.Now(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := config.KafkaWriter.WriteMessages(ctx, msg); err != nil {
		log.Printf("[kafka-producer] 发送 video_basic 失败: %v", err)
		return err
	}
	return nil
}

// SendVideoComplex 发送 video_complex 事件到 Kafka
func SendVideoComplex(event VideoComplexEvent) error {
	if config.KafkaWriter == nil {
		return errors.New("KafkaWriter 未初始化")
	}
	value, err := json.Marshal(event)
	if err != nil {
		return err
	}
	msg := kafka.Message{
		Topic: TopicVideoComplex,
		Value: value,
		Time:  time.Now(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := config.KafkaWriter.WriteMessages(ctx, msg); err != nil {
		log.Printf("[kafka-producer] 发送 video_complex 失败: %v", err)
		return err
	}
	return nil
}

// SendLikeStateEvent 发送点赞关系状态变更事件到 Kafka
func SendLikeStateEvent(event LikeStateEvent) error {
	if config.KafkaWriter == nil {
		log.Println("[kafka-producer] KafkaWriter 未初始化，跳过发送 like.state")
		return nil
	}

	value, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: TopicLikeState,
		Key:   []byte(fmt.Sprintf("%d:%d", event.UserID, event.TargetID)),
		Value: value,
		Time:  time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := config.KafkaWriter.WriteMessages(ctx, msg); err != nil {
		log.Printf("[kafka-producer] 发送 like.state 失败: %v", err)
		return err
	}
	return nil
}
