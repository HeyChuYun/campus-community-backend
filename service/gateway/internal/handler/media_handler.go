package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/commercial"
	"hfjj/backend/gateway/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const maxUploadBytes = 320 << 20

func uploadToMinIO(ctx context.Context, storage config.MediaStorageConfig, file io.Reader, size int64, contentType, ext string) (string, string, error) {
	client, bucket, err := newMediaMinIOClient(storage)
	if err != nil {
		return "", "", errInvalidMinIOConfig
	}
	cfg := storage.MinIO

	if cfg.AutoCreateBucket {
		exists, err := client.BucketExists(ctx, bucket)
		if err != nil {
			return "", "", err
		}
		if !exists {
			if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: strings.TrimSpace(cfg.Region)}); err != nil {
				return "", "", err
			}
		}
	}

	key := newMinIOObjectKey(cfg.ObjectPrefix, ext)
	_, err = client.PutObject(ctx, bucket, key, file, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", "", err
	}
	return key, minIOPublicURL(storage, bucket, key), nil
}

func newMediaMinIOClient(storage config.MediaStorageConfig) (*minio.Client, string, error) {
	cfg := storage.MinIO
	endpoint := strings.TrimSpace(cfg.Endpoint)
	bucket := strings.TrimSpace(cfg.Bucket)
	if endpoint == "" || bucket == "" || strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return nil, "", errInvalidMinIOConfig
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: strings.TrimSpace(cfg.Region),
	})
	if err != nil {
		return nil, "", err
	}
	return client, bucket, nil
}

var errInvalidMinIOConfig = &mediaStorageError{message: "invalid minio media storage config"}

type mediaStorageError struct {
	message string
}

func (e *mediaStorageError) Error() string {
	return e.message
}

func newMinIOObjectKey(prefix, ext string) string {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		prefix = "media"
	}
	return path.Join(prefix, time.Now().UTC().Format("2006/01/02"), newUploadFileName(ext))
}

func minIOPublicURL(storage config.MediaStorageConfig, bucket, key string) string {
	if base := strings.TrimSpace(storage.PublicURL); base != "" {
		return strings.TrimRight(base, "/") + "/" + pathEscapeObjectKey(key)
	}
	cfg := storage.MinIO
	if base := strings.TrimSpace(cfg.Endpoint); base != "" {
		scheme := "http"
		if cfg.UseSSL {
			scheme = "https"
		}
		if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
			return strings.TrimRight(base, "/") + "/" + bucket + "/" + pathEscapeObjectKey(key)
		}
		return scheme + "://" + strings.TrimRight(base, "/") + "/" + bucket + "/" + pathEscapeObjectKey(key)
	}
	return "/" + bucket + "/" + pathEscapeObjectKey(key)
}

func pathEscapeObjectKey(key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func newUploadFileName(ext string) string {
	var randomBytes [12]byte
	_, _ = rand.Read(randomBytes[:])
	if ext == "" {
		ext = ".bin"
	}
	return time.Now().UTC().Format("20060102150405") + "_" + hex.EncodeToString(randomBytes[:]) + ext
}

func isAllowedUpload(contentType, ext string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	ext = strings.ToLower(strings.TrimSpace(ext))
	if strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "video/") {
		return true
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".mp4", ".mov", ".m4v", ".webm", ".avi", ".mkv":
		return true
	default:
		return false
	}
}

func uploadMediaKind(contentType, ext string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	ext = strings.ToLower(strings.TrimSpace(ext))
	if strings.HasPrefix(contentType, "video/") {
		return "video"
	}
	if strings.Contains(contentType, "gif") || ext == ".gif" {
		return "gif"
	}
	if strings.HasPrefix(contentType, "image/") {
		return "image"
	}
	switch ext {
	case ".mp4", ".mov", ".m4v", ".webm", ".avi", ".mkv":
		return "video"
	case ".gif":
		return "gif"
	default:
		return "image"
	}
}

func uploadSizeLimitBytes(policy commercial.MembershipPolicy, mediaType string) int64 {
	switch mediaType {
	case "video":
		return envInt64("CONTENT_MAX_VIDEO_BYTES", 60<<20) + policy.VideoBytesBonus
	case "gif":
		return envInt64("CONTENT_MAX_GIF_BYTES", 60<<20)
	default:
		return envInt64("CONTENT_MAX_SINGLE_IMAGE_BYTES", 5<<20) + policy.SingleImageBytesBonus
	}
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
