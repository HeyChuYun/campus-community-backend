package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	endpoint := requiredEnv("MINIO_ENDPOINT")
	accessKey := requiredEnv("MINIO_ACCESS_KEY")
	secretKey := requiredEnv("MINIO_SECRET_KEY")
	bucketName := envDefault("MINIO_BUCKET", "hfjj-media")
	allowListBucket := envBool("MINIO_ALLOW_LIST_BUCKET", false)
	useSSL := envBool("MINIO_USE_SSL", false)

	ctx := context.Background()

	client, err := minio.New(trimEndpoint(endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("create MinIO client failed: %v", err)
	}

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalf("check bucket failed: %v", err)
	}
	if !exists {
		log.Fatalf("bucket does not exist: %s", bucketName)
	}

	policyJSON := buildBucketPolicy(bucketName, allowListBucket)
	if err := client.SetBucketPolicy(ctx, bucketName, policyJSON); err != nil {
		log.Fatalf("set bucket policy failed: %v", err)
	}

	currentPolicy, err := client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		log.Fatalf("read bucket policy failed: %v", err)
	}

	fmt.Println("bucket policy updated:")
	fmt.Println(currentPolicy)
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func envDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("%s must be a boolean: %v", key, err)
	}
	return parsed
}

func buildBucketPolicy(bucketName string, allowListBucket bool) string {
	objectStatement := fmt.Sprintf(`{
      "Action": ["s3:GetObject"],
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Resource": ["arn:aws:s3:::%s/*"]
    }`, bucketName)

	if !allowListBucket {
		return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    %s
  ]
}`, objectStatement)
	}

	listStatement := fmt.Sprintf(`{
      "Action": ["s3:ListBucket"],
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Resource": ["arn:aws:s3:::%s"]
    }`, bucketName)

	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    %s,
    %s
  ]
}`, listStatement, objectStatement)
}

func trimEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	return endpoint
}
