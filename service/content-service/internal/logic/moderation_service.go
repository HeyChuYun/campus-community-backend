package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"hfjj/backend/content-service/internal/config"
	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"

	"github.com/Baidu-AIP/golang-sdk/aip/censor"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	moderationStatusPassed       = "passed"
	moderationStatusRejected     = "rejected"
	moderationStatusWaitingAsync = "waiting_async"
	moderationStatusManualReview = "manual_review"
)

const (
	baiduQueryTokenURL = "https://aip.baidubce.com/oauth/2.0/token"
	baiduVideoQueryURL = "https://aip.baidubce.com/rest/2.0/solution/v1/video_censor/v2/query"
)

var moderationSeq atomic.Int64

type moderationAttemptResult struct {
	Status          string
	ProviderTaskID  string
	SyncPayload     string
	AsyncPayload    string
	FinalPayload    string
	Conclusion      string
	Categories      []string
	Reason          string
	EvidencePayload string
	LastCheckedAt   time.Time
}

type moderationDecision struct {
	Status         string
	ProviderTaskID string
	Conclusion     string
	Categories     []string
	Reason         string
}

type baiduModerator struct {
	client       *censor.ContentCensorClient
	httpClient   *http.Client
	accessKey    string
	secretKey    string
	mediaBaseURL string
	categorySet  categoryMatcher
}

type categoryMatcher struct {
	rules []categoryRule
}

type categoryRule struct {
	category string
	keywords []string
}

func ProcessPostModeration(ctx context.Context, svcCtx *svc.ServiceContext, postID int64) error {
	if svcCtx == nil || postID <= 0 {
		return nil
	}
	if !svcCtx.Config.Moderation.Enabled {
		return enqueueManualReview(ctx, svcCtx, postID, "moderation_disabled", "内容审核服务未开启")
	}

	post, found, err := svcCtx.PostStore.GetPost(ctx, postID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	task, err := svcCtx.ModerationStore.GetOrCreateTask(ctx, postID, strings.TrimSpace(svcCtx.Config.Moderation.Provider))
	if err != nil {
		return err
	}
	if task.Status == moderationStatusPassed || task.Status == moderationStatusRejected {
		return nil
	}

	attempts := task.Attempts + 1
	moderator := newBaiduModerator(svcCtx.Config.Moderation)

	var result moderationAttemptResult
	if strings.TrimSpace(task.ProviderTaskID) != "" && task.Status == moderationStatusWaitingAsync {
		result, err = moderator.QueryVideoResult(ctx, task.ProviderTaskID)
	} else {
		result, err = moderator.ReviewPost(ctx, post)
	}
	if err != nil {
		return updateTaskAndEnqueueManualReview(
			ctx,
			svcCtx,
			postID,
			attempts,
			moderationAttemptResult{Reason: err.Error()},
			"provider_failed",
		)
	}

	switch result.Status {
	case moderationStatusPassed:
		now := result.LastCheckedAt
		if now.IsZero() {
			now = time.Now()
		}
		reviewedAt := now
		if err := svcCtx.PostStore.UpdatePostStatus(ctx, postID, int32(contentpb.PostStatus_POST_STATUS_NORMAL)); err != nil {
			return err
		}
		return svcCtx.ModerationStore.UpdateTask(ctx, repository.PostModerationTaskUpdate{
			PostID:             postID,
			Status:             moderationStatusPassed,
			ProviderTaskID:     result.ProviderTaskID,
			SyncResultPayload:  result.SyncPayload,
			AsyncResultPayload: result.AsyncPayload,
			FinalResultPayload: nonEmptyJSON(result.FinalPayload, combineModerationPayload(result.SyncPayload, result.AsyncPayload)),
			Conclusion:         nonEmptyString(result.Conclusion, "pass"),
			CategoriesJSON:     marshalStringSlice(result.Categories),
			Reason:             result.Reason,
			Attempts:           attempts,
			LastCheckedAt:      &now,
			ReviewedAt:         &reviewedAt,
		})
	case moderationStatusWaitingAsync:
		now := result.LastCheckedAt
		if now.IsZero() {
			now = time.Now()
		}
		nextRetry := now.Add(svcCtx.Config.Moderation.RetryInterval)
		return svcCtx.ModerationStore.UpdateTask(ctx, repository.PostModerationTaskUpdate{
			PostID:             postID,
			Status:             moderationStatusWaitingAsync,
			ProviderTaskID:     result.ProviderTaskID,
			SyncResultPayload:  nonEmptyJSON(result.SyncPayload, "{}"),
			AsyncResultPayload: nonEmptyJSON(result.AsyncPayload, "{}"),
			FinalResultPayload: nonEmptyJSON(result.FinalPayload, combineModerationPayload(result.SyncPayload, result.AsyncPayload)),
			Conclusion:         nonEmptyString(result.Conclusion, "pending"),
			CategoriesJSON:     marshalStringSlice(result.Categories),
			Reason:             result.Reason,
			Attempts:           attempts,
			NextRetryAt:        &nextRetry,
			LastCheckedAt:      &now,
		})
	case moderationStatusRejected:
		return applyPostViolation(
			ctx,
			svcCtx,
			post,
			"baidu_auto",
			result.Categories,
			nonEmptyString(result.Reason, "content moderation rejected this post"),
			nonEmptyJSON(result.EvidencePayload, combineModerationPayload(result.SyncPayload, result.AsyncPayload)),
			0,
			repository.RestrictionScopePostPublish,
			svcCtx.Config.Moderation.DefaultMuteDuration,
		)
	case moderationStatusManualReview:
		return updateTaskAndEnqueueManualReview(ctx, svcCtx, postID, attempts, result, "baidu_manual_review")
	default:
		return updateTaskAndEnqueueManualReview(
			ctx,
			svcCtx,
			postID,
			attempts,
			moderationAttemptResult{Reason: fmt.Sprintf("unexpected moderation status: %s", result.Status)},
			"unexpected_status",
		)
	}
}

func PollPendingModerationTasks(ctx context.Context, svcCtx *svc.ServiceContext) error {
	if svcCtx == nil || !svcCtx.Config.Moderation.Enabled {
		return nil
	}
	tasks, err := svcCtx.ModerationStore.ListTasksForRetry(ctx, time.Now(), svcCtx.Config.Moderation.PollBatchSize)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := ProcessPostModeration(ctx, svcCtx, task.PostID); err != nil {
			logx.WithContext(ctx).Errorf("poll moderation task failed: postID=%d err=%v", task.PostID, err)
		}
	}
	return nil
}

