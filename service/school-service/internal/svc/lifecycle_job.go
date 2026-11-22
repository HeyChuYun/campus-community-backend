package svc

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

// StartLifecycleJob 启动学生状态纠偏任务。
// 这个任务不会请求学校系统，只根据 school_accounts 中的 admission_at / graduation_at 修正 student_status。
func StartLifecycleJob(svcCtx *ServiceContext) {
	if svcCtx == nil || !svcCtx.Config.LifecycleJob.Enabled {
		return
	}

	interval := svcCtx.Config.LifecycleJob.Interval
	if interval <= 0 {
		interval = 12 * time.Hour
	}

	lockTTL := svcCtx.Config.LifecycleJob.LockTTL
	if lockTTL <= 0 {
		lockTTL = 10 * time.Minute
	}

	startDelay := svcCtx.Config.LifecycleJob.StartDelay
	if startDelay < 0 {
		startDelay = 0
	}

	lockKey := strings.TrimSpace(svcCtx.Config.LifecycleJob.LockKey)
	if lockKey == "" {
		lockKey = svcCtx.RedisPrefix + "lock:lifecycle-reconcile"
	}

	go func() {
		if startDelay > 0 {
			time.Sleep(startDelay)
		}

		runLifecycleReconcile(svcCtx, lockKey, lockTTL)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			runLifecycleReconcile(svcCtx, lockKey, lockTTL)
		}
	}()
}

// runLifecycleReconcile 执行一次学生状态纠偏。
func runLifecycleReconcile(svcCtx *ServiceContext, lockKey string, lockTTL time.Duration) {
	if svcCtx == nil {
		return
	}

	release, ok, err := tryAcquireLifecycleLock(svcCtx.Redis, lockKey, lockTTL)
	if err != nil {
		logx.Errorf("acquire lifecycle reconcile lock failed: %v", err)
		return
	}
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	changed, err := svcCtx.SchoolAccountStore.ReconcileStudentStatuses(context.Background(), time.Now())
	if err != nil {
		logx.Errorf("reconcile student statuses failed: %v", err)
		return
	}
	if changed > 0 {
		logx.Infof("reconciled student statuses, changed=%d", changed)
	}
}

// tryAcquireLifecycleLock 用 Redis 锁避免多实例重复执行。
// 如果当前未配置 Redis，则退化为单进程直接执行。
func tryAcquireLifecycleLock(client *redis.Client, lockKey string, lockTTL time.Duration) (func(), bool, error) {
	if client == nil {
		return nil, true, nil
	}

	ctx := context.Background()
	token := time.Now().Format(time.RFC3339Nano)
	ok, err := client.SetNX(ctx, lockKey, token, lockTTL).Result()
	if err != nil || !ok {
		return nil, ok, err
	}

	return func() {
		currentToken, getErr := client.Get(ctx, lockKey).Result()
		if getErr == nil && currentToken == token {
			_ = client.Del(ctx, lockKey).Err()
		}
	}, true, nil
}
