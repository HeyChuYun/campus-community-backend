package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/passportpb"
	"hfjj/backend/proto/relationpb"
)

type relationTargetRequest struct {
	TargetUserID int64 `json:"target_user_id"`
}

type createPostRequest struct {
	ContentText   string   `json:"content_text"`
	CampusCode    string   `json:"campus_code"`
	Visibility    int32    `json:"visibility"`
	MediaIDs      []string `json:"media_ids"`
	AllowComment  *bool    `json:"allow_comment"`
	HiddenUserIDs []int64  `json:"hidden_user_ids"`
}

type postIDRequest struct {
	PostID int64 `json:"post_id"`
}

type createCommentRequest struct {
	PostID           int64  `json:"post_id"`
	ReplyToCommentID int64  `json:"reply_to_comment_id"`
	ReplyToUserID    int64  `json:"reply_to_user_id"`
	Content          string `json:"content"`
}

type deleteCommentRequest struct {
	CommentID int64 `json:"comment_id"`
}

func getRelationHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.read", 0)
		if !ok {
			return
		}

		targetUserID, ok := parseRequiredQueryInt64(w, r, "target_user_id")
		if !ok {
			return
		}

		resp, err := svcCtx.RelationRPC.GetRelation(r.Context(), &relationpb.GetRelationRequest{
			ActorUserId:  user.GetUserId(),
			TargetUserId: targetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"snapshot": relationSnapshotToMap(resp.GetSnapshot()),
		})
	}
}

func listFollowingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.following.read", 0)
		if !ok {
			return
		}

		targetUserID := parseOptionalQueryInt64(r, "user_id", user.GetUserId())
		page := parseOptionalQueryInt64(r, "page", 1)
		pageSize := parseOptionalQueryInt64(r, "page_size", 20)

		resp, err := svcCtx.RelationRPC.ListFollowing(r.Context(), &relationpb.ListFollowingRequest{
			UserId:   targetUserID,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"items":    relatedUsersToMap(svcCtx, resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func listFollowersHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.followers.read", 0)
		if !ok {
			return
		}

		targetUserID := parseOptionalQueryInt64(r, "user_id", user.GetUserId())
		page := parseOptionalQueryInt64(r, "page", 1)
		pageSize := parseOptionalQueryInt64(r, "page_size", 20)

		resp, err := svcCtx.RelationRPC.ListFollowers(r.Context(), &relationpb.ListFollowersRequest{
			UserId:   targetUserID,
			Page:     page,
			PageSize: pageSize,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"items":    relatedUsersToMap(svcCtx, resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func followHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req relationTargetRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.follow.create", 0)
		if !ok {
			return
		}

		resp, err := svcCtx.RelationRPC.Follow(r.Context(), &relationpb.FollowRequest{
			ActorUserId:  user.GetUserId(),
			TargetUserId: req.TargetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"snapshot":       relationSnapshotToMap(resp.GetSnapshot()),
			"actor_counter":  relationCounterToMap(resp.GetActorCounter()),
			"target_counter": relationCounterToMap(resp.GetTargetCounter()),
		})
	}
}

func unfollowHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req relationTargetRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.follow.delete", 0)
		if !ok {
			return
		}

		resp, err := svcCtx.RelationRPC.Unfollow(r.Context(), &relationpb.UnfollowRequest{
			ActorUserId:  user.GetUserId(),
			TargetUserId: req.TargetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"snapshot":       relationSnapshotToMap(resp.GetSnapshot()),
			"actor_counter":  relationCounterToMap(resp.GetActorCounter()),
			"target_counter": relationCounterToMap(resp.GetTargetCounter()),
		})
	}
}

func blockUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req relationTargetRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.block.create", 0)
		if !ok {
			return
		}

		resp, err := svcCtx.RelationRPC.BlockUser(r.Context(), &relationpb.BlockUserRequest{
			ActorUserId:  user.GetUserId(),
			TargetUserId: req.TargetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"snapshot":      relationSnapshotToMap(resp.GetSnapshot()),
			"actor_counter": relationCounterToMap(resp.GetActorCounter()),
		})
	}
}

func unblockUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req relationTargetRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "relation.block.delete", 0)
		if !ok {
			return
		}

		resp, err := svcCtx.RelationRPC.UnblockUser(r.Context(), &relationpb.UnblockUserRequest{
			ActorUserId:  user.GetUserId(),
			TargetUserId: req.TargetUserID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeRelationStatus(w, resp.GetBase(), map[string]any{
			"snapshot":      relationSnapshotToMap(resp.GetSnapshot()),
			"actor_counter": relationCounterToMap(resp.GetActorCounter()),
		})
	}
}

func createPostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPostRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok {
			return
		}

		preparedMedia, err := preparePostMediaForCreate(r.Context(), svcCtx, user.GetUserId(), req.MediaIDs)
		if err != nil {
			WriteError(w, ErrInvalidArgument.WithDetail("message", err.Error()))
			return
		}

		resp, err := svcCtx.ContentRPC.CreatePost(r.Context(), &contentpb.CreatePostRequest{
			OperatorUserId: user.GetUserId(),
			ContentText:    req.ContentText,
			CampusCode:     req.CampusCode,
			Visibility:     contentpb.PostVisibility(req.Visibility),
			Media:          preparedMedia.Media,
		})
		if err != nil {
			cleanupPreparedPostMedia(r.Context(), preparedMedia)
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if resp.GetBase().GetCode() != contentpb.ContentCode_CONTENT_CODE_OK || resp.GetPost() == nil {
			cleanupPreparedPostMedia(r.Context(), preparedMedia)
		}
		if resp.GetBase().GetCode() == contentpb.ContentCode_CONTENT_CODE_OK && resp.GetPost() != nil {
			if err := finalizePreparedPostMedia(r.Context(), svcCtx, preparedMedia, resp.GetPost().GetPostId()); err != nil {
				WriteError(w, ErrInternalError)
				return
			}
			allowComment := true
			if req.AllowComment != nil {
				allowComment = *req.AllowComment
			}
			hiddenUserIDs := make([]int64, 0, len(req.HiddenUserIDs))
			for _, hiddenUserID := range req.HiddenUserIDs {
				if hiddenUserID > 0 && hiddenUserID != user.GetUserId() {
					hiddenUserIDs = append(hiddenUserIDs, hiddenUserID)
				}
			}
			if err := svcCtx.Privacy.UpsertPostSettings(r.Context(), resp.GetPost().GetPostId(), allowComment, hiddenUserIDs); err != nil {
				WriteError(w, ErrInternalError)
				return
			}
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"post": postToMap(svcCtx, resp.GetPost()),
		})
	}
}

func deletePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postIDRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		operatorUser, _, ok := authorizeBearerTokenAllowDenied(w, r, svcCtx, "content.post.delete.any", 0)
		if !ok || operatorUser == nil {
			return
		}

		allowedAny, _, ok := authorizePermissionResult(r, svcCtx, "content.post.delete.any", 0)
		if !ok {
			return
		}
		forceDelete := allowedAny
		if !forceDelete {
			postResp, err := svcCtx.ContentRPC.GetPost(r.Context(), &contentpb.GetPostRequest{
				OperatorUserId: operatorUser.GetUserId(),
				PostId:         req.PostID,
			})
			if err != nil {
				writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
				return
			}
			if postResp.GetBase().GetCode() != contentpb.ContentCode_CONTENT_CODE_OK || postResp.GetPost() == nil {
				writeContentStatus(w, postResp.GetBase(), nil)
				return
			}

			user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.delete.own", postResp.GetPost().GetAuthorUserId())
			if !ok {
				return
			}
			operatorUser = user
		}

		resp, err := svcCtx.ContentRPC.DeletePost(r.Context(), &contentpb.DeletePostRequest{
			OperatorUserId: operatorUser.GetUserId(),
			PostId:         req.PostID,
			ForceDelete:    forceDelete,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, resp.GetBase(), nil)
	}
}

func getPostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		postID, ok := parseRequiredQueryInt64(w, r, "post_id")
		if !ok {
			return
		}

		resp, err := svcCtx.ContentRPC.GetPost(r.Context(), &contentpb.GetPostRequest{
			OperatorUserId: user.GetUserId(),
			PostId:         postID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if resp.GetPost() != nil {
			allowed, err := svcCtx.Privacy.CanViewerAccessPost(r.Context(), user.GetUserId(), resp.GetPost().GetAuthorUserId(), resp.GetPost().GetPostId())
			if err != nil {
				WriteError(w, ErrInternalError)
				return
			}
			if !allowed {
				WriteError(w, ErrPermissionDenied.WithDetail("message", "鐠囥儱绗樼€涙劒绗夐崣顖濐潌"))
				return
			}
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"post": postToMap(svcCtx, resp.GetPost()),
		})
	}
}

func getProfilePostsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		targetUserID := parseOptionalQueryInt64(r, "user_id", user.GetUserId())
		page := parseOptionalQueryInt64(r, "page", 1)
		pageSize := parseOptionalQueryInt64(r, "page_size", 20)

		resp, err := svcCtx.ContentRPC.GetUserPosts(r.Context(), &contentpb.GetUserPostsRequest{
			OperatorUserId: user.GetUserId(),
			TargetUserId:   targetUserID,
			Page:           page,
			PageSize:       pageSize,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		filteredPosts, err := filterVisiblePosts(r.Context(), svcCtx, user.GetUserId(), resp.GetItems())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":    postsToMap(svcCtx, filteredPosts),
			"has_more": resp.GetHasMore(),
		})
	}
}

func getFollowingFeedHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.feed.read", 0)
		if !ok {
			return
		}

		page := parseOptionalQueryInt64(r, "page", 1)
		pageSize := parseOptionalQueryInt64(r, "page_size", 20)
		resp, err := svcCtx.ContentRPC.GetFollowingFeed(r.Context(), &contentpb.GetFollowingFeedRequest{
			OperatorUserId: user.GetUserId(),
			Page:           page,
			PageSize:       pageSize,
			CampusCode:     strings.TrimSpace(r.URL.Query().Get("campus_code")),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		filteredPosts, err := filterVisiblePosts(r.Context(), svcCtx, user.GetUserId(), resp.GetItems())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		feedItems, err := commercialBuildFeedItems(r.Context(), svcCtx, user.GetUserId(), "following", page, filteredPosts)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":      postsToMap(svcCtx, filteredPosts),
			"feed_items": feedItems,
			"has_more":   resp.GetHasMore(),
		})
	}
}

func getDiscoverFeedHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.feed.read", 0)
		if !ok {
			return
		}

		page := parseOptionalQueryInt64(r, "page", 1)
		pageSize := parseOptionalQueryInt64(r, "page_size", 20)
		resp, err := svcCtx.ContentRPC.GetDiscoverFeed(r.Context(), &contentpb.GetDiscoverFeedRequest{
			OperatorUserId: user.GetUserId(),
			Page:           page,
			PageSize:       pageSize,
			CampusCode:     strings.TrimSpace(r.URL.Query().Get("campus_code")),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		filteredPosts, err := filterVisiblePosts(r.Context(), svcCtx, user.GetUserId(), resp.GetItems())
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		feedItems, err := commercialBuildFeedItems(r.Context(), svcCtx, user.GetUserId(), "discover", page, filteredPosts)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":      postsToMap(svcCtx, filteredPosts),
			"feed_items": feedItems,
			"has_more":   resp.GetHasMore(),
		})
	}
}

func createCommentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createCommentRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.comment.create", 0)
		if !ok {
			return
		}
		authorUserID, exists, err := svcCtx.Privacy.VisiblePostAuthorID(r.Context(), req.PostID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !exists {
			WriteError(w, ErrNotFound)
			return
		}
		allowed, err := svcCtx.Privacy.CanViewerAccessPost(r.Context(), user.GetUserId(), authorUserID, req.PostID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !allowed {
			WriteError(w, ErrPermissionDenied.WithDetail("message", "娴ｇ姵妫ゅ▔鏇＄槑鐠侀缚顕氱敮鏍х摍"))
			return
		}
		settings, err := svcCtx.Privacy.PostSettings(r.Context(), req.PostID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		// 关闭评论后，仍允许帖子作者自己补充评论；仅限制其他用户评论。
		if !settings.AllowComment && user.GetUserId() != authorUserID {
			WriteError(w, ErrPermissionDenied.WithDetail("message", "该帖子已关闭评论"))
			return
		}

		resp, err := svcCtx.ContentRPC.CreateComment(r.Context(), &contentpb.CreateCommentRequest{
			OperatorUserId:   user.GetUserId(),
			PostId:           req.PostID,
			ReplyToCommentId: req.ReplyToCommentID,
			ReplyToUserId:    req.ReplyToUserID,
			Content:          req.Content,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"comment": commentToMap(svcCtx, resp.GetComment()),
		})
	}
}

func listCommentsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		postID, ok := parseRequiredQueryInt64(w, r, "post_id")
		if !ok {
			return
		}
		authorUserID, exists, err := svcCtx.Privacy.VisiblePostAuthorID(r.Context(), postID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !exists {
			WriteError(w, ErrNotFound)
			return
		}
		allowed, err := svcCtx.Privacy.CanViewerAccessPost(r.Context(), user.GetUserId(), authorUserID, postID)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if !allowed {
			WriteError(w, ErrPermissionDenied.WithDetail("message", "该帖子评论不可见"))
			return
		}

		resp, err := svcCtx.ContentRPC.ListComments(r.Context(), &contentpb.ListCommentsRequest{
			OperatorUserId: user.GetUserId(),
			PostId:         postID,
			Page:           parseOptionalQueryInt64(r, "page", 1),
			PageSize:       parseOptionalQueryInt64(r, "page_size", 20),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, resp.GetBase(), map[string]any{
			"items":    commentsToMap(svcCtx, resp.GetItems()),
			"has_more": resp.GetHasMore(),
		})
	}
}

func deleteCommentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req deleteCommentRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, ok := authorizeSelfOwnedAction(w, r, svcCtx, "content.comment.delete.own")
		if !ok {
			return
		}

		resp, err := svcCtx.ContentRPC.DeleteComment(r.Context(), &contentpb.DeleteCommentRequest{
			OperatorUserId: user.GetUserId(),
			CommentId:      req.CommentID,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, resp.GetBase(), nil)
	}
}

func likePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return postActionHandler(svcCtx, "content.like.create", func(ctx *svc.ServiceContext, r *http.Request, userID, postID int64) (*contentpb.BaseResp, error) {
		resp, err := ctx.ContentRPC.LikePost(r.Context(), &contentpb.LikePostRequest{
			OperatorUserId: userID,
			PostId:         postID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetBase(), nil
	})
}

func unlikePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return postActionHandler(svcCtx, "content.like.create", func(ctx *svc.ServiceContext, r *http.Request, userID, postID int64) (*contentpb.BaseResp, error) {
		resp, err := ctx.ContentRPC.UnlikePost(r.Context(), &contentpb.UnlikePostRequest{
			OperatorUserId: userID,
			PostId:         postID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetBase(), nil
	})
}

func favoritePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return postActionHandler(svcCtx, "content.favorite.create", func(ctx *svc.ServiceContext, r *http.Request, userID, postID int64) (*contentpb.BaseResp, error) {
		resp, err := ctx.ContentRPC.FavoritePost(r.Context(), &contentpb.FavoritePostRequest{
			OperatorUserId: userID,
			PostId:         postID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetBase(), nil
	})
}

func unfavoritePostHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return postActionHandler(svcCtx, "content.favorite.create", func(ctx *svc.ServiceContext, r *http.Request, userID, postID int64) (*contentpb.BaseResp, error) {
		resp, err := ctx.ContentRPC.UnfavoritePost(r.Context(), &contentpb.UnfavoritePostRequest{
			OperatorUserId: userID,
			PostId:         postID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetBase(), nil
	})
}

func postActionHandler(
	svcCtx *svc.ServiceContext,
	permissionCode string,
	call func(ctx *svc.ServiceContext, r *http.Request, userID, postID int64) (*contentpb.BaseResp, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postIDRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, permissionCode, 0)
		if !ok {
			return
		}

		base, err := call(svcCtx, r, user.GetUserId(), req.PostID)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeContentStatus(w, base, nil)
	}
}

func writeRelationStatus(w http.ResponseWriter, base *relationpb.BaseResp, data any) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    int32(base.GetCode()),
		Message: base.GetMessage(),
		Data:    data,
	})
}

