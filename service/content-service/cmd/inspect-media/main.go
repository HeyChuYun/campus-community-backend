package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	envPath := flag.String("env", ".env", "env file path")
	fix := flag.Bool("fix", false, "replace old gateway media URLs with MEDIA_PUBLIC_URL")
	flag.Parse()

	env, err := readEnvFile(*envPath)
	if err != nil {
		exitf("读取 .env 失败: %v", err)
	}
	dsn := strings.TrimSpace(env["POSTGRES_DSN"])
	if dsn == "" {
		exitf("POSTGRES_DSN 为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		exitf("连接数据库失败: %v", err)
	}
	defer conn.Close(context.Background())

	if *fix {
		publicURL := strings.TrimRight(strings.TrimSpace(env["MEDIA_PUBLIC_URL"]), "/")
		if publicURL == "" {
			exitf("MEDIA_PUBLIC_URL 为空，不能修复")
		}
		replaceOldGatewayMediaURLs(ctx, conn, publicURL)
	}

	printRows(ctx, conn, "post_media", `
select url, count(*)
from post_media
group by url
order by count(*) desc, url
limit 30`)

	printRows(ctx, conn, "users avatar/cover", `
select kind, url, count(*)
from (
  select 'avatar_url' as kind, avatar_url as url from users where coalesce(avatar_url, '') <> ''
  union all
  select 'cover_image_url' as kind, cover_image_url as url from users where coalesce(cover_image_url, '') <> ''
) t
group by kind, url
order by count(*) desc, url
limit 30`)
}

func replaceOldGatewayMediaURLs(ctx context.Context, conn *pgx.Conn, publicURL string) {
	replacement := publicURL + "/"
	queries := []struct {
		name  string
		query string
	}{
		{
			name: "post_media.url",
			query: `
update post_media
set url = regexp_replace(url, '^https?://[^/]+/api/v1/media/files/', $1)
where url ~ '^https?://[^/]+/api/v1/media/files/'`,
		},
		{
			name: "users.avatar_url",
			query: `
update users
set avatar_url = regexp_replace(avatar_url, '^https?://[^/]+/api/v1/media/files/', $1)
where avatar_url ~ '^https?://[^/]+/api/v1/media/files/'`,
		},
		{
			name: "users.cover_image_url",
			query: `
update users
set cover_image_url = regexp_replace(cover_image_url, '^https?://[^/]+/api/v1/media/files/', $1)
where cover_image_url ~ '^https?://[^/]+/api/v1/media/files/'`,
		},
		{
			name: "report_evidences.file_url",
			query: `
update report_evidences
set file_url = regexp_replace(file_url, '^https?://[^/]+/api/v1/media/files/', $1)
where file_url ~ '^https?://[^/]+/api/v1/media/files/'`,
		},
	}

	for _, item := range queries {
		tag, err := conn.Exec(ctx, item.query, replacement)
		if err != nil {
			fmt.Printf("[%s] 修复失败: %v\n", item.name, err)
			continue
		}
		fmt.Printf("[%s] 已修复 %d 行\n", item.name, tag.RowsAffected())
	}
}

func printRows(ctx context.Context, conn *pgx.Conn, title string, query string) {
	rows, err := conn.Query(ctx, query)
	if err != nil {
		fmt.Printf("[%s] 查询失败: %v\n", title, err)
		return
	}
	defer rows.Close()

	fmt.Printf("[%s]\n", title)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			fmt.Printf("scan failed: %v\n", err)
			continue
		}
		fmt.Println(values...)
	}
	if err := rows.Err(); err != nil {
		fmt.Printf("rows failed: %v\n", err)
	}
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
