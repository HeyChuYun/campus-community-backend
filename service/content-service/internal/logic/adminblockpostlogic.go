package logic

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type AdminBlockPostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlockPostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlockPostLogic {
	return &AdminBlockPostLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlockPostLogic) AdminBlockPost(in *contentpb.AdminBlockPostRequest) (*contentpb.AdminBlockPostResponse, error) {
	if in.GetOperatorUserId() <= 0 || in.GetPostId() <= 0 {
		return &contentpb.AdminBlockPostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid operator_user_id or post_id"),
		}, nil
	}
	post, found, err := l.svcCtx.PostStore.GetPost(l.ctx, in.GetPostId())
	if err != nil {
		return nil, err
	}
	if !found {
		return &contentpb.AdminBlockPostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post not found"),
		}, nil
	}

	categories := normalizeViolationCategories(in.GetCategories())
	reason := strings.TrimSpace(in.GetReason())
	if reason == "" && len(categories) == 0 {
		return &contentpb.AdminBlockPostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "reason or categories is required"),
		}, nil
	}
	muteSeconds := in.GetMuteDurationSeconds()
	if muteSeconds < 0 {
		return &contentpb.AdminBlockPostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid mute_duration_seconds"),
		}, nil
	}

	if err := applyPostViolation(
		l.ctx,
		l.svcCtx,
		post,
		"admin_manual",
		categories,
		reason,
		marshalAny(map[string]any{
			"source":      "admin_manual",
			"categories":  categories,
			"reason":      reason,
			"operator_id": in.GetOperatorUserId(),
		}),
		in.GetOperatorUserId(),
		normalizeRestrictionScope(in.GetMuteScope()),
		time.Duration(muteSeconds)*time.Second,
	); err != nil {
		return nil, err
	}

	post.Status = int32(contentpb.PostStatus_POST_STATUS_BLOCKED)
	return &contentpb.AdminBlockPostResponse{
		Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Post: toPostPB(post),
	}, nil
}

func normalizeViolationCategories(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeRestrictionScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case repository.RestrictionScopeDirectMessage:
		return repository.RestrictionScopeDirectMessage
	default:
		return repository.RestrictionScopePostPublish
	}
}
