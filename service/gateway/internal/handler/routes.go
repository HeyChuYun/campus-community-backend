package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/mapper"
	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/gateway/internal/userquery"
	"hfjj/backend/proto/contentpb"
	"hfjj/backend/proto/passportpb"
	"hfjj/backend/proto/relationpb"
	"hfjj/backend/proto/schoolpb"

	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc/metadata"
)

type jsonResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type behaviorVerifyRequest struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

type authorizeRequest struct {
	AuthSessionID  string `json:"auth_session_id"`
	StudentNo      string `json:"student_no"`
	SchoolPassword string `json:"school_password"`
	CaptchaCode    string `json:"captcha_code"`
}

type registerRequest struct {
	Account          string                `json:"account"`
	Password         string                `json:"password"`
	Nickname         string                `json:"nickname"`
	SchoolAuthTicket string                `json:"school_auth_ticket"`
	BehaviorVerify   behaviorVerifyRequest `json:"behavior_verify"`
}

type loginRequest struct {
	Account        string                `json:"account"`
	Password       string                `json:"password"`
	BehaviorVerify behaviorVerifyRequest `json:"behavior_verify"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type assignUserRolesRequest struct {
	TargetUserID int64    `json:"target_user_id"`
	RoleCodes    []string `json:"role_codes"`
}

type updateProfileRequest struct {
	Nickname      string `json:"nickname"`
	Bio           string `json:"bio"`
	AvatarURL     string `json:"avatar_url"`
	CoverImageURL string `json:"cover_image_url"`
}

func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		// 检测是否正常连接服务端
		{Method: http.MethodGet, Path: "/api/v1/health", Handler: healthCheckHandler(svcCtx)},
		// 检查更新请求
		{Method: http.MethodGet, Path: "/api/v1/app/version", Handler: appVersionHandler(svcCtx)},
		// 极验验证码配置提供
		{Method: http.MethodGet, Path: "/api/v1/auth/behaviorConfig", Handler: behaviorConfigHandler(svcCtx)},

		{Method: http.MethodGet, Path: "/api/v1/school/preAuthorize", Handler: preAuthorizeHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/school/authorize", Handler: authorizeHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/password/reset/resolve-account", Handler: resolvePasswordResetAccountHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/password/reset/by-school", Handler: resetPasswordBySchoolHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/register", Handler: registerHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/login", Handler: loginHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/refresh", Handler: refreshHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/logout", Handler: logoutHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/me", Handler: meHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/me/security", Handler: accountSecurityHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/profile", Handler: updateMyProfileHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/password/change", Handler: changeMyPasswordHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/school/password/initKey", Handler: initCampusPasswordKeyHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/school/password/exchangeKey", Handler: exchangeCampusPasswordKeyHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/school/rebind", Handler: rebindSchoolHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/me/school/session/refresh", Handler: refreshSchoolSessionHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/users/profile", Handler: userProfileHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/relation/detail", Handler: getRelationHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/relation/following", Handler: listFollowingHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/relation/followers", Handler: listFollowersHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/relation/follow", Handler: followHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/relation/unfollow", Handler: unfollowHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/relation/block", Handler: blockUserHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/relation/unblock", Handler: unblockUserHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/privacy/blacklist", Handler: getPrivacyBlacklistHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/privacy/blacklist", Handler: updatePrivacyBlacklistHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/profile/avatar/upload", Handler: uploadAvatarHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/profile/cover/upload", Handler: uploadCoverHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/media/image/upload", Handler: uploadPostImageHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/media/video/init", Handler: initPostVideoUploadHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/media/video/upload", Handler: uploadPostVideoHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/media/video/merge", Handler: mergePostVideoHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts", Handler: createPostHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts/delete", Handler: deletePostHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/content/posts/detail", Handler: getPostHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/content/profile/posts", Handler: getProfilePostsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/content/feed/following", Handler: getFollowingFeedHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/content/feed/discover", Handler: getDiscoverFeedHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/content/comments", Handler: listCommentsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/comments", Handler: createCommentHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/comments/delete", Handler: deleteCommentHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts/like", Handler: likePostHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts/unlike", Handler: unlikePostHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts/favorite", Handler: favoritePostHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/content/posts/unfavorite", Handler: unfavoritePostHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/hot-topics", Handler: listHotTopicsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/suggestions/users", Handler: searchUserSuggestionsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/users", Handler: searchUsersHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/posts", Handler: searchPostsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/topics", Handler: searchTopicsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/search/topics/posts", Handler: getTopicPostsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/connect-token", Handler: imConnectTokenHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/ws", Handler: imRealtimeWSHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ws", Handler: imRealtimeWSHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/conversations", Handler: imListDirectConversationsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/messages", Handler: imListDirectMessagesHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/dm/check", Handler: imDMCheckHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/dm/send", Handler: imDMSendHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/conversations/read", Handler: imMarkConversationReadHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/interactions", Handler: imListInteractionsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/interactions/read", Handler: imMarkInteractionsReadHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/notifications", Handler: imListSystemNotificationsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/im/notifications/read", Handler: imMarkSystemNotificationsReadHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/im/unread-count", Handler: imGetTotalUnreadCountHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/report/settings", Handler: reportSettingsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/report/evidence/upload", Handler: uploadReportEvidenceHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/report/user", Handler: submitUserReportHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/report/post/appeal/context", Handler: reportPostAppealContextHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/report/post/appeal", Handler: submitPostAppealHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/commercial/ad/orders", Handler: commercialCreateAdOrderHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/commercial/membership/orders", Handler: commercialCreateMembershipOrderHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/commercial/ads/candidates", Handler: commercialAdCandidatesHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/commercial/ads/impression", Handler: commercialAdImpressionHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/commercial/ads/click", Handler: commercialAdClickHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/pay/alipay/notify", Handler: commercialAlipayNotifyHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/pay/wechat/notify", Handler: commercialWechatNotifyHandler(svcCtx)},
		// Heartbeat
		{Method: http.MethodPost, Path: "/api/v1/app/heartbeat", Handler: heartbeatHandler(svcCtx)},
		// Admin
		{Method: http.MethodPost, Path: "/api/v1/admin/login", Handler: adminLoginHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/logout", Handler: adminLogoutHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/me", Handler: adminMeHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/stats", Handler: adminStatsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/users", Handler: adminUsersHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users/status", Handler: adminUpdateUserStatusHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users/restrictions", Handler: adminApplyUserRestrictionHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/users/roles", Handler: adminAssignUserRolesHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/roles", Handler: adminListRolesHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/roles", Handler: adminCreateRoleHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/roles/delete", Handler: adminDeleteRoleHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/roles/permissions", Handler: adminSetRolePermissionsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/permissions", Handler: adminListPermissionsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/reports", Handler: adminListReportsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/reports/detail", Handler: adminReportDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/reports/status", Handler: adminUpdateReportStatusHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/content/posts", Handler: adminListContentPostsHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/content/violation-categories", Handler: adminViolationCategoriesHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/content/posts/block", Handler: adminBlockPostHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/content/posts/restore", Handler: adminRestorePostHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/users/violations", Handler: adminUserViolationsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/notifications/send", Handler: adminSendSystemNotificationHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/commercial/ad/orders", Handler: commercialListAdOrdersAdminHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/commercial/ad/review", Handler: commercialReviewAdOrderAdminHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/commercial/membership/orders", Handler: commercialListMembershipOrdersAdminHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/commercial/memberships/detail", Handler: commercialGetMembershipAdminHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/commercial/memberships/grant", Handler: commercialGrantMembershipAdminHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/commercial/memberships/extend", Handler: commercialExtendMembershipAdminHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/commercial/memberships/revoke", Handler: commercialRevokeMembershipAdminHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/app/versions", Handler: adminListAppVersionsHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/app/versions", Handler: adminCreateAppVersionHandler(svcCtx)},
		// CORS preflight for admin
		{Method: http.MethodOptions, Path: "/api/v1/admin/login", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/logout", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/me", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/stats", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/users", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/users/status", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/users/restrictions", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/users/roles", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/roles", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/roles/delete", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/roles/permissions", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/permissions", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/reports", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/reports/detail", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/reports/status", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/content/posts", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/content/violation-categories", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/content/posts/block", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/content/posts/restore", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/users/violations", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/notifications/send", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/ad/orders", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/ad/review", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/membership/orders", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/memberships/detail", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/memberships/grant", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/memberships/extend", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/commercial/memberships/revoke", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/admin/app/versions", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/auth/behaviorConfig", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/report/settings", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/report/user", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/report/post/appeal/context", Handler: adminCORSHandler()},
		{Method: http.MethodOptions, Path: "/api/v1/report/post/appeal", Handler: adminCORSHandler()},
	})
}

func healthCheckHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		WriteSuccess(w, map[string]any{
			"status":    "ok",
			"timestamp": time.Now().Unix(),
			"service":   "gateway",
		})
	}
}

// 验证码 配置提供
func behaviorConfigHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w)
		resp, err := svcCtx.PassportRPC.GetBehaviorVerifyConfig(r.Context(), &passportpb.GetBehaviorVerifyConfigRequest{})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		config := resp.GetConfig()
		writePassportStatus(w, resp.GetBase(), map[string]any{
			"enabled":    config.GetEnabled(),
			"provider":   config.GetProvider(),
			"captcha_id": config.GetCaptchaId(),
		})
	}
}

// 校园授权 pre
func preAuthorizeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := svcCtx.SchoolRPC.PreAuthorize(r.Context(), &schoolpb.PreAuthorizeRequest{})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeSchoolStatus(w, resp.GetBase(), map[string]any{
			"auth_session_id":      resp.GetAuthSessionId(),
			"captcha_image_base64": resp.GetCaptchaImageBase64(),
		})
	}
}

func authorizeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req authorizeRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.SchoolRPC.Authorize(r.Context(), &schoolpb.AuthorizeRequest{
			AuthSessionId:  req.AuthSessionID,
			StudentNo:      req.StudentNo,
			SchoolPassword: req.SchoolPassword,
			CaptchaCode:    req.CaptchaCode,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writeSchoolStatus(w, resp.GetBase(), map[string]any{
			"school_auth_ticket": resp.GetSchoolAuthTicket(),
			"student_no":         resp.GetStudentNo(),
			"student_name":       resp.GetStudentName(),
			"student_status":     int32(resp.GetStudentStatus()),
			"campus_tgt":         resp.GetCampusTgt(),
		})
	}
}

func registerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.PassportRPC.Register(r.Context(), &passportpb.RegisterRequest{
			Account:          req.Account,
			Password:         req.Password,
			Nickname:         req.Nickname,
			SchoolAuthTicket: req.SchoolAuthTicket,
			BehaviorVerify:   toBehaviorVerifyToken(req.BehaviorVerify),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writePassportStatus(w, resp.GetBase(), map[string]any{
			"user":   userToMap(resp.GetUser()),
			"tokens": tokenPairToMap(resp.GetTokens()),
		})
	}
}

func loginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		// 服务端从连接信息和请求头中提取登录来源信息。
		ctx := metadata.AppendToOutgoingContext(
			r.Context(),
			"x-login-ip", clientIP(r),
			"x-device", clientDevice(r),
		)

		resp, err := svcCtx.PassportRPC.Login(ctx, &passportpb.LoginRequest{
			Account:        req.Account,
			Password:       req.Password,
			BehaviorVerify: toBehaviorVerifyToken(req.BehaviorVerify),
		})
		if err != nil {
			WriteError(w, ErrRPCFailed)
			return
		}
		writePassportStatus(w, resp.GetBase(), map[string]any{
			"user":   userToMap(resp.GetUser()),
			"tokens": tokenPairToMap(resp.GetTokens()),
		})
	}
}

func refreshHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.PassportRPC.RefreshToken(r.Context(), &passportpb.RefreshTokenRequest{
			RefreshToken: req.RefreshToken,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writePassportStatus(w, resp.GetBase(), map[string]any{
			"tokens": tokenPairToMap(resp.GetTokens()),
		})
	}
}

func logoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req logoutRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		resp, err := svcCtx.PassportRPC.Logout(r.Context(), &passportpb.LogoutRequest{
			RefreshToken: req.RefreshToken,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writePassportStatus(w, resp.GetBase(), nil)
	}
}

func meHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)

	return func(w http.ResponseWriter, r *http.Request) {
		authResult, err := authMiddleware.VerifyToken(r)
		if err != nil {
			WriteError(w, err)
			return
		}

		user := authResult.User

		// 先准备默认的关系统计，避免下游统计服务异常时整个 /me 失败。
		relationCounter := map[string]any{
			"user_id":         user.GetUserId(),
			"following_count": int64(0),
			"follower_count":  int64(0),
			"friend_count":    int64(0),
			"blocked_count":   int64(0),
		}
		contentStats := map[string]any{
			"user_id":             user.GetUserId(),
			"post_count":          int64(0),
			"received_like_count": int64(0),
		}

		// 读取关系计数，用于个人主页展示关注数和粉丝数。
		if resp, err := svcCtx.RelationRPC.GetUserCounter(r.Context(), &relationpb.GetUserCounterRequest{
			UserId: user.GetUserId(),
		}); err == nil && resp.GetBase().GetCode() == relationpb.RelationCode_RELATION_CODE_OK {
			relationCounter = relationCounterToMap(resp.GetCounter())
		}

		// 读取内容统计，用于个人主页展示帖子数和累计获赞数。
		if resp, err := svcCtx.ContentRPC.GetUserStats(r.Context(), &contentpb.GetUserStatsRequest{
			TargetUserId: user.GetUserId(),
		}); err == nil && resp.GetBase().GetCode() == contentpb.ContentCode_CONTENT_CODE_OK && resp.GetStats() != nil {
			contentStats = map[string]any{
				"user_id":             resp.GetStats().GetUserId(),
				"post_count":          resp.GetStats().GetPostCount(),
				"received_like_count": resp.GetStats().GetReceivedLikeCount(),
			}
		}

		roleCodes := []string{}
		if resp, err := svcCtx.PassportRPC.GetMyAuthorization(r.Context(), &passportpb.GetMyAuthorizationRequest{
			AccessToken: authResult.Token,
		}); err == nil && resp.GetBase().GetCode() == passportpb.PassportCode_PASSPORT_CODE_OK && resp.GetSnapshot() != nil {
			for _, role := range resp.GetSnapshot().GetRoles() {
				roleCodes = append(roleCodes, role.GetRoleCode())
			}
		}

		WriteSuccess(w, map[string]any{
			"user":             userToMap(user),
			"relation_counter": relationCounter,
			"content_stats":    contentStats,
			"role_codes":       roleCodes,
		})
	}
}

func updateMyProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	authMiddleware := NewUnifiedAuthMiddleware(svcCtx)

	return func(w http.ResponseWriter, r *http.Request) {
		var req updateProfileRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		authResult, err := authMiddleware.Authorize(r, "profile.update", 0)
		if err != nil {
			WriteError(w, err)
			return
		}

		resp, err := svcCtx.PassportRPC.UpdateMyProfile(r.Context(), &passportpb.UpdateMyProfileRequest{
			AccessToken:   authResult.Token,
			Nickname:      req.Nickname,
			Bio:           req.Bio,
			AvatarUrl:     req.AvatarURL,
			CoverImageUrl: req.CoverImageURL,
		})
		if err != nil {
			WriteError(w, ErrRPCFailed)
			return
		}
		writePassportStatus(w, resp.GetBase(), map[string]any{
			"user": userToMap(resp.GetUser()),
		})
	}
}

func userProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.read", 0)
		if !ok {
			return
		}

		targetUserID, ok := parseRequiredQueryInt64(w, r, "user_id")
		if !ok {
			return
		}

		user, exists, err := queryGatewayUserByID(svcCtx, targetUserID)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeNotFound, Message: "user not found"})
			return
		}

		relationCounter := map[string]any{
			"user_id":         user.ID,
			"following_count": int64(0),
			"follower_count":  int64(0),
			"friend_count":    int64(0),
			"blocked_count":   int64(0),
		}
		contentStats := map[string]any{
			"user_id":             user.ID,
			"post_count":          int64(0),
			"received_like_count": int64(0),
		}
		relationSnapshot := map[string]any{
			"actor_user_id":    viewer.GetUserId(),
			"target_user_id":   user.ID,
			"is_following":     false,
			"is_followed_by":   false,
			"is_mutual_follow": false,
			"is_friend":        false,
			"is_blocking":      false,
			"is_blocked_by":    false,
		}

		if resp, err := svcCtx.RelationRPC.GetUserCounter(r.Context(), &relationpb.GetUserCounterRequest{
			UserId: user.ID,
		}); err == nil && resp.GetBase().GetCode() == relationpb.RelationCode_RELATION_CODE_OK && resp.GetCounter() != nil {
			relationCounter = relationCounterToMap(resp.GetCounter())
		}

		if resp, err := svcCtx.ContentRPC.GetUserStats(r.Context(), &contentpb.GetUserStatsRequest{
			TargetUserId: user.ID,
		}); err == nil && resp.GetBase().GetCode() == contentpb.ContentCode_CONTENT_CODE_OK && resp.GetStats() != nil {
			contentStats = map[string]any{
				"user_id":             resp.GetStats().GetUserId(),
				"post_count":          resp.GetStats().GetPostCount(),
				"received_like_count": resp.GetStats().GetReceivedLikeCount(),
			}
		}

		if viewer.GetUserId() > 0 && viewer.GetUserId() != user.ID {
			if resp, err := svcCtx.RelationRPC.GetRelation(r.Context(), &relationpb.GetRelationRequest{
				ActorUserId:  viewer.GetUserId(),
				TargetUserId: user.ID,
			}); err == nil && resp.GetBase().GetCode() == relationpb.RelationCode_RELATION_CODE_OK && resp.GetSnapshot() != nil {
				relationSnapshot = relationSnapshotToMap(resp.GetSnapshot())
			}
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"user":             userEntityToMap(user),
				"relation_counter": relationCounter,
				"content_stats":    contentStats,
				"relation":         relationSnapshot,
			},
		})
	}
}

func assignUserRolesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req assignUserRolesRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		_, token, ok := authorizeBearerToken(w, r, svcCtx, "admin.user.role.assign", 0)
		if !ok {
			return
		}
		if ok := ensureReplaySafe(w, r, svcCtx, token, req.TargetUserID); !ok {
			return
		}

		resp, err := svcCtx.PassportRPC.AssignUserRoles(r.Context(), &passportpb.AssignUserRolesRequest{
			OperatorAccessToken: token,
			TargetUserId:        req.TargetUserID,
			RoleCodes:           req.RoleCodes,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writePassportStatus(w, resp.GetBase(), map[string]any{
			"roles": roleInfosToMap(resp.GetRoles()),
		})
	}
}

func verifyBearerToken(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext) (*passportpb.UserProfile, bool) {
	token, ok := bearerTokenFromRequest(w, r)
	if !ok {
		return nil, false
	}

	verifyResp, err := svcCtx.PassportRPC.VerifyAccessToken(r.Context(), &passportpb.VerifyAccessTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return nil, false
	}
	if verifyResp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !verifyResp.GetValid() {
		writePassportStatus(w, verifyResp.GetBase(), nil)
		return nil, false
	}
	return verifyResp.GetUser(), true
}

// authorizeBearerToken 统一完成 Bearer token 提取和权限校验。
func authorizeBearerToken(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, permissionCode string, resourceOwnerUserID int64) (*passportpb.UserProfile, string, bool) {
	token, ok := bearerTokenFromRequest(w, r)
	if !ok {
		return nil, "", false
	}

	resp, err := svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return nil, "", false
	}
	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !resp.GetAllowed() {
		writePassportStatus(w, resp.GetBase(), nil)
		return nil, "", false
	}
	if resp.GetSnapshot() == nil || resp.GetSnapshot().GetUser() == nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeUpstreamInvalidResponse, Message: "passport authorize missing user snapshot"})
		return nil, "", false
	}
	return resp.GetSnapshot().GetUser(), token, true
}

// authorizeBearerTokenAllowDenied 和 authorizeBearerToken 类似，但权限不足时不直接写响应。
func authorizeBearerTokenAllowDenied(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, permissionCode string, resourceOwnerUserID int64) (*passportpb.UserProfile, string, bool) {
	token, ok := bearerTokenFromRequest(w, r)
	if !ok {
		return nil, "", false
	}

	resp, err := svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return nil, "", false
	}
	if resp.GetSnapshot() == nil || resp.GetSnapshot().GetUser() == nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeUpstreamInvalidResponse, Message: "passport authorize missing user snapshot"})
		return nil, "", false
	}
	return resp.GetSnapshot().GetUser(), token, true
}

// authorizePermissionResult 返回权限判定结果，但不直接写回响应。
func authorizePermissionResult(r *http.Request, svcCtx *svc.ServiceContext, permissionCode string, resourceOwnerUserID int64) (bool, *passportpb.AuthorizeResponse, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false, nil, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	resp, err := svcCtx.PassportRPC.Authorize(r.Context(), &passportpb.AuthorizeRequest{
		AccessToken:         token,
		PermissionCode:      permissionCode,
		ResourceOwnerUserId: resourceOwnerUserID,
	})
	if err != nil {
		return false, nil, false
	}
	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK {
		return false, resp, true
	}
	return resp.GetAllowed(), resp, true
}

func bearerTokenFromRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID), Message: "缂傚搫鐨?Bearer access_token"})
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer ")), true
}

// ensureReplaySafe 对高风险写接口做时间戳和 nonce 去重。
func ensureReplaySafe(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext, accessToken string, targetUserID int64) bool {
	timestampText := strings.TrimSpace(r.Header.Get("X-Request-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-Request-Nonce"))
	if timestampText == "" || nonce == "" {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT), Message: "缂傚搫鐨?X-Request-Timestamp 閹?X-Request-Nonce"})
		return false
	}

	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT), Message: "X-Request-Timestamp 格式错误"})
		return false
	}

	skewSeconds := svcCtx.Config.Replay.SkewSeconds
	if skewSeconds <= 0 {
		skewSeconds = 300
	}
	now := time.Now().Unix()
	if timestamp < now-skewSeconds || timestamp > now+skewSeconds {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT), Message: "鐠囬攱鐪伴弮鍫曟？閹村啿鍑℃潻鍥ㄦ埂"})
		return false
	}

	ttlSeconds := svcCtx.Config.Replay.NonceTTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	scope := fmt.Sprintf("assign-role:%s:%d:%s", accessToken, targetUserID, nonce)
	ok, err := svcCtx.Replay.CheckAndStore(scope, time.Duration(ttlSeconds)*time.Second)
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return false
	}
	if !ok {
		writeJSON(w, http.StatusConflict, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_PERMISSION_DENIED), Message: "闁插秴顦茬拠閿嬬湴瀹歌尪顫﹂幏鎺旂卜"})
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT), Message: "请求 JSON 格式错误"})
		return false
	}
	return true
}

func writeSchoolStatus(w http.ResponseWriter, base *schoolpb.BaseResp, data any) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    int32(base.GetCode()),
		Message: base.GetMessage(),
		Data:    data,
	})
}

func writePassportStatus(w http.ResponseWriter, base *passportpb.BaseResp, data any) {
	writeJSON(w, http.StatusOK, jsonResponse{
		Code:    int32(base.GetCode()),
		Message: base.GetMessage(),
		Data:    data,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload jsonResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func userToMap(user *passportpb.UserProfile) map[string]any {
	return mapper.UserProfileToMap(user)
}

type gatewayUser struct {
	ID            int64
	Account       string
	Nickname      string
	AvatarURL     string
	CoverImageURL string
	Bio           string
	AccountStatus int32
	CreatedAt     time.Time
}

func userEntityToMap(user gatewayUser) map[string]any {
	return mapper.UserToMap(&mapper.UserProfile{
		UserID:        user.ID,
		Account:       user.Account,
		Nickname:      user.Nickname,
		AvatarURL:     user.AvatarURL,
		CoverImageURL: user.CoverImageURL,
		Bio:           user.Bio,
		AccountStatus: user.AccountStatus,
		CreatedAt:     user.CreatedAt,
	})
}

func queryGatewayUserByID(svcCtx *svc.ServiceContext, userID int64) (gatewayUser, bool, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || userID <= 0 {
		return gatewayUser{}, false, nil
	}
	user, ok, err := svcCtx.UserQuery.ByID(context.Background(), userID)
	return gatewayUserFromQuery(user), ok, err
}

func gatewayUserFromQuery(user userquery.User) gatewayUser {
	return gatewayUser{
		ID:            user.ID,
		Account:       user.Account,
		Nickname:      user.Nickname,
		AvatarURL:     user.AvatarURL,
		CoverImageURL: user.CoverImageURL,
		Bio:           user.Bio,
		AccountStatus: user.AccountStatus,
		CreatedAt:     user.CreatedAt,
	}
}

func tokenPairToMap(tokens *passportpb.TokenPair) map[string]any {
	if tokens == nil {
		return nil
	}
	return map[string]any{
		"access_token":  tokens.GetAccessToken(),
		"refresh_token": tokens.GetRefreshToken(),
	}
}

func toBehaviorVerifyToken(req behaviorVerifyRequest) *passportpb.BehaviorVerifyToken {
	return &passportpb.BehaviorVerifyToken{
		LotNumber:     strings.TrimSpace(req.LotNumber),
		CaptchaOutput: strings.TrimSpace(req.CaptchaOutput),
		PassToken:     strings.TrimSpace(req.PassToken),
		GenTime:       strings.TrimSpace(req.GenTime),
	}
}

func roleInfosToMap(roles []*passportpb.RoleInfo) []map[string]any {
	items := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		items = append(items, map[string]any{
			"role_id":   role.GetRoleId(),
			"role_code": role.GetRoleCode(),
			"role_name": role.GetRoleName(),
		})
	}
	return items
}

func clientIP(r *http.Request) string { // 优先读取反向代理透传的真实客户端 IP。
	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	} // 没有代理头时，从 TCP 连接地址里解析客户端 IP。
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func clientDevice(r *http.Request) string { // 设备标识优先使用 X-Device-Id。
	if deviceID := strings.TrimSpace(r.Header.Get("X-Device-Id")); deviceID != "" {
		return deviceID
	}
	if device := strings.TrimSpace(r.Header.Get("X-Device")); device != "" {
		return device
	}
	return strings.TrimSpace(r.UserAgent())
}

func extractScheduleCurrentWeek(resp any) int32 { // 兼容旧生成代码缺少 current_week getter 的情况。
	if resp == nil {
		return 0
	}

	if getter := reflect.ValueOf(resp).MethodByName("GetCurrentWeek"); getter.IsValid() {
		values := getter.Call(nil)
		if len(values) == 1 {
			if week, ok := values[0].Interface().(int32); ok {
				return week
			}
		}
	}

	value := reflect.ValueOf(resp)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.IsValid() && value.Kind() == reflect.Struct {
		field := value.FieldByName("CurrentWeek")
		if field.IsValid() && field.CanInterface() {
			switch week := field.Interface().(type) {
			case int32:
				return week
			case int64:
				return int32(week)
			case int:
				return int32(week)
			}
		}
	}
	return 0
}
