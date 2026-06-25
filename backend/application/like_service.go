/*
 * like_service.go
 * 功能：点赞应用服务，支持视频点赞/取消点赞、评论点赞/取消点赞；
 *      视频点赞改为 Kafka 异步落库：服务层只负责校验、投递 like.state 事件与维护 pending/dirty 协调元数据；
 *      Kafka 消费侧/兜底 flush 才修改 MySQL likes 表与 Redis total 计数器，保证 Redis 数据一致性；
 *      评论点赞保持同步，且 likes 记录与 comments.like_count 在同一事务内完成；
 *      返回值：基于当前 Redis total 的期望点赞数（前端在收到后端确认后更新，避免提前乐观显示）。
 * 时间戳：2026-06-22
 */

package application

import (
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
	"gorm.io/gorm"
)

// LikeService 处理点赞相关的应用层业务逻辑
type LikeService struct {
	likeDAO      *persistence.LikeDAO
	commentDAO   *persistence.CommentDAO
	blacklistDAO *persistence.BlacklistDAO
	videoDAO     *persistence.VideoDAO
}

// NewLikeService 创建 LikeService 实例
func NewLikeService() *LikeService {
	return &LikeService{
		likeDAO:      persistence.NewLikeDAO(),
		commentDAO:   persistence.NewCommentDAO(),
		blacklistDAO: persistence.NewBlacklistDAO(),
		videoDAO:     persistence.NewVideoDAO(),
	}
}

// LikeVideo 点赞视频（Kafka 异步落库路线）
// 写路径：校验 -> Kafka like.state -> consumer 同步更新 MySQL + Redis total
// 判重：Cuckoo Filter 快速路径 + Redis pending + DB 兜底
// 并发控制：per-user-per-video 分布式锁，确保同一用户同一视频串行操作
// 返回值：基于当前 Redis total 的期望点赞数
func (s *LikeService) LikeVideo(userID, videoID uint64) (int64, error) {
	bizType := domain.BizTypeVideo

	// 1. 获取分布式锁，防止并发点赞/取消点赞
	lockVal := uuid.NewString()
	locked, err := cache.AcquireLikeUserLock(bizType, userID, videoID, lockVal)
	if err != nil {
		log.Printf("[like-service] 获取点赞锁失败 user=%d video=%d: %v", userID, videoID, err)
		return 0, errors.New("操作过于频繁")
	}
	if !locked {
		return 0, errors.New("操作过于频繁")
	}
	defer cache.ReleaseLikeUserLock(bizType, userID, videoID, lockVal)

	ts := time.Now().UnixMilli()
	cuckooKey := cache.VideoLikeCuckooKey(videoID)
	cache.EnsureCuckooReserved(cuckooKey)

	currentCount := s.GetVideoLikeCount(videoID)

	// 2. Cuckoo Filter 快速判重
	exists, _ := cache.CuckooExists(cuckooKey, userID)
	if !exists {
		// 确定未点赞；但 pending 中可能有未落库的点赞
		pending, _ := cache.GetLikePending(bizType, userID, videoID)
		if pending != nil && pending.State == 1 {
			return currentCount, nil // 已点赞，幂等
		}
	} else {
		// Cuckoo 存在说明大概率已点赞；pending 为最新状态，DB 兜底
		pending, _ := cache.GetLikePending(bizType, userID, videoID)
		if pending != nil {
			if pending.State == 1 {
				return currentCount, nil // 已点赞，幂等
			}
			// state=0 表示待取消，允许重新点赞
		} else {
			// pending 已清理，回查 DB 兜底
			existing, err := s.likeDAO.GetByUserAndTarget(userID, videoID, bizType)
			if err == nil && existing.Cancel == 0 {
				return currentCount, nil // DB 已有点赞记录，幂等
			}
			if err == nil && existing.Cancel == 1 {
				// 已取消（软删除兼容）：允许重新点赞，清理残留 Cuckoo
				_ = cache.CuckooDel(cuckooKey, userID)
			} else if err != nil {
				// DB 记录不存在（已物理删除）：允许重新点赞，清理残留 Cuckoo
				_ = cache.CuckooDel(cuckooKey, userID)
			}
		}
	}

	// 3. 发送 Kafka 落库事件；Kafka 是点赞关系持久化的第一落点
	if err := messaging.SendLikeStateEvent(messaging.LikeStateEvent{
		UserID:   userID,
		TargetID: videoID,
		BizType:  bizType,
		Action:   1,
		Ts:       ts,
	}); err != nil {
		log.Printf("[like-service] 发送 like.state 失败 user=%d video=%d: %v", userID, videoID, err)
		return currentCount, errors.New("点赞失败")
	}

	// 4. 写入 Redis pending + dirty，用于 consumer 幂等与兜底 flush
	if err := cache.SetLikePending(bizType, userID, videoID, 1, ts); err != nil {
		return currentCount, errors.New("点赞失败")
	}
	if err := cache.AddLikeDirty(bizType, userID, videoID, ts); err != nil {
		log.Printf("[like-service] 加入 dirty 失败 user=%d video=%d: %v", userID, videoID, err)
	}

	// 5. 获取视频作者发送通知（通知走独立 msg.like topic，不阻塞点赞主流程）
	video, err := s.videoDAO.GetByID(videoID)
	if err == nil {
		isBlocked, _ := s.blacklistDAO.IsBlocked(video.AuthorID, userID)
		if !isBlocked {
			event := messaging.MessageEvent{
				FromUserID: int64(userID),
				ToUserID:   int64(video.AuthorID),
				ActionType: domain.ActionTypeLike,
				BizID:      int64(videoID),
				BizType:    domain.BizTypeVideo,
				Content:    "赞了你的视频",
				CreatedAt:  time.Now(),
			}
			if err := messaging.SendMessageEvent(event); err != nil {
				log.Printf("[like-service] 发送点赞消息事件失败: %v", err)
			}
		}
	}

	expected := currentCount + 1
	log.Printf("[like-service] LikeVideo result user=%d video=%d currentCount=%d expected=%d", userID, videoID, currentCount, expected)
	return expected, nil
}

