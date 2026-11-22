package logic

import (
	"context"

	"hfjj/backend/relation-service/internal/svc"
	"hfjj/backend/proto/relationpb"
)

type BatchGetRelationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewBatchGetRelationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchGetRelationLogic {
	return &BatchGetRelationLogic{ctx: ctx, svcCtx: svcCtx}
}

// BatchGetRelation 批量返回当前用户和多个目标用户之间的关系快照。
func (l *BatchGetRelationLogic) BatchGetRelation(in *relationpb.BatchGetRelationRequest) (*relationpb.BatchGetRelationResponse, error) {
	if in.GetActorUserId() <= 0 {
		return &relationpb.BatchGetRelationResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid actor_user_id")}, nil
	}
	snapshots, err := l.svcCtx.RelationStore.BatchGetRelation(l.ctx, in.GetActorUserId(), in.GetTargetUserIds())
	if err != nil {
		return nil, err
	}
	items := make([]*relationpb.RelationSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items = append(items, toSnapshotPB(snapshot))
	}
	return &relationpb.BatchGetRelationResponse{
		Base:      newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshots: items,
	}, nil
}
