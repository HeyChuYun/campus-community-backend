package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrQuotaExceeded = errors.New("direct message quota exceeded")

const (
	directMessageBizType   int64 = 1
	directConversationType int64 = 1
	pageLookaheadLimit     int64 = 1
)

type UserBasic struct {
	UserID        int64  `gorm:"column:id"`
	Account       string `gorm:"column:account"`
	Nickname      string `gorm:"column:nickname"`
	AvatarURL     string `gorm:"column:avatar_url"`
	AccountStatus int64  `gorm:"column:account_status"`
}

type IMBinding struct {
	UserID int64  `gorm:"column:user_id"`
	IMUID  string `gorm:"column:im_uid"`
	Status string `gorm:"column:status"`
}

type DirectMessageQuota struct {
	SenderUserID   int64 `gorm:"column:sender_user_id"`
	ReceiverUserID int64 `gorm:"column:receiver_user_id"`
	UsedCount      int64 `gorm:"column:used_count"`
	LimitCount     int64 `gorm:"column:limit_count"`
}

type MessageAudit struct {
	ID               int64     `gorm:"column:id"`
	MessageIDInWK    int64     `gorm:"column:message_id_in_wk"`
	MessageSeq       int64     `gorm:"column:message_seq"`
	ClientMsgNo      string    `gorm:"column:client_msg_no"`
	ConversationType int64     `gorm:"column:conversation_type"`
	SenderUserID     int64     `gorm:"column:sender_user_id"`
	ReceiverUserID   int64     `gorm:"column:receiver_user_id"`
	BizType          int64     `gorm:"column:biz_type"`
	ContentText      string    `gorm:"column:content_text"`
	ContentPreview   string    `gorm:"column:content_preview"`
	Status           string    `gorm:"column:status"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (MessageAudit) TableName() string { return "im_message_audits" }

type DirectConversationSummary struct {
	PeerUserID              int64     `gorm:"column:peer_user_id"`
	PeerAccount             string    `gorm:"column:peer_account"`
	PeerNickname            string    `gorm:"column:peer_nickname"`
	PeerAvatarURL           string    `gorm:"column:peer_avatar_url"`
	LastMessagePreview      string    `gorm:"column:last_message_preview"`
	LastMessageSenderUserID int64     `gorm:"column:sender_user_id"`
	LastMessageCreatedAt    time.Time `gorm:"column:created_at"`
	UnreadCount             int64     `gorm:"column:unread_count"`
}

type DirectMessageRecord struct {
	SenderUserID   int64     `gorm:"column:sender_user_id"`
	ReceiverUserID int64     `gorm:"column:receiver_user_id"`
	ClientMsgNo    string    `gorm:"column:client_msg_no"`
	ContentText    string    `gorm:"column:content_text"`
	MessageIDInWK  int64     `gorm:"column:message_id_in_wk"`
	MessageSeq     int64     `gorm:"column:message_seq"`
	Status         string    `gorm:"column:status"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

type DirectReadCursor struct {
	UserID      int64     `gorm:"column:user_id"`
	PeerUserID  int64     `gorm:"column:peer_user_id"`
	LastReadSeq int64     `gorm:"column:last_read_seq"`
	LastReadAt  time.Time `gorm:"column:last_read_at"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (DirectReadCursor) TableName() string { return "im_direct_read_cursors" }

type InteractionMessage struct {
	ID                int64     `gorm:"column:id"`
	UserID            int64     `gorm:"column:user_id"`
	EventID           string    `gorm:"column:event_id"`
	EventType         int64     `gorm:"column:event_type"`
	OperatorUserID    int64     `gorm:"column:operator_user_id"`
	OperatorNickname  string    `gorm:"column:operator_nickname"`
	OperatorAvatarURL string    `gorm:"column:operator_avatar_url"`
	Title             string    `gorm:"column:title"`
	Content           string    `gorm:"column:content"`
	ResourceType      string    `gorm:"column:resource_type"`
	ResourceID        int64     `gorm:"column:resource_id"`
	IsRead            bool      `gorm:"column:is_read"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (InteractionMessage) TableName() string { return "im_interaction_messages" }

type SystemNotification struct {
	ID               int64     `gorm:"column:id"`
	UserID           int64     `gorm:"column:user_id"`
	EventID          string    `gorm:"column:event_id"`
	NoticeType       string    `gorm:"column:notice_type"`
	Title            string    `gorm:"column:title"`
	Content          string    `gorm:"column:content"`
	ResourceType     string    `gorm:"column:resource_type"`
	ResourceID       int64     `gorm:"column:resource_id"`
	MediaPayloadJSON string    `gorm:"column:media_payload"`
	IsRead           bool      `gorm:"column:is_read"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (SystemNotification) TableName() string { return "im_system_notifications" }

type Repository struct {
	db *gorm.DB
}

type userRow struct {
	ID            int64      `gorm:"column:id"`
	Account       string     `gorm:"column:account"`
	Nickname      string     `gorm:"column:nickname"`
	AvatarURL     string     `gorm:"column:avatar_url"`
	AccountStatus int64      `gorm:"column:account_status"`
	DeletedAt     *time.Time `gorm:"column:deleted_at"`
}

func (userRow) TableName() string { return "users" }

type imBindingRow struct {
	UserID    int64     `gorm:"column:user_id"`
	IMUID     string    `gorm:"column:im_uid"`
	Status    string    `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (imBindingRow) TableName() string { return "im_user_bindings" }

type quotaRow struct {
	SenderUserID   int64     `gorm:"column:sender_user_id"`
	ReceiverUserID int64     `gorm:"column:receiver_user_id"`
	UsedCount      int64     `gorm:"column:used_count"`
	LimitCount     int64     `gorm:"column:limit_count"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (quotaRow) TableName() string { return "im_dm_quotas" }

type interactionMessageRow struct {
	ID             int64     `gorm:"column:id"`
	UserID         int64     `gorm:"column:user_id"`
	EventID        string    `gorm:"column:event_id"`
	EventType      int64     `gorm:"column:event_type"`
	OperatorUserID int64     `gorm:"column:operator_user_id"`
	Title          string    `gorm:"column:title"`
	Content        string    `gorm:"column:content"`
	ResourceType   string    `gorm:"column:resource_type"`
	ResourceID     int64     `gorm:"column:resource_id"`
	IsRead         bool      `gorm:"column:is_read"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (interactionMessageRow) TableName() string { return "im_interaction_messages" }

type systemNotificationRow struct {
	ID           int64     `gorm:"column:id"`
	UserID       int64     `gorm:"column:user_id"`
	EventID      string    `gorm:"column:event_id"`
	NoticeType   string    `gorm:"column:notice_type"`
	Title        string    `gorm:"column:title"`
	Content      string    `gorm:"column:content"`
	ResourceType string    `gorm:"column:resource_type"`
	ResourceID   int64     `gorm:"column:resource_id"`
	MediaPayload string    `gorm:"column:media_payload"`
	IsRead       bool      `gorm:"column:is_read"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (systemNotificationRow) TableName() string { return "im_system_notifications" }

type eventConsumeLogRow struct {
	EventID    string    `gorm:"column:event_id"`
	Consumer   string    `gorm:"column:consumer"`
	Status     string    `gorm:"column:status"`
	ConsumedAt time.Time `gorm:"column:consumed_at;autoCreateTime"`
}

func (eventConsumeLogRow) TableName() string { return "im_event_consume_logs" }

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: requireGormDB(db)}
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("im repository: gorm db is required")
	}
	return db
}

func (r *Repository) GetUserBasic(ctx context.Context, userID int64) (UserBasic, error) {
	if userID <= 0 {
		return UserBasic{}, errors.New("invalid user id")
	}

	var row userRow
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", userID).
		Take(&row).Error; err != nil {
		return UserBasic{}, err
	}

	return UserBasic{
		UserID:        row.ID,
		Account:       row.Account,
		Nickname:      row.Nickname,
		AvatarURL:     row.AvatarURL,
		AccountStatus: row.AccountStatus,
	}, nil
}

func (r *Repository) UpsertBinding(ctx context.Context, userID int64, imUID string) (IMBinding, error) {
	row := imBindingRow{
		UserID:    userID,
		IMUID:     imUID,
		Status:    "active",
		UpdatedAt: time.Now(),
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"im_uid":     imUID,
				"status":     "active",
				"updated_at": gorm.Expr("now()"),
			}),
		}).
		Create(&row).Error; err != nil {
		return IMBinding{}, err
	}

	return IMBinding{
		UserID: row.UserID,
		IMUID:  imUID,
		Status: "active",
	}, nil
}

func (r *Repository) GetBinding(ctx context.Context, userID int64) (IMBinding, bool, error) {
	var row imBindingRow
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IMBinding{}, false, nil
	}
	if err != nil {
		return IMBinding{}, false, err
	}
	return IMBinding{UserID: row.UserID, IMUID: row.IMUID, Status: row.Status}, true, nil
}

func (r *Repository) GetQuota(ctx context.Context, senderUserID, receiverUserID, defaultLimit int64) (DirectMessageQuota, error) {
	var row quotaRow
	err := r.db.WithContext(ctx).
		Where("sender_user_id = ? AND receiver_user_id = ?", senderUserID, receiverUserID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DirectMessageQuota{
			SenderUserID:   senderUserID,
			ReceiverUserID: receiverUserID,
			LimitCount:     defaultLimit,
		}, nil
	}
	if err != nil {
		return DirectMessageQuota{}, err
	}

	return DirectMessageQuota{
		SenderUserID:   row.SenderUserID,
		ReceiverUserID: row.ReceiverUserID,
		UsedCount:      row.UsedCount,
		LimitCount:     row.LimitCount,
	}, nil
}

func (r *Repository) ConsumeQuota(ctx context.Context, senderUserID, receiverUserID, defaultLimit int64) (DirectMessageQuota, error) {
	var quota DirectMessageQuota
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row quotaRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("sender_user_id = ? AND receiver_user_id = ?", senderUserID, receiverUserID).
			Take(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = quotaRow{
				SenderUserID:   senderUserID,
				ReceiverUserID: receiverUserID,
				UsedCount:      pageLookaheadLimit,
				LimitCount:     defaultLimit,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			quota = DirectMessageQuota{
				SenderUserID:   row.SenderUserID,
				ReceiverUserID: row.ReceiverUserID,
				UsedCount:      row.UsedCount,
				LimitCount:     row.LimitCount,
			}
			return nil
		}

		if row.UsedCount >= row.LimitCount {
			quota = DirectMessageQuota{
				SenderUserID:   row.SenderUserID,
				ReceiverUserID: row.ReceiverUserID,
				UsedCount:      row.UsedCount,
				LimitCount:     row.LimitCount,
			}
			return ErrQuotaExceeded
		}

		row.UsedCount++
		if err := tx.Model(&quotaRow{}).
			Where("sender_user_id = ? AND receiver_user_id = ?", senderUserID, receiverUserID).
			Updates(map[string]any{
				"used_count": row.UsedCount,
				"updated_at": gorm.Expr("now()"),
			}).Error; err != nil {
			return err
		}

		quota = DirectMessageQuota{
			SenderUserID:   row.SenderUserID,
			ReceiverUserID: row.ReceiverUserID,
			UsedCount:      row.UsedCount,
			LimitCount:     row.LimitCount,
		}
		return nil
	})

	if err != nil {
		return quota, err
	}
	return quota, nil
}

func (r *Repository) CreateMessageAudit(ctx context.Context, audit MessageAudit) error {
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now()
	}
	return r.db.WithContext(ctx).Create(&audit).Error
}

func (r *Repository) ListDirectConversations(ctx context.Context, userID, page, pageSize int64) ([]DirectConversationSummary, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)

	peerExpr := `case when sender_user_id = ? then receiver_user_id else sender_user_id end`
	ranked := r.db.WithContext(ctx).
		Table("im_message_audits").
		Select(
			peerExpr+` as peer_user_id,
sender_user_id,
coalesce(nullif(content_text, ''), content_preview) as last_message_preview,
created_at,
row_number() over (
	partition by `+peerExpr+`
	order by created_at desc, id desc
) as rn`, userID, userID,
		).
		Where("biz_type = ? AND conversation_type = ? AND (sender_user_id = ? OR receiver_user_id = ?)", directMessageBizType, directConversationType, userID, userID)

	unread := r.db.WithContext(ctx).
		Table("im_message_audits m").
		Select("m.sender_user_id as peer_user_id, count(*) as unread_count").
		Joins("LEFT JOIN im_direct_read_cursors c ON c.user_id = ? AND c.peer_user_id = m.sender_user_id", userID).
		Where("m.biz_type = ? AND m.conversation_type = ? AND m.receiver_user_id = ?", directMessageBizType, directConversationType, userID).
		Where(`NOT (
			(m.message_seq > 0 AND m.message_seq <= coalesce(c.last_read_seq, 0))
			OR (m.created_at <= coalesce(c.last_read_at, ?))
		)`, time.Unix(0, 0)).
		Group("m.sender_user_id")

	var items []DirectConversationSummary
	err := r.db.WithContext(ctx).
		Table("(?) AS ranked", ranked).
		Select(`ranked.peer_user_id,
coalesce(users.account, '') as peer_account,
coalesce(users.nickname, '') as peer_nickname,
coalesce(users.avatar_url, '') as peer_avatar_url,
ranked.last_message_preview,
ranked.sender_user_id,
ranked.created_at,
coalesce(unread.unread_count, 0) as unread_count`).
		Joins("LEFT JOIN users ON users.id = ranked.peer_user_id AND users.deleted_at IS NULL").
		Joins("LEFT JOIN (?) AS unread ON unread.peer_user_id = ranked.peer_user_id", unread).
		Where("ranked.rn = 1").
		Order("ranked.created_at DESC").
		Offset(int(offset)).
		Limit(int(limit + 1)).
		Scan(&items).Error
	if err != nil {
		return nil, false, err
	}

	hasMore := int64(len(items)) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) ListDirectMessages(ctx context.Context, userID, peerUserID, page, pageSize int64) ([]DirectMessageRecord, bool, error) {
	_, _, offset, limit := normalizePage(page, pageSize)

	descItems := make([]DirectMessageRecord, 0, limit+pageLookaheadLimit)
	err := r.db.WithContext(ctx).
		Table("im_message_audits").
		Select(`sender_user_id,
receiver_user_id,
client_msg_no,
coalesce(nullif(content_text, ''), content_preview) as content_text,
message_id_in_wk,
message_seq,
status,
created_at`).
		Where("biz_type = ? AND conversation_type = ?", directMessageBizType, directConversationType).
		Where("((sender_user_id = ? AND receiver_user_id = ?) OR (sender_user_id = ? AND receiver_user_id = ?))", userID, peerUserID, peerUserID, userID).
		Order("created_at DESC, id DESC").
		Offset(int(offset)).
		Limit(int(limit + pageLookaheadLimit)).
		Scan(&descItems).Error
	if err != nil {
		return nil, false, err
	}

	hasMore := int64(len(descItems)) > limit
	if hasMore {
		descItems = descItems[:limit]
	}

	myCursor, peerCursor, err := r.getDirectReadCursorPair(ctx, userID, peerUserID)
	if err != nil {
		return nil, false, err
	}
	for i := range descItems {
		item := &descItems[i]
		switch {
		case item.SenderUserID == userID && item.ReceiverUserID == peerUserID:
			if isMessageCoveredByCursor(item.MessageSeq, item.CreatedAt, peerCursor) {
				item.Status = "read"
			} else {
				item.Status = "sent"
			}
		case item.SenderUserID == peerUserID && item.ReceiverUserID == userID:
			if isMessageCoveredByCursor(item.MessageSeq, item.CreatedAt, myCursor) {
				item.Status = "read"
			} else {
				item.Status = "sent"
			}
		}
	}
	reverseDirectMessages(descItems)
	return descItems, hasMore, nil
}

func (r *Repository) CreateInteractionMessage(ctx context.Context, message InteractionMessage) error {
	row := interactionMessageRow{
		UserID:         message.UserID,
		EventID:        message.EventID,
		EventType:      message.EventType,
		OperatorUserID: message.OperatorUserID,
		Title:          message.Title,
		Content:        message.Content,
		ResourceType:   message.ResourceType,
		ResourceID:     message.ResourceID,
		IsRead:         message.IsRead,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "event_id"}},
			DoNothing: true,
		}).
		Create(&row).Error
}

func (r *Repository) ListInteractionMessages(ctx context.Context, userID, page, pageSize int64) ([]InteractionMessage, bool, int64, error) {
	_, _, offset, limit := normalizePage(page, pageSize)

	var unreadCount int64
	if err := r.db.WithContext(ctx).
		Model(&interactionMessageRow{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount).Error; err != nil {
		return nil, false, 0, err
	}

	items := make([]InteractionMessage, 0, limit+pageLookaheadLimit)
	err := r.db.WithContext(ctx).
		Table("im_interaction_messages m").
		Select(`m.id, m.user_id, m.event_id, m.event_type, m.operator_user_id,
coalesce(u.nickname, '') as operator_nickname,
coalesce(u.avatar_url, '') as operator_avatar_url,
m.title, m.content, m.resource_type, m.resource_id, m.is_read, m.created_at`).
		Joins("LEFT JOIN users u ON u.id = m.operator_user_id AND u.deleted_at IS NULL").
		Where("m.user_id = ?", userID).
		Order("m.id DESC").
		Offset(int(offset)).
		Limit(int(limit + pageLookaheadLimit)).
		Scan(&items).Error
	if err != nil {
		return nil, false, 0, err
	}

	hasMore := int64(len(items)) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, unreadCount, nil
}

func (r *Repository) MarkInteractionMessagesRead(ctx context.Context, userID int64, interactionIDs []int64) (int64, error) {
	if len(interactionIDs) == 0 {
		return r.CountUnreadInteractions(ctx, userID)
	}
	if err := r.db.WithContext(ctx).
		Model(&interactionMessageRow{}).
		Where("user_id = ? AND id IN ?", userID, interactionIDs).
		Update("is_read", true).Error; err != nil {
		return 0, err
	}
	return r.CountUnreadInteractions(ctx, userID)
}

func (r *Repository) CountUnreadInteractions(ctx context.Context, userID int64) (int64, error) {
	var unread int64
	err := r.db.WithContext(ctx).
		Model(&interactionMessageRow{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unread).Error
	return unread, err
}

func (r *Repository) CreateSystemNotification(ctx context.Context, item SystemNotification) error {
	row := systemNotificationRow{
		UserID:       item.UserID,
		EventID:      item.EventID,
		NoticeType:   item.NoticeType,
		Title:        item.Title,
		Content:      item.Content,
		ResourceType: item.ResourceType,
		ResourceID:   item.ResourceID,
		MediaPayload: item.MediaPayloadJSON,
		IsRead:       item.IsRead,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "event_id"}},
			DoNothing: true,
		}).
		Create(&row).Error
}

func (r *Repository) ListSystemNotifications(ctx context.Context, userID, page, pageSize int64) ([]SystemNotification, bool, int64, error) {
	_, _, offset, limit := normalizePage(page, pageSize)

	var unreadCount int64
	if err := r.db.WithContext(ctx).
		Model(&systemNotificationRow{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount).Error; err != nil {
		return nil, false, 0, err
	}

	items := make([]SystemNotification, 0, limit+pageLookaheadLimit)
	err := r.db.WithContext(ctx).
		Table("im_system_notifications").
		Select(`id, user_id, event_id, notice_type, title, content,
resource_type, resource_id, media_payload, is_read, created_at`).
		Where("user_id = ?", userID).
		Order("id DESC").
		Offset(int(offset)).
		Limit(int(limit + pageLookaheadLimit)).
		Scan(&items).Error
	if err != nil {
		return nil, false, 0, err
	}

	hasMore := int64(len(items)) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, unreadCount, nil
}

func (r *Repository) MarkSystemNotificationsRead(ctx context.Context, userID int64, notificationIDs []int64) (int64, error) {
	if len(notificationIDs) == 0 {
		return r.CountUnreadSystemNotifications(ctx, userID)
	}
	if err := r.db.WithContext(ctx).
		Model(&systemNotificationRow{}).
		Where("user_id = ? AND id IN ?", userID, notificationIDs).
		Update("is_read", true).Error; err != nil {
		return 0, err
	}
	return r.CountUnreadSystemNotifications(ctx, userID)
}

func (r *Repository) CountUnreadSystemNotifications(ctx context.Context, userID int64) (int64, error) {
	var unread int64
	err := r.db.WithContext(ctx).
		Model(&systemNotificationRow{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unread).Error
	return unread, err
}

func (r *Repository) ListActiveUserIDs(ctx context.Context) ([]int64, error) {
	var userIDs []int64
	err := r.db.WithContext(ctx).
		Model(&userRow{}).
		Where("deleted_at IS NULL").
		Order("id ASC").
		Pluck("id", &userIDs).Error
	return userIDs, err
}

func (r *Repository) GetTotalUnreadDirectMessageCount(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, nil
	}

	var count int64
	err := r.db.WithContext(ctx).
		Table("im_message_audits m").
		Joins("LEFT JOIN im_direct_read_cursors c ON c.user_id = ? AND c.peer_user_id = m.sender_user_id", userID).
		Where("m.biz_type = ? AND m.conversation_type = ? AND m.receiver_user_id = ?", directMessageBizType, directConversationType, userID).
		Where(`NOT (
			(m.message_seq > 0 AND m.message_seq <= coalesce(c.last_read_seq, 0))
			OR (m.created_at <= coalesce(c.last_read_at, ?))
		)`, time.Unix(0, 0)).
		Count(&count).Error
	return count, err
}

func (r *Repository) MarkDirectMessagesRead(ctx context.Context, receiverUserID, senderUserID, messageSeq int64) error {
	if receiverUserID <= 0 || senderUserID <= 0 {
		return nil
	}

	if _, err := r.AdvanceDirectReadCursor(ctx, receiverUserID, senderUserID, messageSeq); err != nil {
		return err
	}

	query := r.db.WithContext(ctx).
		Model(&MessageAudit{}).
		Where("biz_type = ? AND conversation_type = ? AND sender_user_id = ? AND receiver_user_id = ? AND status <> ?", directMessageBizType, directConversationType, senderUserID, receiverUserID, "read")
	if messageSeq > 0 {
		query = query.Where("(message_seq <= 0 OR message_seq <= ?)", messageSeq)
	}
	return query.Update("status", "read").Error
}

func (r *Repository) GetDirectReadCursor(ctx context.Context, userID, peerUserID int64) (DirectReadCursor, error) {
	if userID <= 0 || peerUserID <= 0 {
		return DirectReadCursor{}, nil
	}

	var cursor DirectReadCursor
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND peer_user_id = ?", userID, peerUserID).
		Take(&cursor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DirectReadCursor{}, nil
	}
	if err != nil {
		return DirectReadCursor{}, err
	}
	return cursor, nil
}

func (r *Repository) getDirectReadCursorPair(ctx context.Context, userID, peerUserID int64) (DirectReadCursor, DirectReadCursor, error) {
	if userID <= 0 || peerUserID <= 0 {
		return DirectReadCursor{}, DirectReadCursor{}, nil
	}

	var rows []DirectReadCursor
	err := r.db.WithContext(ctx).
		Where("(user_id = ? AND peer_user_id = ?) OR (user_id = ? AND peer_user_id = ?)", userID, peerUserID, peerUserID, userID).
		Find(&rows).Error
	if err != nil {
		return DirectReadCursor{}, DirectReadCursor{}, err
	}

	var myCursor DirectReadCursor
	var peerCursor DirectReadCursor
	for _, row := range rows {
		switch {
		case row.UserID == userID && row.PeerUserID == peerUserID:
			myCursor = row
		case row.UserID == peerUserID && row.PeerUserID == userID:
			peerCursor = row
		}
	}
	return myCursor, peerCursor, nil
}

func (r *Repository) AdvanceDirectReadCursor(ctx context.Context, userID, peerUserID, messageSeq int64) (DirectReadCursor, error) {
	if userID <= 0 || peerUserID <= 0 {
		return DirectReadCursor{}, nil
	}

	var resolved struct {
		LastReadSeq int64     `gorm:"column:last_read_seq"`
		LastReadAt  time.Time `gorm:"column:last_read_at"`
	}

	query := r.db.WithContext(ctx).
		Model(&MessageAudit{}).
		Select("coalesce(max(message_seq), 0) as last_read_seq, coalesce(max(created_at), ?) as last_read_at", time.Unix(0, 0)).
		Where("biz_type = ? AND conversation_type = ? AND sender_user_id = ? AND receiver_user_id = ?", directMessageBizType, directConversationType, peerUserID, userID)
	if messageSeq > 0 {
		query = query.Where("(message_seq <= 0 OR message_seq <= ?)", messageSeq)
	}
	if err := query.Scan(&resolved).Error; err != nil {
		return DirectReadCursor{}, err
	}

	current, err := r.GetDirectReadCursor(ctx, userID, peerUserID)
	if err != nil {
		return DirectReadCursor{}, err
	}
	next := mergeDirectReadCursor(current, DirectReadCursor{
		UserID:      userID,
		PeerUserID:  peerUserID,
		LastReadSeq: resolved.LastReadSeq,
		LastReadAt:  resolved.LastReadAt,
	})
	if next.UserID == 0 {
		next.UserID = userID
		next.PeerUserID = peerUserID
	}

	row := DirectReadCursor{
		UserID:      next.UserID,
		PeerUserID:  next.PeerUserID,
		LastReadSeq: next.LastReadSeq,
		LastReadAt:  next.LastReadAt,
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "peer_user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"last_read_seq": next.LastReadSeq,
				"last_read_at":  next.LastReadAt,
				"updated_at":    gorm.Expr("now()"),
			}),
		}).
		Create(&row).Error; err != nil {
		return DirectReadCursor{}, err
	}
	return next, nil
}

func (r *Repository) HasConsumedEvent(ctx context.Context, eventID, consumer string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&eventConsumeLogRow{}).
		Where("event_id = ? AND consumer = ?", eventID, consumer).
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) MarkConsumedEvent(ctx context.Context, eventID, consumer, status string) error {
	row := eventConsumeLogRow{
		EventID:    eventID,
		Consumer:   consumer,
		Status:     status,
		ConsumedAt: time.Now(),
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}, {Name: "consumer"}},
			DoNothing: true,
		}).
		Create(&row).Error
}

func normalizePage(page, pageSize int64) (int64, int64, int64, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	return page, pageSize, offset, pageSize
}

func mergeDirectReadCursor(current, next DirectReadCursor) DirectReadCursor {
	cursor := current
	if cursor.UserID == 0 {
		cursor.UserID = next.UserID
	}
	if cursor.PeerUserID == 0 {
		cursor.PeerUserID = next.PeerUserID
	}
	if next.LastReadSeq > cursor.LastReadSeq {
		cursor.LastReadSeq = next.LastReadSeq
	}
	if next.LastReadAt.After(cursor.LastReadAt) {
		cursor.LastReadAt = next.LastReadAt
	}
	return cursor
}

func isMessageCoveredByCursor(messageSeq int64, createdAt time.Time, cursor DirectReadCursor) bool {
	if cursor.LastReadSeq > 0 && messageSeq > 0 && messageSeq <= cursor.LastReadSeq {
		return true
	}
	if !cursor.LastReadAt.IsZero() && !createdAt.After(cursor.LastReadAt) {
		return true
	}
	return false
}

func TrimPreview(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= 80 {
		return content
	}
	return string(runes[:80])
}

func reverseDirectMessages(items []DirectMessageRecord) {
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
}
