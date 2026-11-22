package commercial

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/proto/contentpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type Service struct {
	repo     Repository
	content  contentpb.ContentServiceClient
	payments *PaymentRegistry
	cache    *Cache
	events   EventPublisher
	options  Options
}

func NewService(repo Repository, content contentpb.ContentServiceClient, payments *PaymentRegistry, cache *Cache, events EventPublisher, options Options) *Service {
	if events == nil {
		events = noopEventPublisher{}
	}
	return &Service{
		repo:     repo,
		content:  content,
		payments: payments,
		cache:    cache,
		events:   events,
		options:  options,
	}
}

type CreateAdOrderInput struct {
	UserID     int64
	PostID     int64
	PayChannel string
}

type CreateMembershipOrderInput struct {
	UserID       int64
	PlanCode     string
	BillingCycle string
	PayChannel   string
}

type ReviewAdOrderInput struct {
	OrderNo    string
	ReviewerID int64
	Approved   bool
	Note       string
}

type OrderResult struct {
	OrderNo      string
	AmountCents  int
	DurationDays int
	PlanCode     string
	BillingCycle string
	PayChannel   string
	PayParams    map[string]any
}

type AdminMembershipManageInput struct {
	TargetUserID   int64
	OperatorUserID int64
	PlanCode       string
	DurationValue  int
	DurationUnit   string
	Reason         string
}

type AdminMembershipRevokeInput struct {
	TargetUserID   int64
	OperatorUserID int64
	Reason         string
}

type AdminMembershipDetail struct {
	UserID        int64
	UserNickname  string
	PlanCode      string
	StartsAt      time.Time
	EndsAt        time.Time
	SourceOrderNo string
	UpdatedAt     time.Time
	Operations    []MembershipAdminOperationView
}

