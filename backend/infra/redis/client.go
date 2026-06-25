/*
 * client.go
 * 功能：Redis 客户端初始化与管理，提供全局单例访问；新增启动后定时健康检查并记录异常日志
 * 时间戳：2026-06-19
 */

package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// Client 是全局 Redis 客户端实例
var Client *redis.Client

// InitRedis 读取 config.yaml 中的 redis 配置并初始化全局客户端
func InitRedis() error {
	host := viper.GetString("redis.host")
	if host == "" {
		host = "localhost"
	}
	port := viper.GetString("redis.port")
	if port == "" {
		port = "16379"
	}
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")

	Client = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	// 健康检查
	if err := Client.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("Redis 连接失败: %w", err)
	}

	log.Println("Redis 初始化成功")

	// 启动后定期健康检查，异常时仅记录日志
	go startHealthCheck()

	return nil
}

// startHealthCheck 定时 Ping Redis，运行中连接异常时输出日志但不退出
func startHealthCheck() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := Client.Ping(context.Background()).Err(); err != nil {
			log.Printf("[redis-health] Redis 连接异常: %v", err)
		}
	}
}
