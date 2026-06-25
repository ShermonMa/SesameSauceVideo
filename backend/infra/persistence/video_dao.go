/*
 * video_dao.go
 * 功能：视频数据访问对象，封装 videos 表操作；新增公开列表分页查询；新增按作者分页查询（含全部状态）；
 *      新增 UpdateConfirmed / UpdateBasicInfo 精准列更新方法，避免 Save 全字段覆写导致 cover_url 等被并发清空
 *      2026-06-22 新增 ListPublicByIDs 支持 ES 混合搜索的 ID 批量查询与字段排序；新增 ListPublishedByAuthor
 */

package persistence

import (
	"fmt"
	"strings"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"gorm.io/gorm"
)

// VideoDAO 提供视频表的数据库操作
type VideoDAO struct{}

// NewVideoDAO 创建 VideoDAO 实例
func NewVideoDAO() *VideoDAO {
	return &VideoDAO{}
}

// Create 插入视频记录
func (dao *VideoDAO) Create(video *domain.Video) error {
	return config.DB.Create(video).Error
}

// GetByID 根据主键查询视频
func (dao *VideoDAO) GetByID(id uint64) (*domain.Video, error) {
	var video domain.Video
	err := config.DB.First(&video, id).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

// GetByHashID 根据 HashID 查询视频
func (dao *VideoDAO) GetByHashID(hashid string) (*domain.Video, error) {
	var video domain.Video
	err := config.DB.Where("hashid = ?", hashid).First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

// Update 仅更新视频的 hashid 与 cover_hashid，避免 Save 全字段覆写 publish_time 等字段
func (dao *VideoDAO) Update(video *domain.Video) error {
	return config.DB.Model(&domain.Video{}).
		Where("id = ?", video.ID).
		Updates(map[string]interface{}{
			"hashid":       video.HashID,
			"cover_hashid": video.CoverHashID,
		}).Error
}

// CountByAuthorAndStatus 统计指定作者指定状态的视频数量
func (dao *VideoDAO) CountByAuthorAndStatus(authorID uint64, status int8) (int64, error) {
	var count int64
	err := config.DB.Model(&domain.Video{}).Where("author_id = ? AND status = ?", authorID, status).Count(&count).Error
	return count, err
}

// IncrementPlayCount 将指定视频的播放次数加 1
func (dao *VideoDAO) IncrementPlayCount(id uint64) error {
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).UpdateColumn("play_count", config.DB.Raw("play_count + 1")).Error
}

// UpdateStatus 仅更新视频状态字段，避免覆盖其他列；用于异步处理失败时打标
func (dao *VideoDAO) UpdateStatus(id uint64, status int8) error {
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).UpdateColumn("status", status).Error
}

// UpdateProgress 更新视频转码进度
func (dao *VideoDAO) UpdateProgress(id uint64, progress int8) error {
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).UpdateColumn("progress", progress).Error
}

// UpdateConfirmed 仅更新 is_confirmed 字段，避免 Save 全字段覆写其他列
func (dao *VideoDAO) UpdateConfirmed(id uint64, isConfirmed int8) error {
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).UpdateColumn("is_confirmed", isConfirmed).Error
}

// UpdateBasicInfo 仅更新基础信息（title/description/cover_url），避免 Save 全字段覆写引发并发竞态
// description 为空字符串时不写入，保持原值；cover_url 始终写入（包括空字符串覆盖）
func (dao *VideoDAO) UpdateBasicInfo(id uint64, title, description, coverURL string) error {
	updates := map[string]interface{}{
		"title":     title,
		"cover_url": coverURL,
	}
	if description != "" {
		updates["description"] = description
	}
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).Updates(updates).Error
}

