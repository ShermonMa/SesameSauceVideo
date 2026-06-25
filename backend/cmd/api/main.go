/*
 * main.go
 * 功能：SesameSauce 后端 API 服务入口，初始化数据库、MinIO、Hashids、ES、路由并启动服务；新增 Kafka 视频消费者，并将 ES 初始化改为启动强依赖
 *      2026-06-21 视频详情接口使用 OptionalJWTAuth，支持登录态下返回当前用户点赞/播放进度，未登录仍可匿名访问
 * 时间戳：2026-06-19
 */

package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/shermon/SesameSauce/application"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/handler"
	"github.com/shermon/SesameSauce/handler/middleware"
	"github.com/shermon/SesameSauce/infra/elasticsearch"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/infra/redis"
	"github.com/shermon/SesameSauce/infra/seed"
	"github.com/spf13/viper"
)

func main() {
	// 加载配置文件
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("../configs/configuration")
	viper.AddConfigPath("../../../configs/configuration")
	viper.AddConfigPath("./configs/configuration")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("警告: 未找到配置文件，将使用默认配置: %v", err)
	}

	// 加载业务参数配置
	config.ParamViper = viper.New()
	config.ParamViper.SetConfigName("parameters")
	config.ParamViper.SetConfigType("yaml")
	config.ParamViper.AddConfigPath("../configs/parameters")
	config.ParamViper.AddConfigPath("../../../configs/parameters")
	config.ParamViper.AddConfigPath("./configs/parameters")
	config.ParamViper.AddConfigPath(".")
	if err := config.ParamViper.ReadInConfig(); err != nil {
		log.Printf("警告: 未找到参数配置文件，将使用默认配置: %v", err)
	}

	// 初始化数据库
	if err := config.InitDatabase(); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化默认分类数据
	seed.SeedCategories()

	// 初始化 MinIO
	if err := config.InitMinio(); err != nil {
		log.Fatalf("MinIO 初始化失败: %v", err)
	}

	// 初始化 Redis
	if err := redis.InitRedis(); err != nil {
		log.Fatalf("Redis 初始化失败: %v", err)
	}

	// 启动计数器刷盘守护任务（替代旧 play_count_queue）
	application.StartCounterFlushTasks()

	// 启动点赞关系兜底刷盘任务
	application.StartLikeFlushTasks()

	// 启动每日对账任务
	application.StartCounterReconcileTask()

	// 启动播放进度聚合任务
	persistence.StartProgressAggregateTask()

	// 初始化 Kafka
	if err := config.InitKafka(); err != nil {
		log.Fatalf("Kafka 初始化失败: %v", err)
	}
	defer config.CloseKafka()

	// 启动 Kafka 消息消费者
	messaging.StartMessageConsumer()
	messaging.StartVideoBasicConsumer()
	messaging.StartVideoComplexConsumer()
	messaging.StartLikeStateConsumer()

	// 初始化 Hashids
	hashidsSalt := viper.GetString("hashids.salt")
	if hashidsSalt == "" {
		hashidsSalt = "sesame-sauce-secret-salt-2026"
	}
	hashidsMinLen := viper.GetInt("hashids.min_length")
	if hashidsMinLen <= 0 {
		hashidsMinLen = 6
	}
	if err := hashids.InitEncoder(hashidsSalt, hashidsMinLen); err != nil {
		log.Fatalf("Hashids 初始化失败: %v", err)
	}

	// 初始化 Elasticsearch（启动强依赖，失败即退出）
	esAddresses := viper.GetStringSlice("elasticsearch.addresses")
	if len(esAddresses) == 0 {
		esAddresses = []string{"http://localhost:9200"}
	}
	if err := elasticsearch.InitES(esAddresses); err != nil {
		log.Fatalf("Elasticsearch 初始化失败: %v", err)
	}

	// 初始化 Gin
	r := gin.Default()

	// 全局 CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 注册路由
	userHandler := handler.NewUserHandler()
	videoHandler := handler.NewVideoHandler()
	commentHandler := handler.NewCommentHandler()
	messageHandler := handler.NewMessageHandler()
	blacklistHandler := handler.NewBlacklistHandler()
	followHandler := handler.NewFollowHandler()
	betaHandler := handler.NewBetaHandler()

	// 路由表
	apiV1 := r.Group("/api/v1")
	apiV1.Use(middleware.BetaAuth())
	{
		apiV1.POST("/user/register", userHandler.Register)
		apiV1.POST("/user/login", userHandler.Login)
		apiV1.GET("/users/:id", middleware.OptionalJWTAuth(), userHandler.GetUserProfile)
		apiV1.GET("/users/:id/videos", videoHandler.ListUserVideos)
		apiV1.GET("/categories", videoHandler.ListCategories)
		apiV1.GET("/videos", videoHandler.ListVideos)
		apiV1.GET("/videos/:video_id", middleware.OptionalJWTAuth(), videoHandler.GetVideo)
		apiV1.GET("/videos/:video_id/play", videoHandler.GetPlayUrl)
		apiV1.GET("/videos/:video_id/status", videoHandler.GetVideoStatus)

		// 内测准入接口（无需登录，但需通过 BetaAuth 中间件内部豁免）
		apiV1.POST("/beta/verify", betaHandler.Verify)
		apiV1.GET("/beta/check", betaHandler.Check)

		// 公开评论接口（支持未登录浏览，登录后启用拉黑过滤与点赞状态）
		apiV1.GET("/comment/list", middleware.OptionalJWTAuth(), commentHandler.ListComments)
		apiV1.GET("/comment/replies", commentHandler.ListReplies)

		// 需要登录的路由
		authorized := apiV1.Group("")
		authorized.Use(middleware.JWTAuth())
		{
			authorized.POST("/videos", videoHandler.CreateVideo)
			authorized.POST("/videos/:video_id/confirm", videoHandler.ConfirmUpload)
			authorized.POST("/videos/:video_id/cancel", videoHandler.CancelUpload)
			authorized.POST("/videos/:video_id/progress", videoHandler.ReportProgress)
			authorized.GET("/users/me/videos", videoHandler.ListMyVideos)

			// 评论
			authorized.POST("/comment/publish", commentHandler.PublishComment)
			authorized.POST("/comment/like", commentHandler.LikeComment)

			// 消息/信箱
			authorized.POST("/message/private", messageHandler.SendPrivate)
			authorized.GET("/message/unread-count", messageHandler.GetUnreadCount)
			authorized.GET("/message/list", messageHandler.ListMessages)
			authorized.POST("/message/read", messageHandler.MarkRead)
			authorized.GET("/message/system-list", messageHandler.ListSystemNotifications)
			authorized.POST("/message/system-read", messageHandler.MarkSystemRead)

			// 关注
			authorized.POST("/follow", followHandler.FollowUser)

			// 视频点赞
			authorized.POST("/videos/:video_id/like", videoHandler.LikeVideo)

			// 拉黑
			authorized.POST("/blacklist/block", blacklistHandler.BlockUser)
			authorized.POST("/blacklist/unblock", blacklistHandler.UnblockUser)
		}
	}

	// 启动服务
	log.Println("服务启动于 :18565")
	if err := r.Run(":18565"); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
