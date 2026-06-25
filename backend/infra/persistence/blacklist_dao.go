/*
 * blacklist_dao.go
 * 功能：用户拉黑关系数据访问对象
 * 时间戳：2026-04-26
 */

package persistence

import (
	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
)

// BlacklistDAO 提供拉黑关系表的数据库操作
type BlacklistDAO struct{}

// NewBlacklistDAO 创建 BlacklistDAO 实例
func NewBlacklistDAO() *BlacklistDAO {
	return &BlacklistDAO{}
}

// Create 创建拉黑记录
func (dao *BlacklistDAO) Create(blacklist *domain.Blacklist) error {
	return config.DB.Create(blacklist).Error
}

// Delete 取消拉黑
func (dao *BlacklistDAO) Delete(userID, blockedUserID uint64) error {
	return config.DB.Where("user_id = ? AND blocked_user_id = ?", userID, blockedUserID).
		Delete(&domain.Blacklist{}).Error
}

// IsBlocked 查询 userID 是否拉黑了 blockedUserID
func (dao *BlacklistDAO) IsBlocked(userID, blockedUserID uint64) (bool, error) {
	var count int64
	err := config.DB.Model(&domain.Blacklist{}).
		Where("user_id = ? AND blocked_user_id = ?", userID, blockedUserID).
		Count(&count).Error
	return count > 0, err
}

// ListBlockedUserIDs 查询用户拉黑的所有用户ID列表
func (dao *BlacklistDAO) ListBlockedUserIDs(userID uint64) ([]uint64, error) {
	var ids []uint64
	err := config.DB.Model(&domain.Blacklist{}).
		Where("user_id = ?", userID).
		Pluck("blocked_user_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}