func applyPostViolation(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	post repository.Post,
	source string,
	categories []string,
	reason string,
	evidencePayload string,
	operatorUserID int64,
	muteScope string,
	muteDuration time.Duration,
) error {
	if svcCtx == nil {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "content violates community policy"
	}
	if muteScope == "" {
		muteScope = repository.RestrictionScopePostPublish
	}

	if _, err := svcCtx.ModerationStore.GetOrCreateTask(ctx, post.ID, strings.TrimSpace(svcCtx.Config.Moderation.Provider)); err != nil {
		return err
	}

	record, err := svcCtx.ModerationStore.ApplyViolation(ctx, repository.ViolationDecision{
		ViolationNo:         newModerationID("vio"),
		PostID:              post.ID,
		UserID:              post.AuthorUserID,
		Source:              source,
		ActionType:          "block_post",
		CategoriesJSON:      marshalStringSlice(categories),
		Reason:              reason,
		EvidencePayloadJSON: nonEmptyJSON(evidencePayload, "{}"),
		MuteScope:           muteScope,
		MuteDurationSeconds: int64(muteDuration / time.Second),
		OperatorUserID:      operatorUserID,
	})
	if err != nil {
		return err
	}

	emitContentEvent(ctx, svcCtx, "content.post.blocked", operatorUserID, post.AuthorUserID, "post", post.ID, map[string]any{
		"post_id":               post.ID,
		"author_user_id":        post.AuthorUserID,
		"source":                source,
		"categories":            categories,
		"reason":                reason,
		"mute_scope":            muteScope,
		"mute_duration_seconds": int64(muteDuration / time.Second),
		"violation_no":          record.ViolationNo,
		"campus_code":           normalizeCampusCode(post.CampusCode),
	})
	return nil
}

func updateTaskAndEnqueueManualReview(
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	postID int64,
	attempts int,
	result moderationAttemptResult,
	source string,
) error {
	now := result.LastCheckedAt
	if now.IsZero() {
		now = time.Now()
	}
	reason := nonEmptyString(result.Reason, "需要人工审核")
	if err := svcCtx.ModerationStore.UpdateTask(ctx, repository.PostModerationTaskUpdate{
		PostID:             postID,
		Status:             moderationStatusManualReview,
		ProviderTaskID:     result.ProviderTaskID,
		SyncResultPayload:  nonEmptyJSON(result.SyncPayload, "{}"),
		AsyncResultPayload: nonEmptyJSON(result.AsyncPayload, "{}"),
		FinalResultPayload: nonEmptyJSON(result.FinalPayload, combineModerationPayload(result.SyncPayload, result.AsyncPayload)),
		Conclusion:         nonEmptyString(result.Conclusion, "manual_review"),
		CategoriesJSON:     marshalStringSlice(result.Categories),
		Reason:             reason,
		Attempts:           attempts,
		LastCheckedAt:      &now,
		ReviewedAt:         &now,
	}); err != nil {
		return err
	}
	return enqueueManualReview(ctx, svcCtx, postID, source, reason)
}

