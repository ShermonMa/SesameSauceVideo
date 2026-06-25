/*
 * comment_service.go
 * 功能：评论应用服务，负责发布评论、查询列表、楼中楼、深度校验、拉黑过滤、软删除脱敏；
 *      PublishComment 内部解码外部 hashid 形式的 video_id
 *      2026-06-21 ListComments 批量查询当前用户对一级评论的点赞状态并返回 is_liked，修复刷新后评论点赞状态丢失
 * 时间戳：2026-05-01
 */

package application

import (
	"errors"
	"log"
	"time"

	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/cache"
	"github.com/shermon/SesameSauce/infra/hashids"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
)

// CommentService 处理评论相关的应用层业务逻辑
type CommentService struct {
	commentDAO         *persistence.CommentDAO
	videoDAO           *persistence.VideoDAO
	userDAO            *persistence.UserDAO
	blacklistDAO       *persistence.BlacklistDAO
	likeDAO            *persistence.LikeDAO
	systemNotificationDAO *persistence.SystemNotificationDAO
}

// NewCommentService 创建 CommentService 实例
func NewCommentService() *CommentService {
	return &CommentService{
		commentDAO:         persistence.NewCommentDAO(),
		videoDAO:           persistence.NewVideoDAO(),
		userDAO:            persistence.NewUserDAO(),
		blacklistDAO:       persistence.NewBlacklistDAO(),
		likeDAO:            persistence.NewLikeDAO(),
		systemNotificationDAO: persistence.NewSystemNotificationDAO(),
	}
}

// PublishComment 发布评论（含深度校验、自回复拦截、Kafka 消息生产）
func (s *CommentService) PublishComment(userID uint64, req PublishCommentRequest) (*PublishCommentResponse, error) {
	if err := domain.ValidateContent(req.Content); err != nil {
		return nil, err
	}

	// 解码外部 hashid，得到内部自增视频 ID
	videoID, err := hashids.Decode(req.VideoID)
	if err != nil {
		return nil, errors.New("无效的 video_id")
	}

	// 校验视频存在
	video, err := s.videoDAO.GetByID(videoID)
	if err != nil {
		return nil, errors.New("视频不存在")
	}
	if err := domain.CheckVisibility(video.Status); err != nil {
		return nil, err
	}

	comment := &domain.Comment{
		UserID:  userID,
		VideoID: videoID,
		Content: req.Content,
	}

	var toUserID uint64
	// 楼中楼深度校验
	if req.ParentID != nil && *req.ParentID > 0 {
		parent, err := s.commentDAO.GetParentByID(uint64(*req.ParentID))
		if err != nil {
			return nil, errors.New("目标评论不存在")
		}
		if parent.ParentID != nil {
			return nil, domain.ErrCommentTooDeep
		}
		pid := uint64(*req.ParentID)
		comment.ParentID = &pid
		comment.RootID = &pid
		toUserID = parent.UserID

		// 自回复拦截
		if userID == toUserID {
			return nil, errors.New("不能回复自己的评论")
		}
	}

	if err := s.commentDAO.Create(comment); err != nil {
		return nil, errors.New("评论发布失败")
	}

	// 递增视频评论数 Redis delta（异步失败不影响主流程）
	if err := cache.IncrDelta(cache.CounterFieldComment, videoID); err != nil {
		log.Printf("[comment-service] 递增评论 delta 失败 video=%d: %v", videoID, err)
	}

	// 如果是回复，发 Kafka msg.reply（拉黑检查在 shouldProduceMsg 中）
	if req.ParentID != nil && *req.ParentID > 0 {
		isBlocked, _ := s.blacklistDAO.IsBlocked(toUserID, userID)
		if !isBlocked {
			event := messaging.MessageEvent{
				FromUserID: int64(userID),
				ToUserID:   int64(toUserID),
				ActionType: domain.ActionTypeReply,
				BizID:      int64(comment.ID),
				BizType:    domain.BizTypeComment,
				Content:    req.Content,
				CreatedAt:  time.Now(),
			}
			if err := messaging.SendMessageEvent(event); err != nil {
				log.Printf("[comment-service] 发送回复消息事件失败: %v", err)
			}
		}
	}

	return &PublishCommentResponse{
		CommentID: comment.ID,
		CreatedAt: comment.CreatedAt,
	}, nil
}

