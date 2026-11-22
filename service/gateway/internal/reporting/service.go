package reporting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Service struct {
	repo   Repository
	policy StatusPolicy
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, policy: DefaultStatusPolicy()}
}

func (s *Service) Settings(ctx context.Context) (Settings, error) {
	return s.repo.Settings(ctx)
}

func (s *Service) Categories() []Category {
	items := []Category{
		{Code: "fraud", Label: "诈骗广告", Description: "用于诈骗、售假、博彩、刷单或异常营销。"},
		{Code: "harassment", Label: "骚扰谩骂", Description: "用于持续骚扰、辱骂、威胁或恶意挑衅。"},
		{Code: "illegal_redirect", Label: "违规引流", Description: "用于诱导加群、导流到外部平台或售卖资源。"},
		{Code: "impersonation", Label: "冒充他人", Description: "用于冒充同学、老师、组织或其他身份的账号。"},
		{Code: "infringement", Label: "侵权盗用", Description: "用于盗用头像、资料、身份或其他侵权行为。"},
		{Code: "other", Label: "其他违规", Description: "其他不符合平台规则的异常行为。"},
		{Code: "pornography", Label: "色情低俗", Description: "用于发布或传播色情、低俗、擦边内容。"},
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Code < items[j].Code
	})
	return items
}

func (s *Service) IsAllowedCategory(categoryCode string) bool {
	for _, item := range s.Categories() {
		if item.Code == categoryCode {
			return true
		}
	}
	return false
}

func (s *Service) PostAppealContext(ctx context.Context, requesterUserID, postID int64) (PostAppealContext, error) {
	result, err := s.repo.PostAppealContext(ctx, requesterUserID, postID)
	if err != nil {
		return PostAppealContext{}, err
	}

	latestStatus, latestCreatedAt, exists, err := s.repo.LatestReporterPostStatus(ctx, requesterUserID, postID)
	if err != nil {
		return PostAppealContext{}, err
	}
	result.HasAppeal = exists
	result.CanSubmitAppeal = true
	if exists {
		result.LatestAppealStatus = strings.TrimSpace(latestStatus)
		result.LatestAppealAt = &latestCreatedAt
		if s.policy.BlocksDuplicate(latestStatus) {
			result.CanSubmitAppeal = false
			result.BlockSubmitReason = "该帖子已有待处理申诉，请等待审核结果后再提交。"
		}
	}
	return result, nil
}

func (s *Service) TargetUserSnapshot(ctx context.Context, userID int64) (map[string]any, error) {
	return s.repo.TargetUserSnapshot(ctx, userID)
}

func (s *Service) CanSubmitUserReport(ctx context.Context, reporterUserID, targetUserID int64, settings Settings) (string, error) {
	latestStatus, latestCreatedAt, exists, err := s.repo.LatestReporterTargetStatus(ctx, reporterUserID, targetUserID)
	if err != nil {
		return "", err
	}
	if exists {
		if s.policy.BlocksDuplicate(latestStatus) {
			return "该用户已有待处理举报，请等待管理员处理后再提交。", nil
		}
		if latestCreatedAt.After(time.Now().AddDate(0, 0, -settings.SameTargetCooldown)) &&
			!s.policy.SkipsCooldown(latestStatus) {
			return fmt.Sprintf("同一用户 %d 天内只能举报一次。", settings.SameTargetCooldown), nil
		}
	}

	distinctTargets, err := s.repo.TodayDistinctTargetCount(ctx, reporterUserID)
	if err != nil {
		return "", err
	}
	reportedToday, err := s.repo.ReportedTargetToday(ctx, reporterUserID, targetUserID)
	if err != nil {
		return "", err
	}
	if !reportedToday && distinctTargets >= settings.DailyTargetLimit {
		return fmt.Sprintf("一天内最多举报 %d 个不同用户。", settings.DailyTargetLimit), nil
	}
	return "", nil
}

func (s *Service) CreateUserReport(ctx context.Context, input CreateUserReportInput) (CreateUserReportResult, error) {
	reportNo, err := generateReportNo()
	if err != nil {
		return CreateUserReportResult{}, err
	}
	snapshotRaw, err := json.Marshal(input.TargetSnapshot)
	if err != nil {
		return CreateUserReportResult{}, err
	}
	result, err := s.repo.CreateUserReport(ctx, input, reportNo, snapshotRaw)
	if err != nil {
		return CreateUserReportResult{}, err
	}
	return result, nil
}

func (s *Service) CanSubmitPostAppeal(ctx context.Context, reporterUserID, postID int64) (string, error) {
	latestStatus, _, exists, err := s.repo.LatestReporterPostStatus(ctx, reporterUserID, postID)
	if err != nil {
		return "", err
	}
	if exists && s.policy.BlocksDuplicate(latestStatus) {
		return "该帖子已有待处理申诉，请等待审核结果后再提交。", nil
	}
	return "", nil
}

func (s *Service) CreatePostAppeal(ctx context.Context, input CreatePostAppealInput) (CreatePostAppealResult, error) {
	reportNo, err := generateReportNo()
	if err != nil {
		return CreatePostAppealResult{}, err
	}
	snapshotRaw, err := json.Marshal(input.TargetSnapshot)
	if err != nil {
		return CreatePostAppealResult{}, err
	}
	result, err := s.repo.CreatePostAppeal(ctx, input, reportNo, snapshotRaw)
	if err != nil {
		return CreatePostAppealResult{}, err
	}
	return result, nil
}

func (s *Service) AdminList(ctx context.Context, filter AdminListFilter) (AdminListResult, error) {
	return s.repo.AdminList(ctx, filter)
}

func (s *Service) AdminDetail(ctx context.Context, reportID int64) (AdminReportDetail, bool, error) {
	return s.repo.AdminDetail(ctx, reportID)
}

func (s *Service) UpdateAdminStatus(ctx context.Context, input AdminStatusUpdate) error {
	input.Status = strings.TrimSpace(input.Status)
	input.DecisionCode = strings.TrimSpace(input.DecisionCode)
	input.DecisionNote = strings.TrimSpace(input.DecisionNote)
	return s.repo.UpdateAdminStatus(ctx, input)
}

func generateReportNo() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"R%s%s",
		time.Now().Format("20060102150405"),
		strings.ToUpper(hex.EncodeToString(randomBytes)),
	), nil
}
