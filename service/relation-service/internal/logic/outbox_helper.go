package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"hfjj/backend/relation-service/internal/repository"
	"hfjj/backend/relation-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

var relationEventSeq atomic.Int64

type relationDomainEvent struct {
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

func emitRelationEvent(ctx context.Context, svcCtx *svc.ServiceContext, eventType string, operatorUserID, targetUserID int64, payload any) {
	// 当前先采用“业务成功后补写 outbox”的方式，后续如需绝对事务一致性，再下沉为仓储层统一事务。
	event := relationDomainEvent{
		EventID:        newRelationEventID(),
		EventType:      eventType,
		OccurredAt:     time.Now().Format(time.RFC3339),
		Producer:       "relation-service",
		TraceID:        "",
		OperatorUserID: operatorUserID,
		TargetUserID:   targetUserID,
		ResourceType:   "user_relation",
		ResourceID:     targetUserID,
		Payload:        payload,
	}

	payloadJSON, err := json.Marshal(event)
	if err != nil {
		logx.WithContext(ctx).Errorf("marshal relation outbox event failed: eventType=%s err=%v", eventType, err)
		return
	}

	if err := svcCtx.OutboxStore.Create(ctx, repository.OutboxEvent{
		EventID:     event.EventID,
		EventType:   event.EventType,
		RoutingKey:  event.EventType,
		PayloadJSON: string(payloadJSON),
	}); err != nil {
		logx.WithContext(ctx).Errorf("create relation outbox event failed: eventType=%s err=%v", eventType, err)
	}
}

func newRelationEventID() string {
	seq := relationEventSeq.Add(1)
	return fmt.Sprintf("rel_evt_%d_%d", time.Now().UnixNano(), seq)
}