// ListComments 获取视频一级评论列表（含前3条二级、reply_count、拉黑过滤、脱敏）
func (s *CommentService) ListComments(req CommentListRequest) (*CommentListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > MaxPageSize {
		req.PageSize = DefaultPageSize
	}

	// 获取当前用户拉黑的用户ID列表
	blockedUserIDs, _ := s.blacklistDAO.ListBlockedUserIDs(req.UserID)

	comments, total, err := s.commentDAO.ListByVideoID(req.VideoID, blockedUserIDs, req.Page, req.PageSize)
	if err != nil {
		return nil, errors.New("评论列表查询失败")
	}

	if len(comments) == 0 {
		return &CommentListResponse{
			Comments: []CommentItemDTO{},
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    0,
		}, nil
	}

	// 批量查前3条二级评论
	rootIDs := make([]uint64, 0, len(comments))
	for _, c := range comments {
		rootIDs = append(rootIDs, c.ID)
	}

	topReplies, _ := s.commentDAO.ListTopRepliesByRootIDs(rootIDs, 3)
	replyCounts, _ := s.commentDAO.CountRepliesByRootIDs(rootIDs)

	// 聚合用户信息
	userIDSet := make(map[uint64]struct{})
	for _, c := range comments {
		userIDSet[c.UserID] = struct{}{}
	}
	for _, r := range topReplies {
		userIDSet[r.UserID] = struct{}{}
	}
	uids := make([]uint64, 0, len(userIDSet))
	for id := range userIDSet {
		uids = append(uids, id)
	}

	users, _ := s.userDAO.GetByIDs(uids)
	userMap := make(map[uint64]domain.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	// 批量查询当前登录用户对一级评论的点赞状态（未登录跳过）
	likedMap := make(map[uint64]bool)
	if req.UserID > 0 && len(comments) > 0 {
		commentIDs := make([]uint64, 0, len(comments))
		for _, c := range comments {
			commentIDs = append(commentIDs, c.ID)
		}
		likedMap, _ = s.likeDAO.BatchGetByUserAndTargets(req.UserID, commentIDs, domain.BizTypeComment)
	}

	// 构建回复映射：root_id -> []ReplyItemDTO
	replyMap := make(map[uint64][]ReplyItemDTO)
	for _, r := range topReplies {
		replyMap[*r.RootID] = append(replyMap[*r.RootID], s.toReplyItemDTO(r, userMap))
	}

	items := make([]CommentItemDTO, 0, len(comments))
	for _, c := range comments {
		item := s.toCommentItemDTO(c, userMap, likedMap)
		item.Replies = replyMap[c.ID]
		if cnt, ok := replyCounts[c.ID]; ok {
			item.ReplyCount = cnt
		}
		items = append(items, item)
	}

	return &CommentListResponse{
		Comments: items,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}

// ListReplies 获取楼中楼完整列表
func (s *CommentService) ListReplies(req ReplyListRequest) (*ReplyListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > MaxPageSize {
		req.PageSize = DefaultPageSize
	}

	replies, total, err := s.commentDAO.ListRepliesByRootID(req.RootID, req.Page, req.PageSize)
	if err != nil {
		return nil, errors.New("楼中楼查询失败")
	}

	userIDSet := make(map[uint64]struct{})
	for _, r := range replies {
		userIDSet[r.UserID] = struct{}{}
	}
	uids := make([]uint64, 0, len(userIDSet))
	for id := range userIDSet {
		uids = append(uids, id)
	}

	users, _ := s.userDAO.GetByIDs(uids)
	userMap := make(map[uint64]domain.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	items := make([]ReplyItemDTO, 0, len(replies))
	for _, r := range replies {
		items = append(items, s.toReplyItemDTO(r, userMap))
	}

	return &ReplyListResponse{
		Replies:  items,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}

// toCommentItemDTO 将领域实体转为一级评论 DTO（含脱敏与当前用户点赞状态）
func (s *CommentService) toCommentItemDTO(c domain.Comment, userMap map[uint64]domain.User, likedMap map[uint64]bool) CommentItemDTO {
	item := CommentItemDTO{
		ID:        c.ID,
		UserID:    c.UserID,
		Content:   c.Content,
		LikeCount: c.LikeCount,
		IsLiked:   likedMap[c.ID],
		CreatedAt: c.CreatedAt,
		IsDeleted: c.IsDeleted(),
	}

	if c.IsDeleted() {
		item.Content = "评论已删除"
		item.UserName = ""
		item.UserAvatar = ""
		item.UserID = 0
	} else if u, ok := userMap[c.UserID]; ok {
		item.UserName = u.Name
		if u.Avatar != nil {
			item.UserAvatar = *u.Avatar
		}
	}

	return item
}

// toReplyItemDTO 将领域实体转为二级评论 DTO（含脱敏）
func (s *CommentService) toReplyItemDTO(c domain.Comment, userMap map[uint64]domain.User) ReplyItemDTO {
	item := ReplyItemDTO{
		ID:        c.ID,
		UserID:    c.UserID,
		Content:   c.Content,
		LikeCount: c.LikeCount,
		CreatedAt: c.CreatedAt,
		IsDeleted: c.IsDeleted(),
	}

	if c.IsDeleted() {
		item.Content = "评论已删除"
		item.UserName = ""
		item.UserAvatar = ""
		item.UserID = 0
	} else if u, ok := userMap[c.UserID]; ok {
		item.UserName = u.Name
		if u.Avatar != nil {
			item.UserAvatar = *u.Avatar
		}
	}

	return item
}
