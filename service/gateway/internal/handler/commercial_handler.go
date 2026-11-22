package handler

import (
	"context"
	"net/http"
	"time"

	"hfjj/backend/gateway/internal/commercial"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/pkg/constants"
	"hfjj/backend/proto/contentpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type commercialCreateAdOrderReq struct {
	PostID     int64  `json:"post_id"`
	PayChannel string `json:"pay_channel"`
}

type commercialCreateMembershipOrderReq struct {
	PlanCode     string `json:"plan_code"`
	BillingCycle string `json:"billing_cycle"`
	PayChannel   string `json:"pay_channel"`
}

type commercialReviewAdOrderReq struct {
	OrderNo  string `json:"order_no"`
	Approved bool   `json:"approved"`
	Note     string `json:"note"`
}

type commercialAdEventReq struct {
	OrderNo  string `json:"order_no"`
	FeedType string `json:"feed_type"`
}

type commercialAdminMembershipManageReq struct {
	UserID        int64  `json:"user_id"`
	TargetUserID  int64  `json:"target_user_id"`
	PlanCode      string `json:"plan_code"`
	DurationValue int    `json:"duration_value"`
	DurationUnit  string `json:"duration_unit"`
	Reason        string `json:"reason"`
}

type commercialAdminMembershipRevokeReq struct {
	UserID       int64  `json:"user_id"`
	TargetUserID int64  `json:"target_user_id"`
	Reason       string `json:"reason"`
}

func commercialCreateAdOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req commercialCreateAdOrderReq
		if !decodeJSON(w, r, &req) {
			return
		}
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}
		result, err := svcCtx.Commercial.CreateAdOrder(r.Context(), commercial.CreateAdOrderInput{
			UserID:     user.GetUserId(),
			PostID:     req.PostID,
			PayChannel: req.PayChannel,
		})
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, map[string]any{
			"order_no":      result.OrderNo,
			"amount_cents":  result.AmountCents,
			"duration_days": result.DurationDays,
			"pay_channel":   result.PayChannel,
			"pay_params":    result.PayParams,
		})
	}
}

func commercialCreateMembershipOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req commercialCreateMembershipOrderReq
		if !decodeJSON(w, r, &req) {
			return
		}
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}
		result, err := svcCtx.Commercial.CreateMembershipOrder(r.Context(), commercial.CreateMembershipOrderInput{
			UserID:       user.GetUserId(),
			PlanCode:     req.PlanCode,
			BillingCycle: req.BillingCycle,
			PayChannel:   req.PayChannel,
		})
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, map[string]any{
			"order_no":      result.OrderNo,
			"plan_code":     result.PlanCode,
			"billing_cycle": result.BillingCycle,
			"amount_cents":  result.AmountCents,
			"pay_channel":   result.PayChannel,
			"pay_params":    result.PayParams,
		})
	}
}

func commercialAlipayNotifyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialPaymentNotifyHandler(svcCtx, commercial.PayChannelAlipay)
}

func commercialWechatNotifyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialPaymentNotifyHandler(svcCtx, commercial.PayChannelWechat)
}

func commercialPaymentNotifyHandler(svcCtx *svc.ServiceContext, channel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider, err := svcCtx.Commercial.Provider(channel)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("fail"))
			return
		}
		notify, err := provider.ParseNotify(r)
		if err != nil {
			logx.Errorf("parse %s notify: %v", channel, err)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write(provider.FailureResponse("invalid notify"))
			return
		}
		if err = svcCtx.Commercial.HandlePaymentNotify(r.Context(), channel, notify); err != nil {
			logx.Errorf("handle %s notify: order=%s err=%v", channel, notify.OrderNo, err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write(provider.FailureResponse("internal error"))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(provider.SuccessResponse())
	}
}

func commercialListAdOrdersAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.ad.review"); !ok {
			return
		}
		limit := int(parseOptionalQueryInt64(r, "page_size", 20))
		status := int16(parseOptionalQueryInt64(r, "review_status", -1))
		items, err := svcCtx.Commercial.ListAdOrders(r.Context(), status, limit)
		if err != nil {
			logx.Errorf("list ad orders: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{"items": commercialAdOrdersToMap(items)})
	}
}

func commercialReviewAdOrderAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.ad.review")
		if !ok {
			return
		}
		var req commercialReviewAdOrderReq
		if !decodeJSON(w, r, &req) {
			return
		}
		reviewerID := int64(0)
		if snapshot != nil && snapshot.GetUser() != nil {
			reviewerID = snapshot.GetUser().GetUserId()
		}
		err := svcCtx.Commercial.ReviewAdOrder(r.Context(), commercial.ReviewAdOrderInput{
			OrderNo:    req.OrderNo,
			ReviewerID: reviewerID,
			Approved:   req.Approved,
			Note:       req.Note,
		})
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, nil)
	}
}

func commercialListMembershipOrdersAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.membership.read"); !ok {
			return
		}
		limit := int(parseOptionalQueryInt64(r, "page_size", 20))
		items, err := svcCtx.Commercial.ListMembershipOrders(r.Context(), limit)
		if err != nil {
			logx.Errorf("list membership orders: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{"items": commercialMembershipOrdersToMap(items)})
	}
}

func commercialGetMembershipAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		if _, ok := authorizeAdminPermission(w, r, svcCtx, "admin.membership.read"); !ok {
			return
		}
		userID, ok := parseRequiredQueryInt64(w, r, "user_id")
		if !ok {
			return
		}
		detail, err := svcCtx.Commercial.GetMembershipDetail(r.Context(), userID)
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, commercialMembershipDetailToMap(detail))
	}
}

func commercialGrantMembershipAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialManageMembershipAdminHandler(svcCtx, commercial.AdminMembershipActionGrant)
}

func commercialExtendMembershipAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialManageMembershipAdminHandler(svcCtx, commercial.AdminMembershipActionExtend)
}

func commercialManageMembershipAdminHandler(svcCtx *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.membership.manage")
		if !ok {
			return
		}
		var req commercialAdminMembershipManageReq
		if !decodeJSON(w, r, &req) {
			return
		}
		targetUserID := req.UserID
		if targetUserID <= 0 {
			targetUserID = req.TargetUserID
		}
		operatorUserID := int64(0)
		if snapshot != nil && snapshot.GetUser() != nil {
			operatorUserID = snapshot.GetUser().GetUserId()
		}
		input := commercial.AdminMembershipManageInput{
			TargetUserID:   targetUserID,
			OperatorUserID: operatorUserID,
			PlanCode:       req.PlanCode,
			DurationValue:  req.DurationValue,
			DurationUnit:   req.DurationUnit,
			Reason:         req.Reason,
		}
		var (
			result commercial.MembershipMutationResult
			err    error
		)
		switch action {
		case commercial.AdminMembershipActionGrant:
			result, err = svcCtx.Commercial.GrantMembership(r.Context(), input)
		case commercial.AdminMembershipActionExtend:
			result, err = svcCtx.Commercial.ExtendMembership(r.Context(), input)
		default:
			WriteError(w, ErrInternalError)
			return
		}
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, commercialMembershipMutationToMap(result))
	}
}

func commercialRevokeMembershipAdminHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		snapshot, ok := authorizeAdminPermission(w, r, svcCtx, "admin.membership.manage")
		if !ok {
			return
		}
		var req commercialAdminMembershipRevokeReq
		if !decodeJSON(w, r, &req) {
			return
		}
		targetUserID := req.UserID
		if targetUserID <= 0 {
			targetUserID = req.TargetUserID
		}
		operatorUserID := int64(0)
		if snapshot != nil && snapshot.GetUser() != nil {
			operatorUserID = snapshot.GetUser().GetUserId()
		}
		result, err := svcCtx.Commercial.RevokeMembership(r.Context(), commercial.AdminMembershipRevokeInput{
			TargetUserID:   targetUserID,
			OperatorUserID: operatorUserID,
			Reason:         req.Reason,
		})
		if err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, commercialMembershipMutationToMap(result))
	}
}

func commercialAdCandidatesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.feed.read", 0)
		if !ok {
			return
		}
		limit := int(parseOptionalQueryInt64(r, "page_size", 1))
		feedType := commercial.NormalizeFeedType(r.URL.Query().Get("feed_type"))
		candidates, err := svcCtx.Commercial.AdCandidates(r.Context(), user.GetUserId(), feedType, limit)
		if err != nil {
			logx.Errorf("ad candidates: %v", err)
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{
			"items":            commercialAdCandidatesToMap(svcCtx, candidates),
			"insertion_policy": svcCtx.Commercial.InsertionPolicy(),
		})
	}
}

func commercialAdImpressionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialAdEventHandler(svcCtx, "impression")
}

func commercialAdClickHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return commercialAdEventHandler(svcCtx, "click")
}

func commercialAdEventHandler(svcCtx *svc.ServiceContext, eventType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req commercialAdEventReq
		if !decodeJSON(w, r, &req) {
			return
		}
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.feed.read", 0)
		if !ok {
			return
		}
		if err := svcCtx.Commercial.RecordAdEvent(r.Context(), req.OrderNo, user.GetUserId(), req.FeedType, eventType); err != nil {
			writeCommercialError(w, err)
			return
		}
		WriteSuccess(w, nil)
	}
}

func commercialBuildFeedItems(ctx context.Context, svcCtx *svc.ServiceContext, viewerUserID int64, feedType string, page int64, posts []*contentpb.PostItem) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(posts)+1)
	for _, post := range posts {
		items = append(items, map[string]any{
			"type": "post",
			"post": postToMap(svcCtx, post),
		})
	}
	policy := svcCtx.Commercial.MembershipPolicy(ctx, viewerUserID)
	if !policy.AllowsFeedAds() || len(posts) == 0 {
		return items, nil
	}
	insertAfter := svcCtx.Commercial.FeedInsertAfter(policy)
	if page <= 1 && len(posts) < insertAfter {
		return items, nil
	}
	candidates, err := svcCtx.Commercial.AdCandidates(ctx, viewerUserID, feedType, 1)
	if err != nil || len(candidates) == 0 {
		return items, err
	}
	insertIndex := insertAfter
	if insertIndex > len(items) {
		insertIndex = len(items)
	}
	adItem := map[string]any{
		"type": "ad",
		"ad":   commercialAdToMap(candidates[0]),
		"post": postToMap(svcCtx, candidates[0].Post),
	}
	items = append(items[:insertIndex], append([]map[string]any{adItem}, items[insertIndex:]...)...)
	return items, nil
}

func writeCommercialError(w http.ResponseWriter, err error) {
	if bizErr, ok := commercial.AsError(err); ok {
		code := gatewayCodeInvalidArgument
		if bizErr.HTTPStatus == http.StatusNotFound {
			code = gatewayCodeNotFound
		}
		if bizErr.HTTPStatus >= http.StatusInternalServerError {
			code = constants.CodeInternalError
		}
		writeJSON(w, bizErr.HTTPStatus, jsonResponse{Code: code, Message: bizErr.Message})
		return
	}
	logx.Errorf("commercial handler error: %v", err)
	WriteError(w, ErrInternalError)
}

func commercialAdCandidatesToMap(svcCtx *svc.ServiceContext, candidates []commercial.AdCandidate) []map[string]any {
	items := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, map[string]any{
			"ad":   commercialAdToMap(candidate),
			"post": postToMap(svcCtx, candidate.Post),
		})
	}
	return items
}

func commercialAdToMap(candidate commercial.AdCandidate) map[string]any {
	return map[string]any{
		"order_no":           candidate.OrderNo,
		"advertiser_user_id": candidate.AdvertiserUserID,
		"label":              "推广",
		"feed_type":          candidate.FeedType,
		"starts_at":          candidate.StartsAt.Format(time.RFC3339),
		"ends_at":            candidate.EndsAt.Format(time.RFC3339),
	}
}

