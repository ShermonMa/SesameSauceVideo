/*
 * comment_dao.go
 * 功能：评论数据访问对象，支持一级评论分页、楼中楼分页、批量统计、软删除
 *      2026-06-21 修复 IncrementLikeCountWithTx/DecrementLikeCountWithTx 使用参数化 gorm.Expr 并增加 RowsAffected 检查
 * 时间戳：2026-06-21
 */

package persistence

import (
	"fmt"
	"log"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// CommentDAO 提供评论表的数据库操作
type CommentDAO struct{}

// NewCommentDAO 创建 CommentDAO 实例
func NewCommentDAO() *CommentDAO {
	return &CommentDAO{}
}

// Create 插入评论记录
func (dao *CommentDAO) Create(comment *domain.Comment) error {
	return config.DB.Create(comment).Error
}

// GetByID 根据主键查询评论
func (dao *CommentDAO) GetByID(id uint64) (*domain.Comment, error) {
	var comment domain.Comment
	err := config.DB.First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// GetParentByID 查询指定评论的父级评论（用于深度校验）
func (dao *CommentDAO) GetParentByID(id uint64) (*domain.Comment, error) {
	var comment domain.Comment
	err := config.DB.First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// ListByVideoID 分页查询视频下的一级评论，支持拉黑过滤
func (dao *CommentDAO) ListByVideoID(videoID uint64, blockedUserIDs []uint64, page, pageSize int) ([]domain.Comment, int64, error) {
	query := config.DB.Model(&domain.Comment{}).
		Where("video_id = ? AND parent_id IS NULL AND deleted_at IS NULL", videoID)

	if len(blockedUserIDs) > 0 {
		query = query.Where("user_id NOT IN ?", blockedUserIDs)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Comment
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListRepliesByRootID 分页查询楼中楼（二级评论）
func (dao *CommentDAO) ListRepliesByRootID(rootID uint64, page, pageSize int) ([]domain.Comment, int64, error) {
	query := config.DB.Model(&domain.Comment{}).
		Where("root_id = ? AND deleted_at IS NULL", rootID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Comment
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListTopRepliesByRootIDs 批量查询每个一级评论的前 N 条二级评论
func (dao *CommentDAO) ListTopRepliesByRootIDs(rootIDs []uint64, limit int) ([]domain.Comment, error) {
	if len(rootIDs) == 0 {
		return []domain.Comment{}, nil
	}

	// 使用子查询 + row_number 模拟每个 root_id 取前 N 条
	// MySQL 8.0 支持窗口函数
	sql := `
		SELECT id, user_id, video_id, content, like_count, parent_id, root_id, created_at, updated_at, deleted_at
		FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY root_id ORDER BY created_at ASC) as rn
			FROM comments
			WHERE root_id IN ? AND deleted_at IS NULL
		) t
		WHERE t.rn <= ?
	`
	var list []domain.Comment
	if err := config.DB.Raw(sql, rootIDs, limit).Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CountRepliesByRootIDs 批量统计各一级评论下的二级评论总数
func (dao *CommentDAO) CountRepliesByRootIDs(rootIDs []uint64) (map[uint64]int64, error) {
	result := make(map[uint64]int64)
	if len(rootIDs) == 0 {
		return result, nil
	}

	type countResult struct {
		RootID uint64
		Count  int64
	}
	var rows []countResult
	err := config.DB.Model(&domain.Comment{}).
		Select("root_id, COUNT(*) as count").
		Where("root_id IN ? AND deleted_at IS NULL", rootIDs).
		Group("root_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, r := range rows {
		result[r.RootID] = r.Count
	}
	return result, nil
}

// SoftDelete 软删除评论
func (dao *CommentDAO) SoftDelete(id uint64) error {
	return config.DB.Model(&domain.Comment{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

// IncrementLikeCount 原子递增评论点赞数
func (dao *CommentDAO) IncrementLikeCount(id uint64) error {
	return dao.IncrementLikeCountWithTx(nil, id)
}

// IncrementLikeCountWithTx 事务版本
// 使用参数化 gorm.Expr 避免 GORM 表达式处理差异，RowsAffected=0 时报错保证事务回滚
func (dao *CommentDAO) IncrementLikeCountWithTx(tx *gorm.DB, id uint64) error {
	db := pickDB(tx)
	result := db.Model(&domain.Comment{}).
		Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("like_count + ?", 1))
	log.Printf("[comment-dao] IncrementLikeCount comment=%d rows=%d err=%v", id, result.RowsAffected, result.Error)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("评论 %d 不存在或已删除", id)
	}
	return nil
}

// DecrementLikeCount 原子递减评论点赞数
func (dao *CommentDAO) DecrementLikeCount(id uint64) error {
	return dao.DecrementLikeCountWithTx(nil, id)
}

// DecrementLikeCountWithTx 事务版本
// 使用参数化 gorm.Expr 避免 GORM 表达式处理差异，RowsAffected=0 时报错保证事务回滚
func (dao *CommentDAO) DecrementLikeCountWithTx(tx *gorm.DB, id uint64) error {
	db := pickDB(tx)
	result := db.Model(&domain.Comment{}).
		Where("id = ?", id).
		UpdateColumn("like_count", gorm.Expr("GREATEST(like_count - ?, 0)", 1))
	log.Printf("[comment-dao] DecrementLikeCount comment=%d rows=%d err=%v", id, result.RowsAffected, result.Error)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("评论 %d 不存在或已删除", id)
	}
	return nil
}
