// Package oss 定义对象存储（S3 兼容）抽象接口。
//
// 业务层只依赖本包的 Store 接口，不直接引用任何厂商 SDK；
// 适配器放在子包（如 oss/local、oss/s3），通过 Register/New 选择实现，
// 后续切换客户端或厂商只需新增/替换适配器，不改业务代码。
package oss

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"
)

// ObjectInfo 对象元数据。
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
}

// Store 对象存储接口（最小可用集合）。
type Store interface {
	// Put 写入对象；size 为内容长度（未知时传 -1）。
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	// Get 读取对象；调用方负责关闭返回的 ReadCloser。
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Stat 查询对象元数据。
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	// Delete 删除对象（不存在不报错）。
	Delete(ctx context.Context, key string) error
}

// Config 对象存储配置。
type Config struct {
	Provider  string // local / s3 / oss ...
	Endpoint  string
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Prefix    string // 对象 key 前缀
}

// Factory 适配器构造函数。
type Factory func(Config) (Store, error)

var factories = map[string]Factory{}

// Register 注册适配器（由子包 init 调用）。
func Register(provider string, factory Factory) {
	if provider == "" || factory == nil {
		return
	}
	factories[provider] = factory
}

// New 按 cfg.Provider 创建 Store；未注册的 provider 返回错误。
func New(cfg Config) (Store, error) {
	factory, ok := factories[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("未注册的对象存储 provider: %q（可用: %v）", cfg.Provider, Providers())
	}
	return factory(cfg)
}

// Providers 返回已注册的 provider 列表（排序）。
func Providers() []string {
	out := make([]string, 0, len(factories))
	for provider := range factories {
		out = append(out, provider)
	}
	sort.Strings(out)
	return out
}
