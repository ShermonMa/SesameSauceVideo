/*
 * kafka.go
 * 功能：Kafka 连接配置初始化，基于 segmentio/kafka-go；新增启动时 broker 连通性检测
 * 时间戳：2026-06-19
 */

package config

import (
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
)

// KafkaWriter 全局 Kafka 生产者实例
var KafkaWriter *kafka.Writer

// KafkaBroker Kafka  broker 地址列表，用于创建 Reader
var KafkaBroker []string

// InitKafka 从配置文件中读取 Kafka 配置并初始化 Writer
func InitKafka() error {
	brokers := viper.GetStringSlice("kafka.brokers")
	if len(brokers) == 0 {
		brokers = []string{"localhost:9092"}
	}
	KafkaBroker = brokers

	KafkaWriter = &kafka.Writer{
		Addr:         kafka.TCP(KafkaBroker...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		BatchTimeout: 100 * time.Millisecond,
	}

	// 启动时检测至少一个 broker 是否可达
	if err := checkBrokerReachable(KafkaBroker); err != nil {
		return fmt.Errorf("Kafka broker 不可达: %w", err)
	}

	log.Printf("Kafka 初始化完成，brokers: %v", KafkaBroker)
	return nil
}

// checkBrokerReachable 尝试连接第一个 broker，用于启动时健康检查
func checkBrokerReachable(brokers []string) error {
	if len(brokers) == 0 {
		return fmt.Errorf("broker 列表为空")
	}
	conn, err := kafka.Dial("tcp", brokers[0])
	if err != nil {
		return err
	}
	if err := conn.Close(); err != nil {
		log.Printf("Kafka 检测连接关闭失败: %v", err)
	}
	return nil
}

// NewKafkaReader 创建指定 topic 和 consumer group 的 Reader
func NewKafkaReader(topic, groupID string) *kafka.Reader {
	log.Printf("[kafka-diagnose] 创建 Reader: brokers=%v topic=%s groupID=%s", KafkaBroker, topic, groupID)
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:        KafkaBroker,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: time.Second,
	})
}

// CloseKafka 优雅关闭 Kafka Writer
func CloseKafka() {
	if KafkaWriter != nil {
		if err := KafkaWriter.Close(); err != nil {
			log.Printf("Kafka Writer 关闭失败: %v", err)
		}
	}
}
