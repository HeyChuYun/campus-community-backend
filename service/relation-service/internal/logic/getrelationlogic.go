package logic

import (
	"context"

	"hfjj/backend/relation-service/internal/svc"
	"hfjj/backend/proto/relationpb"
)

type GetRelationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRelationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRelationLogic {
	return &GetRelationLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetRelation 返回两个人之间的关系快照。
func (l *GetRelationLogic) GetRelation(in *relationpb.GetRelationRequest) (*relationpb.GetRelationResponse, error) {
	if in.GetActorUserId() <= 0 || in.GetTargetUserId() <= 0 {
		return &relationpb.GetRelationResponse{Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid relation request")}, nil
	}
	snapshot, err := l.svcCtx.RelationStore.GetRelation(l.ctx, in.GetActorUserId(), in.GetTargetUserId())
	if err != nil {
		return nil, err
	}
	return &relationpb.GetRelationResponse{
		Base:     newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Snapshot: toSnapshotPB(snapshot),
	}, nil
}
