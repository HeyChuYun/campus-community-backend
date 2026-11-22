package server

import (
	"context"

	"hfjj/backend/content-service/internal/logic"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type ContentServiceServer struct {
	ctx *svc.ServiceContext
	contentpb.UnimplementedContentServiceServer
}

func NewContentServiceServer(ctx *svc.ServiceContext) *ContentServiceServer {
	return &ContentServiceServer{ctx: ctx}
}

// CreatePost 创建帖子。
func (s *ContentServiceServer) CreatePost(ctx context.Context, in *contentpb.CreatePostRequest) (*contentpb.CreatePostResponse, error) {
	l := logic.NewCreatePostLogic(ctx, s.ctx)
	return l.CreatePost(in)
}

// DeletePost 删除帖子。
func (s *ContentServiceServer) DeletePost(ctx context.Context, in *contentpb.DeletePostRequest) (*contentpb.DeletePostResponse, error) {
	l := logic.NewDeletePostLogic(ctx, s.ctx)
	return l.DeletePost(in)
}

// GetPost 查询帖子详情。
func (s *ContentServiceServer) GetPost(ctx context.Context, in *contentpb.GetPostRequest) (*contentpb.GetPostResponse, error) {
	l := logic.NewGetPostLogic(ctx, s.ctx)
	return l.GetPost(in)
}

// GetFollowingFeed 获取关注流。
func (s *ContentServiceServer) GetFollowingFeed(ctx context.Context, in *contentpb.GetFollowingFeedRequest) (*contentpb.GetFollowingFeedResponse, error) {
	l := logic.NewGetFollowingFeedLogic(ctx, s.ctx)
	return l.GetFollowingFeed(in)
}

// GetDiscoverFeed 获取推荐流。
func (s *ContentServiceServer) GetDiscoverFeed(ctx context.Context, in *contentpb.GetDiscoverFeedRequest) (*contentpb.GetDiscoverFeedResponse, error) {
	l := logic.NewGetDiscoverFeedLogic(ctx, s.ctx)
	return l.GetDiscoverFeed(in)
}

// GetUserPosts 获取用户个人主页帖子列表。
func (s *ContentServiceServer) GetUserPosts(ctx context.Context, in *contentpb.GetUserPostsRequest) (*contentpb.GetUserPostsResponse, error) {
	l := logic.NewGetUserPostsLogic(ctx, s.ctx)
	return l.GetUserPosts(in)
}

// GetUserStats 获取用户内容统计。
func (s *ContentServiceServer) GetUserStats(ctx context.Context, in *contentpb.GetUserStatsRequest) (*contentpb.GetUserStatsResponse, error) {
	l := logic.NewGetUserStatsLogic(ctx, s.ctx)
	return l.GetUserStats(in)
}

// CreateComment 创建评论。
func (s *ContentServiceServer) CreateComment(ctx context.Context, in *contentpb.CreateCommentRequest) (*contentpb.CreateCommentResponse, error) {
	l := logic.NewCreateCommentLogic(ctx, s.ctx)
	return l.CreateComment(in)
}

// ListComments 查询评论列表。
func (s *ContentServiceServer) ListComments(ctx context.Context, in *contentpb.ListCommentsRequest) (*contentpb.ListCommentsResponse, error) {
	l := logic.NewListCommentsLogic(ctx, s.ctx)
	return l.ListComments(in)
}

// DeleteComment 删除评论。
func (s *ContentServiceServer) DeleteComment(ctx context.Context, in *contentpb.DeleteCommentRequest) (*contentpb.DeleteCommentResponse, error) {
	l := logic.NewDeleteCommentLogic(ctx, s.ctx)
	return l.DeleteComment(in)
}

// LikePost 点赞帖子。
func (s *ContentServiceServer) LikePost(ctx context.Context, in *contentpb.LikePostRequest) (*contentpb.LikePostResponse, error) {
	l := logic.NewLikePostLogic(ctx, s.ctx)
	return l.LikePost(in)
}

// UnlikePost 取消点赞帖子。
func (s *ContentServiceServer) UnlikePost(ctx context.Context, in *contentpb.UnlikePostRequest) (*contentpb.UnlikePostResponse, error) {
	l := logic.NewUnlikePostLogic(ctx, s.ctx)
	return l.UnlikePost(in)
}

// FavoritePost 收藏帖子。
func (s *ContentServiceServer) FavoritePost(ctx context.Context, in *contentpb.FavoritePostRequest) (*contentpb.FavoritePostResponse, error) {
	l := logic.NewFavoritePostLogic(ctx, s.ctx)
	return l.FavoritePost(in)
}

// UnfavoritePost 取消收藏帖子。
func (s *ContentServiceServer) UnfavoritePost(ctx context.Context, in *contentpb.UnfavoritePostRequest) (*contentpb.UnfavoritePostResponse, error) {
	l := logic.NewUnfavoritePostLogic(ctx, s.ctx)
	return l.UnfavoritePost(in)
}

// SearchPosts 搜索帖子。
func (s *ContentServiceServer) SearchPosts(ctx context.Context, in *contentpb.SearchPostsRequest) (*contentpb.SearchPostsResponse, error) {
	l := logic.NewSearchPostsLogic(ctx, s.ctx)
	return l.SearchPosts(in)
}

// SearchTopics 搜索话题。
func (s *ContentServiceServer) SearchTopics(ctx context.Context, in *contentpb.SearchTopicsRequest) (*contentpb.SearchTopicsResponse, error) {
	l := logic.NewSearchTopicsLogic(ctx, s.ctx)
	return l.SearchTopics(in)
}

// GetTopicPosts 按话题查询帖子列表。
func (s *ContentServiceServer) GetTopicPosts(ctx context.Context, in *contentpb.GetTopicPostsRequest) (*contentpb.GetTopicPostsResponse, error) {
	l := logic.NewGetTopicPostsLogic(ctx, s.ctx)
	return l.GetTopicPosts(in)
}

// ListHotTopics 获取热门话题。
func (s *ContentServiceServer) ListHotTopics(ctx context.Context, in *contentpb.ListHotTopicsRequest) (*contentpb.ListHotTopicsResponse, error) {
	l := logic.NewListHotTopicsLogic(ctx, s.ctx)
	return l.ListHotTopics(in)
}

// AdminBlockPost 管理端执行帖子违规封禁。
func (s *ContentServiceServer) AdminBlockPost(ctx context.Context, in *contentpb.AdminBlockPostRequest) (*contentpb.AdminBlockPostResponse, error) {
	l := logic.NewAdminBlockPostLogic(ctx, s.ctx)
	return l.AdminBlockPost(in)
}