// UnlikeVideo 取消点赞视频（Kafka 异步落库路线）
// 判重：Redis pending + Cuckoo Filter + DB 兜底，确认点过赞才投递取消事件
// 并发控制：per-user-per-video 分布式锁，确保同一用户同一视频串行操作
// 返回值：基于当前 Redis total 的期望点赞数
func (s *LikeService) UnlikeVideo(userID, videoID uint64) (int64, error) {
	bizType := domain.BizTypeVideo

	// 1. 获取分布式锁，防止并发点赞/取消点赞
	lockVal := uuid.NewString()
	locked, err := cache.AcquireLikeUserLock(bizType, userID, videoID, lockVal)
	if err != nil {
		log.Printf("[like-service] 获取取消点赞锁失败 user=%d video=%d: %v", userID, videoID, err)
		return 0, errors.New("操作过于频繁")
	}
	if !locked {
		return 0, errors.New("操作过于频繁")
	}
	defer cache.ReleaseLikeUserLock(bizType, userID, videoID, lockVal)

	ts := time.Now().UnixMilli()
	cuckooKey := cache.VideoLikeCuckooKey(videoID)

	currentCount := s.GetVideoLikeCount(videoID)

	// 2. 判重：优先读 Redis pending
	pending, _ := cache.GetLikePending(bizType, userID, videoID)
	if pending != nil {
		if pending.State == 0 {
			return currentCount, nil // 已取消，幂等
		}
		// state=1 表示待点赞，现在改为取消
	} else {
		// pending 不存在，通过 Cuckoo 快速排除；Cuckoo 存在再回查 DB
		exists, _ := cache.CuckooExists(cuckooKey, userID)
		if !exists {
			return currentCount, nil // 确定未点赞，幂等
		}
		existing, err := s.likeDAO.GetByUserAndTarget(userID, videoID, bizType)
		if err == nil && existing.Cancel == 1 {
			return currentCount, nil // 已取消，幂等
		}
		// DB 不存在但 Cuckoo 存在：说明点赞尚未落库，仍可取消
	}

	// 3. 发送 Kafka 取消事件
	if err := messaging.SendLikeStateEvent(messaging.LikeStateEvent{
		UserID:   userID,
		TargetID: videoID,
		BizType:  bizType,
		Action:   0,
		Ts:       ts,
	}); err != nil {
		log.Printf("[like-service] 发送 like.state 失败 user=%d video=%d: %v", userID, videoID, err)
		return currentCount, errors.New("取消点赞失败")
	}

	// 4. 写入 Redis pending state=0
	if err := cache.SetLikePending(bizType, userID, videoID, 0, ts); err != nil {
		return currentCount, errors.New("取消点赞失败")
	}
	if err := cache.AddLikeDirty(bizType, userID, videoID, ts); err != nil {
		log.Printf("[like-service] 加入 dirty 失败 user=%d video=%d: %v", userID, videoID, err)
	}

	expected := currentCount - 1
	if expected < 0 {
		expected = 0
	}
	log.Printf("[like-service] UnlikeVideo result user=%d video=%d currentCount=%d expected=%d", userID, videoID, currentCount, expected)
	return expected, nil
}

