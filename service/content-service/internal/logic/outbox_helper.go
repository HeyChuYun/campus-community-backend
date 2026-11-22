package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

var contentEventSeq atomic.Int64

type contentDomainEvent struct {
	EventID        string `json:"event_id"`
	EventType      string `json:"event_type"`
	OccurredAt     string `json:"occurred_at"`
	Producer       string `json:"producer"`
	TraceID        string `json:"trace_id"`
	OperatorUserID int64  `json:"operator_user_id"`
	TargetUserID   int64  `json:"target_user_id"`
	ResourceType   string `json:"resource_type"`
	ResourceID     int64  `json:"resource_id"`
	Payload        any    `json:"payload"`
}

func emitContentEvent(ctx context.Context, svcCtx *svc.ServiceContext, eventType string, operatorUserID, targetUserID int64, resourceType string, resourceID int64, payload any) {
	// 当前先采用“业务成功后补写 outbox”的方式，后续如需绝对事务一致性，再下沉为仓储层统一事务。
	event := contentDomainEvent{
		EventID:        newContentEventID(),
		EventType:      eventType,
		OccurredAt:     time.Now().Format(time.RFC3339),
		Producer:       "content-service",
		TraceID:        "",
		OperatorUserID: operatorUserID,
		TargetUserID:   targetUserID,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Payload:        payload,
	}

	payloadJSON, err := json.Marshal(event)
	if err != nil {
		logx.WithContext(ctx).Errorf("marshal content outbox event failed: eventType=%s err=%v", eventType, err)
		return
	}

	if err := svcCtx.OutboxStore.Create(ctx, repository.OutboxEvent{
		EventID:     event.EventID,
		EventType:   event.EventType,
		RoutingKey:  event.EventType,
		PayloadJSON: string(payloadJSON),
	}); err != nil {
		logx.WithContext(ctx).Errorf("create content outbox event failed: eventType=%s err=%v", eventType, err)
		return
	}
	logx.WithContext(ctx).Infof(
		"content outbox event created: eventId=%s eventType=%s resourceType=%s resourceId=%d operatorUserId=%d targetUserId=%d",
		event.EventID,
		event.EventType,
		event.ResourceType,
		event.ResourceID,
		event.OperatorUserID,
		event.TargetUserID,
	)
}

func newContentEventID() string {
	seq := contentEventSeq.Add(1)
	return fmt.Sprintf("cnt_evt_%d_%d", time.Now().UnixNano(), seq)
}
