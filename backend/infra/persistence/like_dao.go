/*
 * like_dao.go
 * 功能：点赞数据访问对象，支持视频/评论通用点赞
 *      2026-06-20 CreateOrRestore/Cancel 返回是否确实修改了 DB 状态，供消息消费侧幂等更新 Redis
 *      2026-06-21 新增 RecreateLike：取消赞后再次点赞时删除旧记录并创建新记录，避免消息重复
 *      2026-06-22 支持传入 *gorm.DB 事务，保证评论点赞与 comments.like_count 原子一致
 *      2026-06-21 修复 CancelWithTx/CreateOrRestoreWithTx 链式 UpdateColumn 产生多条 SQL 导致 RowsAffected 恒为 0 的 Bug
 * 时间戳：2026-06-21
 */

package persistence

import (
	"log"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// LikeDAO 提供点赞表的数据库操作
type LikeDAO struct{}

// NewLikeDAO 创建 LikeDAO 实例
func NewLikeDAO() *LikeDAO {
	return &LikeDAO{}
}

// pickDB 返回传入事务或默认 DB
func pickDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return config.DB
}

// CreateOrRestore 创建点赞记录，若已存在则恢复（cancel=0 -> 1）
// 返回值 ok 表示本次操作确实产生了一条有效点赞（新增或从取消状态恢复）
func (dao *LikeDAO) CreateOrRestore(like *domain.Like) (bool, error) {
	return dao.CreateOrRestoreWithTx(nil, like)
}

// CreateOrRestoreWithTx 事务版本
func (dao *LikeDAO) CreateOrRestoreWithTx(tx *gorm.DB, like *domain.Like) (bool, error) {
	db := pickDB(tx)
	var existing domain.Like
	err := db.Where("user_id = ? AND target_id = ? AND biz_type = ?",
		like.UserID, like.TargetID, like.BizType).
		First(&existing).Error
	if err == nil {
		// 已存在，恢复点赞
		if existing.Cancel == 1 {
			log.Printf("[like-dao] CreateOrRestore 恢复点赞 user=%d target=%d biz=%d", like.UserID, like.TargetID, like.BizType)
			return true, db.Model(&existing).Updates(map[string]interface{}{
				"cancel":     0,
				"updated_at": gorm.Expr("NOW()"),
			}).Error
		}
		log.Printf("[like-dao] CreateOrRestore 已存在有效点赞 user=%d target=%d biz=%d", like.UserID, like.TargetID, like.BizType)
		return false, nil
	}

	// 不存在，创建新记录
	log.Printf("[like-dao] CreateOrRestore 新建点赞 user=%d target=%d biz=%d", like.UserID, like.TargetID, like.BizType)
	return true, db.Create(like).Error
}

// Cancel 取消点赞（软删除，cancel = 1 表示已删除）
// 返回值 ok 表示本次操作确实取消了一条有效点赞
func (dao *LikeDAO) Cancel(userID, targetID uint64, bizType int8) (bool, error) {
	return dao.CancelWithTx(nil, userID, targetID, bizType)
}

// CancelWithTx 事务版本
// 注意：必须使用 Updates 单次更新多条字段，链式 UpdateColumn 会产生多条独立 SQL，
// 第一条 SET cancel=1 之后第二条 WHERE cancel=0 无法匹配，导致 RowsAffected 始终为 0
func (dao *LikeDAO) CancelWithTx(tx *gorm.DB, userID, targetID uint64, bizType int8) (bool, error) {
	db := pickDB(tx)
	result := db.Model(&domain.Like{}).
		Where("user_id = ? AND target_id = ? AND biz_type = ? AND cancel = 0",
			userID, targetID, bizType).
		Updates(map[string]interface{}{
			"cancel":     1,
			"updated_at": gorm.Expr("NOW()"),
		})
	log.Printf("[like-dao] Cancel user=%d target=%d biz=%d rows=%d err=%v", userID, targetID, bizType, result.RowsAffected, result.Error)
	return result.RowsAffected > 0, result.Error
}

// RecreateLike 删除用户与目标已有的点赞记录（无论 cancel 状态），并创建一条新记录
// 用于取消赞后再次点赞的场景：保证消息侧能识别为一次新的点赞行为，同时避免 counter 重复计数
// 返回 ok 表示确实替换了旧记录（删除+创建视为一次替换，不计入总数变化）
func (dao *LikeDAO) RecreateLike(like *domain.Like) (bool, error) {
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND target_id = ? AND biz_type = ?",
			like.UserID, like.TargetID, like.BizType).
			Delete(&domain.Like{}).Error; err != nil {
			return err
		}
		return tx.Create(like).Error
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Delete 物理删除点赞记录（取消点赞）
func (dao *LikeDAO) Delete(userID, targetID uint64, bizType int8) error {
	return config.DB.Where("user_id = ? AND target_id = ? AND biz_type = ?",
		userID, targetID, bizType).Delete(&domain.Like{}).Error
}

// BatchGetByUserAndTargets 批量查询用户对多个目标的点赞状态
// 返回 map[targetID]bool，仅 cancel=0 的有效点赞为 true
func (dao *LikeDAO) BatchGetByUserAndTargets(userID uint64, targetIDs []uint64, bizType int8) (map[uint64]bool, error) {
	if len(targetIDs) == 0 {
		return map[uint64]bool{}, nil
	}
	var likes []domain.Like
	err := config.DB.Where("user_id = ? AND target_id IN ? AND biz_type = ? AND cancel = 0", userID, targetIDs, bizType).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint64]bool, len(likes))
	for _, like := range likes {
		result[like.TargetID] = true
	}
	return result, nil
}

// GetByUserAndTarget 查询用户对指定目标的点赞记录
func (dao *LikeDAO) GetByUserAndTarget(userID, targetID uint64, bizType int8) (*domain.Like, error) {
	var like domain.Like
	err := config.DB.Where("user_id = ? AND target_id = ? AND biz_type = ?",
		userID, targetID, bizType).
		First(&like).Error
	if err != nil {
		return nil, err
	}
	return &like, nil
}

// CountByTarget 统计指定目标的点赞数
func (dao *LikeDAO) CountByTarget(targetID uint64, bizType int8) (int64, error) {
	var count int64
	err := config.DB.Model(&domain.Like{}).
		Where("target_id = ? AND biz_type = ? AND cancel = 0", targetID, bizType).
		Count(&count).Error
	log.Printf("[like-dao] CountByTarget target=%d biz=%d count=%d err=%v", targetID, bizType, count, err)
	return count, err
}

// BatchCountByTarget 批量统计多个目标的点赞数
func (dao *LikeDAO) BatchCountByTarget(targetIDs []uint64, bizType int8) (map[uint64]int64, error) {
	result := make(map[uint64]int64, len(targetIDs))
	if len(targetIDs) == 0 {
		return result, nil
	}
	type countResult struct {
		TargetID uint64
		Count    int64
	}
	var rows []countResult
	err := config.DB.Model(&domain.Like{}).
		Select("target_id, COUNT(*) as count").
		Where("target_id IN ? AND biz_type = ? AND cancel = 0", targetIDs, bizType).
		Group("target_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.TargetID] = r.Count
	}
	// 未返回的 targetID 计数为 0，调用方无需再判断
	for _, id := range targetIDs {
		if _, ok := result[id]; !ok {
			result[id] = 0
		}
	}
	return result, nil
}
