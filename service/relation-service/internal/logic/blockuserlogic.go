package logic

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/svc"
)

type BlockUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBlockUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BlockUserLogic {
	return &BlockUserLogic{ctx: ctx, svcCtx: svcCtx}
}

// BlockUser 拉黑目标用户，并在成功后补写一条 outbox 事件。
func (l *BlockUserLogic) BlockUser(in *relationpb.BlockUserRequest) (*relationpb.BlockUserResponse, error) {
	if in.GetActorUserId() <= 0 || in.GetTargetUserId() <= 0 || in.GetActorUserId() == in.GetTargetUserId() {
		return &relationpb.BlockUserResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid block request")}, nil
	}

	snapshot, actorCounter, err := l.svcCtx.RelationStore.Block(l.ctx, in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		return nil, err
	}

	// 拉黑成功后写入 outbox，供私信资格、关注资格等异步模块同步状态。
	emitRelationEvent(l.ctx, l.svcCtx, "relation.user.blocked", in.GetActorUserId(), in.GetTargetUserId(), map[string]any{
		"snapshot":        toSnapshotPB(snapshot),
		"actor_counter":   toCounterPB(actorCounter),
		"user_id":         in.GetActorUserId(),
		"blocked_user_id": in.GetTargetUserId(),
	})

	return &relationpb.BlockUserResponse{
		Base:         newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshot:     toSnapshotPB(snapshot),
		ActorCounter: toCounterPB(actorCounter),
	}, nil
}
