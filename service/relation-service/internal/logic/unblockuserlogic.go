package logic

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/svc"
)

type UnblockUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnblockUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnblockUserLogic {
	return &UnblockUserLogic{ctx: ctx, svcCtx: svcCtx}
}

// UnblockUser 取消拉黑目标用户，并在成功后补写一条 outbox 事件。
func (l *UnblockUserLogic) UnblockUser(in *relationpb.UnblockUserRequest) (*relationpb.UnblockUserResponse, error) {
	if in.GetActorUserId() <= 0 || in.GetTargetUserId() <= 0 || in.GetActorUserId() == in.GetTargetUserId() {
		return &relationpb.UnblockUserResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid unblock request")}, nil
	}

	snapshot, actorCounter, err := l.svcCtx.RelationStore.Unblock(l.ctx, in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		return nil, err
	}

	// 取消拉黑成功后写入 outbox，供异步消费者恢复候选关系状态。
	emitRelationEvent(l.ctx, l.svcCtx, "relation.user.unblocked", in.GetActorUserId(), in.GetTargetUserId(), map[string]any{
		"snapshot":        toSnapshotPB(snapshot),
		"actor_counter":   toCounterPB(actorCounter),
		"user_id":         in.GetActorUserId(),
		"blocked_user_id": in.GetTargetUserId(),
	})

	return &relationpb.UnblockUserResponse{
		Base:         newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshot:     toSnapshotPB(snapshot),
		ActorCounter: toCounterPB(actorCounter),
	}, nil
}
