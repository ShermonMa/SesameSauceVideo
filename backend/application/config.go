/*
 * config.go
 * 功能：application 层配置常量，集中管理视频业务相关的魔法数字
 * 时间戳：2026-04-26
 */

package application

const (
	// DefaultPresignedPutExpiresSeconds 上传预签名 URL 默认过期时间（秒）
	DefaultPresignedPutExpiresSeconds = 3000
	// DefaultPresignedGetExpiresSeconds 播放预签名 URL 默认过期时间（秒）
	DefaultPresignedGetExpiresSeconds = 3600
	// MaxUploadConcurrency 最大并发上传数
	MaxUploadConcurrency = 3
	// MaxTitleLength 标题最大长度
	MaxTitleLength = 128
	// MaxDescriptionLength 描述最大长度
	MaxDescriptionLength = 3000
	// MinCategoryCount 最小分类数量
	MinCategoryCount = 1
	// MaxCategoryCount 最大分类数量
	MaxCategoryCount = 3
	// MaxKeywordLength 搜索关键词最大长度
	MaxKeywordLength = 64
	// DefaultPageSize 默认分页大小
	DefaultPageSize = 20
	// MaxPageSize 最大分页大小
	MaxPageSize = 50
)
