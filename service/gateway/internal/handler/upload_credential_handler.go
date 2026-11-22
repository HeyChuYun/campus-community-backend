package handler

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/contentpb"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const (
	uploadCredentialTTL             = 5 * time.Minute
	tempMediaExpireAfterUse         = time.Hour
	uploadCredentialMaxPerMin       = 10
	minUploadBytes            int64 = 1
	maxCredentialUploadBytes  int64 = 100 << 20

	uploadCredentialRateLimitMessage       = "upload credential requests are too frequent, please try again later"
	uploadCredentialInvalidInputMessage    = "file_name and content_type are required"
	uploadCredentialUnsupportedTypeMessage = "unsupported media type"
	uploadCredentialEmptyMediaIDMessage    = "media_id is required"
	uploadCredentialOwnershipMessage       = "media_id is invalid or does not belong to current user"
	uploadCredentialUsedMessage            = "media_id has already been used"
	uploadCredentialMissingObjectMessage   = "temporary media file is missing or empty"
)

type uploadCredentialRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
}

type uploadCompleteRequest struct {
	MediaID string `json:"media_id"`
}

func uploadCredentialHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req uploadCredentialRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok {
			return
		}
		if !allowUploadCredentialRequest(r.Context(), svcCtx, user.GetUserId()) {
			WriteError(w, ErrPermissionDenied.WithDetail("message", uploadCredentialRateLimitMessage))
			return
		}

		originalName := strings.TrimSpace(filepath.Base(req.FileName))
		contentType := strings.TrimSpace(req.ContentType)
		if originalName == "" || contentType == "" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialInvalidInputMessage))
			return
		}

		ext := strings.ToLower(filepath.Ext(originalName))
		if ext == "" {
			if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
				ext = strings.ToLower(exts[0])
			}
		}
		if !isAllowedUpload(contentType, ext) {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialUnsupportedTypeMessage))
			return
		}

		mediaID := uuid.NewString()
		objectKey := buildTempObjectKey(user.GetUserId(), mediaID, ext)
		client, bucket, err := newMediaMinIOClient(svcCtx.Config.MediaStorage)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}

		policy := minio.NewPostPolicy()
		_ = policy.SetBucket(bucket)
		_ = policy.SetKey(objectKey)
		_ = policy.SetExpires(time.Now().UTC().Add(uploadCredentialTTL))
		_ = policy.SetContentLengthRange(minUploadBytes, maxCredentialUploadBytes)
		_ = policy.SetContentType(contentType)

		uploadURL, formData, err := client.PresignedPostPolicy(r.Context(), policy)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}

		if err := upsertTempMedia(r.Context(), svcCtx.GormDB, tempMediaRecord{
			MediaID:      mediaID,
			UserID:       user.GetUserId(),
			ObjectKey:    objectKey,
			Status:       tempMediaStatusIssued,
			ExpiresAt:    time.Now().Add(uploadCredentialTTL),
			MediaType:    int32(detectMediaType(contentType, ext)),
			ContentType:  contentType,
			OriginalName: originalName,
		}); err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"media_id":   mediaID,
			"upload_url": uploadURL.String(),
			"form_data":  formData,
		})
	}
}

func uploadCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req uploadCompleteRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "content.post.create", 0)
		if !ok {
			return
		}

		mediaID := strings.TrimSpace(req.MediaID)
		if mediaID == "" {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialEmptyMediaIDMessage))
			return
		}

		items, err := listTempMediaByIDs(r.Context(), svcCtx.GormDB, []string{mediaID})
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}
		if len(items) == 0 || items[0].UserID != user.GetUserId() {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialOwnershipMessage))
			return
		}
		if items[0].Status == tempMediaStatusUsed {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialUsedMessage))
			return
		}

		client, bucket, err := newMediaMinIOClient(svcCtx.Config.MediaStorage)
		if err != nil {
			WriteError(w, ErrInternalError.WithDetail("message", err.Error()))
			return
		}
		info, err := client.StatObject(r.Context(), bucket, items[0].ObjectKey, minio.StatObjectOptions{})
		if err != nil || info.Size <= 0 {
			WriteError(w, ErrInvalidArgument.WithDetail("message", uploadCredentialMissingObjectMessage))
			return
		}

		record := items[0]
		record.Status = tempMediaStatusPending
		record.ExpiresAt = time.Now().Add(tempMediaExpireAfterUse)
		record.SizeBytes = info.Size
		if strings.TrimSpace(info.ContentType) != "" {
			record.ContentType = info.ContentType
		}
		record.MediaType = int32(detectMediaType(record.ContentType, filepath.Ext(record.ObjectKey)))
		if err := upsertTempMedia(r.Context(), svcCtx.GormDB, record); err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		previewURL, err := client.PresignedGetObject(
			r.Context(),
			bucket,
			record.ObjectKey,
			15*time.Minute,
			url.Values{},
		)
		if err != nil {
			WriteError(w, ErrInternalError)
			return
		}

		WriteSuccess(w, map[string]any{
			"success":     true,
			"preview_url": previewURL.String(),
		})
	}
}

func allowUploadCredentialRequest(ctx context.Context, svcCtx *svc.ServiceContext, userID int64) bool {
	if svcCtx == nil || svcCtx.Redis == nil || userID <= 0 {
		return false
	}
	key := "gateway:upload:credential:" + strconv.FormatInt(userID, 10) + ":" + time.Now().UTC().Format("200601021504")
	count, err := svcCtx.Redis.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if count == 1 {
		_ = svcCtx.Redis.Expire(ctx, key, time.Minute).Err()
	}
	return count <= uploadCredentialMaxPerMin
}

func buildTempObjectKey(userID int64, mediaID, ext string) string {
	trimmedExt := strings.ToLower(strings.TrimSpace(ext))
	if trimmedExt == "" {
		trimmedExt = ".bin"
	}
	return "temp/" + strconv.FormatInt(userID, 10) + "/" + mediaID + trimmedExt
}

func detectMediaType(contentType, ext string) contentpb.PostMediaType {
	switch uploadMediaKind(contentType, ext) {
	case "video":
		return contentpb.PostMediaType_POST_MEDIA_TYPE_VIDEO
	case "gif":
		return contentpb.PostMediaType_POST_MEDIA_TYPE_GIF
	default:
		return contentpb.PostMediaType_POST_MEDIA_TYPE_IMAGE
	}
}
