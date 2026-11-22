package privacy

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) BlacklistStatus(ctx context.Context, userID, targetUserID int64) (BlacklistStatus, error) {
	return s.repo.BlacklistStatus(ctx, userID, targetUserID)
}

func (s *Service) ReplaceBlacklistScopes(ctx context.Context, userID, targetUserID int64, scopes []string) error {
	return s.repo.ReplaceBlacklistScopes(ctx, userID, targetUserID, scopes)
}

func (s *Service) IsScopedBlocked(ctx context.Context, leftUserID, rightUserID int64, scope string) (bool, error) {
	return s.repo.IsScopedBlocked(ctx, leftUserID, rightUserID, scope)
}

func (s *Service) UpsertPostSettings(ctx context.Context, postID int64, allowComment bool, hiddenUserIDs []int64) error {
	return s.repo.UpsertPostSettings(ctx, postID, allowComment, hiddenUserIDs)
}

func (s *Service) PostSettings(ctx context.Context, postID int64) (PostSettings, error) {
	return s.repo.PostSettings(ctx, postID)
}

func (s *Service) VisiblePostAuthorID(ctx context.Context, postID int64) (int64, bool, error) {
	return s.repo.VisiblePostAuthorID(ctx, postID)
}

func (s *Service) CanViewerAccessPost(ctx context.Context, viewerUserID, authorUserID, postID int64) (bool, error) {
	if viewerUserID <= 0 || authorUserID <= 0 || postID <= 0 {
		return false, nil
	}
	blocked, err := s.repo.IsScopedBlocked(ctx, viewerUserID, authorUserID, ScopePost)
	if err != nil || blocked {
		return !blocked, err
	}
	hidden, err := s.repo.IsViewerHiddenForPost(ctx, postID, viewerUserID)
	if err != nil || hidden {
		return !hidden, err
	}
	return true, nil
}