func (s *Service) CreateAdOrder(ctx context.Context, input CreateAdOrderInput) (OrderResult, error) {
	if s == nil || s.repo == nil {
		return OrderResult{}, ErrDatabaseUnavailable
	}
	if input.UserID <= 0 || input.PostID <= 0 {
		return OrderResult{}, ErrInvalidArgument
	}
	channel, err := s.options.NormalizePayChannel(input.PayChannel)
	if err != nil {
		return OrderResult{}, err
	}
	owned, err := s.repo.PostOwnedByUser(ctx, input.PostID, input.UserID)
	if err != nil {
		return OrderResult{}, err
	}
	if !owned {
		return OrderResult{}, ErrPostNotOwned
	}
	orderNo := NewOrderNo("AD")
	now := time.Now()
	order := AdCampaignOrder{
		OrderNo:      orderNo,
		UserID:       input.UserID,
		PostID:       input.PostID,
		AmountCents:  s.options.Ad.AmountCents,
		DurationDays: s.options.Ad.DurationDays,
		PayChannel:   channel,
		PayStatus:    OrderStatusPending,
		ReviewStatus: AdReviewPendingPayment,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err = s.repo.CreateAdOrder(ctx, order); err != nil {
		return OrderResult{}, err
	}
	result := OrderResult{
		OrderNo:      orderNo,
		AmountCents:  order.AmountCents,
		DurationDays: order.DurationDays,
		PayChannel:   channel,
		PayParams:    s.buildPayParams(ctx, BizTypeAd, orderNo, order.AmountCents, channel, "帖子广告推广"),
	}
	s.events.Publish(ctx, "commercial.ad.order.created", map[string]any{
		"order_no": orderNo,
		"user_id":  input.UserID,
		"post_id":  input.PostID,
	})
	return result, nil
}

func (s *Service) CreateMembershipOrder(ctx context.Context, input CreateMembershipOrderInput) (OrderResult, error) {
	if s == nil || s.repo == nil {
		return OrderResult{}, ErrDatabaseUnavailable
	}
	if input.UserID <= 0 {
		return OrderResult{}, ErrInvalidArgument
	}
	planCode, err := s.options.NormalizePlanCode(input.PlanCode)
	if err != nil {
		return OrderResult{}, err
	}
	cycle, err := s.options.NormalizeBillingCycle(input.BillingCycle)
	if err != nil {
		return OrderResult{}, err
	}
	channel, err := s.options.NormalizePayChannel(input.PayChannel)
	if err != nil {
		return OrderResult{}, err
	}
	amount, err := s.repo.MembershipPrice(ctx, planCode, cycle)
	if err != nil {
		return OrderResult{}, err
	}
	orderNo := NewOrderNo("MB")
	now := time.Now()
	order := MembershipOrder{
		OrderNo:      orderNo,
		UserID:       input.UserID,
		PlanCode:     planCode,
		BillingCycle: cycle,
		AmountCents:  amount,
		PayChannel:   channel,
		PayStatus:    OrderStatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err = s.repo.CreateMembershipOrder(ctx, order); err != nil {
		return OrderResult{}, err
	}
	result := OrderResult{
		OrderNo:      orderNo,
		PlanCode:     planCode,
		BillingCycle: cycle,
		AmountCents:  amount,
		PayChannel:   channel,
		PayParams:    s.buildPayParams(ctx, BizTypeMembership, orderNo, amount, channel, planCode+" 增值服务"),
	}
	s.events.Publish(ctx, "commercial.membership.order.created", map[string]any{
		"order_no":  orderNo,
		"user_id":   input.UserID,
		"plan_code": planCode,
	})
	return result, nil
}

func (s *Service) HandlePaymentNotify(ctx context.Context, channel string, notify PaymentNotify) error {
	if strings.TrimSpace(notify.OrderNo) == "" {
		return NewError("MISSING_ORDER_NO", "missing out_trade_no", 400)
	}
	if notify.Success {
		result, err := s.repo.MarkOrderPaid(ctx, notify.OrderNo, channel, notify.ProviderTradeNo, notify.Payload)
		if err != nil {
			return err
		}
		if result.UserID > 0 {
			s.cache.InvalidateMembershipPolicy(ctx, result.UserID)
			s.cache.PublishRBACReload(ctx)
		}
		s.events.Publish(ctx, "commercial."+result.BizType+".paid", map[string]any{
			"order_no":          notify.OrderNo,
			"provider_trade_no": notify.ProviderTradeNo,
			"channel":           channel,
		})
		return nil
	}
	_ = s.repo.RecordPaymentTransaction(ctx, notify.OrderNo, BizTypeByOrderNo(notify.OrderNo), channel, notify.ProviderTradeNo, 0, notify.Status, notify.Payload)
	s.events.Publish(ctx, "commercial.payment.notify.failed", map[string]any{
		"order_no": notify.OrderNo,
		"channel":  channel,
		"status":   notify.Status,
	})
	return nil
}

func (s *Service) ReviewAdOrder(ctx context.Context, input ReviewAdOrderInput) error {
	orderNo := strings.TrimSpace(input.OrderNo)
	if orderNo == "" {
		return ErrInvalidArgument
	}
	if err := s.repo.ReviewAdOrder(ctx, orderNo, input.ReviewerID, input.Approved, strings.TrimSpace(input.Note)); err != nil {
		return err
	}
	s.events.Publish(ctx, "commercial.ad.reviewed", map[string]any{
		"order_no":    orderNo,
		"reviewer_id": input.ReviewerID,
		"approved":    input.Approved,
	})
	return nil
}

func (s *Service) ListAdOrders(ctx context.Context, reviewStatus int16, limit int) ([]AdOrderView, error) {
	return s.repo.ListAdOrders(ctx, reviewStatus, normalizeLimit(limit, 20, 100))
}

func (s *Service) ListMembershipOrders(ctx context.Context, limit int) ([]MembershipOrderView, error) {
	return s.repo.ListMembershipOrders(ctx, normalizeLimit(limit, 20, 100))
}

func (s *Service) GetMembershipDetail(ctx context.Context, userID int64) (AdminMembershipDetail, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return AdminMembershipDetail{}, ErrInvalidArgument
	}
	detail, err := s.repo.GetMembershipDetail(ctx, userID)
	if err != nil {
		return AdminMembershipDetail{}, err
	}
	operations, err := s.repo.ListMembershipAdminOperations(ctx, userID, 20)
	if err != nil {
		return AdminMembershipDetail{}, err
	}
	return AdminMembershipDetail{
		UserID:        detail.UserID,
		UserNickname:  NullableStringToString(detail.UserNickname),
		PlanCode:      detail.PlanCode,
		StartsAt:      detail.StartsAt,
		EndsAt:        detail.EndsAt,
		SourceOrderNo: detail.SourceOrderNo,
		UpdatedAt:     detail.UpdatedAt,
		Operations:    operations,
	}, nil
}

func (s *Service) GrantMembership(ctx context.Context, input AdminMembershipManageInput) (MembershipMutationResult, error) {
	return s.runMembershipAdminMutation(ctx, AdminMembershipActionGrant, input)
}

func (s *Service) ExtendMembership(ctx context.Context, input AdminMembershipManageInput) (MembershipMutationResult, error) {
	return s.runMembershipAdminMutation(ctx, AdminMembershipActionExtend, input)
}

func (s *Service) RevokeMembership(ctx context.Context, input AdminMembershipRevokeInput) (MembershipMutationResult, error) {
	if s == nil || s.repo == nil || input.TargetUserID <= 0 || input.OperatorUserID <= 0 {
		return MembershipMutationResult{}, ErrInvalidArgument
	}
	result, err := s.repo.RevokeMembership(ctx, MembershipMutationInput{
		TargetUserID:   input.TargetUserID,
		OperatorUserID: input.OperatorUserID,
		Reason:         strings.TrimSpace(input.Reason),
	}, s.options.ManagedMembershipRoleCodes())
	if err != nil {
		return MembershipMutationResult{}, err
	}
	s.afterMembershipMutation(ctx, result)
	s.events.Publish(ctx, "commercial.membership.admin.revoked", map[string]any{
		"operation_no":     result.OperationNo,
		"target_user_id":   result.UserID,
		"operator_user_id": input.OperatorUserID,
	})
	return result, nil
}

func (s *Service) AdCandidates(ctx context.Context, viewerUserID int64, feedType string, limit int) ([]AdCandidate, error) {
	if s == nil || s.repo == nil || s.content == nil {
		return nil, ErrDatabaseUnavailable
	}
	limit = normalizeLimit(limit, 1, s.options.Ad.MaxAdsPerPage)
	feedType = NormalizeFeedType(feedType)
	campaigns, err := s.repo.QueryAdCampaigns(ctx, viewerUserID, limit, s.options.Ad.SameCooldownHours)
	if err != nil {
		return nil, err
	}
	items := make([]AdCandidate, 0, len(campaigns))
	for _, campaign := range campaigns {
		postResp, err := s.content.GetPost(ctx, &contentpb.GetPostRequest{
			OperatorUserId: viewerUserID,
			PostId:         campaign.PostID,
		})
		if err != nil || postResp.GetBase().GetCode() != contentpb.ContentCode_CONTENT_CODE_OK || postResp.GetPost() == nil {
			if err != nil {
				logx.WithContext(ctx).Errorf("load ad post failed: postID=%d err=%v", campaign.PostID, err)
			}
			continue
		}
		items = append(items, AdCandidate{
			OrderNo:          campaign.OrderNo,
			AdvertiserUserID: campaign.UserID,
			FeedType:         feedType,
			StartsAt:         campaign.StartsAt.Time,
			EndsAt:           campaign.EndsAt.Time,
			Post:             postResp.GetPost(),
		})
	}
	return items, nil
}

func (s *Service) RecordAdEvent(ctx context.Context, orderNo string, viewerUserID int64, feedType string, eventType string) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || viewerUserID <= 0 {
		return ErrInvalidArgument
	}
	feedType = NormalizeFeedType(feedType)
	if err := s.repo.RecordAdEvent(ctx, orderNo, viewerUserID, feedType, eventType); err != nil {
		return err
	}
	s.events.Publish(ctx, "commercial.ad."+eventType, map[string]any{
		"order_no":       orderNo,
		"viewer_user_id": viewerUserID,
		"feed_type":      feedType,
	})
	return nil
}