func enqueueManualReview(ctx context.Context, svcCtx *svc.ServiceContext, postID int64, source, reason string) error {
	if svcCtx == nil || svcCtx.ModerationStore == nil {
		return nil
	}
	return svcCtx.ModerationStore.EnqueueHumanReview(ctx, postID, source, reason)
}

func newBaiduModerator(cfg config.ModerationConfig) *baiduModerator {
	return &baiduModerator{
		client:       censor.NewClient(strings.TrimSpace(cfg.AccessKey), strings.TrimSpace(cfg.SecretKey)),
		httpClient:   &http.Client{Timeout: 20 * time.Second},
		accessKey:    strings.TrimSpace(cfg.AccessKey),
		secretKey:    strings.TrimSpace(cfg.SecretKey),
		mediaBaseURL: strings.TrimSpace(cfg.MediaBaseURL),
		categorySet:  buildCategoryMatcher(cfg.CategoryRules),
	}
}

func (m *baiduModerator) ReviewPost(ctx context.Context, post repository.Post) (moderationAttemptResult, error) {
	result := moderationAttemptResult{LastCheckedAt: time.Now()}
	syncPayload := map[string]any{
		"text":   map[string]any{},
		"images": []any{},
		"videos": []any{},
	}

	if text := strings.TrimSpace(post.ContentText); text != "" {
		raw := m.client.TextCensor(text)
		payload, decision, err := parseModerationResponse(raw, m.categorySet)
		if err != nil {
			return moderationAttemptResult{}, err
		}
		syncPayload["text"] = payload
		if decision.Status == moderationStatusRejected || decision.Status == moderationStatusManualReview {
			result.Status = decision.Status
			result.Categories = decision.Categories
			result.Reason = decision.Reason
			result.Conclusion = decision.Conclusion
			result.SyncPayload = marshalAny(syncPayload)
			result.FinalPayload = result.SyncPayload
			result.EvidencePayload = marshalAny(map[string]any{"text": payload})
			return result, nil
		}
	}

	imagePayloads := make([]any, 0)
	for _, media := range post.Media {
		if media.MediaType != 1 && media.MediaType != 3 {
			continue
		}
		mediaURL := m.resolveMediaURL(media)
		if mediaURL == "" {
			continue
		}
		raw := m.client.ImgCensorUrl(mediaURL, map[string]interface{}{})
		payload, decision, err := parseModerationResponse(raw, m.categorySet)
		if err != nil {
			return moderationAttemptResult{}, err
		}
		imagePayloads = append(imagePayloads, map[string]any{
			"media_id": media.ID,
			"url":      mediaURL,
			"result":   payload,
		})
		if decision.Status == moderationStatusRejected || decision.Status == moderationStatusManualReview {
			result.Status = decision.Status
			result.Categories = decision.Categories
			result.Reason = decision.Reason
			result.Conclusion = decision.Conclusion
			syncPayload["images"] = imagePayloads
			result.SyncPayload = marshalAny(syncPayload)
			result.FinalPayload = result.SyncPayload
			result.EvidencePayload = marshalAny(map[string]any{"image": payload, "media_id": media.ID})
			return result, nil
		}
	}
	syncPayload["images"] = imagePayloads

	videoPayloads := make([]any, 0)
	for _, media := range post.Media {
		if media.MediaType != 2 {
			continue
		}
		mediaURL := m.resolveMediaURL(media)
		if mediaURL == "" {
			continue
		}
		name := strings.TrimSpace(media.StorageKey)
		if name == "" {
			name = fmt.Sprintf("post_%d_video_%d", post.ID, media.ID)
		}
		raw := m.client.VideoCensor(path.Base(name), mediaURL, fmt.Sprintf("post_%d_media_%d", post.ID, media.ID), map[string]interface{}{})
		payload, decision, err := parseModerationResponse(raw, m.categorySet)
		if err != nil {
			return moderationAttemptResult{}, err
		}
		videoPayloads = append(videoPayloads, map[string]any{
			"media_id": media.ID,
			"url":      mediaURL,
			"result":   payload,
		})
		if decision.Status == moderationStatusRejected || decision.Status == moderationStatusManualReview {
			result.Status = decision.Status
			result.Categories = decision.Categories
			result.Reason = decision.Reason
			result.Conclusion = decision.Conclusion
			syncPayload["videos"] = videoPayloads
			result.SyncPayload = marshalAny(syncPayload)
			result.FinalPayload = result.SyncPayload
			result.EvidencePayload = marshalAny(map[string]any{"video": payload, "media_id": media.ID})
			return result, nil
		}
		if decision.Status == moderationStatusWaitingAsync {
			result.Status = moderationStatusWaitingAsync
			result.ProviderTaskID = decision.ProviderTaskID
			result.Categories = decision.Categories
			result.Reason = decision.Reason
			result.Conclusion = nonEmptyString(decision.Conclusion, "pending")
			syncPayload["videos"] = videoPayloads
			result.SyncPayload = marshalAny(syncPayload)
			result.FinalPayload = result.SyncPayload
			return result, nil
		}
	}
	syncPayload["videos"] = videoPayloads

	result.Status = moderationStatusPassed
	result.Conclusion = "pass"
	result.SyncPayload = marshalAny(syncPayload)
	result.FinalPayload = result.SyncPayload
	return result, nil
}

