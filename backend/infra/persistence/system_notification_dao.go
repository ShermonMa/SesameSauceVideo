/*
 * system_notification_dao.go
 * 功能：系统通知数据访问对象，支持分页查询与未读数统计，含指定用户范围过滤
 *      2026-06-21 未读统计改用时间戳语义：publish_time > last_sys_msg_check，
 *      移除废弃的 ID 水位线方法 MaxIDVisible
 * 时间戳：2026-04-26
 */

package persistence

import (
	"strconv"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// SystemNotificationDAO 提供系统通知表的数据库操作
type SystemNotificationDAO struct{}

// NewSystemNotificationDAO 创建 SystemNotificationDAO 实例
func NewSystemNotificationDAO() *SystemNotificationDAO {
	return &SystemNotificationDAO{}
}

// buildUserVisibleQuery 构建当前用户可见的系统通知查询条件
// target_type=1 全体可见；target_type=2 时 target_user_ids JSON 数组需包含当前用户
func buildUserVisibleQuery(userID uint64) *gorm.DB {
	return config.DB.Model(&domain.SystemNotification{}).
		Where("target_type = ? OR (target_type = ? AND JSON_CONTAINS(target_user_ids, CAST(? AS JSON)))",
			domain.TargetTypeAll, domain.TargetTypeSome, strconv.FormatUint(userID, 10))
}

// ListPublished 分页查询指定用户可见的系统通知（按 publish_time 倒序）
func (dao *SystemNotificationDAO) ListPublished(userID uint64, page, pageSize int) ([]domain.SystemNotification, int64, error) {
	query := buildUserVisibleQuery(userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.SystemNotification
	offset := (page - 1) * pageSize
	if err := query.Order("publish_time DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// CountNewSince 统计 publish_time > since 的系统通知数，用于未读气泡
func (dao *SystemNotificationDAO) CountNewSince(userID uint64, since time.Time) (int64, error) {
	var count int64
	err := buildUserVisibleQuery(userID).
		Where("publish_time > ?", since).
		Count(&count).Error
	return count, err
}
