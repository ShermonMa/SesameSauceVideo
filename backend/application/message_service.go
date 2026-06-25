/*
 * message_service.go
 * 功能：消息/信箱应用服务，负责私信、未读统计、消息列表、系统通知；
 *      新增并发改造：GetUnreadCount 中 5 类未读计数 + 系统通知计数共 5 个独立 DB 查询并发执行，
 *      通过 timing.Tracker 输出加速比报告，显著降低用户切到消息页时的接口 RT
 *      2026-06-21 last_sys_msg_check 语义变更为”用户上次查看信箱的时间戳”，
 *      所有消息类型统一按 created_at / publish_time > last_sys_msg_check 统计未读气泡
 * 时间戳：2026-05-03
 */

package application

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/shermon/SesameSauce/config"
	"github.com/shermon/SesameSauce/domain"
	"github.com/shermon/SesameSauce/infra/messaging"
	"github.com/shermon/SesameSauce/infra/persistence"
	"github.com/shermon/SesameSauce/pkg/timing"
)

// MessageService 处理消息相关的应用层业务逻辑
type MessageService struct {
	messageDAO            *persistence.MessageDAO
	userDAO               *persistence.UserDAO
	blacklistDAO          *persistence.BlacklistDAO
	systemNotificationDAO *persistence.SystemNotificationDAO
}

// NewMessageService 创建 MessageService 实例
func NewMessageService() *MessageService {
	return &MessageService{
		messageDAO:            persistence.NewMessageDAO(),
		userDAO:               persistence.NewUserDAO(),
		blacklistDAO:          persistence.NewBlacklistDAO(),
		systemNotificationDAO: persistence.NewSystemNotificationDAO(),
	}
}

// SendPrivate 发送私信（含拉黑阻断、自回复拦截）
func (s *MessageService) SendPrivate(fromUserID uint64, req SendPrivateRequest) error {
	if req.Content == "" {
		return errors.New("私信内容不能为空")
	}
	if len(req.Content) > 512 {
		return errors.New("私信内容超过最大长度限制")
	}

	toUserID := uint64(req.ToUserID)
	if fromUserID == toUserID {
		return errors.New("不能给自己发送私信")
	}

	// 校验接收者存在
	if _, err := s.userDAO.GetByID(toUserID); err != nil {
		return errors.New("接收者不存在")
	}

	// 拉黑检查
	isBlocked, _ := s.blacklistDAO.IsBlocked(toUserID, fromUserID)
	if isBlocked {
		return errors.New("对方暂不接受消息")
	}

	msg := &domain.Message{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Content:    req.Content,
		ActionType: domain.ActionTypePrivate,
		IsRead:     0,
		IsDeleted:  0,
	}
	if err := s.messageDAO.Create(msg); err != nil {
		return errors.New("私信发送失败")
	}

	// 发送 Kafka 事件
	event := messaging.MessageEvent{
		MsgID:      int64(msg.ID),
		FromUserID: int64(fromUserID),
		ToUserID:   int64(toUserID),
		ActionType: domain.ActionTypePrivate,
		Content:    req.Content,
		CreatedAt:  time.Now(),
	}
	if err := messaging.SendMessageEvent(event); err != nil {
		log.Printf("[message-service] 发送私信消息事件失败: %v", err)
	}

	return nil
}

// GetUnreadCount 聚合六 Tab 未读气泡数
// 所有消息类型统一按 created_at / publish_time > last_sys_msg_check 统计未读
func (s *MessageService) GetUnreadCount(userID uint64) (*UnreadCountResponse, error) {
	tracker := timing.NewTracker()

	// 取用户上次查看信箱的时间戳；NULL 表示从未查看，用零值时间使全部消息计为未读
	user, err := s.userDAO.GetByID(userID)
	var since time.Time
	if err == nil && user.LastSysMsgCheck != nil {
		since = *user.LastSysMsgCheck
	}

	var (
		private, reply, like, follow int64
		sysCount                     int64
		wg                           sync.WaitGroup
	)

	wg.Add(5)
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("count_private", time.Now())
		private, _ = s.messageDAO.CountByTypeSince(userID, domain.ActionTypePrivate, since)
	}()
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("count_reply", time.Now())
		reply, _ = s.messageDAO.CountByTypeSince(userID, domain.ActionTypeReply, since)
	}()
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("count_like", time.Now())
		like, _ = s.messageDAO.CountByTypeSince(userID, domain.ActionTypeLike, since)
	}()
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("count_follow", time.Now())
		follow, _ = s.messageDAO.CountByTypeSince(userID, domain.ActionTypeFollow, since)
	}()
	go func() {
		defer wg.Done()
		defer tracker.RecordSince("count_system", time.Now())
		sysCount, _ = s.systemNotificationDAO.CountNewSince(userID, since)
	}()
	wg.Wait()

	tracker.Report(fmt.Sprintf("GetUnreadCount(user=%d)", userID))

	return &UnreadCountResponse{
		Private: private,
		Reply:   reply,
		Mention: 0, // 预留，固定返回 0
		Like:    like,
		Follow:  follow,
		System:  sysCount,
	}, nil
}

