/*
 * message_dao.go
 * 功能：消息数据访问对象，支持按类型分页、按时间戳统计未读、标记已读
 *      2026-06-21 未读统计改用时间戳语义：created_at > last_sys_msg_check，
 *      移除废弃的 is_read 统计与 ID 水位线方法
 * 时间戳：2026-04-26
 */

package persistence

import (
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
)

// MessageDAO 提供消息表的数据库操作
type MessageDAO struct{}

// NewMessageDAO 创建 MessageDAO 实例
func NewMessageDAO() *MessageDAO {
	return &MessageDAO{}
}

// Create 插入消息记录
func (dao *MessageDAO) Create(msg *domain.Message) error {
	return config.DB.Create(msg).Error
}

// ListByType 按消息类型分页查询接收者的消息
func (dao *MessageDAO) ListByType(toUserID uint64, actionType int8, page, pageSize int) ([]domain.Message, int64, error) {
	query := config.DB.Model(&domain.Message{}).
		Where("to_user_id = ? AND action_type = ? AND is_deleted = 0", toUserID, actionType)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Message
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// CountByTypeSince 统计指定类型消息中 created_at > since 的数量，用于未读气泡
func (dao *MessageDAO) CountByTypeSince(toUserID uint64, actionType int8, since time.Time) (int64, error) {
	var count int64
	err := config.DB.Model(&domain.Message{}).
		Where("to_user_id = ? AND action_type = ? AND is_deleted = 0 AND created_at > ?",
			toUserID, actionType, since).
		Count(&count).Error
	return count, err
}

// MarkReadByIDs 批量标记消息已读
func (dao *MessageDAO) MarkReadByIDs(toUserID uint64, ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}
	return config.DB.Model(&domain.Message{}).
		Where("to_user_id = ? AND id IN ?", toUserID, ids).
		UpdateColumn("is_read", 1).Error
}

// MarkReadByType 按类型全部标记已读
func (dao *MessageDAO) MarkReadByType(toUserID uint64, actionType int8) error {
	return config.DB.Model(&domain.Message{}).
		Where("to_user_id = ? AND action_type = ? AND is_read = 0", toUserID, actionType).
		UpdateColumn("is_read", 1).Error
}
