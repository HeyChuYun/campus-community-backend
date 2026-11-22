package moderation

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListPosts(ctx context.Context, filter PostListFilter) (PostListResult, error) {
	return s.repo.ListPosts(ctx, filter)
}

func (s *Service) ListUserViolations(ctx context.Context, filter UserViolationFilter) (UserViolationResult, error) {
	return s.repo.ListUserViolations(ctx, filter)
}

func (s *Service) RestorePostVisibility(ctx context.Context, postID int64) (bool, error) {
	return s.repo.RestorePostVisibility(ctx, postID)
}

func (s *Service) ViolationCategories() []ViolationCategoryOption {
	return []ViolationCategoryOption{
		{Code: "色情", Label: "色情"},
		{Code: "政治敏感", Label: "政治敏感"},
		{Code: "暴恐违禁", Label: "暴恐违禁"},
		{Code: "广告引流", Label: "广告引流"},
		{Code: "辱骂不良", Label: "辱骂不良"},
		{Code: "其他", Label: "其他"},
	}
}
