package commercial

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	PostOwnedByUser(ctx context.Context, postID, userID int64) (bool, error)
	MembershipPrice(ctx context.Context, planCode, cycle string) (int, error)
	CreateAdOrder(ctx context.Context, order AdCampaignOrder) error
	CreateMembershipOrder(ctx context.Context, order MembershipOrder) error
	MarkOrderPaid(ctx context.Context, orderNo, channel, providerTradeNo string, payload any) (PaidResult, error)
	ReviewAdOrder(ctx context.Context, orderNo string, reviewerID int64, approved bool, note string) error
	ListAdOrders(ctx context.Context, reviewStatus int16, limit int) ([]AdOrderView, error)
	ListMembershipOrders(ctx context.Context, limit int) ([]MembershipOrderView, error)
	QueryAdCampaigns(ctx context.Context, viewerUserID int64, limit int, cooldownHours int) ([]AdCampaignOrder, error)
	RecordAdEvent(ctx context.Context, orderNo string, viewerUserID int64, feedType string, eventType string) error
	RecordPaymentTransaction(ctx context.Context, orderNo, bizType, channel, providerTradeNo string, amount int, status string, payload any) error
	ActiveMembershipPlan(ctx context.Context, userID int64) (string, error)
	GetMembershipDetail(ctx context.Context, userID int64) (MembershipDetail, error)
	ListMembershipAdminOperations(ctx context.Context, userID int64, limit int) ([]MembershipAdminOperationView, error)
	GrantMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error)
	ExtendMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error)
	RevokeMembership(ctx context.Context, input MembershipMutationInput, managedRoleCodes []string) (MembershipMutationResult, error)
	CreateMediaProcessingJob(ctx context.Context, job MediaProcessingJob) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewGormRepository(db *gorm.DB) Repository {
	return &gormRepository{db: requireGormDB(db)}
}

type MembershipPlan struct {
	PlanCode           string `gorm:"column:plan_code;primaryKey"`
	MonthlyAmountCents int    `gorm:"column:monthly_amount_cents"`
	YearlyAmountCents  int    `gorm:"column:yearly_amount_cents"`
	IsActive           bool   `gorm:"column:is_active"`
}

func (MembershipPlan) TableName() string { return "membership_plans" }

