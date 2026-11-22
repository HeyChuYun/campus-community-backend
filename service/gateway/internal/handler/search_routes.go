package handler

import (
	"context"
	"net/http"
	"strings"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/relationpb"
)

const (
	searchUserSuggestLimit = int64(5)
	searchUserPageSize     = int64(20)
)

func listHotTopicsHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		resp, err := svcCtx.ContentRPC.ListHotTopics(r.Context(), &contentpb.ListHotTopicsRequest{})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items": topicItemsToMap(resp.GetItems()),
		})
	}
}

func searchUserSuggestionsHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok || user == nil {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusOK, jsonResponse{Code: 0, Message: "ok", Data: map[string]any{"items": []map[string]any{}}})
			return
		}

		items, _, err := queryGatewayUsersByKeyword(r.Context(), svcCtx, query, 1, searchUserSuggestLimit)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"items": usersWithRelationToMap(svcCtx, r.Context(), user.GetUserId(), items),
			},
		})
	}
}

func searchUsersHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok || user == nil {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "missing query parameter: q"})
			return
		}

		page := parseOptionalQueryInt64(r, "page", 1)
		items, hasMore, err := queryGatewayUsersByKeyword(r.Context(), svcCtx, query, page, searchUserPageSize)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"items":    usersWithRelationToMap(svcCtx, r.Context(), user.GetUserId(), items),
				"has_more": hasMore,
			},
		})
	}
}

func searchPostsHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "missing query parameter: q"})
			return
		}

		resp, err := svcCtx.ContentRPC.SearchPosts(r.Context(), &contentpb.SearchPostsRequest{
			OperatorUserId: user.GetUserId(),
			Query:          query,
			Page:           parseOptionalQueryInt64(r, "page", 1),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":    postsToMap(svcCtx, resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func searchTopicsHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		query := strings.TrimSpace(r.URL.Query().Get("q"))
		if query == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "missing query parameter: q"})
			return
		}

		resp, err := svcCtx.ContentRPC.SearchTopics(r.Context(), &contentpb.SearchTopicsRequest{
			Query: query,
			Page:  parseOptionalQueryInt64(r, "page", 1),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":    topicItemsToMap(resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func getTopicPostsHandler(svcCtx *svc.ServiceContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok || user == nil {
			return
		}

		topicID, ok := parseRequiredQueryInt64(w, r, "topic_id")
		if !ok {
			return
		}

		resp, err := svcCtx.ContentRPC.GetTopicPosts(r.Context(), &contentpb.GetTopicPostsRequest{
			OperatorUserId: user.GetUserId(),
			TopicId:        topicID,
			Page:           parseOptionalQueryInt64(r, "page", 1),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeContentStatus(w, resp.GetBase(), map[string]any{
			"topic":    topicItemToMap(resp.GetTopic()),
			"items":    postsToMap(svcCtx, resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func queryGatewayUsersByKeyword(ctx context.Context, svcCtx *svc.ServiceContext, query string, page, pageSize int64) ([]gatewayUser, bool, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil {
		return []gatewayUser{}, false, nil
	}

	users, hasMore, err := svcCtx.UserQuery.Search(ctx, query, page, pageSize)
	if err != nil {
		return nil, false, err
	}
	items := make([]gatewayUser, 0, len(users))
	for _, user := range users {
		items = append(items, gatewayUserFromQuery(user))
	}
	return items, hasMore, nil
}

func normalizeSearchPage(page, pageSize int64) (int64, int64) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return offset, pageSize
}

func usersToMap(items []gatewayUser) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, userEntityToMap(item))
	}
	return result
}

func usersWithRelationToMap(svcCtx *svc.ServiceContext, ctx context.Context, viewerUserID int64, items []gatewayUser) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	if len(items) == 0 {
		return result
	}

	relationByUserID := make(map[int64]*relationpb.RelationSnapshot, len(items))
	if svcCtx != nil && svcCtx.RelationRPC != nil && viewerUserID > 0 {
		targetUserIDs := make([]int64, 0, len(items))
		for _, item := range items {
			if item.ID <= 0 || item.ID == viewerUserID {
				continue
			}
			targetUserIDs = append(targetUserIDs, item.ID)
		}
		if len(targetUserIDs) > 0 {
			if resp, err := svcCtx.RelationRPC.BatchGetRelation(ctx, &relationpb.BatchGetRelationRequest{
				ActorUserId:   viewerUserID,
				TargetUserIds: targetUserIDs,
			}); err == nil {
				for _, snapshot := range resp.GetSnapshots() {
					if snapshot == nil {
						continue
					}
					relationByUserID[snapshot.GetTargetUserId()] = snapshot
				}
			}
		}
	}

	for _, item := range items {
		mapped := userEntityToMap(item)
		snapshot := relationByUserID[item.ID]
		mapped["relation"] = map[string]any{
			"is_following":   snapshot != nil && snapshot.GetIsFollowing(),
			"is_followed_by": snapshot != nil && snapshot.GetIsFollowedBy(),
			"is_friend":      snapshot != nil && snapshot.GetIsFriend(),
		}
		result = append(result, mapped)
	}
	return result
}

func topicItemsToMap(items []*contentpb.TopicItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, topicItemToMap(item))
	}
	return result
}

func topicItemToMap(item *contentpb.TopicItem) map[string]any {
	if item == nil {
		return nil
	}
	return map[string]any{
		"topic_id":          item.GetTopicId(),
		"topic_name":        item.GetTopicName(),
		"display_name":      item.GetDisplayName(),
		"post_count":        item.GetPostCount(),
		"hot_score":         item.GetHotScore(),
		"last_refreshed_at": item.GetLastRefreshedAt(),
	}
}
