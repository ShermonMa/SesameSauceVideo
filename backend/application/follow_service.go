/*
 * follow_service.go
 * 功能：关注应用服务，支持关注/取消关注、Cuckoo 判重、Redis delta 计数
 * 时间戳：2026-05-22
 */

package application

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// FollowService 处理关注相关的应用层业务逻辑
type FollowService struct {
	followDAO *persistence.FollowDAO
	userDAO   *persistence.UserDAO
}

// NewFollowService 创建 FollowService 实例
func NewFollowService() *FollowService {
	return &FollowService{
		followDAO: persistence.NewFollowDAO(),
		userDAO:   persistence.NewUserDAO(),
	}
}

// FollowUser 关注用户
// 防重：Cuckoo Filter + MySQL 唯一约束双保险
func (s *FollowService) FollowUser(userID, targetUserID uint64) error {
	if userID == targetUserID {
		return errors.New("不能关注自己")
	}

	cuckooKey := fmt.Sprintf("ss:user:following:cuckoo:%d", userID)

	// 1. 确保 Cuckoo Filter 已初始化
	cache.EnsureCuckooReserved(cuckooKey)

	// 2. Cuckoo Filter 前置判重
	exists, _ := cache.CuckooExists(cuckooKey, targetUserID)

	// 3. 写关系表
	follow := &domain.Follow{
		UserID:     userID,
		FollowerID: targetUserID,
	}
	err := s.followDAO.CreateOrRestore(follow)
	if err != nil {
		return errors.New("关注失败")
	}

	// 4. 判定快速路径 / 兜底路径 / 真重复
	// CreateOrRestore 在已有记录时返回 nil（不报错），无法直接区分新增 vs 恢复
	// 这里简化为：若 Cuckoo 已存在则大概率是重复关注
	if exists {
		return nil // 已关注，幂等
	}

	// 5. Cuckoo Filter 添加 + Redis 计数
	if err := cache.CuckooAdd(cuckooKey, targetUserID); err != nil {
		log.Printf("[follow-service] Cuckoo 添加失败 user=%d target=%d: %v", userID, targetUserID, err)
	}

	// 6. 增量更新 Redis delta
	// follower_count: targetUserID 的粉丝数 +1
	if err := cache.IncrDelta(cache.CounterFieldFollower, targetUserID); err != nil {
		log.Printf("[follow-service] 递增粉丝 delta 失败 target=%d: %v", targetUserID, err)
	}
	// following_count: userID 的关注数 +1
	if err := cache.IncrDelta(cache.CounterFieldFollowing, userID); err != nil {
		log.Printf("[follow-service] 递增关注 delta 失败 user=%d: %v", userID, err)
	}

	// 7. 发送 Kafka 关注通知
	event := messaging.MessageEvent{
		FromUserID: int64(userID),
		ToUserID:   int64(targetUserID),
		ActionType: domain.ActionTypeFollow,
		Content:    "关注了你",
		CreatedAt:  time.Now(),
	}
	if err := messaging.SendMessageEvent(event); err != nil {
		log.Printf("[follow-service] 发送关注消息事件失败: %v", err)
	}

	return nil
}

// UnfollowUser 取消关注
func (s *FollowService) UnfollowUser(userID, targetUserID uint64) error {
	if err := s.followDAO.Cancel(userID, targetUserID); err != nil {
		return errors.New("取消关注失败")
	}

	// 从 Cuckoo Filter 删除
	cuckooKey := fmt.Sprintf("ss:user:following:cuckoo:%d", userID)
	if err := cache.CuckooDel(cuckooKey, targetUserID); err != nil {
		log.Printf("[follow-service] Cuckoo 删除失败 user=%d target=%d: %v", userID, targetUserID, err)
	}

	// 递减 Redis delta
	cache.DecrDelta(cache.CounterFieldFollower, targetUserID)
	cache.DecrDelta(cache.CounterFieldFollowing, userID)

	return nil
}

// IsFollowing 是否已关注
func (s *FollowService) IsFollowing(userID, targetUserID uint64) bool {
	cuckooKey := fmt.Sprintf("ss:user:following:cuckoo:%d", userID)
	exists, err := cache.CuckooExists(cuckooKey, targetUserID)
	if err != nil {
		// Cuckoo 失败降级查 DB
		_, err = s.followDAO.GetByUserAndFollower(userID, targetUserID)
		return err == nil
	}
	return exists
}

// GetFollowerCount 获取粉丝数（base + delta）
func (s *FollowService) GetFollowerCount(userID uint64) int64 {
	base, delta, err := cache.GetCounter(cache.CounterFieldFollower, userID)
	if err != nil {
		log.Printf("[follow-service] 读取粉丝计数失败 user=%d: %v", userID, err)
	}
	if base == 0 {
		// base miss，回源 DB
		count, err := s.followDAO.CountFollowers(userID)
		if err != nil {
			log.Printf("[follow-service] 回源粉丝数失败 user=%d: %v", userID, err)
			return 0
		}
		if err := cache.SetBase(cache.CounterFieldFollower, userID, count); err != nil {
			log.Printf("[follow-service] 回填粉丝 base 失败 user=%d: %v", userID, err)
		}
		base = count
	}
	return base + delta
}

// GetFollowingCount 获取关注数（base + delta）
func (s *FollowService) GetFollowingCount(userID uint64) int64 {
	base, delta, err := cache.GetCounter(cache.CounterFieldFollowing, userID)
	if err != nil {
		log.Printf("[follow-service] 读取关注计数失败 user=%d: %v", userID, err)
	}
	if base == 0 {
		count, err := s.followDAO.CountFollowing(userID)
		if err != nil {
			log.Printf("[follow-service] 回源关注数失败 user=%d: %v", userID, err)
			return 0
		}
		if err := cache.SetBase(cache.CounterFieldFollowing, userID, count); err != nil {
			log.Printf("[follow-service] 回填关注 base 失败 user=%d: %v", userID, err)
		}
		base = count
	}
	return base + delta
}
