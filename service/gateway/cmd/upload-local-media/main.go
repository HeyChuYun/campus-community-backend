package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	envPath := flag.String("env", "../.env", "env file path")
	sourceDir := flag.String("dir", "uploads", "local media directory")
	publicRead := flag.Bool("public-read", true, "set bucket anonymous read policy")
	flag.Parse()

	env, err := readEnvFile(*envPath)
	if err != nil {
		exitf("read env: %v", err)
	}

	endpoint := env["MINIO_ENDPOINT"]
	accessKey := env["MINIO_ACCESS_KEY_ID"]
	secretKey := env["MINIO_SECRET_ACCESS_KEY"]
	bucket := env["MINIO_BUCKET"]
	useSSL := strings.EqualFold(env["MINIO_USE_SSL"], "true")
	region := env["MINIO_REGION"]
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		exitf("missing minio config, require MINIO_ENDPOINT/MINIO_ACCESS_KEY_ID/MINIO_SECRET_ACCESS_KEY/MINIO_BUCKET")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		exitf("create minio client: %v", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		exitf("check bucket %s: %v", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			exitf("create bucket %s: %v", bucket, err)
		}
		fmt.Printf("created bucket %s\n", bucket)
	}
	if *publicRead {
		if err := client.SetBucketPolicy(ctx, bucket, publicReadPolicy(bucket)); err != nil {
			exitf("set public read policy for %s: %v", bucket, err)
		}
		fmt.Printf("set bucket %s public-read policy\n", bucket)
	}

	files, err := os.ReadDir(*sourceDir)
	if err != nil {
		exitf("read source dir %s: %v", *sourceDir, err)
	}

	uploaded := 0
	for _, item := range files {
		if item.IsDir() {
			continue
		}
		name := item.Name()
		fullPath := filepath.Join(*sourceDir, name)
		info, err := item.Info()
		if err != nil {
			exitf("stat %s: %v", fullPath, err)
		}
		contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if _, err := client.FPutObject(ctx, bucket, name, fullPath, minio.PutObjectOptions{
			ContentType: contentType,
		}); err != nil {
			exitf("upload %s: %v", fullPath, err)
		}
		uploaded++
		fmt.Printf("uploaded %s size=%d content_type=%s\n", name, info.Size(), contentType)
	}

	fmt.Printf("done, uploaded=%d bucket=%s endpoint=%s\n", uploaded, bucket, endpoint)
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		result[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return result, scanner.Err()
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, bucket)
}
