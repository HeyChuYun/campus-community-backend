package handler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/contentpb"

	"github.com/minio/minio-go/v7"
	"gorm.io/gorm"
)

type preparedPostMedia struct {
	Client      *minio.Client
	Bucket      string
	Media       []*contentpb.PostMedia
	TempRecords []tempMediaRecord
	FinalKeys   []string
}

func preparePostMediaForCreate(ctx context.Context, svcCtx *svc.ServiceContext, userID int64, mediaIDs []string) (preparedPostMedia, error) {
	if len(mediaIDs) == 0 {
		return preparedPostMedia{}, nil
	}
	if svcCtx == nil || svcCtx.GormDB == nil {
		return preparedPostMedia{}, fmt.Errorf("gateway database is unavailable")
	}

	records, err := listTempMediaByIDs(ctx, svcCtx.GormDB, mediaIDs)
	if err != nil {
		return preparedPostMedia{}, err
	}
	recordMap := make(map[string]tempMediaRecord, len(records))
	for _, record := range records {
		recordMap[record.MediaID] = record
	}

	client, bucket, err := newMediaMinIOClient(svcCtx.Config.MediaStorage)
	if err != nil {
		return preparedPostMedia{}, err
	}

	prepared := preparedPostMedia{
		Client:      client,
		Bucket:      bucket,
		Media:       make([]*contentpb.PostMedia, 0, len(mediaIDs)),
		TempRecords: make([]tempMediaRecord, 0, len(mediaIDs)),
		FinalKeys:   make([]string, 0, len(mediaIDs)),
	}
	now := time.Now()
	seenMediaIDs := make(map[string]struct{}, len(mediaIDs))

	for index, mediaID := range mediaIDs {
		trimmedMediaID := strings.TrimSpace(mediaID)
		if trimmedMediaID == "" {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id cannot be empty")
		}
		if _, ok := seenMediaIDs[trimmedMediaID]; ok {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id %s is duplicated", trimmedMediaID)
		}
		seenMediaIDs[trimmedMediaID] = struct{}{}

		record, ok := recordMap[trimmedMediaID]
		if !ok {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id %s does not exist", trimmedMediaID)
		}
		if record.UserID != userID {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id %s does not belong to current user", trimmedMediaID)
		}
		if record.Status != tempMediaStatusPending {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id %s has not completed upload confirmation", trimmedMediaID)
		}
		if !record.ExpiresAt.IsZero() && record.ExpiresAt.Before(now) {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("media_id %s is expired", trimmedMediaID)
		}

		info, err := client.StatObject(ctx, bucket, record.ObjectKey, minio.StatObjectOptions{})
		if err != nil || info.Size <= 0 {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("temporary object for media_id %s is missing", trimmedMediaID)
		}

		finalKey := buildPublishedObjectKey(userID, record.MediaID, filepath.Ext(record.ObjectKey))
		exists, err := objectExists(ctx, client, bucket, finalKey)
		if err != nil {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, err
		}
		if exists {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("target media object already exists")
		}

		src := minio.CopySrcOptions{Bucket: bucket, Object: record.ObjectKey}
		dst := minio.CopyDestOptions{Bucket: bucket, Object: finalKey}
		if _, err := client.CopyObject(ctx, dst, src); err != nil {
			cleanupPreparedPostMedia(ctx, prepared)
			return preparedPostMedia{}, fmt.Errorf("copy media object failed: %w", err)
		}

		prepared.Media = append(prepared.Media, &contentpb.PostMedia{
			MediaType:  contentpb.PostMediaType(record.MediaType),
			StorageKey: finalKey,
			Url:        minIOPublicURL(svcCtx.Config.MediaStorage, bucket, finalKey),
			SizeBytes:  info.Size,
			SortOrder:  int64(index),
		})
		prepared.TempRecords = append(prepared.TempRecords, record)
		prepared.FinalKeys = append(prepared.FinalKeys, finalKey)
	}

	return prepared, nil
}

func finalizePreparedPostMedia(ctx context.Context, svcCtx *svc.ServiceContext, prepared preparedPostMedia, postID int64) error {
	if len(prepared.TempRecords) == 0 {
		return nil
	}
	mediaIDs := make([]string, 0, len(prepared.TempRecords))
	for _, record := range prepared.TempRecords {
		mediaIDs = append(mediaIDs, record.MediaID)
	}
	if err := svcCtx.GormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return markTempMediaUsed(ctx, tx, mediaIDs, postID)
	}); err != nil {
		return err
	}
	for _, record := range prepared.TempRecords {
		_ = prepared.Client.RemoveObject(ctx, prepared.Bucket, record.ObjectKey, minio.RemoveObjectOptions{})
	}
	return nil
}

func cleanupPreparedPostMedia(ctx context.Context, prepared preparedPostMedia) {
	if prepared.Client == nil || prepared.Bucket == "" {
		return
	}
	for _, objectKey := range prepared.FinalKeys {
		_ = prepared.Client.RemoveObject(ctx, prepared.Bucket, objectKey, minio.RemoveObjectOptions{})
	}
}

func buildPublishedObjectKey(userID int64, mediaID, ext string) string {
	trimmedExt := strings.ToLower(strings.TrimSpace(ext))
	if trimmedExt == "" {
		trimmedExt = ".bin"
	}
	return fmt.Sprintf("posts/%d/%s%s", userID, mediaID, trimmedExt)
}

func objectExists(ctx context.Context, client *minio.Client, bucket, objectKey string) (bool, error) {
	if client == nil || bucket == "" || objectKey == "" {
		return false, fmt.Errorf("invalid minio parameters")
	}
	_, err := client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	response := minio.ToErrorResponse(err)
	if response.Code == "NoSuchKey" || response.Code == "NoSuchObject" || response.StatusCode == 404 {
		return false, nil
	}
	return false, err
}
