/*
 * main.go
 * 功能：SesameSauce 定时任务入口，每日清理 24h 内未被 confirm 的幽灵视频数据；将 ES 初始化改为启动强依赖
 * 时间戳：2026-06-19
 */

package main

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/infra/elasticsearch"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/infra/storage"
	"github.com/spf13/viper"
)

func main() {
	// 加载配置文件
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../../configs/configuration")
	viper.AddConfigPath("../../../configs/configuration")
	viper.AddConfigPath("./configs/configuration")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("配置文件加载失败: %v", err)
	}

	// 初始化数据库
	if err := config.InitDatabase(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化 MinIO
	if err := config.InitMinio(); err != nil {
		log.Fatalf("MinIO 初始化失败: %v", err)
	}

	// 初始化 Elasticsearch（启动强依赖，失败即退出）
	esAddresses := viper.GetStringSlice("elasticsearch.addresses")
	if len(esAddresses) == 0 {
		esAddresses = []string{"http://localhost:9200"}
	}
	if err := elasticsearch.InitES(esAddresses); err != nil {
		log.Fatalf("Elasticsearch 初始化失败: %v", err)
	}

	c := cron.New(cron.WithLocation(time.Local))
	// 每日 03:00 执行清理
	_, err := c.AddFunc("0 3 * * *", cleanupUnconfirmedRaw)
	if err != nil {
		log.Fatalf("定时任务注册失败: %v", err)
	}

	log.Println("定时任务已启动，每日 03:00 执行幽灵数据清理")
	c.Run()
}

func cleanupUnconfirmedRaw() {
	log.Println("[cleanup] 开始执行幽灵数据清理...")
	videoDAO := persistence.NewVideoDAO()
	videoStorage := storage.NewMinIOVideoStorage()

	before := time.Now().Add(-24 * time.Hour)
	videos, err := videoDAO.ListUnconfirmedBefore(before)
	if err != nil {
		log.Printf("[cleanup] 查询幽灵数据失败: %v", err)
		return
	}

	if len(videos) == 0 {
		log.Println("[cleanup] 未发现幽灵数据")
		return
	}

	var deletedCount int
	var freedEstimate int64

	ctx := context.Background()
	for _, v := range videos {
		log.Printf("[cleanup] 清理 video_id=%d hashid=%s", v.ID, v.HashID)

		// 删除 MinIO raw 文件
		if err := videoStorage.DeleteRaw(ctx, v.ID); err != nil {
			log.Printf("[cleanup] 删除 raw 失败 video=%d: %v", v.ID, err)
		}

		// 删除 MinIO 封面
		if err := videoStorage.DeleteCover(ctx, v.CoverHashID); err != nil {
			log.Printf("[cleanup] 删除封面失败 video=%d cover_hashid=%s: %v", v.ID, v.CoverHashID, err)
		}

		// 删除 ES 索引
		if v.HashID != "" {
			if err := elasticsearch.DeleteVideo(ctx, v.HashID); err != nil {
				log.Printf("[cleanup] 删除 ES 索引失败 video=%s: %v", v.HashID, err)
			}
		}

		// 物理删除视频记录
		if err := videoDAO.DeleteByID(v.ID); err != nil {
			log.Printf("[cleanup] 删除数据库记录失败 video=%d: %v", v.ID, err)
			continue
		}

		deletedCount++
		if v.FileSize != nil {
			freedEstimate += *v.FileSize
		}
	}

	log.Printf("[cleanup] 完成：删除记录 %d 条，估算释放存储 %d MB", deletedCount, freedEstimate/1024/1024)
}