// IsVideoLiked 用户是否点赞了视频
// 查询顺序：Redis pending -> Cuckoo Filter -> DB
func (s *LikeService) IsVideoLiked(userID, videoID uint64) bool {
	bizType := domain.BizTypeVideo

	// 1. 优先读 Redis pending
	pending, err := cache.GetLikePending(bizType, userID, videoID)
	if err != nil {
		log.Printf("[like-service] 读取 pending 失败 user=%d video=%d: %v", userID, videoID, err)
	} else if pending != nil {
		return pending.State == 1
	}

	// 2. Cuckoo Filter 快速判重
	cuckooKey := cache.VideoLikeCuckooKey(videoID)
	exists, err := cache.CuckooExists(cuckooKey, userID)
	if err != nil {
		log.Printf("[like-service] Cuckoo 查询失败 user=%d video=%d: %v", userID, videoID, err)
	}
	if !exists {
		return false // 确定未点赞
	}

	// 3. 回查 DB 兜底（软删除后 Cuckoo 可能残留）
	existing, err := s.likeDAO.GetByUserAndTarget(userID, videoID, bizType)
	return err == nil && existing.Cancel == 0
}

// GetVideoLikeCount 取视频点赞数
// 优先读取 Redis total counter；未命中时原子回源 likes 表并回填 total
func (s *LikeService) GetVideoLikeCount(videoID uint64) int64 {
	count, err := cache.InitTotalCounter(cache.CounterFieldLike, videoID, func() (int64, error) {
		return s.likeDAO.CountByTarget(videoID, domain.BizTypeVideo)
	})
	if err != nil {
		log.Printf("[like-service] 读取/回源点赞数失败 video=%d: %v", videoID, err)
		// 兜底：直接读 DB
		c, _ := s.likeDAO.CountByTarget(videoID, domain.BizTypeVideo)
		return c
	}
	return count
}

