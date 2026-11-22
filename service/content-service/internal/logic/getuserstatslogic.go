package logic

import (
	"context"

	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type GetUserStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserStatsLogic {
	return &GetUserStatsLogic{ctx: ctx, svcCtx: svcCtx}
}

// GetUserStats 返回内容域下的用户统计，例如发帖数和累计获赞数。
func (l *GetUserStatsLogic) GetUserStats(in *contentpb.GetUserStatsRequest) (*contentpb.GetUserStatsResponse, error) {
	// 这里只统计目标用户自身的数据，所以只校验 target_user_id 即可。
	if in.GetTargetUserId() <= 0 {
		return &contentpb.GetUserStatsResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid target_user_id"),
		}, nil
	}

	// 从帖子存储聚合统计，当前先返回资料页需要的发帖数和获赞数。
	stats, err := l.svcCtx.PostStore.GetUserStats(l.ctx, in.GetTargetUserId())
	if err != nil {
		return nil, err
	}

	return &contentpb.GetUserStatsResponse{
		Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Stats: &contentpb.UserContentStats{
			UserId:            stats.UserID,
			PostCount:         stats.PostCount,
			ReceivedLikeCount: stats.ReceivedLikeCount,
		},
	}, nil
}
