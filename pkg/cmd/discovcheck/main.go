package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/zrpc"
)

func main() {
	var (
		endpoints string
		key       string
		seconds   int
	)

	flag.StringVar(&endpoints, "endpoints", "", "etcd 地址，多个用逗号分隔")
	flag.StringVar(&key, "key", "", "要订阅的 etcd 服务发现 key")
	flag.IntVar(&seconds, "seconds", 5, "持续观察秒数")
	flag.Parse()

	if strings.TrimSpace(endpoints) == "" {
		panic("endpoints is empty")
	}
	if strings.TrimSpace(key) == "" {
		panic("key is empty")
	}

	hosts := splitCSV(endpoints)
	// 同时打印 go-zero 最终构造的 target，确认 gateway 侧的目标字符串与预期一致。
	conf := zrpc.NewEtcdClientConf(hosts, key, "", "")
	target, err := conf.BuildTarget()
	if err != nil {
		panic(err)
	}
	fmt.Println("discov target:", target)

	sub, err := discov.NewSubscriber(hosts, key)
	if err != nil {
		panic(err)
	}
	defer sub.Close()

	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for {
		values := sub.Values()
		fmt.Printf("%s values=%q\n", time.Now().Format(time.RFC3339), values)
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
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
