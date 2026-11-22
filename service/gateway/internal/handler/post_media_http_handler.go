package handler

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	defaultDailyPostLimit   int64 = 3
	defaultMaxImagesPerPost int64 = 4
	defaultMaxVideosPerPost int64 = 1
	postImageMediaType      int32 = 1
	postVideoMediaType      int32 = 2
)

type postVideoInitRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
}

type postVideoMergeRequest struct {
	MediaID string `json:"media_id"`
}

func uploadAvatarHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return profileMediaUploadHandler(svcCtx, "avatar")
}

func uploadCoverHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return profileMediaUploadHandler(svcCtx, "cover")
}

func profileMediaUploadHandler(svcCtx *svc.ServiceContext, scene string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "profile.update", 0)
		if !ok || user == nil {
			return
		}
		file, header, contentType, ext, ok := parseUploadFile(w, r, 20<<20)
		if !ok {
			return
		}
		defer file.Close()
		if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "only image upload is allowed"))
			return
		}

		key, publicURL, err := uploadToMinIO(r.Context(), svcCtx.Config.MediaStorage, file, header.Size, contentType, ext)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		WriteSuccess(w, map[string]any{
			"scene":       scene,
			"storage_key": key,
			"url":         publicURL,
			"size_bytes":  header.Size,
		})
	}
}

func uploadPostImageHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok || user == nil {
			return
		}
		if err := validatePostMediaUploadAllowed(r.Context(), svcCtx, user.GetUserId(), "image"); err != nil {
			WriteError(w, ErrPermissionDenied.WithDetail("message", err.Error()))
			return
		}
		file, header, contentType, ext, ok := parseUploadFile(w, r, maxUploadBytes)
		if !ok {
			return
		}
		defer file.Close()
		if uploadMediaKind(contentType, ext) == "video" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "use video upload endpoints for video files"))
			return
		}

		key, publicURL, err := uploadToMinIO(r.Context(), svcCtx.Config.MediaStorage, file, header.Size, contentType, ext)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		mediaID := uuid.NewString()
		if err := upsertTempMedia(r.Context(), svcCtx.GormDB, tempMediaRecord{
			MediaID:      mediaID,
			UserID:       user.GetUserId(),
			ObjectKey:    key,
			Status:       tempMediaStatusPending,
			ExpiresAt:    time.Now().Add(tempMediaExpireAfterUse),
			MediaType:    postImageMediaType,
			ContentType:  contentType,
			OriginalName: filepath.Base(header.Filename),
			SizeBytes:    header.Size,
		}); err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		increaseDailyUploadCounter(r.Context(), svcCtx, user.GetUserId(), "image")
		WriteSuccess(w, map[string]any{
			"media_id":     mediaID,
			"storage_key":  key,
			"url":          publicURL,
			"size_bytes":   header.Size,
			"content_type": contentType,
		})
	}
}

func initPostVideoUploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postVideoInitRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok {
			return
		}
		if err := validatePostMediaUploadAllowed(r.Context(), svcCtx, user.GetUserId(), "video"); err != nil {
			WriteError(w, ErrPermissionDenied.WithDetail("message", err.Error()))
			return
		}
		fileName := strings.TrimSpace(filepath.Base(req.FileName))
		contentType := strings.TrimSpace(req.ContentType)
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext == "" {
			if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
				ext = strings.ToLower(exts[0])
			}
		}
		if uploadMediaKind(contentType, ext) != "video" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "invalid video content type"))
			return
		}
		mediaID := uuid.NewString()
		objectKey := buildTempObjectKey(user.GetUserId(), mediaID, ext)
		if err := upsertTempMedia(r.Context(), svcCtx.GormDB, tempMediaRecord{
			MediaID:      mediaID,
			UserID:       user.GetUserId(),
			ObjectKey:    objectKey,
			Status:       tempMediaStatusIssued,
			ExpiresAt:    time.Now().Add(uploadCredentialTTL),
			MediaType:    postVideoMediaType,
			ContentType:  contentType,
			OriginalName: fileName,
		}); err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		WriteSuccess(w, map[string]any{
			"media_id":  mediaID,
			"upload_id": mediaID,
		})
	}
}

func uploadPostVideoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok || user == nil {
			return
		}
		mediaID := strings.TrimSpace(r.FormValue("media_id"))
		if mediaID == "" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "media_id is required"))
			return
		}
		if err := validatePostMediaUploadAllowed(r.Context(), svcCtx, user.GetUserId(), "video"); err != nil {
			WriteError(w, ErrPermissionDenied.WithDetail("message", err.Error()))
			return
		}
		items, err := listTempMediaByIDs(r.Context(), svcCtx.GormDB, []string{mediaID})
		if err != nil || len(items) == 0 || items[0].UserID != user.GetUserId() {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "media_id is invalid"))
			return
		}
		if items[0].Status != tempMediaStatusIssued {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "video upload state is invalid"))
			return
		}
		file, header, contentType, _, ok := parseUploadFile(w, r, maxUploadBytes)
		if !ok {
			return
		}
		defer file.Close()
		if uploadMediaKind(contentType, filepath.Ext(items[0].ObjectKey)) != "video" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "only video upload is allowed"))
			return
		}
		client, bucket, err := newMediaMinIOClient(svcCtx.Config.MediaStorage)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		if _, err := client.PutObject(r.Context(), bucket, items[0].ObjectKey, file, header.Size, minio.PutObjectOptions{
			ContentType: contentType,
		}); err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		WriteSuccess(w, map[string]any{
			"media_id":     mediaID,
			"uploaded":     true,
			"content_type": contentType,
			"size_bytes":   header.Size,
		})
	}
}

func mergePostVideoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req postVideoMergeRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok {
			return
		}
		items, err := listTempMediaByIDs(r.Context(), svcCtx.GormDB, []string{strings.TrimSpace(req.MediaID)})
		if err != nil || len(items) == 0 || items[0].UserID != user.GetUserId() {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "media_id is invalid"))
			return
		}
		client, bucket, err := newMediaMinIOClient(svcCtx.Config.MediaStorage)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		info, err := client.StatObject(r.Context(), bucket, items[0].ObjectKey, minio.StatObjectOptions{})
		if err != nil || info.Size <= 0 {
			WriteError(w, ErrInvalidArgument.WithDetail("message", "video file is missing"))
			return
		}
		record := items[0]
		record.Status = tempMediaStatusPending
		record.ExpiresAt = time.Now().Add(tempMediaExpireAfterUse)
		record.SizeBytes = info.Size
		if strings.TrimSpace(info.ContentType) != "" {
			record.ContentType = info.ContentType
		}
		if err := upsertTempMedia(r.Context(), svcCtx.GormDB, record); err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		increaseDailyUploadCounter(r.Context(), svcCtx, user.GetUserId(), "video")
		WriteSuccess(w, map[string]any{
			"media_id": req.MediaID,
			"merged":   true,
			"url":      minIOPublicURL(svcCtx.Config.MediaStorage, bucket, record.ObjectKey),
		})
	}
}

func parseUploadFile(w http.ResponseWriter, r *http.Request, maxBytes int64) (multipart.File, *multipart.FileHeader, string, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "invalid multipart upload"})
		return nil, nil, "", "", false
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "missing file"})
		return nil, nil, "", "", false
	}
	ct := strings.TrimSpace(hdr.Header.Get("Content-Type"))
	extension := strings.ToLower(filepath.Ext(filepath.Base(hdr.Filename)))
	if ct == "" {
		ct = mime.TypeByExtension(extension)
	}
	if !isAllowedUpload(ct, extension) {
		_ = file.Close()
		writeJSON(w, http.StatusBadRequest, jsonResponse{Code: gatewayCodeInvalidArgument, Message: "unsupported media type"})
		return nil, nil, "", "", false
	}
	return file, hdr, ct, extension, true
}

func validatePostMediaUploadAllowed(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, mediaType string) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user")
	}
	user, ok, err := queryGatewayUserByID(svcCtx, userID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user not found")
	}
	if user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_BLOCKED) ||
		user.AccountStatus == int32(passportpb.AccountStatus_ACCOUNT_STATUS_DEACTIVATED) {
		return fmt.Errorf("current account is blocked")
	}
	blocked, until, err := hasActivePostRestriction(ctx, svcCtx, userID)
	if err != nil {
		return err
	}
	if blocked {
		if until != nil {
			return fmt.Errorf("posting is restricted until %s", until.Local().Format("2006-01-02 15:04:05"))
		}
		return fmt.Errorf("current account is restricted from posting")
	}
	if exceeded, limit, used, err := exceedsDailyPostMediaQuota(ctx, svcCtx, userID, mediaType); err != nil {
		return err
	} else if exceeded {
		return fmt.Errorf("daily %s upload quota reached (%d/%d)", mediaType, used, limit)
	}
	return nil
}

func hasActivePostRestriction(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) (bool, *time.Time, error) {
	type row struct {
		EndsAt *time.Time `gorm:"column:ends_at"`
	}
	var item row
	now := time.Now()
	err := svcCtx.GormDB.WithContext(ctx).
		Table("user_restrictions").
		Select("ends_at").
		Where("user_id = ? AND scope = ? AND status = ?", userID, "post_publish", "active").
		Where("starts_at <= ?", now).
		Where("ends_at IS NULL OR ends_at > ?", now).
		Order("ends_at DESC NULLS LAST, id DESC").
		Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	if item.EndsAt != nil {
		return true, item.EndsAt, nil
	}
	return true, nil, nil
}

func exceedsDailyPostMediaQuota(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, mediaType string) (bool, int64, int64, error) {
	policy := svcCtx.Commercial.MembershipPolicy(ctx, userID)
	dailyPosts := defaultDailyPostLimit + policy.DailyPostBonus
	if dailyPosts <= 0 {
		dailyPosts = defaultDailyPostLimit
	}
	imagePerPost := defaultMaxImagesPerPost + policy.ImageCountBonus
	if imagePerPost <= 0 {
		imagePerPost = defaultMaxImagesPerPost
	}
	videoPerPost := defaultMaxVideosPerPost

	var limit int64
	switch mediaType {
	case "video":
		limit = dailyPosts * videoPerPost
	default:
		limit = dailyPosts * imagePerPost
	}
	key := "gateway:post-media:" + mediaType + ":" + strconv.FormatInt(userID, 10) + ":" + time.Now().Format("20060102")
	used, err := svcCtx.Redis.Get(ctx, key).Int64()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, 0, 0, err
	}
	return used >= limit, limit, used, nil
}

func increaseDailyUploadCounter(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, mediaType string) {
	if svcCtx == nil || svcCtx.Redis == nil {
		return
	}
	key := "gateway:post-media:" + mediaType + ":" + strconv.FormatInt(userID, 10) + ":" + time.Now().Format("20060102")
	count, err := svcCtx.Redis.Incr(ctx, key).Result()
	if err == nil && count == 1 {
		_ = svcCtx.Redis.Expire(ctx, key, 24*time.Hour).Err()
	}
}
