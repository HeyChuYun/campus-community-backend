package logic

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/svc"
)

type FollowLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFollowLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FollowLogic {
	return &FollowLogic{ctx: ctx, svcCtx: svcCtx}
}

// Follow 创建关注关系，并在成功后补写一条 outbox 事件。
func (l *FollowLogic) Follow(in *relationpb.FollowRequest) (*relationpb.FollowResponse, error) {
	if in.GetActorUserId() <= 0 || in.GetTargetUserId() <= 0 || in.GetActorUserId() == in.GetTargetUserId() {
		return &relationpb.FollowResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid follow request")}, nil
	}

	snapshot, actorCounter, targetCounter, err := l.svcCtx.RelationStore.Follow(l.ctx, in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		return nil, err
	}

	// 关注成功后写入 outbox，供 IM 互动消息、推荐图谱等异步模块消费。
	emitRelationEvent(l.ctx, l.svcCtx, "relation.user.followed", in.GetActorUserId(), in.GetTargetUserId(), map[string]any{
		"snapshot":         toSnapshotPB(snapshot),
		"actor_counter":    toCounterPB(actorCounter),
		"target_counter":   toCounterPB(targetCounter),
		"follower_user_id": in.GetActorUserId(),
		"followee_user_id": in.GetTargetUserId(),
	})

	return &relationpb.FollowResponse{
		Base:          newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshot:      toSnapshotPB(snapshot),
		ActorCounter:  toCounterPB(actorCounter),
		TargetCounter: toCounterPB(targetCounter),
	}, nil
}
