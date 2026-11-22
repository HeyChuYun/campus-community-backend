package logic

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/svc"
)

type UnfollowLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnfollowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnfollowLogic {
	return &UnfollowLogic{ctx: ctx, svcCtx: svcCtx}
}

// Unfollow 取消关注关系，并在成功后补写一条 outbox 事件。
func (l *UnfollowLogic) Unfollow(in *relationpb.UnfollowRequest) (*relationpb.UnfollowResponse, error) {
	if in.GetActorUserId() <= 0 || in.GetTargetUserId() <= 0 || in.GetActorUserId() == in.GetTargetUserId() {
		return &relationpb.UnfollowResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid unfollow request")}, nil
	}

	snapshot, actorCounter, targetCounter, err := l.svcCtx.RelationStore.Unfollow(l.ctx, in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		return nil, err
	}

	// 取关成功后写入 outbox，供异步消费者更新关系图与互动资格。
	emitRelationEvent(l.ctx, l.svcCtx, "relation.user.unfollowed", in.GetActorUserId(), in.GetTargetUserId(), map[string]any{
		"snapshot":         toSnapshotPB(snapshot),
		"actor_counter":    toCounterPB(actorCounter),
		"target_counter":   toCounterPB(targetCounter),
		"follower_user_id": in.GetActorUserId(),
		"followee_user_id": in.GetTargetUserId(),
	})

	return &relationpb.UnfollowResponse{
		Base:          newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshot:      toSnapshotPB(snapshot),
		ActorCounter:  toCounterPB(actorCounter),
		TargetCounter: toCounterPB(targetCounter),
	}, nil
}