func commercialAdOrdersToMap(items []commercial.AdOrderView) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"order_no":          item.OrderNo,
			"user_id":           item.UserID,
			"user_nickname":     commercial.NullableStringToString(item.UserNickname),
			"post_id":           item.PostID,
			"amount_cents":      item.AmountCents,
			"duration_days":     item.DurationDays,
			"pay_channel":       item.PayChannel,
			"pay_status":        item.PayStatus,
			"review_status":     item.ReviewStatus,
			"provider_trade_no": commercial.NullableStringToString(item.ProviderTradeNo),
			"starts_at":         commercial.NullableTimeToAny(item.StartsAt),
			"ends_at":           commercial.NullableTimeToAny(item.EndsAt),
			"reviewer_user_id":  commercial.NullableInt64ToAny(item.ReviewerUserID),
			"review_note":       commercial.NullableStringToString(item.ReviewNote),
			"paid_at":           commercial.NullableTimeToAny(item.PaidAt),
			"rejected_at":       commercial.NullableTimeToAny(item.RejectedAt),
			"created_at":        item.CreatedAt.Format(time.RFC3339),
			"updated_at":        item.UpdatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func commercialMembershipOrdersToMap(items []commercial.MembershipOrderView) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"order_no":          item.OrderNo,
			"user_id":           item.UserID,
			"user_nickname":     commercial.NullableStringToString(item.UserNickname),
			"plan_code":         item.PlanCode,
			"billing_cycle":     item.BillingCycle,
			"amount_cents":      item.AmountCents,
			"pay_channel":       item.PayChannel,
			"pay_status":        item.PayStatus,
			"provider_trade_no": commercial.NullableStringToString(item.ProviderTradeNo),
			"paid_at":           commercial.NullableTimeToAny(item.PaidAt),
			"created_at":        item.CreatedAt.Format(time.RFC3339),
			"updated_at":        item.UpdatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func commercialMembershipDetailToMap(detail commercial.AdminMembershipDetail) map[string]any {
	return map[string]any{
		"user_id":         detail.UserID,
		"user_nickname":   detail.UserNickname,
		"plan_code":       detail.PlanCode,
		"starts_at":       detail.StartsAt.Format(time.RFC3339),
		"ends_at":         detail.EndsAt.Format(time.RFC3339),
		"source_order_no": detail.SourceOrderNo,
		"updated_at":      detail.UpdatedAt.Format(time.RFC3339),
		"operations":      commercialMembershipOperationsToMap(detail.Operations),
	}
}

func commercialMembershipOperationsToMap(items []commercial.MembershipAdminOperationView) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"operation_no":       item.OperationNo,
			"user_id":            item.UserID,
			"operator_user_id":   item.OperatorUserID,
			"action":             item.Action,
			"plan_code":          item.PlanCode,
			"duration_value":     item.DurationValue,
			"duration_unit":      item.DurationUnit,
			"previous_plan_code": item.PreviousPlanCode,
			"previous_starts_at": commercial.NullableTimeToAny(item.PreviousStartsAt),
			"previous_ends_at":   commercial.NullableTimeToAny(item.PreviousEndsAt),
			"new_starts_at":      commercial.NullableTimeToAny(item.NewStartsAt),
			"new_ends_at":        commercial.NullableTimeToAny(item.NewEndsAt),
			"reason":             item.Reason,
			"created_at":         item.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func commercialMembershipMutationToMap(result commercial.MembershipMutationResult) map[string]any {
	return map[string]any{
		"action":             result.Action,
		"operation_no":       result.OperationNo,
		"user_id":            result.UserID,
		"previous_plan_code": result.PreviousPlanCode,
		"previous_starts_at": timePointerToAny(result.PreviousStartsAt),
		"previous_ends_at":   timePointerToAny(result.PreviousEndsAt),
		"current_plan_code":  result.CurrentPlanCode,
		"current_starts_at":  timePointerToAny(result.CurrentStartsAt),
		"current_ends_at":    timePointerToAny(result.CurrentEndsAt),
	}
}

func timePointerToAny(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.Format(time.RFC3339)
}