type MembershipOrder struct {
	OrderNo         string         `gorm:"column:order_no;primaryKey"`
	UserID          int64          `gorm:"column:user_id"`
	PlanCode        string         `gorm:"column:plan_code"`
	BillingCycle    string         `gorm:"column:billing_cycle"`
	AmountCents     int            `gorm:"column:amount_cents"`
	PayChannel      string         `gorm:"column:pay_channel"`
	PayStatus       int16          `gorm:"column:pay_status"`
	ProviderTradeNo sql.NullString `gorm:"column:provider_trade_no"`
	PaidAt          sql.NullTime   `gorm:"column:paid_at"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
}

func (MembershipOrder) TableName() string { return "membership_orders" }

type UserMembership struct {
	UserID        int64     `gorm:"column:user_id;primaryKey"`
	PlanCode      string    `gorm:"column:plan_code"`
	StartsAt      time.Time `gorm:"column:starts_at"`
	EndsAt        time.Time `gorm:"column:ends_at"`
	SourceOrderNo string    `gorm:"column:source_order_no"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (UserMembership) TableName() string { return "user_memberships" }

type AdCampaignOrder struct {
	OrderNo         string         `gorm:"column:order_no;primaryKey"`
	UserID          int64          `gorm:"column:user_id"`
	PostID          int64          `gorm:"column:post_id"`
	AmountCents     int            `gorm:"column:amount_cents"`
	DurationDays    int            `gorm:"column:duration_days"`
	PayChannel      string         `gorm:"column:pay_channel"`
	PayStatus       int16          `gorm:"column:pay_status"`
	ReviewStatus    int16          `gorm:"column:review_status"`
	ProviderTradeNo sql.NullString `gorm:"column:provider_trade_no"`
	StartsAt        sql.NullTime   `gorm:"column:starts_at"`
	EndsAt          sql.NullTime   `gorm:"column:ends_at"`
	ReviewerUserID  sql.NullInt64  `gorm:"column:reviewer_user_id"`
	ReviewNote      string         `gorm:"column:review_note"`
	PaidAt          sql.NullTime   `gorm:"column:paid_at"`
	RejectedAt      sql.NullTime   `gorm:"column:rejected_at"`
	CreatedAt       time.Time      `gorm:"column:created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at"`
}

func (AdCampaignOrder) TableName() string { return "ad_campaign_orders" }

type AdImpression struct {
	CampaignOrderNo string    `gorm:"column:campaign_order_no"`
	ViewerUserID    int64     `gorm:"column:viewer_user_id"`
	FeedType        string    `gorm:"column:feed_type"`
	ImpressionAt    time.Time `gorm:"column:impression_at"`
}

func (AdImpression) TableName() string { return "ad_impressions" }

type AdClick struct {
	CampaignOrderNo string    `gorm:"column:campaign_order_no"`
	ViewerUserID    int64     `gorm:"column:viewer_user_id"`
	FeedType        string    `gorm:"column:feed_type"`
	ClickedAt       time.Time `gorm:"column:clicked_at"`
}

func (AdClick) TableName() string { return "ad_clicks" }

type MediaProcessingJob struct {
	UserID      int64     `gorm:"column:user_id"`
	StorageKey  string    `gorm:"column:storage_key"`
	MediaType   string    `gorm:"column:media_type"`
	ContentType string    `gorm:"column:content_type"`
	SizeBytes   int64     `gorm:"column:size_bytes"`
	Priority    int       `gorm:"column:priority"`
	Status      int16     `gorm:"column:status"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (MediaProcessingJob) TableName() string { return "media_processing_jobs" }

type PaymentTransaction struct {
	OrderNo         string    `gorm:"column:order_no"`
	BizType         string    `gorm:"column:biz_type"`
	Channel         string    `gorm:"column:channel"`
	ProviderTradeNo string    `gorm:"column:provider_trade_no"`
	AmountCents     int       `gorm:"column:amount_cents"`
	Status          string    `gorm:"column:status"`
	NotifyPayload   string    `gorm:"column:notify_payload"`
	CreatedAt       time.Time `gorm:"column:created_at"`
}

func (PaymentTransaction) TableName() string { return "payment_transactions" }

type CasbinRule struct {
	ID    uint   `gorm:"column:id;primaryKey"`
	PType string `gorm:"column:ptype"`
	V0    string `gorm:"column:v0"`
	V1    string `gorm:"column:v1"`
}

func (CasbinRule) TableName() string { return "casbin_rule" }

type PaidResult struct {
	BizType string
	UserID  int64
}

type AdOrderView struct {
	OrderNo         string
	UserID          int64
	UserNickname    sql.NullString
	PostID          int64
	AmountCents     int
	DurationDays    int
	PayChannel      string
	PayStatus       int16
	ReviewStatus    int16
	ProviderTradeNo sql.NullString
	StartsAt        sql.NullTime
	EndsAt          sql.NullTime
	ReviewerUserID  sql.NullInt64
	ReviewNote      sql.NullString
	PaidAt          sql.NullTime
	RejectedAt      sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type MembershipOrderView struct {
	OrderNo         string
	UserID          int64
	UserNickname    sql.NullString
	PlanCode        string
	BillingCycle    string
	AmountCents     int
	PayChannel      string
	PayStatus       int16
	ProviderTradeNo sql.NullString
	PaidAt          sql.NullTime
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r *gormRepository) PostOwnedByUser(ctx context.Context, postID, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("posts").
		Where("id = ? and author_user_id = ? and visibility = 1 and status = 1 and deleted_at is null", postID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) MembershipPrice(ctx context.Context, planCode, cycle string) (int, error) {
	var plan MembershipPlan
	if err := r.db.WithContext(ctx).Where("plan_code = ? and is_active = true", planCode).First(&plan).Error; err != nil {
		return 0, err
	}
	if cycle == BillingCycleYear {
		return plan.YearlyAmountCents, nil
	}
	return plan.MonthlyAmountCents, nil
}

func (r *gormRepository) CreateAdOrder(ctx context.Context, order AdCampaignOrder) error {
	return r.db.WithContext(ctx).Create(&order).Error
}

func (r *gormRepository) CreateMembershipOrder(ctx context.Context, order MembershipOrder) error {
	return r.db.WithContext(ctx).Create(&order).Error
}

func (r *gormRepository) MarkOrderPaid(ctx context.Context, orderNo, channel, providerTradeNo string, payload any) (PaidResult, error) {
	switch BizTypeByOrderNo(orderNo) {
	case BizTypeMembership:
		return r.markMembershipOrderPaid(ctx, orderNo, channel, providerTradeNo, payload)
	case BizTypeAd:
		return r.markAdOrderPaid(ctx, orderNo, channel, providerTradeNo, payload)
	default:
		return PaidResult{}, ErrUnsupportedOrderType
	}
}

func (r *gormRepository) markMembershipOrderPaid(ctx context.Context, orderNo, channel, providerTradeNo string, payload any) (PaidResult, error) {
	var result PaidResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order MembershipOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return mapNotFound(err)
		}
		result = PaidResult{BizType: BizTypeMembership, UserID: order.UserID}
		if order.PayStatus == OrderStatusPaid {
			return nil
		}
		now := time.Now()
		startsAt := now
		recordStartsAt := now
		current, err := queryMembershipForUpdate(tx, "user_id = ?", order.UserID)
		if err != nil {
			return err
		}
		if current != nil && current.PlanCode == order.PlanCode && current.EndsAt.After(now) {
			startsAt = current.EndsAt
			recordStartsAt = current.StartsAt
		}
		endsAt := startsAt.AddDate(0, 1, 0)
		if order.BillingCycle == BillingCycleYear {
			endsAt = startsAt.AddDate(1, 0, 0)
		}

		if err := tx.Model(&MembershipOrder{}).Where("order_no = ?", orderNo).Updates(map[string]any{
			"pay_status":        OrderStatusPaid,
			"provider_trade_no": providerTradeNo,
			"paid_at":           now,
			"updated_at":        now,
		}).Error; err != nil {
			return err
		}
		membership := UserMembership{
			UserID:        order.UserID,
			PlanCode:      order.PlanCode,
			StartsAt:      recordStartsAt,
			EndsAt:        endsAt,
			SourceOrderNo: orderNo,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"plan_code":       membership.PlanCode,
				"starts_at":       membership.StartsAt,
				"ends_at":         membership.EndsAt,
				"source_order_no": membership.SourceOrderNo,
				"updated_at":      now,
			}),
		}).Create(&membership).Error; err != nil {
			return err
		}
		userIDText := strconv.FormatInt(order.UserID, 10)
		if err := tx.Where("ptype = ? and v0 = ? and v1 in ?", "g", userIDText, []string{PlanPremium, PlanPremiumPro}).Delete(&CasbinRule{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&CasbinRule{PType: "g", V0: userIDText, V1: order.PlanCode}).Error; err != nil {
			return err
		}
		return r.recordPaymentTransaction(ctx, tx, orderNo, BizTypeMembership, channel, providerTradeNo, order.AmountCents, "paid", payload)
	})
	return result, err
}

func (r *gormRepository) markAdOrderPaid(ctx context.Context, orderNo, channel, providerTradeNo string, payload any) (PaidResult, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order AdCampaignOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return mapNotFound(err)
		}
		if order.PayStatus != OrderStatusPaid {
			now := time.Now()
			if err := tx.Model(&AdCampaignOrder{}).Where("order_no = ?", orderNo).Updates(map[string]any{
				"pay_status":        OrderStatusPaid,
				"review_status":     AdReviewPendingReview,
				"provider_trade_no": providerTradeNo,
				"paid_at":           now,
				"updated_at":        now,
			}).Error; err != nil {
				return err
			}
		}
		return r.recordPaymentTransaction(ctx, tx, orderNo, BizTypeAd, channel, providerTradeNo, order.AmountCents, "paid", payload)
	})
	return PaidResult{BizType: BizTypeAd}, err
}

func (r *gormRepository) ReviewAdOrder(ctx context.Context, orderNo string, reviewerID int64, approved bool, note string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var order AdCampaignOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_no = ?", orderNo).First(&order).Error; err != nil {
			return mapNotFound(err)
		}
		if order.PayStatus != OrderStatusPaid {
			return ErrOrderNotPaid
		}
		if order.ReviewStatus != AdReviewPendingReview {
			return ErrOrderNotPendingReview
		}
		now := time.Now()
		updates := map[string]any{
			"reviewer_user_id": nullableReviewerID(reviewerID),
			"review_note":      note,
			"updated_at":       now,
		}
		if approved {
			updates["review_status"] = AdReviewApproved
			updates["starts_at"] = now
			updates["ends_at"] = now.AddDate(0, 0, order.DurationDays)
		} else {
			updates["review_status"] = AdReviewRejected
			updates["rejected_at"] = now
		}
		return tx.Model(&AdCampaignOrder{}).Where("order_no = ?", orderNo).Updates(updates).Error
	})
}

func (r *gormRepository) ListAdOrders(ctx context.Context, reviewStatus int16, limit int) ([]AdOrderView, error) {
	limit = normalizeListLimit(limit)
	query := r.db.WithContext(ctx).Table("ad_campaign_orders a").
		Select(`a.order_no, a.user_id, u.nickname as user_nickname, a.post_id, a.amount_cents, a.duration_days,
			a.pay_channel, a.pay_status, a.review_status, a.provider_trade_no, a.starts_at, a.ends_at,
			a.reviewer_user_id, a.review_note, a.paid_at, a.rejected_at, a.created_at, a.updated_at`).
		Joins("left join users u on u.id = a.user_id").
		Order("a.created_at desc").
		Limit(limit)
	if reviewStatus >= 0 {
		query = query.Where("a.review_status = ?", reviewStatus)
	}
	var items []AdOrderView
	return items, query.Scan(&items).Error
}

func (r *gormRepository) ListMembershipOrders(ctx context.Context, limit int) ([]MembershipOrderView, error) {
	limit = normalizeListLimit(limit)
	var items []MembershipOrderView
	err := r.db.WithContext(ctx).Table("membership_orders m").
		Select(`m.order_no, m.user_id, u.nickname as user_nickname, m.plan_code, m.billing_cycle,
			m.amount_cents, m.pay_channel, m.pay_status, m.provider_trade_no, m.paid_at, m.created_at, m.updated_at`).
		Joins("left join users u on u.id = m.user_id").
		Order("m.created_at desc").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *gormRepository) QueryAdCampaigns(ctx context.Context, viewerUserID int64, limit int, cooldownHours int) ([]AdCampaignOrder, error) {
	limit = normalizeListLimit(limit)
	var items []AdCampaignOrder
	err := r.db.WithContext(ctx).Table("ad_campaign_orders a").
		Select("a.*").
		Joins("join posts p on p.id = a.post_id").
		Where(`a.pay_status = ?
			and a.review_status = ?
			and a.starts_at <= now()
			and a.ends_at > now()
			and a.user_id <> ?
			and p.visibility = 1
			and p.status = 1
			and p.deleted_at is null
			and not exists (
				select 1 from ad_impressions i
				where i.campaign_order_no = a.order_no
				  and i.viewer_user_id = ?
				  and i.impression_at >= now() - (? * interval '1 hour')
			)`, OrderStatusPaid, AdReviewApproved, viewerUserID, viewerUserID, cooldownHours).
		Order("(select count(*) from ad_impressions i where i.campaign_order_no = a.order_no and i.impression_at >= now() - interval '24 hours') asc").
		Order("a.created_at desc").
		Limit(limit).
		Scan(&items).Error
	return items, err
}

func (r *gormRepository) RecordAdEvent(ctx context.Context, orderNo string, viewerUserID int64, feedType string, eventType string) error {
	now := time.Now()
	if eventType == "click" {
		return r.db.WithContext(ctx).Create(&AdClick{
			CampaignOrderNo: orderNo,
			ViewerUserID:    viewerUserID,
			FeedType:        feedType,
			ClickedAt:       now,
		}).Error
	}
	return r.db.WithContext(ctx).Create(&AdImpression{
		CampaignOrderNo: orderNo,
		ViewerUserID:    viewerUserID,
		FeedType:        feedType,
		ImpressionAt:    now,
	}).Error
}

func (r *gormRepository) RecordPaymentTransaction(ctx context.Context, orderNo, bizType, channel, providerTradeNo string, amount int, status string, payload any) error {
	return r.recordPaymentTransaction(ctx, r.db, orderNo, bizType, channel, providerTradeNo, amount, status, payload)
}

func (r *gormRepository) ActiveMembershipPlan(ctx context.Context, userID int64) (string, error) {
	membership, err := queryMembership(
		r.db.WithContext(ctx),
		"user_id = ? and starts_at <= now() and ends_at > now()",
		userID,
	)
	if err != nil {
		return "", err
	}
	if membership == nil {
		return "", nil
	}
	return membership.PlanCode, nil
}

func (r *gormRepository) CreateMediaProcessingJob(ctx context.Context, job MediaProcessingJob) error {
	return r.db.WithContext(ctx).Create(&job).Error
}

func (r *gormRepository) recordPaymentTransaction(ctx context.Context, tx *gorm.DB, orderNo, bizType, channel, providerTradeNo string, amount int, status string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payment payload: %w", err)
	}
	return tx.WithContext(ctx).Create(&PaymentTransaction{
		OrderNo:         orderNo,
		BizType:         bizType,
		Channel:         channel,
		ProviderTradeNo: providerTradeNo,
		AmountCents:     amount,
		Status:          status,
		NotifyPayload:   string(payloadBytes),
		CreatedAt:       time.Now(),
	}).Error
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	return err
}

func nullableReviewerID(reviewerID int64) any {
	if reviewerID <= 0 {
		return nil
	}
	return reviewerID
}

func requireGormDB(db *gorm.DB) *gorm.DB {
	if db == nil {
		panic("commercial: gorm db is required")
	}
	return db
}

func queryMembership(tx *gorm.DB, condition string, args ...any) (*UserMembership, error) {
	var membership UserMembership
	result := tx.Where(condition, args...).Limit(1).Find(&membership)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	return &membership, nil
}

func queryMembershipForUpdate(tx *gorm.DB, condition string, args ...any) (*UserMembership, error) {
	return queryMembership(tx.Clauses(clause.Locking{Strength: "UPDATE"}), condition, args...)
}

func normalizeListLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}
