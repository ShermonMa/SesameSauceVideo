/*
 * event.go
 * 功能：Kafka 消息事件结构体与 Topic 常量定义
 * 时间戳：2026-04-26
 */

package messaging

import "time"

// Kafka Topic 常量
const (
	TopicPrivate      = "msg.private"
	TopicReply        = "msg.reply"
	TopicLike         = "msg.like"
	TopicFollow       = "msg.follow"
	TopicSystem       = "msg.system"
	TopicMention      = "msg.mention"
	TopicVideoBasic   = "video.basic"
	TopicVideoComplex = "video.complex"
	TopicLikeState    = "like.state" // 点赞关系异步落库事件
)

// MessageEvent Kafka 消息体 JSON 结构
type MessageEvent struct {
	MsgID      int64     `json:"msg_id"`
	FromUserID int64     `json:"from_user_id"`
	ToUserID   int64     `json:"to_user_id"`
	ActionType int8      `json:"action_type"`
	BizID      int64     `json:"biz_id"`
	BizType    int8      `json:"biz_type"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// VideoBasicEvent video_basic Topic 消息结构，负责基础信息持久化、分类关联与 ES 索引
type VideoBasicEvent struct {
	VideoID      uint64    `json:"video_id"`
	HashID       string    `json:"hashid"`
	CoverHashID  string    `json:"cover_hashid"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	AuthorID     uint64    `json:"author_id"`
	CategoryIDs  []uint64  `json:"category_ids"`
	CoverURL     string    `json:"cover_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// VideoComplexEvent video_complex Topic 消息结构，负责视频转码
type VideoComplexEvent struct {
	VideoID       uint64 `json:"video_id"`
	HashID        string `json:"hashid"`
	RawVideoURL   string `json:"raw_video_url"`
	SourceFormat  string `json:"source_format"`
	HasUserCover  bool   `json:"has_user_cover"`
}

// LikeStateEvent 点赞关系状态变更事件，用于异步落库 likes 表
type LikeStateEvent struct {
	UserID   uint64 `json:"user_id"`
	TargetID uint64 `json:"target_id"`
	BizType  int8   `json:"biz_type"`
	Action   int8   `json:"action"` // 1=点赞 0=取消
	Ts       int64  `json:"ts"`     // 操作时间戳（毫秒）
}
