package logic

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/svc"
)

type GetUserCounterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserCounterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserCounterLogic {
	return &GetUserCounterLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetUserCounter 返回指定用户当前的关注数、粉丝数、好友数和拉黑数。
func (l *GetUserCounterLogic) GetUserCounter(in *relationpb.GetUserCounterRequest) (*relationpb.GetUserCounterResponse, error) {
	// 先校验 user_id，避免无意义查询直接落到存储层。
	if in.GetUserId() <= 0 {
		return &relationpb.GetUserCounterResponse{
			Base: newStatus(relationpb.RelationCode_RELATION_CODE_INVALID_ARGUMENT, "invalid user_id"),
		}, nil
	}

	// 从关系存储中读取聚合计数，数据库模式和内存模式都复用同一套接口。
	counter, err := l.svcCtx.RelationStore.GetCounter(l.ctx, in.GetUserId())
	if err != nil {
		return nil, err
	}

	return &relationpb.GetUserCounterResponse{
		Base:    newStatus(relationpb.RelationCode_RELATION_CODE_OK, "ok"),
		Counter: toCounterPB(counter),
	}, nil
}
