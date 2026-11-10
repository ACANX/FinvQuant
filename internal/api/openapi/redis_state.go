package openapi

// 防重放（nonce）与限流/配额（Redis，需求 11168）。
//
// 表里**不建** nonce 表（用户 2026-11-15 决策）：Redis `SET NX PX` 自带 TTL，
// 多实例共享，进程重启不残留。

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NonceKeyPrefix Redis 键前缀。
const (
	nonceKeyPrefix = "finv:openapi:nonce:"
	rpmKeyPrefix   = "finv:openapi:rpm:"
	quotaKeyPrefix = "finv:openapi:quota:"
)

// RedisStore Redis 侧的状态存储；rdb 为 nil 时全部检查按「通过」处理（不启用限流/防重放）。
type RedisStore struct {
	rdb *redis.Client
	now func() time.Time
}

// NewRedisStore 创建 Redis 状态存储。
func NewRedisStore(rdb *redis.Client) *RedisStore {
	return &RedisStore{rdb: rdb, now: time.Now}
}

// AcquireNonce 占用 nonce：返回 true 表示首次使用；false 表示窗口内重复（重放）。
//
// Redis 不可用时返回 (true, err)：宁可放行也不因缓存故障把外部调用方全部挡住，
// 由中间件把 err 写进审计日志细节。
func (s *RedisStore) AcquireNonce(ctx context.Context, credentialIDN int64, nonce string, keep time.Duration) (bool, error) {
	if s.rdb == nil {
		return true, nil
	}
	if nonce == "" {
		return false, nil
	}
	if keep <= 0 {
		keep = DefaultSettings.NonceKeepWindow
	}
	key := fmt.Sprintf("%s%d:%s", nonceKeyPrefix, credentialIDN, nonce)
	return s.rdb.SetNX(ctx, key, "1", keep).Result()
}

// AllowRate 判定每分钟速率与每日配额；返回 false 表示超限。
//
// 两类计数都用 `INCR` + 首次设置过期，Redis 不可用时返回 true（同上放行原则）。
func (s *RedisStore) AllowRate(ctx context.Context, clientIDN int64, rpm, quotaDay int) (bool, error) {
	if s.rdb == nil || (rpm <= 0 && quotaDay <= 0) {
		return true, nil
	}
	now := s.now()

	if rpm > 0 {
		key := fmt.Sprintf("%s%d:%d", rpmKeyPrefix, clientIDN, now.Unix()/60)
		used, err := s.incr(ctx, key, time.Minute)
		if err != nil {
			return true, err
		}
		if used > int64(rpm) {
			return false, nil
		}
	}

	if quotaDay > 0 {
		key := fmt.Sprintf("%s%d:%s", quotaKeyPrefix, clientIDN, now.Format("20060102"))
		used, err := s.incr(ctx, key, 48*time.Hour)
		if err != nil {
			return true, err
		}
		if used > int64(quotaDay) {
			return false, nil
		}
	}
	return true, nil
}

// incr 自增并在首次写入时设置 TTL。
func (s *RedisStore) incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := s.rdb.Pipeline()
	countCmd := pipe.Incr(ctx, key)
	pipe.ExpireNX(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return countCmd.Val(), nil
}
