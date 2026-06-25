/*
 * counter_dao.go
 * 功能：计数器刷盘 DAO，提供 MySQL 计数字段的轻量查询与批量增量更新
 *      2026-06-20 增加 video/user 计数字段到数据库列名的显式映射，修复 like->favorite_count、following->follow_count 的列名不一致
 * 时间戳：2026-05-22
 */

package persistence

import (
	"fmt"
	"log"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// videoCounterColumns 视频维度的 counter 字段到 MySQL 列名映射
// 字段命名与 domain.Video 结构体中的 gorm column 标签保持一致
var videoCounterColumns = map[string]string{
	"play":    "play_count",
	"like":    "favorite_count",
	"comment": "comment_count",
}

// userCounterColumns 用户维度的 counter 字段到 MySQL 列名映射
// 字段命名与 domain.User 结构体中的 gorm column 标签保持一致
var userCounterColumns = map[string]string{
	"follower":  "follower_count",
	"following": "follow_count",
}

// videoCounterColumn 根据字段名获取视频计数字段对应的数据库列名
func videoCounterColumn(field string) string {
	if col, ok := videoCounterColumns[field]; ok {
		return col
	}
	return field + "_count"
}

// userCounterColumn 根据字段名获取用户计数字段对应的数据库列名
func userCounterColumn(field string) string {
	if col, ok := userCounterColumns[field]; ok {
		return col
	}
	return field + "_count"
}

// CounterDAO 提供计数器相关的数据库操作
type CounterDAO struct{}

// NewCounterDAO 创建 CounterDAO 实例
func NewCounterDAO() *CounterDAO {
	return &CounterDAO{}
}

// IncrementVideoCounter 对视频指定计数字段原子加 delta
func (dao *CounterDAO) IncrementVideoCounter(videoID uint64, field string, delta int64) error {
	column := videoCounterColumn(field)
	return config.DB.Model(&domain.Video{}).
		Where("id = ?", videoID).
		UpdateColumn(column, gorm.Expr(column+" + ?", delta)).Error
}

// IncrementUserCounter 对用户指定计数字段原子加 delta
func (dao *CounterDAO) IncrementUserCounter(userID uint64, field string, delta int64) error {
	column := userCounterColumn(field)
	return config.DB.Model(&domain.User{}).
		Where("id = ?", userID).
		UpdateColumn(column, gorm.Expr(column+" + ?", delta)).Error
}

// GetVideoCounter 查询视频单个计数字段当前值（base 加载用）
func (dao *CounterDAO) GetVideoCounter(videoID uint64, field string) (int64, error) {
	column := videoCounterColumn(field)
	var result struct {
		Count int64
	}
	err := config.DB.Model(&domain.Video{}).
		Select(column).
		Where("id = ?", videoID).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

// GetUserCounter 查询用户单个计数字段当前值（base 加载用）
func (dao *CounterDAO) GetUserCounter(userID uint64, field string) (int64, error) {
	column := userCounterColumn(field)
	var result struct {
		Count int64
	}
	err := config.DB.Model(&domain.User{}).
		Select(column).
		Where("id = ?", userID).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

// BatchIncrementVideoCounter 批量更新视频计数字段
func (dao *CounterDAO) BatchIncrementVideoCounter(field string, updates map[uint64]int64) error {
	column := videoCounterColumn(field)
	for id, delta := range updates {
		if delta == 0 {
			continue
		}
		if err := config.DB.Model(&domain.Video{}).
			Where("id = ?", id).
			UpdateColumn(column, gorm.Expr(column+" + ?", delta)).Error; err != nil {
			log.Printf("[counter_dao] 批量更新视频计数失败 video=%d field=%s delta=%d: %v", id, field, delta, err)
			return fmt.Errorf("video=%d: %w", id, err)
		}
	}
	return nil
}

// BatchIncrementUserCounter 批量更新用户计数字段
func (dao *CounterDAO) BatchIncrementUserCounter(field string, updates map[uint64]int64) error {
	column := userCounterColumn(field)
	for id, delta := range updates {
		if delta == 0 {
			continue
		}
		if err := config.DB.Model(&domain.User{}).
			Where("id = ?", id).
			UpdateColumn(column, gorm.Expr(column+" + ?", delta)).Error; err != nil {
			log.Printf("[counter_dao] 批量更新用户计数失败 user=%d field=%s delta=%d: %v", id, field, delta, err)
			return fmt.Errorf("user=%d: %w", id, err)
		}
	}
	return nil
}

// SetVideoCounter 直接设置视频指定计数字段为指定值（用于 like total 刷盘，避免增量漂移）
func (dao *CounterDAO) SetVideoCounter(videoID uint64, field string, value int64) error {
	column := videoCounterColumn(field)
	return config.DB.Model(&domain.Video{}).
		Where("id = ?", videoID).
		UpdateColumn(column, value).Error
}

// SetUserCounter 直接设置用户指定计数字段为指定值
func (dao *CounterDAO) SetUserCounter(userID uint64, field string, value int64) error {
	column := userCounterColumn(field)
	return config.DB.Model(&domain.User{}).
		Where("id = ?", userID).
		UpdateColumn(column, value).Error
}
