/*
 * follow_dao.go
 * 功能：关注数据访问对象，封装 follows 表操作；支持创建/取消/查询/计数
 * 时间戳：2026-05-22
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// FollowDAO 提供关注表的数据库操作
type FollowDAO struct{}

// NewFollowDAO 创建 FollowDAO 实例
func NewFollowDAO() *FollowDAO {
	return &FollowDAO{}
}

// CreateOrRestore 创建关注记录，若已存在则恢复（cancel=0）
func (dao *FollowDAO) CreateOrRestore(follow *domain.Follow) error {
	var existing domain.Follow
	err := config.DB.Where("user_id = ? AND follower_id = ?", follow.UserID, follow.FollowerID).
		First(&existing).Error

	if err == nil {
		// 已存在，恢复关注
		if existing.Cancel == 1 {
			return config.DB.Model(&existing).
				UpdateColumn("cancel", 0).
				UpdateColumn("updated_at", gorm.Expr("NOW()")).Error
		}
		return nil
	}

	// 不存在，创建新记录
	return config.DB.Create(follow).Error
}

// Cancel 取消关注（软取消）
func (dao *FollowDAO) Cancel(userID, followerID uint64) error {
	return config.DB.Model(&domain.Follow{}).
		Where("user_id = ? AND follower_id = ? AND cancel = 0", userID, followerID).
		UpdateColumn("cancel", 1).
		UpdateColumn("updated_at", gorm.Expr("NOW()")).Error
}

// GetByUserAndFollower 查询用户对指定用户的关注记录
func (dao *FollowDAO) GetByUserAndFollower(userID, followerID uint64) (*domain.Follow, error) {
	var follow domain.Follow
	err := config.DB.Where("user_id = ? AND follower_id = ?", userID, followerID).
		First(&follow).Error
	if err != nil {
		return nil, err
	}
	return &follow, nil
}

// CountFollowers 统计指定用户的粉丝数
func (dao *FollowDAO) CountFollowers(userID uint64) (int64, error) {
	var count int64
	err := config.DB.Model(&domain.Follow{}).
		Where("follower_id = ? AND cancel = 0", userID).
		Count(&count).Error
	return count, err
}

// CountFollowing 统计指定用户的关注数
func (dao *FollowDAO) CountFollowing(userID uint64) (int64, error) {
	var count int64
	err := config.DB.Model(&domain.Follow{}).
		Where("user_id = ? AND cancel = 0", userID).
		Count(&count).Error
	return count, err
}

// ListFollowerIDs 查询用户的粉丝 ID 列表
func (dao *FollowDAO) ListFollowerIDs(userID uint64, page, pageSize int) ([]uint64, error) {
	var follows []domain.Follow
	offset := (page - 1) * pageSize
	err := config.DB.Model(&domain.Follow{}).
		Where("follower_id = ? AND cancel = 0", userID).
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&follows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(follows))
	for _, f := range follows {
		ids = append(ids, f.UserID)
	}
	return ids, nil
}

// ListFollowingIDs 查询用户关注的 ID 列表
func (dao *FollowDAO) ListFollowingIDs(userID uint64, page, pageSize int) ([]uint64, error) {
	var follows []domain.Follow
	offset := (page - 1) * pageSize
	err := config.DB.Model(&domain.Follow{}).
		Where("user_id = ? AND cancel = 0", userID).
		Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&follows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(follows))
	for _, f := range follows {
		ids = append(ids, f.FollowerID)
	}
	return ids, nil
}