func (m *baiduModerator) QueryVideoResult(ctx context.Context, providerTaskID string) (moderationAttemptResult, error) {
	values := url.Values{}
	values.Set("taskId", strings.TrimSpace(providerTaskID))
	raw, err := m.postBaiduForm(ctx, baiduVideoQueryURL, values)
	if err != nil {
		return moderationAttemptResult{}, err
	}
	payload, decision, err := parseModerationResponse(raw, m.categorySet)
	if err != nil {
		return moderationAttemptResult{}, err
	}
	return moderationAttemptResult{
		Status:          decision.Status,
		ProviderTaskID:  providerTaskID,
		AsyncPayload:    marshalAny(payload),
		FinalPayload:    marshalAny(payload),
		Conclusion:      decision.Conclusion,
		Categories:      decision.Categories,
		Reason:          decision.Reason,
		EvidencePayload: marshalAny(payload),
		LastCheckedAt:   time.Now(),
	}, nil
}

func (m *baiduModerator) resolveMediaURL(media repository.PostMedia) string {
	raw := strings.TrimSpace(media.URL)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	if m.mediaBaseURL == "" || strings.TrimSpace(media.StorageKey) == "" {
		return ""
	}
	return strings.TrimRight(m.mediaBaseURL, "/") + "/" + strings.TrimLeft(media.StorageKey, "/")
}