// UpdatePublishMeta 仅更新视频转码完成后的发布元数据（duration/width/height/file_size/play_url/status/publish_time），
// 不触碰 cover_url、title 等由其他流程负责的字段，避免 Save 全字段引发并发覆写
func (dao *VideoDAO) UpdatePublishMeta(id uint64, duration, width, height int, fileSize int64, playURL string, status int8, publishTime time.Time) error {
	updates := map[string]interface{}{
		"duration":     duration,
		"width":        width,
		"height":       height,
		"file_size":    fileSize,
		"play_url":     playURL,
		"status":       status,
		"publish_time": publishTime,
	}
	return config.DB.Model(&domain.Video{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateStatusWithCondition CAS 更新视频状态，仅当 fromStatus 匹配时才更新
func (dao *VideoDAO) UpdateStatusWithCondition(id uint64, fromStatus, toStatus int8) error {
	return config.DB.Model(&domain.Video{}).Where("id = ? AND status = ?", id, fromStatus).UpdateColumn("status", toStatus).Error
}

// ListUnconfirmedBefore 查询创建时间早于指定时间且未确认的视频记录
func (dao *VideoDAO) ListUnconfirmedBefore(before time.Time) ([]domain.Video, error) {
	var list []domain.Video
	err := config.DB.Model(&domain.Video{}).
		Where("status = ? AND is_confirmed = ? AND created_at < ?", domain.VideoStatusProcessing, 0, before).
		Find(&list).Error
	return list, err
}

// DeleteByID 物理删除视频记录（仅用于幽灵数据清理）
func (dao *VideoDAO) DeleteByID(id uint64) error {
	return config.DB.Unscoped().Delete(&domain.Video{}, id).Error
}

// ListByAuthor 分页查询指定作者的全部视频，包含所有状态（处理中/已发布/失败/已取消），按 ID 倒序
// 返回：列表、总数、错误
func (dao *VideoDAO) ListByAuthor(authorID uint64, page, pageSize int) ([]domain.Video, int64, error) {
	query := config.DB.Model(&domain.Video{}).
		Where("author_id = ?", authorID).
		Where("deleted_at IS NULL")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Video
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListPublishedByAuthor 分页查询指定作者的已发布视频，用于个人主页视频列表
// 2026-06-22 新增，支撑个人主页"投稿"Tab
func (dao *VideoDAO) ListPublishedByAuthor(authorID uint64, page, pageSize int) ([]domain.Video, int64, error) {
	query := config.DB.Model(&domain.Video{}).
		Where("author_id = ?", authorID).
		Where("status = ?", 0). // 仅已发布
		Where("deleted_at IS NULL")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []domain.Video
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ListPublic 分页查询已发布视频，支持分类筛选与关键词模糊匹配
// page 从 1 开始；categoryID=0 表示不过滤；keyword 为空字符串表示不搜索
// orderBy 取值：publish_time_desc（默认）、play_count_desc
// 返回：列表、总数、错误
func (dao *VideoDAO) ListPublic(page, pageSize int, categoryID uint64, keyword, orderBy string) ([]domain.Video, int64, error) {
	// 构建基础查询：仅返回已发布且未软删除的视频
	query := config.DB.Model(&domain.Video{}).
		Where("videos.status = ?", 0).
		Where("videos.deleted_at IS NULL")

	// 分类过滤：按需 JOIN video_categories
	if categoryID > 0 {
		query = query.Joins("JOIN video_categories vc ON vc.video_id = videos.id").
			Where("vc.category_id = ?", categoryID)
	}

	// 关键词模糊匹配：转义 SQL LIKE 通配符避免误匹配
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		escaped := escapeLike(keyword)
		pattern := "%" + escaped + "%"
		query = query.Where("videos.title LIKE ? OR videos.description LIKE ?", pattern, pattern)
	}

	// 先统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序映射；id DESC 兜底防止同值翻页错乱
	var orderClause string
	switch orderBy {
	case "play_count_desc":
		orderClause = "videos.play_count DESC, videos.id DESC"
	default:
		orderClause = "videos.publish_time DESC, videos.id DESC"
	}

	// 分页查询
	var list []domain.Video
	offset := (page - 1) * pageSize
	if err := query.Order(orderClause).
		Limit(pageSize).
		Offset(offset).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// GetVideoCounts 仅查询视频的三个计数字段（轻量查询，供缓存命中时使用）
func (dao *VideoDAO) GetVideoCounts(id uint64) (playCount, favoriteCount, commentCount int64, err error) {
	var result struct {
		PlayCount     int64 `gorm:"column:play_count"`
		FavoriteCount int64 `gorm:"column:favorite_count"`
		CommentCount  int64 `gorm:"column:comment_count"`
	}
	err = config.DB.Model(&domain.Video{}).
		Select("play_count, favorite_count, comment_count").
		Where("id = ?", id).
		Scan(&result).Error
	if err != nil {
		return 0, 0, 0, err
	}
	return result.PlayCount, result.FavoriteCount, result.CommentCount, nil
}

// IncrementCommentCount 原子递增视频评论数
func (dao *VideoDAO) IncrementCommentCount(id uint64) error {
	return config.DB.Model(&domain.Video{}).
		Where("id = ?", id).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
}

// ListPublicByIDs 根据一组 ID 查询已发布视频，支持 MySQL 字段排序
// orderBy 取值：publish_time_desc（ESI 相关性排序）、play_count_desc/asc、duration_desc/asc、publish_time_asc
// 当 orderBy 为 publish_time_desc 时通过 FIELD() 保留 ES 返回的相关性顺序
func (dao *VideoDAO) ListPublicByIDs(ids []uint64, orderBy string) ([]domain.Video, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := config.DB.Model(&domain.Video{}).
		Where("status = ?", 0).
		Where("deleted_at IS NULL").
		Where("id IN ?", ids)

	var orderClause string
	switch orderBy {
	case "play_count_desc":
		orderClause = "videos.play_count DESC, videos.id DESC"
	case "play_count_asc":
		orderClause = "videos.play_count ASC, videos.id DESC"
	case "duration_desc":
		orderClause = "videos.duration DESC, videos.id DESC"
	case "duration_asc":
		orderClause = "videos.duration ASC, videos.id DESC"
	case "publish_time_asc":
		orderClause = "videos.publish_time ASC, videos.id DESC"
	default: // "publish_time_desc" —— 保留 ES 相关性顺序
		orderClause = fieldOrder(ids)
	}

	var list []domain.Video
	if err := query.Order(orderClause).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// fieldOrder 生成 FIELD(videos.id, 1, 2, 3, ...) 以保留 ES 返回的相关性顺序
func fieldOrder(ids []uint64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("%d", id)
	}
	return "FIELD(videos.id, " + strings.Join(parts, ", ") + ")"
}

// escapeLike 转义 SQL LIKE 模式中的特殊字符（\ % _），避免用户输入误命中通配符
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