// ListMessages 获取指定 Tab 的消息列表
// 返回后将用户 last_sys_msg_check 更新为当前时间，清除该类型未读气泡
func (s *MessageService) ListMessages(req MessageListRequest) (*MessageListResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > MaxPageSize {
		req.PageSize = DefaultPageSize
	}

	// @我的预留，直接返回空
	if req.Type == domain.ActionTypeMention {
		return &MessageListResponse{
			Messages: []MessageItemDTO{},
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    0,
		}, nil
	}

	messages, total, err := s.messageDAO.ListByType(req.UserID, req.Type, req.Page, req.PageSize)
	if err != nil {
		return nil, errors.New("消息列表查询失败")
	}

	// 批量查发送者信息
	fromUserIDSet := make(map[uint64]struct{})
	for _, m := range messages {
		fromUserIDSet[m.FromUserID] = struct{}{}
	}
	uids := make([]uint64, 0, len(fromUserIDSet))
	for id := range fromUserIDSet {
		uids = append(uids, id)
	}

	users, _ := s.userDAO.GetByIDs(uids)
	userMap := make(map[uint64]domain.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	items := make([]MessageItemDTO, 0, len(messages))
	for _, m := range messages {
		item := MessageItemDTO{
			ID:         m.ID,
			FromUserID: m.FromUserID,
			Content:    m.Content,
			IsRead:     m.IsRead,
			CreatedAt:  m.CreatedAt,
		}
		if m.BizID != nil {
			bid := *m.BizID
			item.BizID = &bid
		}
		if m.BizType != nil {
			bt := *m.BizType
			item.BizType = &bt
		}
		if u, ok := userMap[m.FromUserID]; ok {
			item.FromUserName = u.Name
			if u.Avatar != nil {
				item.FromUserAvatar = *u.Avatar
			}
		}
		items = append(items, item)
	}

	// 更新最后查阅时间戳，消除未读气泡
	if err := s.updateLastCheck(req.UserID); err != nil {
		log.Printf("[message-service] 更新 last_check 失败 user=%d: %v", req.UserID, err)
	}

	return &MessageListResponse{
		Messages: items,
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
	}, nil
}

// MarkRead 标记消息已读（支持按 IDs 或按 type 批量）
// 直接更新 is_read 字段，不影响 last_sys_msg_check
func (s *MessageService) MarkRead(userID uint64, req MarkReadRequest) error {
	if len(req.IDs) > 0 {
		ids := make([]uint64, 0, len(req.IDs))
		for _, id := range req.IDs {
			ids = append(ids, uint64(id))
		}
		if err := s.messageDAO.MarkReadByIDs(userID, ids); err != nil {
			return err
		}
	}

	if req.Type > 0 {
		if err := s.messageDAO.MarkReadByType(userID, req.Type); err != nil {
			return err
		}
	}

	if len(req.IDs) == 0 && req.Type <= 0 {
		return errors.New("请指定消息 IDs 或消息类型")
	}

	return nil
}

// updateLastCheck 将用户 last_sys_msg_check 更新为当前时间，表示用户刚查看了信箱
func (s *MessageService) updateLastCheck(userID uint64) error {
	now := time.Now()
	return config.DB.Model(&domain.User{}).
		Where("id = ?", userID).
		UpdateColumn("last_sys_msg_check", now).Error
}

// ListSystemNotifications 获取系统通知列表
// 返回后将用户 last_sys_msg_check 更新为当前时间，清除未读气泡
func (s *MessageService) ListSystemNotifications(userID uint64, page, pageSize int) (*SystemListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}

	notifications, total, err := s.systemNotificationDAO.ListPublished(userID, page, pageSize)
	if err != nil {
		return nil, errors.New("系统通知查询失败")
	}

	// 取用户上次查看时间，用于判断是否有新通知
	user, _ := s.userDAO.GetByID(userID)
	var lastCheck time.Time
	if user != nil && user.LastSysMsgCheck != nil {
		lastCheck = *user.LastSysMsgCheck
	}
	hasNew, _ := s.systemNotificationDAO.CountNewSince(userID, lastCheck)

	items := make([]SystemNotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		items = append(items, SystemNotificationDTO{
			ID:          n.ID,
			Title:       n.Title,
			Content:     n.Content,
			PublishTime: n.PublishTime,
		})
	}

	// 更新最后查阅时间戳，消除未读气泡
	if err := s.updateLastCheck(userID); err != nil {
		log.Printf("[message-service] 更新 last_check 失败 user=%d: %v", userID, err)
	}

	return &SystemListResponse{
		Notifications: items,
		HasNew:        hasNew > 0,
		Page:          page,
		PageSize:      pageSize,
		Total:         total,
	}, nil
}

// MarkSystemRead 标记系统通知已读（将 last_sys_msg_check 更新为当前时间）
func (s *MessageService) MarkSystemRead(userID uint64) error {
	return s.updateLastCheck(userID)
}
