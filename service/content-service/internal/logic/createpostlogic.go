package logic

import (
	"context"

	"hfjj/backend/content-service/internal/repository"
	"hfjj/backend/content-service/internal/svc"
	"hfjj/backend/proto/contentpb"
)

type CreatePostLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePostLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePostLogic {
	return &CreatePostLogic{ctx: ctx, svcCtx: svcCtx}
}

// CreatePost 创建帖子，并在成功后补写一条 outbox 事件。
func (l *CreatePostLogic) CreatePost(in *contentpb.CreatePostRequest) (*contentpb.CreatePostResponse, error) {
	if in.GetOperatorUserId() <= 0 {
		return &contentpb.CreatePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "invalid operator_user_id")}, nil
	}
	if restriction, ok, err := l.svcCtx.ModerationStore.ActiveRestriction(l.ctx, in.GetOperatorUserId(), repository.RestrictionScopePostPublish); err != nil {
		return nil, err
	} else if ok {
		message := "当前已被限制发帖，请稍后再试"
		if restriction.EndsAt.Valid {
			message = "当前已被限制发帖，请于 " + restriction.EndsAt.Time.Local().Format("2006-01-02 15:04:05") + " 后重试"
		}
		return &contentpb.CreatePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_PERMISSION_DENIED, message),
		}, nil
	}
	if cleanText(in.GetContentText()) == "" && len(in.GetMedia()) == 0 {
		return &contentpb.CreatePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "empty post content")}, nil
	}
	if !ValidateContent(in.GetContentText()) {
		return &contentpb.CreatePostResponse{Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "post content contains unsafe script pattern")}, nil
	}
	if status := validateDailyPostLimit(l.ctx, l.svcCtx, in.GetOperatorUserId()); status != nil {
		return &contentpb.CreatePostResponse{Base: status}, nil
	}
	if status := validatePostMediaLimits(l.ctx, l.svcCtx, in.GetOperatorUserId(), in.GetMedia()); status != nil {
		return &contentpb.CreatePostResponse{Base: status}, nil
	}
	postCampusCode, ok := normalizePostCampusCode(in.GetCampusCode())
	if !ok {
		return &contentpb.CreatePostResponse{
			Base: newStatus(contentpb.ContentCode_CONTENT_CODE_INVALID_ARGUMENT, "post campus_code must be xinqiao or gaoji"),
		}, nil
	}

	// 先整理主帖子数据。
	post := repository.Post{
		AuthorUserID: in.GetOperatorUserId(),
		ContentText:  SanitizeContent(cleanText(in.GetContentText())),
		ContentType:  int32(normalizeContentType(in.GetMedia())),
		CampusCode:   postCampusCode,
		Visibility:   int32(normalizeVisibility(in.GetVisibility())),
		// 新帖子先进入审核中，审核通过后再转为正式可见。
		Status:       int32(contentpb.PostStatus_POST_STATUS_REVIEWING),
	}

	// 再从正文中提取 #话题#，后面用于落 topics/post_topics。
	topicDrafts := extractTopicDrafts(in.GetContentText())

	// 将媒体参数转换成仓储层结构。
	for _, item := range in.GetMedia() {
		post.Media = append(post.Media, repository.PostMedia{
			MediaType:  int32(item.GetMediaType()),
			StorageKey: item.GetStorageKey(),
			URL:        item.GetUrl(),
			Width:      item.GetWidth(),
			Height:     item.GetHeight(),
			DurationMS: item.GetDurationMs(),
			SizeBytes:  item.GetSizeBytes(),
			SortOrder:  item.GetSortOrder(),
		})
	}

	// 先落主帖子。
	created, err := l.svcCtx.PostStore.CreatePost(l.ctx, post)
	if err != nil {
		return nil, err
	}

	// 再补充帖子和话题的绑定关系。
	// 这里不回滚主帖子，避免话题写入异常时让客户端误以为整条帖子创建失败。
	if l.svcCtx.TopicStore != nil && len(topicDrafts) > 0 {
		if topics, bindErr := l.svcCtx.TopicStore.BindPostTopics(l.ctx, created.ID, topicDrafts); bindErr == nil {
			created.Topics = make([]string, 0, len(topics))
			for _, topic := range topics {
				if topic.DisplayName != "" {
					created.Topics = append(created.Topics, topic.DisplayName)
				}
			}
		}
	}

	// 发帖成功后写入 outbox，供时间线、统计和通知等异步模块消费。
	emitContentEvent(l.ctx, l.svcCtx, "content.post.created", in.GetOperatorUserId(), created.AuthorUserID, "post", created.ID, map[string]any{
		"post": toPostPB(created),
	})

	return &contentpb.CreatePostResponse{
		Base: newStatus(contentpb.ContentCode_CONTENT_CODE_OK, "ok"),
		Post: toPostPB(created),
	}, nil
}
