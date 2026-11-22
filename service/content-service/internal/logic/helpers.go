package logic

import (
	"context"
	"regexp"
	"strings"
	"time"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/pkg/constants"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/relationpb"
)

var topicPattern = regexp.MustCompile(`#([^#\n]{1,30})#`)

func toPostPB(post repository.Post) *contentpb.PostItem {
	media := make([]*contentpb.PostMedia, 0, len(post.Media))
	for _, item := range post.Media {
		media = append(media, &contentpb.PostMedia{
			MediaId:    item.ID,
			MediaType:  contentpb.PostMediaType(item.MediaType),
			StorageKey: item.StorageKey,
			Url:        item.URL,
			Width:      item.Width,
			Height:     item.Height,
			DurationMs: item.DurationMS,
			SizeBytes:  item.SizeBytes,
			SortOrder:  item.SortOrder,
		})
	}
	return &contentpb.PostItem{
		PostId:        post.ID,
		AuthorUserId:  post.AuthorUserID,
		ContentText:   post.ContentText,
		Status:        contentpb.PostStatus(post.Status),
		Visibility:    contentpb.PostVisibility(post.Visibility),
		LikeCount:     post.LikeCount,
		CommentCount:  post.CommentCount,
		FavoriteCount: post.FavoriteCount,
		Media:         media,
		CreatedAt:     formatTime(post.CreatedAt),
		UpdatedAt:     formatTime(post.UpdatedAt),
		Topics:        append([]string(nil), post.Topics...),
		CampusCode:    normalizeCampusCode(post.CampusCode),
	}
}

func toCommentPB(comment repository.Comment) *contentpb.CommentItem {
	return &contentpb.CommentItem{
		CommentId:        comment.ID,
		PostId:           comment.PostID,
		UserId:           comment.UserID,
		ReplyToCommentId: comment.ReplyToCommentID,
		ReplyToUserId:    comment.ReplyToUserID,
		Content:          comment.Content,
		CreatedAt:        formatTime(comment.CreatedAt),
	}
}

func normalizeVisibility(visibility contentpb.PostVisibility) contentpb.PostVisibility {
	if visibility == contentpb.PostVisibility_POST_VISIBILITY_FOLLOWERS_ONLY {
		return visibility
	}
	return contentpb.PostVisibility_POST_VISIBILITY_PUBLIC
}

func normalizeCampusCode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "all":
		return "all"
	case "xinqiao":
		return "xinqiao"
	case "gaoji":
		return "gaoji"
	default:
		return "all"
	}
}

func normalizePostCampusCode(value string) (string, bool) {
	switch normalizeCampusCode(value) {
	case "xinqiao":
		return "xinqiao", true
	case "gaoji":
		return "gaoji", true
	default:
		return "", false
	}
}

func toTopicPB(topic repository.Topic) *contentpb.TopicItem {
	return &contentpb.TopicItem{
		TopicId:         topic.ID,
		TopicName:       topic.TopicName,
		DisplayName:     topic.DisplayName,
		PostCount:       topic.PostCount,
		HotScore:        topic.HotScore,
		LastRefreshedAt: formatTime(topic.LastRefreshedAt),
	}
}

func toHotTopicPB(topic repository.HotTopic) *contentpb.TopicItem {
	return &contentpb.TopicItem{
		TopicId:         topic.TopicID,
		TopicName:       topic.TopicName,
		DisplayName:     topic.DisplayName,
		PostCount:       topic.PostCount,
		HotScore:        topic.HotScore,
		LastRefreshedAt: formatTime(topic.LastRefreshedAt),
	}
}

func normalizeContentType(media []*contentpb.PostMedia) contentpb.PostMediaType {
	if len(media) == 0 {
		return contentpb.PostMediaType(constants.ContentTypeText)
	}
	switch media[0].GetMediaType() {
	case contentpb.PostMediaType_POST_MEDIA_TYPE_IMAGE:
		return contentpb.PostMediaType(constants.ContentTypeImage)
	case contentpb.PostMediaType_POST_MEDIA_TYPE_VIDEO:
		return contentpb.PostMediaType(constants.ContentTypeVideo)
	case contentpb.PostMediaType_POST_MEDIA_TYPE_GIF:
		return contentpb.PostMediaType(constants.ContentTypeGIF)
	default:
		return contentpb.PostMediaType(constants.ContentTypeText)
	}
}

type contentMembershipLimits struct {
	PlanCode              string
	DailyPostBonus        int64
	ImageCountBonus       int64
	SingleImageBytesBonus int64
	VideoBytesBonus       int64
}

type configMediaLimits struct {
	MaxImageCount       int64
	MaxSingleImageBytes int64
	MaxVideoBytes       int64
	MaxGifBytes         int64
}