func writeContentStatus(w http.ResponseWriter, base *contentpb.BaseResp, data any) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    int32(base.GetCode()),
		Message: base.GetMessage(),
		Data:    data,
	})
}

func relationSnapshotToMap(snapshot *relationpb.RelationSnapshot) map[string]any {
	if snapshot == nil {
		return nil
	}
	return map[string]any{
		"actor_user_id":    snapshot.GetActorUserId(),
		"target_user_id":   snapshot.GetTargetUserId(),
		"is_following":     snapshot.GetIsFollowing(),
		"is_followed_by":   snapshot.GetIsFollowedBy(),
		"is_mutual_follow": snapshot.GetIsMutualFollow(),
		"is_friend":        snapshot.GetIsFriend(),
		"is_blocking":      snapshot.GetIsBlocking(),
		"is_blocked_by":    snapshot.GetIsBlockedBy(),
	}
}

func relationCounterToMap(counter *relationpb.UserCounter) map[string]any {
	if counter == nil {
		return nil
	}
	return map[string]any{
		"user_id":         counter.GetUserId(),
		"following_count": counter.GetFollowingCount(),
		"follower_count":  counter.GetFollowerCount(),
		"friend_count":    counter.GetFriendCount(),
		"blocked_count":   counter.GetBlockedCount(),
	}
}

func relatedUsersToMap(svcCtx *svc.ServiceContext, items []*relationpb.RelatedUserItem) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		relatedUserID := item.GetRelatedUserId()
		result = append(result, map[string]any{
			"user_id":         item.GetUserId(),
			"related_user_id": relatedUserID,
			"user":            loadUserSummary(svcCtx, relatedUserID),
			"cursor":          item.GetCursor(),
		})
	}
	return result
}

func filterVisiblePosts(ctx context.Context, svcCtx *svc.ServiceContext, viewerUserID int64, posts []*contentpb.PostItem) ([]*contentpb.PostItem, error) {
	if len(posts) == 0 {
		return posts, nil
	}
	filtered := make([]*contentpb.PostItem, 0, len(posts))
	for _, post := range posts {
		if post == nil {
			continue
		}
		allowed, err := svcCtx.Privacy.CanViewerAccessPost(ctx, viewerUserID, post.GetAuthorUserId(), post.GetPostId())
		if err != nil {
			return nil, err
		}
		if allowed {
			filtered = append(filtered, post)
		}
	}
	return filtered, nil
}

func postToMap(svcCtx *svc.ServiceContext, post *contentpb.PostItem) map[string]any {
	if post == nil {
		return nil
	}
	settings, _ := svcCtx.Privacy.PostSettings(context.Background(), post.GetPostId())
	return map[string]any{
		"post_id":        post.GetPostId(),
		"author_user_id": post.GetAuthorUserId(),
		"author":         loadUserSummary(svcCtx, post.GetAuthorUserId()),
		"content_text":   post.GetContentText(),
		"status":         int32(post.GetStatus()),
		"visibility":     int32(post.GetVisibility()),
		"like_count":     post.GetLikeCount(),
		"comment_count":  post.GetCommentCount(),
		"favorite_count": post.GetFavoriteCount(),
		"topics":         append([]string(nil), post.GetTopics()...),
		"campus_code":    post.GetCampusCode(),
		"media":          postMediaToMap(post.GetMedia()),
		"allow_comment":  settings.AllowComment,
		"created_at":     post.GetCreatedAt(),
		"updated_at":     post.GetUpdatedAt(),
	}
}

func postsToMap(svcCtx *svc.ServiceContext, posts []*contentpb.PostItem) []map[string]any {
	result := make([]map[string]any, 0, len(posts))
	for _, post := range posts {
		result = append(result, postToMap(svcCtx, post))
	}
	return result
}

func commentToMap(svcCtx *svc.ServiceContext, comment *contentpb.CommentItem) map[string]any {
	if comment == nil {
		return nil
	}
	return map[string]any{
		"comment_id":          comment.GetCommentId(),
		"post_id":             comment.GetPostId(),
		"user_id":             comment.GetUserId(),
		"author":              loadUserSummary(svcCtx, comment.GetUserId()),
		"reply_to_comment_id": comment.GetReplyToCommentId(),
		"reply_to_user_id":    comment.GetReplyToUserId(),
		"content":             comment.GetContent(),
		"created_at":          comment.GetCreatedAt(),
	}
}

