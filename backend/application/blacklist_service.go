/*
 * blacklist_service.go
 * 功能：拉黑应用服务，负责拉黑/取消拉黑用户
 * 时间戳：2026-04-26
 */

package application

import (
	"errors"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// BlacklistService 处理拉黑相关的应用层业务逻辑
type BlacklistService struct {
	blacklistDAO *persistence.BlacklistDAO
	userDAO      *persistence.UserDAO
}

// NewBlacklistService 创建 BlacklistService 实例
func NewBlacklistService() *BlacklistService {
	return &BlacklistService{
		blacklistDAO: persistence.NewBlacklistDAO(),
		userDAO:      persistence.NewUserDAO(),
	}
}

// BlockUser 拉黑用户
func (s *BlacklistService) BlockUser(userID uint64, req BlacklistRequest) error {
	blockedUserID := uint64(req.BlockedUserID)
	if userID == blockedUserID {
		return errors.New("不能拉黑自己")
	}

	// 校验被拉黑用户存在
	if _, err := s.userDAO.GetByID(blockedUserID); err != nil {
		return errors.New("目标用户不存在")
	}

	blacklist := &domain.Blacklist{
		UserID:        userID,
		BlockedUserID: blockedUserID,
	}
	if err := s.blacklistDAO.Create(blacklist); err != nil {
		return errors.New("拉黑失败")
	}
	return nil
}

// UnblockUser 取消拉黑
func (s *BlacklistService) UnblockUser(userID uint64, req BlacklistRequest) error {
	blockedUserID := uint64(req.BlockedUserID)
	if err := s.blacklistDAO.Delete(userID, blockedUserID); err != nil {
		return errors.New("取消拉黑失败")
	}
	return nil
}
