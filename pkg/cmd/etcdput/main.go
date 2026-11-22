package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	var (
		endpoints string
		root      string
		base      string
		verifyKey string
		verifyFile string
	)

	flag.StringVar(&endpoints, "endpoints", "", "etcd endpoints, comma separated")
	flag.StringVar(&root, "root", "/hfjj/prod", "etcd config root")
	flag.StringVar(&base, "base", "deploy/config", "config base directory")
	flag.StringVar(&verifyKey, "verify-key", "", "verify etcd key")
	flag.StringVar(&verifyFile, "verify-file", "", "verify local file path")
	flag.Parse()

	if strings.TrimSpace(endpoints) == "" {
		panic("endpoints is empty")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   splitCSV(endpoints),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	if strings.TrimSpace(verifyKey) != "" {
		if strings.TrimSpace(verifyFile) == "" {
			panic("verify-file is empty")
		}
		if err := verifyValue(cli, verifyKey, verifyFile); err != nil {
			panic(err)
		}
		fmt.Println("VERIFY OK", verifyKey)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, item := range buildPutList(root, base) {
		data, err := os.ReadFile(item.path)
		if err != nil {
			panic(err)
		}
		if _, err := cli.Put(ctx, item.key, string(data)); err != nil {
			panic(err)
		}
		fmt.Println("OK", item.key)
	}
}

func verifyValue(cli *clientv3.Client, key string, file string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, key)
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("key not found: %s", key)
	}

	local, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	remote := resp.Kvs[0].Value

	if string(local) != string(remote) {
		return fmt.Errorf("value mismatch: local=%d remote=%d", len(local), len(remote))
	}
	return nil
}

type putItem struct {
	key  string
	path string
}

func buildPutList(root string, base string) []putItem {
	root = strings.TrimRight(root, "/")
	return []putItem{
		{key: root + "/shared", path: filepath.Join(base, "shared.prod.yaml")},
		{key: root + "/services/gateway", path: filepath.Join(base, "services", "gateway.yaml")},
		{key: root + "/services/passport", path: filepath.Join(base, "services", "passport.yaml")},
		{key: root + "/services/school", path: filepath.Join(base, "services", "school.yaml")},
		{key: root + "/services/relation", path: filepath.Join(base, "services", "relation.yaml")},
		{key: root + "/services/content", path: filepath.Join(base, "services", "content.yaml")},
		{key: root + "/services/im", path: filepath.Join(base, "services", "im.yaml")},
		{key: root + "/services/update", path: filepath.Join(base, "services", "update.yaml")},
		{key: root + "/services/captcha-solver", path: filepath.Join(base, "services", "captcha-solver.yaml")},
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