func validateDailyPostLimit(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) *contentpb.BaseResp {
	if svcCtx == nil || svcCtx.PostStore == nil || userID <= 0 {
		return nil
	}
	limit := svcCtx.Config.Content.Posts.DailyPostLimit + queryContentMembershipLimits(ctx, svcCtx, userID).DailyPostBonus
	if limit <= 0 {
		return nil
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	count, err := svcCtx.PostStore.CountPostsSince(ctx, userID, int32(contentpb.PostStatus_POST_STATUS_DELETED), todayStart)
	if err != nil {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_INTERNAL_ERROR, "检查发帖频率失败")
	}
	if count >= limit {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "今日发帖次数已达上限")
	}
	return nil
}

func validatePostMediaLimits(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, media []*contentpb.PostMedia) *contentpb.BaseResp {
	cfg := effectiveContentMediaConfig(ctx, svcCtx, userID)
	var imageCount int64
	var hasVideo bool
	var hasGIF bool

	for _, item := range media {
		switch item.GetMediaType() {
		case contentpb.PostMediaType_POST_MEDIA_TYPE_IMAGE:
			imageCount++
			if item.GetSizeBytes() > cfg.MaxSingleImageBytes {
				return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "single image too large")
			}
		case contentpb.PostMediaType_POST_MEDIA_TYPE_VIDEO:
			hasVideo = true
			if item.GetSizeBytes() > cfg.MaxVideoBytes {
				return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "video too large")
			}
		case contentpb.PostMediaType_POST_MEDIA_TYPE_GIF:
			hasGIF = true
			if item.GetSizeBytes() > cfg.MaxGifBytes {
				return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "gif too large")
			}
		default:
			return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "unsupported media type")
		}
	}

	if imageCount > cfg.MaxImageCount {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "too many images")
	}
	if imageCount > 0 && hasVideo {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "images and video cannot coexist")
	}
	if imageCount > 0 && hasGIF {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "images and gif cannot coexist")
	}
	if hasVideo && hasGIF {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "video and gif cannot coexist")
	}
	return nil
}

func effectiveContentMediaConfig(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) configMediaLimits {
	cfg := svcCtx.Config.Content.Media
	limits := queryContentMembershipLimits(ctx, svcCtx, userID)
	return configMediaLimits{
		MaxImageCount:       cfg.MaxImageCount + limits.ImageCountBonus,
		MaxSingleImageBytes: cfg.MaxSingleImageBytes + limits.SingleImageBytesBonus,
		MaxVideoBytes:       cfg.MaxVideoBytes + limits.VideoBytesBonus,
		MaxGifBytes:         cfg.MaxGifBytes,
	}
}

func queryContentMembershipLimits(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) contentMembershipLimits {
	if svcCtx == nil || svcCtx.MembershipStore == nil || userID <= 0 {
		return contentMembershipLimits{}
	}
	planCode, ok, err := svcCtx.MembershipStore.ActivePlan(ctx, userID)
	if err != nil || !ok {
		return contentMembershipLimits{}
	}
	policy, ok := contentMembershipPolicyMap(svcCtx)[strings.TrimSpace(planCode)]
	if !ok {
		return contentMembershipLimits{}
	}
	return policy
}

func contentMembershipPolicyMap(svcCtx *svc.ServiceContext) map[string]contentMembershipLimits {
	policies := map[string]contentMembershipLimits{
		"Premium": {
			PlanCode:              "Premium",
			DailyPostBonus:        3,
			ImageCountBonus:       3,
			SingleImageBytesBonus: 20 << 20,
			VideoBytesBonus:       40 << 20,
		},
		"PremiumPro": {
			PlanCode:              "PremiumPro",
			DailyPostBonus:        12,
			ImageCountBonus:       12,
			SingleImageBytesBonus: 60 << 20,
			VideoBytesBonus:       200 << 20,
		},
	}
	for _, item := range svcCtx.Config.Content.Membership.Policies {
		planCode := strings.TrimSpace(item.PlanCode)
		if planCode == "" {
			continue
		}
		policies[planCode] = contentMembershipLimits{
			PlanCode:              planCode,
			DailyPostBonus:        item.DailyPostBonus,
			ImageCountBonus:       item.ImageCountBonus,
			SingleImageBytesBonus: item.SingleImageBytesBonus,
			VideoBytesBonus:       item.VideoBytesBonus,
		}
	}
	return policies
}

func checkPostVisibility(ctx context.Context, svcCtx *svc.ServiceContext, operatorUserID int64, post repository.Post) *contentpb.BaseResp {
	if operatorUserID == post.AuthorUserID {
		return nil
	}
	if post.Status != int32(contentpb.PostStatus_POST_STATUS_NORMAL) {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_NOT_FOUND, "post is not visible")
	}
	if post.Visibility == constants.PostVisibilityPublic {
		return nil
	}
	if post.Visibility != constants.PostVisibilityFollowersOnly {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "post is not visible")
	}
	if svcCtx.RelationRPC == nil {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "relation rpc is unavailable")
	}
	resp, err := svcCtx.RelationRPC.GetRelation(ctx, &relationpb.GetRelationRequest{
		ActorUserId:  operatorUserID,
		TargetUserId: post.AuthorUserID,
	})
	if err != nil {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "relation check failed")
	}
	if resp.GetSnapshot() == nil || !resp.GetSnapshot().GetIsFollowing() {
		return newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, "post is followers only")
	}
	return nil
}