func (s *Service) MembershipPolicy(ctx context.Context, userID int64) MembershipPolicy {
	if userID <= 0 || s == nil || s.repo == nil {
		return MembershipPolicy{}
	}
	if policy, ok := s.cache.GetMembershipPolicy(ctx, userID); ok {
		return policy
	}
	planCode, err := s.repo.ActiveMembershipPlan(ctx, userID)
	if err != nil {
		return MembershipPolicy{}
	}
	policy := s.options.Membership.Policies[strings.TrimSpace(planCode)]
	s.cache.SetMembershipPolicy(ctx, userID, policy, time.Minute)
	return policy
}

func (s *Service) CreateMediaProcessingJob(ctx context.Context, userID int64, storageKey, mediaType, contentType string, sizeBytes int64, priority int) error {
	if s == nil || s.repo == nil || userID <= 0 || strings.TrimSpace(storageKey) == "" {
		return nil
	}
	if _, ok := s.options.MediaPriorityRule[priority]; !ok {
		priority = 0
	}
	now := time.Now()
	err := s.repo.CreateMediaProcessingJob(ctx, MediaProcessingJob{
		UserID:      userID,
		StorageKey:  storageKey,
		MediaType:   mediaType,
		ContentType: contentType,
		SizeBytes:   sizeBytes,
		Priority:    priority,
		Status:      0,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err == nil {
		s.events.Publish(ctx, "commercial.media.processing.queued", map[string]any{
			"user_id":     userID,
			"storage_key": storageKey,
			"media_type":  mediaType,
			"priority":    priority,
		})
	}
	return err
}

func (s *Service) InsertionPolicy() FeedInsertionPolicy {
	return s.options.InsertionPolicy()
}

func (s *Service) FeedInsertAfter(policy MembershipPolicy) int {
	if policy.LightAdReduction() {
		return s.options.Ad.LightReductionAfter
	}
	return s.options.Ad.FirstAfterPosts
}

func (s *Service) Provider(channel string) (PaymentProvider, error) {
	channel, err := s.options.NormalizePayChannel(channel)
	if err != nil {
		return nil, err
	}
	return s.payments.Provider(channel)
}

func (s *Service) buildPayParams(ctx context.Context, bizType, orderNo string, amount int, channel string, subject string) map[string]any {
	provider, err := s.Provider(channel)
	if err != nil {
		return map[string]any{
			"biz_type":     bizType,
			"channel":      channel,
			"order_no":     orderNo,
			"amount_cents": amount,
			"subject":      subject,
			"mode":         "provider_unconfigured",
			"message":      err.Error(),
		}
	}
	return provider.BuildAppPay(ctx, PaymentOrder{
		BizType:     bizType,
		OrderNo:     orderNo,
		AmountCents: amount,
		Subject:     subject,
	})
}

func normalizeLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func (s *Service) runMembershipAdminMutation(ctx context.Context, action string, input AdminMembershipManageInput) (MembershipMutationResult, error) {
	if s == nil || s.repo == nil || input.TargetUserID <= 0 || input.OperatorUserID <= 0 {
		return MembershipMutationResult{}, ErrInvalidArgument
	}
	planCode, err := s.options.NormalizePlanCode(input.PlanCode)
	if err != nil {
		return MembershipMutationResult{}, err
	}
	durationUnit, err := s.options.NormalizeDurationUnit(input.DurationUnit)
	if err != nil {
		return MembershipMutationResult{}, err
	}
	if input.DurationValue <= 0 {
		return MembershipMutationResult{}, ErrInvalidDuration
	}
	mutationInput := MembershipMutationInput{
		TargetUserID:   input.TargetUserID,
		OperatorUserID: input.OperatorUserID,
		PlanCode:       planCode,
		DurationValue:  input.DurationValue,
		DurationUnit:   durationUnit,
		Reason:         strings.TrimSpace(input.Reason),
	}
	var result MembershipMutationResult
	switch action {
	case AdminMembershipActionGrant:
		result, err = s.repo.GrantMembership(ctx, mutationInput, s.options.ManagedMembershipRoleCodes())
	case AdminMembershipActionExtend:
		result, err = s.repo.ExtendMembership(ctx, mutationInput, s.options.ManagedMembershipRoleCodes())
	default:
		return MembershipMutationResult{}, ErrInvalidArgument
	}
	if err != nil {
		return MembershipMutationResult{}, err
	}
	s.afterMembershipMutation(ctx, result)
	s.events.Publish(ctx, "commercial.membership.admin."+action, map[string]any{
		"operation_no":     result.OperationNo,
		"target_user_id":   result.UserID,
		"operator_user_id": input.OperatorUserID,
		"plan_code":        result.CurrentPlanCode,
		"duration_value":   input.DurationValue,
		"duration_unit":    durationUnit,
	})
	return result, nil
}

func (s *Service) afterMembershipMutation(ctx context.Context, result MembershipMutationResult) {
	if result.UserID <= 0 {
		return
	}
	s.cache.InvalidateMembershipPolicy(ctx, result.UserID)
	s.cache.PublishRBACReload(ctx)
}