func commentsToMap(svcCtx *svc.ServiceContext, comments []*contentpb.CommentItem) []map[string]any {
	result := make([]map[string]any, 0, len(comments))
	for _, comment := range comments {
		result = append(result, commentToMap(svcCtx, comment))
	}
	return result
}

func loadUserSummary(svcCtx *svc.ServiceContext, userID int64) map[string]any {
	if svcCtx == nil || userID <= 0 {
		return nil
	}
	user, ok, err := queryGatewayUserByID(svcCtx, userID)
	if err != nil || !ok {
		return map[string]any{
			"user_id":  userID,
			"account":  "",
			"nickname": "",
		}
	}
	return userEntityToMap(user)
}

func BatchLoadUserSummaries(svcCtx *svc.ServiceContext, userIDs []int64) map[int64]map[string]any {
	if svcCtx == nil || len(userIDs) == 0 {
		return make(map[int64]map[string]any)
	}

	result := make(map[int64]map[string]any, len(userIDs))

	uniqueIDs := make([]int64, 0, len(userIDs))
	seen := make(map[int64]bool)
	for _, id := range userIDs {
		if id > 0 && !seen[id] {
			uniqueIDs = append(uniqueIDs, id)
			seen[id] = true
		}
	}

	if len(uniqueIDs) == 0 {
		return result
	}

	users, err := batchQueryGatewayUsersByIDs(svcCtx, uniqueIDs)
	if err != nil {
		for _, id := range uniqueIDs {
			result[id] = map[string]any{
				"user_id":  id,
				"account":  "",
				"nickname": "",
			}
		}
		return result
	}

	for _, user := range users {
		result[user.ID] = userEntityToMap(user)
	}

	// 鐞涖儱鍘栭張顏呭閸掓壆娈戦悽銊﹀煕
	for _, id := range uniqueIDs {
		if _, ok := result[id]; !ok {
			result[id] = map[string]any{
				"user_id":  id,
				"account":  "",
				"nickname": "",
			}
		}
	}

	return result
}

func batchQueryGatewayUsersByIDs(svcCtx *svc.ServiceContext, userIDs []int64) ([]gatewayUser, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || len(userIDs) == 0 {
		return []gatewayUser{}, nil
	}
	users, err := svcCtx.UserQuery.BatchByIDs(context.Background(), userIDs)
	if err != nil {
		return nil, err
	}
	result := make([]gatewayUser, 0, len(users))
	for _, user := range users {
		result = append(result, gatewayUserFromQuery(user))
	}
	return result, nil
}
func postMediaToMap(media []*contentpb.PostMedia) []map[string]any {
	result := make([]map[string]any, 0, len(media))
	for _, item := range media {
		result = append(result, map[string]any{
			"media_id":    item.GetMediaId(),
			"media_type":  int32(item.GetMediaType()),
			"storage_key": item.GetStorageKey(),
			"url":         item.GetUrl(),
			"width":       item.GetWidth(),
			"height":      item.GetHeight(),
			"duration_ms": item.GetDurationMs(),
			"size_bytes":  item.GetSizeBytes(),
			"sort_order":  item.GetSortOrder(),
		})
	}
	return result
}

func parseRequiredQueryInt64(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "missing query parameter: " + key})
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "invalid query parameter: " + key})
		return 0, false
	}
	return parsed, true
}

func parseOptionalQueryInt64(r *http.Request, key string, defaultValue int64) int64 {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func authorizeSelfOwnedAction(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, permissionCode string) (*passportpb.UserProfile, bool) {
	user, ok := verifyBearerToken(w, r, svcCtx)
	if !ok || user == nil {
		return nil, false
	}

	token, ok := bearerTokenFromRequest(w, r)
	if !ok {
		return nil, false
	}
	resp, err := svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: user.GetUserId(),
	})
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return nil, false
	}
	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !resp.GetAllowed() {
		writePassportStatus(w, resp.GetBase(), nil)
		return nil, false
	}
	return user, true
}