// BatchGetVideoLikeCount 批量获取视频点赞数
// 先 MGET Redis total，未命中的批量回源 DB 并回填 Redis
func (s *LikeService) BatchGetVideoLikeCount(videoIDs []uint64) map[uint64]int64 {
	result := make(map[uint64]int64, len(videoIDs))
	if len(videoIDs) == 0 {
		return result
	}

	cached, missing, err := cache.MGetTotalCounters(cache.CounterFieldLike, videoIDs)
	if err != nil {
		log.Printf("[like-service] MGET 点赞数失败: %v", err)
		missing = videoIDs
	}
	for id, val := range cached {
		result[id] = val
	}
	if len(missing) == 0 {
		return result
	}

	// 批量回源 DB
	dbCounts, err := s.likeDAO.BatchCountByTarget(missing, domain.BizTypeVideo)
	if err != nil {
		log.Printf("[like-service] 批量回源点赞数失败: %v", err)
		for _, id := range missing {
			result[id] = 0
		}
		return result
	}

	// 回填 Redis（异步失败不影响本次返回）
	if err := cache.MSetTotalCounters(cache.CounterFieldLike, dbCounts); err != nil {
		log.Printf("[like-service] 批量回填点赞 total 失败: %v", err)
	}
	for id, val := range dbCounts {
		result[id] = val
	}
	return result
}

// LikeComment 点赞评论（同步落库）
// likes 记录与 comments.like_count 在同一事务内完成，保证两者一致
func (s *LikeService) LikeComment(userID uint64, commentID uint64) (*LikeResponse, error) {
	comment, err := s.commentDAO.GetByID(commentID)
	if err != nil {
		log.Printf("[like-service] LikeComment 评论不存在 user=%d comment=%d", userID, commentID)
		return nil, errors.New("评论不存在")
	}

	var changed bool
	err = config.DB.Transaction(func(db *gorm.DB) error {
		like := &domain.Like{
			UserID:   userID,
			TargetID: commentID,
			BizType:  domain.BizTypeComment,
				VideoID:  0, // 评论点赞无关联视频
		}
		var err error
		changed, err = s.likeDAO.CreateOrRestoreWithTx(db, like)
		if err != nil {
			return err
		}
		if changed {
			return s.commentDAO.IncrementLikeCountWithTx(db, commentID)
		}
		return nil
	})
	log.Printf("[like-service] LikeComment user=%d comment=%d changed=%v err=%v", userID, commentID, changed, err)
	if err != nil {
		return nil, errors.New("点赞失败")
	}

	// 发送 Kafka 消息（如果被拉黑则不发送）
	isBlocked, _ := s.blacklistDAO.IsBlocked(comment.UserID, userID)
	if !isBlocked {
		event := messaging.MessageEvent{
			FromUserID: int64(userID),
			ToUserID:   int64(comment.UserID),
			ActionType: domain.ActionTypeLike,
			BizID:      int64(commentID),
			BizType:    domain.BizTypeComment,
			Content:    "赞了你的评论",
			CreatedAt:  time.Now(),
		}
		if err := messaging.SendMessageEvent(event); err != nil {
			log.Printf("[like-service] 发送点赞消息事件失败: %v", err)
		}
	}

	count, _ := s.likeDAO.CountByTarget(commentID, domain.BizTypeComment)
	log.Printf("[like-service] LikeComment result user=%d comment=%d changed=%v count=%d", userID, commentID, changed, count)
	return &LikeResponse{LikeCount: count}, nil
}

// UnlikeComment 取消点赞评论（同步落库）
// likes 记录与 comments.like_count 在同一事务内完成
func (s *LikeService) UnlikeComment(userID uint64, commentID uint64) (*LikeResponse, error) {
	var changed bool
	err := config.DB.Transaction(func(db *gorm.DB) error {
		var err error
		changed, err = s.likeDAO.CancelWithTx(db, userID, commentID, domain.BizTypeComment)
		if err != nil {
			return err
		}
		if changed {
			return s.commentDAO.DecrementLikeCountWithTx(db, commentID)
		}
		return nil
	})
	log.Printf("[like-service] UnlikeComment user=%d comment=%d changed=%v err=%v", userID, commentID, changed, err)
	if err != nil {
		return nil, errors.New("取消点赞失败")
	}

	count, _ := s.likeDAO.CountByTarget(commentID, domain.BizTypeComment)
	log.Printf("[like-service] UnlikeComment result user=%d comment=%d changed=%v count=%d", userID, commentID, changed, count)
	return &LikeResponse{LikeCount: count}, nil
}
