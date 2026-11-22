package logic

import (
	"context"
	"strconv"
	"strings"

	"hfjj/backend/im-service/internal/repository"
	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"

	"github.com/google/uuid"
)

type CreateSystemNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateSystemNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateSystemNotificationLogic {
	return &CreateSystemNotificationLogic{ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateSystemNotificationLogic) CreateSystemNotification(in *impb.CreateSystemNotificationRequest) (*impb.CreateSystemNotificationResponse, error) {
	title := strings.TrimSpace(in.GetTitle())
	content := strings.TrimSpace(in.GetContent())
	if title == "" || content == "" {
		return &impb.CreateSystemNotificationResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "title and content are required"),
		}, nil
	}

	targetUserIDs, err := l.resolveTargetUserIDs(in)
	if err != nil {
		return nil, err
	}
	if len(targetUserIDs) == 0 {
		return &impb.CreateSystemNotificationResponse{
			Base: newStatus(impb.IMCode_IM_CODE_INVALID_ARGUMENT, "target users are required"),
		}, nil
	}

	baseEventID := "admin_notice_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	createdCount := int64(0)
	for _, userID := range targetUserIDs {
		if userID <= 0 {
			continue
		}
		item := repository.SystemNotification{
			UserID:           userID,
			EventID:          baseEventID + "_" + strconv.FormatInt(userID, 10),
			NoticeType:       strings.TrimSpace(in.GetNoticeType()),
			Title:            title,
			Content:          content,
			ResourceType:     strings.TrimSpace(in.GetResourceType()),
			ResourceID:       in.GetResourceId(),
			MediaPayloadJSON: strings.TrimSpace(in.GetMediaPayloadJson()),
			IsRead:           false,
		}
		if err := l.svcCtx.Repo.CreateSystemNotification(l.ctx, item); err != nil {
			return nil, err
		}
		createdCount++
	}

	return &impb.CreateSystemNotificationResponse{
		Base:         newStatus(impb.IMCode_IM_CODE_OK, "ok"),
		CreatedCount: createdCount,
	}, nil
}

func (l *CreateSystemNotificationLogic) resolveTargetUserIDs(in *impb.CreateSystemNotificationRequest) ([]int64, error) {
	if in.GetSendToAll() {
		return l.svcCtx.Repo.ListActiveUserIDs(l.ctx)
	}

	seen := make(map[int64]struct{}, len(in.GetTargetUserIds()))
	targets := make([]int64, 0, len(in.GetTargetUserIds()))
	for _, userID := range in.GetTargetUserIds() {
		if userID <= 0 {
			continue
		}
		if _, exists := seen[userID]; exists {
			continue
		}
		seen[userID] = struct{}{}
		targets = append(targets, userID)
	}
	return targets, nil
}