func filterVisiblePosts(ctx context.Context, svcCtx *svc.ServiceContext, operatorUserID int64, campusCode string, posts []repository.Post) []*contentpb.PostItem {
	if len(posts) == 0 {
		return []*contentpb.PostItem{}
	}
	campusCode = normalizeCampusCode(campusCode)
	enrichPostsWithTopics(ctx, svcCtx, posts)

	items := make([]*contentpb.PostItem, 0, len(posts))
	following := make(map[int64]bool)
	if svcCtx != nil && svcCtx.RelationRPC != nil {
		authorIDs := make([]int64, 0, len(posts))
		seen := make(map[int64]struct{}, len(posts))
		for _, post := range posts {
			if post.AuthorUserID == operatorUserID {
				continue
			}
			if _, ok := seen[post.AuthorUserID]; ok {
				continue
			}
			seen[post.AuthorUserID] = struct{}{}
			authorIDs = append(authorIDs, post.AuthorUserID)
		}
		if len(authorIDs) > 0 {
			resp, err := svcCtx.RelationRPC.BatchGetRelation(ctx, &relationpb.BatchGetRelationRequest{
				ActorUserId:   operatorUserID,
				TargetUserIds: authorIDs,
			})
			if err == nil {
				for _, item := range resp.GetSnapshots() {
					following[item.GetTargetUserId()] = item.GetIsFollowing()
				}
			}
		}
	}

	for _, post := range posts {
		if !matchesCampusFilter(campusCode, post.CampusCode) {
			continue
		}
		if post.AuthorUserID == operatorUserID {
			items = append(items, toPostPB(post))
			continue
		}
		if post.Status != int32(contentpb.PostStatus_POST_STATUS_NORMAL) {
			continue
		}
		if post.Visibility == constants.PostVisibilityPublic {
			items = append(items, toPostPB(post))
			continue
		}
		if post.Visibility == constants.PostVisibilityFollowersOnly && following[post.AuthorUserID] {
			items = append(items, toPostPB(post))
		}
	}
	return items
}

func matchesCampusFilter(selectedCampus, postCampus string) bool {
	selectedCampus = normalizeCampusCode(selectedCampus)
	postCampus = normalizeCampusCode(postCampus)
	if selectedCampus == "all" {
		return true
	}
	return postCampus == "all" || postCampus == selectedCampus
}

func buildFollowingAuthorIDs(operatorUserID int64, resp *relationpb.ListFollowingResponse) []int64 {
	result := make([]int64, 0)
	if resp == nil {
		return result
	}
	seen := map[int64]struct{}{operatorUserID: {}}
	for _, item := range resp.GetItems() {
		if item == nil {
			continue
		}
		if _, ok := seen[item.GetRelatedUserId()]; ok {
			continue
		}
		seen[item.GetRelatedUserId()] = struct{}{}
		result = append(result, item.GetRelatedUserId())
	}
	return result
}

func cleanText(value string) string {
	return strings.TrimSpace(value)
}

func extractTopicDrafts(text string) []repository.TopicDraft {
	matches := topicPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return []repository.TopicDraft{}
	}
	seen := make(map[string]struct{}, len(matches))
	items := make([]repository.TopicDraft, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		displayName := strings.TrimSpace(match[1])
		normalized := strings.ToLower(displayName)
		if displayName == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		items = append(items, repository.TopicDraft{
			TopicName:   displayName,
			DisplayName: displayName,
		})
	}
	return items
}

func enrichPostsWithTopics(ctx context.Context, svcCtx *svc.ServiceContext, posts []repository.Post) {
	if len(posts) == 0 || svcCtx == nil || svcCtx.TopicStore == nil {
		return
	}
	postIDs := make([]int64, 0, len(posts))
	for _, post := range posts {
		if post.ID > 0 {
			postIDs = append(postIDs, post.ID)
		}
	}
	if len(postIDs) == 0 {
		return
	}
	postTopics, err := svcCtx.TopicStore.ListTopicsByPostIDs(ctx, postIDs)
	if err != nil {
		return
	}
	for index := range posts {
		topics := postTopics[posts[index].ID]
		posts[index].Topics = make([]string, 0, len(topics))
		for _, topic := range topics {
			if strings.TrimSpace(topic.DisplayName) == "" {
				continue
			}
			posts[index].Topics = append(posts[index].Topics, topic.DisplayName)
		}
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func containsUnsafeScriptPattern(value string) bool {
	return !ValidateContent(value)
}