func (m *baiduModerator) postBaiduForm(ctx context.Context, endpoint string, values url.Values) (string, error) {
	token, err := m.fetchAccessToken(ctx)
	if err != nil {
		return "", err
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := u.Query()
	query.Set("access_token", token)
	query.Set("aipSdk", "golang")
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (m *baiduModerator) fetchAccessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", m.accessKey)
	form.Set("client_secret", m.secretKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baiduQueryTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	payload := map[string]any{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	token := strings.TrimSpace(stringValue(payload["access_token"]))
	if token == "" {
		return "", fmt.Errorf("baidu access token is empty")
	}
	return token, nil
}

func parseModerationResponse(raw string, matcher categoryMatcher) (map[string]any, moderationDecision, error) {
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, moderationDecision{}, fmt.Errorf("baidu moderation returned invalid json: %w", err)
	}
	if errorCode, ok := asInt(payload["error_code"]); ok && errorCode != 0 {
		return nil, moderationDecision{}, fmt.Errorf("baidu moderation error %d: %s", errorCode, stringValue(payload["error_msg"]))
	}

	decision := moderationDecision{
		ProviderTaskID: findTaskID(payload),
		Conclusion:     strings.TrimSpace(stringValue(payload["conclusion"])),
	}
	if conclusionType, ok := asInt(payload["conclusionType"]); ok {
		switch conclusionType {
		case 1:
			decision.Status = moderationStatusPassed
		case 2:
			decision.Status = moderationStatusManualReview
		case 3:
			decision.Status = moderationStatusRejected
		}
	}
	if decision.Status == "" {
		normalized := strings.ToLower(decision.Conclusion)
		switch {
		case strings.Contains(normalized, "pass"), strings.Contains(normalized, "approve"), strings.Contains(normalized, "合规"):
			decision.Status = moderationStatusPassed
		case strings.Contains(normalized, "疑似"), strings.Contains(normalized, "review"):
			decision.Status = moderationStatusManualReview
		case strings.Contains(normalized, "不合规"), strings.Contains(normalized, "违规"), strings.Contains(normalized, "block"), strings.Contains(normalized, "reject"):
			decision.Status = moderationStatusRejected
		}
	}
	if decision.Status == "" && decision.ProviderTaskID != "" {
		decision.Status = moderationStatusWaitingAsync
	}
	if decision.Status == "" {
		decision.Status = moderationStatusManualReview
	}

	decision.Categories = matcher.MatchStrings(extractPayloadTexts(payload))
	if len(decision.Categories) == 0 && decision.Status == moderationStatusRejected {
		decision.Categories = []string{"其他"}
	}
	decision.Reason = buildModerationReason(decision.Status, decision.Categories, decision.Conclusion)
	return payload, decision, nil
}

func buildCategoryMatcher(custom []config.ModerationCategoryRule) categoryMatcher {
	rules := make([]categoryRule, 0, len(custom)+5)
	for _, item := range custom {
		if strings.TrimSpace(item.Category) == "" || len(item.Keywords) == 0 {
			continue
		}
		rules = append(rules, categoryRule{
			category: strings.TrimSpace(item.Category),
			keywords: normalizeKeywords(item.Keywords),
		})
	}
	if len(rules) == 0 {
		rules = append(rules,
			categoryRule{category: "色情", keywords: normalizeKeywords([]string{"色情", "porn", "sex", "adult", "低俗"})},
			categoryRule{category: "政治敏感", keywords: normalizeKeywords([]string{"政治", "涉政", "敏感", "politic"})},
			categoryRule{category: "暴恐违禁", keywords: normalizeKeywords([]string{"暴恐", "违禁", "刀", "枪", "terror", "weapon"})},
			categoryRule{category: "广告引流", keywords: normalizeKeywords([]string{"广告", "引流", "推广", "spam"})},
			categoryRule{category: "辱骂不良", keywords: normalizeKeywords([]string{"辱骂", "恶俗", "低质", "abuse"})},
		)
	}
	return categoryMatcher{rules: rules}
}

func (m categoryMatcher) MatchStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		for _, rule := range m.rules {
			for _, keyword := range rule.keywords {
				if strings.Contains(normalized, keyword) {
					seen[rule.category] = struct{}{}
					break
				}
			}
		}
	}
	result := make([]string, 0, len(seen))
	for category := range seen {
		result = append(result, category)
	}
	return result
}

func normalizeKeywords(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func buildModerationReason(status string, categories []string, conclusion string) string {
	switch status {
	case moderationStatusRejected:
		if len(categories) == 0 {
			return nonEmptyString(conclusion, "内容审核判定违规")
		}
		return "命中违规分类：" + strings.Join(categories, "、")
	case moderationStatusWaitingAsync:
		return "视频内容仍在审核中"
	case moderationStatusManualReview:
		if strings.TrimSpace(conclusion) != "" {
			return conclusion
		}
		return "需要人工复核"
	default:
		return "内容审核通过"
	}
}

func extractPayloadTexts(value any) []string {
	result := make([]string, 0)
	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			for key, item := range typed {
				if strings.TrimSpace(key) != "" {
					result = append(result, key)
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	walk(value)
	return result
}

func findTaskID(payload map[string]any) string {
	if taskID := strings.TrimSpace(stringValue(payload["taskId"])); taskID != "" {
		return taskID
	}
	if taskID := strings.TrimSpace(stringValue(payload["task_id"])); taskID != "" {
		return taskID
	}
	if data, ok := payload["data"].(map[string]any); ok {
		if taskID := strings.TrimSpace(stringValue(data["taskId"])); taskID != "" {
			return taskID
		}
		if taskID := strings.TrimSpace(stringValue(data["task_id"])); taskID != "" {
			return taskID
		}
	}
	return ""
}

func asInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func marshalStringSlice(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	return marshalAny(values)
}

func marshalAny(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func combineModerationPayload(syncPayload, asyncPayload string) string {
	return marshalAny(map[string]any{
		"sync":  decodeJSONMap(syncPayload),
		"async": decodeJSONMap(asyncPayload),
	})
}

func decodeJSONMap(raw string) map[string]any {
	result := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}

func nonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nonEmptyJSON(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func newModerationID(prefix string) string {
	seq := moderationSeq.Add(1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), seq)
}
