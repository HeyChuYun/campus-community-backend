package server

import (
	"context"

	"hfjj/backend/proto/relationpb"
	"hfjj/backend/relation-service/internal/logic"
	"hfjj/backend/relation-service/internal/svc"
)

type RelationServiceServer struct {
	ctx *svc.ServiceContext
	relationpb.UnimplementedRelationServiceServer
}

func NewRelationServiceServer(ctx *svc.ServiceContext) *RelationServiceServer {
	return &RelationServiceServer{ctx: ctx}
}

// Follow 负责创建一条关注关系。
func (s *RelationServiceServer) Follow(ctx context.Context, in *relationpb.FollowRequest) (*relationpb.FollowResponse, error) {
	l := logic.NewFollowLogic(ctx, s.ctx)
	return l.Follow(in)
}

// Unfollow 负责取消一条关注关系。
func (s *RelationServiceServer) Unfollow(ctx context.Context, in *relationpb.UnfollowRequest) (*relationpb.UnfollowResponse, error) {
	l := logic.NewUnfollowLogic(ctx, s.ctx)
	return l.Unfollow(in)
}

// BlockUser 负责拉黑目标用户。
func (s *RelationServiceServer) BlockUser(ctx context.Context, in *relationpb.BlockUserRequest) (*relationpb.BlockUserResponse, error) {
	l := logic.NewBlockUserLogic(ctx, s.ctx)
	return l.BlockUser(in)
}

// UnblockUser 负责取消拉黑目标用户。
func (s *RelationServiceServer) UnblockUser(ctx context.Context, in *relationpb.UnblockUserRequest) (*relationpb.UnblockUserResponse, error) {
	l := logic.NewUnblockUserLogic(ctx, s.ctx)
	return l.UnblockUser(in)
}

// GetRelation 返回两个人之间的关系快照。
func (s *RelationServiceServer) GetRelation(ctx context.Context, in *relationpb.GetRelationRequest) (*relationpb.GetRelationResponse, error) {
	l := logic.NewGetRelationLogic(ctx, s.ctx)
	return l.GetRelation(in)
}

// BatchGetRelation 批量返回关系快照，后续供内容流和私信规则查询。
func (s *RelationServiceServer) BatchGetRelation(ctx context.Context, in *relationpb.BatchGetRelationRequest) (*relationpb.BatchGetRelationResponse, error) {
	l := logic.NewBatchGetRelationLogic(ctx, s.ctx)
	return l.BatchGetRelation(in)
}

// ListFollowing 返回当前用户关注列表占位结果。
func (s *RelationServiceServer) ListFollowing(ctx context.Context, in *relationpb.ListFollowingRequest) (*relationpb.ListFollowingResponse, error) {
	l := logic.NewListFollowingLogic(ctx, s.ctx)
	return l.ListFollowing(in)
}

// ListFollowers 返回当前用户粉丝列表占位结果。
func (s *RelationServiceServer) ListFollowers(ctx context.Context, in *relationpb.ListFollowersRequest) (*relationpb.ListFollowersResponse, error) {
	l := logic.NewListFollowersLogic(ctx, s.ctx)
	return l.ListFollowers(in)
}

// GetUserCounter 返回指定用户的关系计数快照。
func (s *RelationServiceServer) GetUserCounter(ctx context.Context, in *relationpb.GetUserCounterRequest) (*relationpb.GetUserCounterResponse, error) {
	l := logic.NewGetUserCounterLogic(ctx, s.ctx)
	return l.GetUserCounter(in)
}
