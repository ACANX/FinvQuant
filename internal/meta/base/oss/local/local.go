// Package local 提供对象存储的本地目录适配器（单测/离线环境）。
package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

func init() {
	oss.Register("local", New)
}

// Store 本地目录实现。
type Store struct {
	root   string
	prefix string
}

// New 创建本地适配器；root 为空时使用 Endpoint 或默认目录。
func New(cfg oss.Config) (oss.Store, error) {
	root := strings.TrimSpace(cfg.Endpoint)
	if root == "" {
		root = strings.TrimSpace(cfg.Bucket)
	}
	if root == "" {
		return nil, fmt.Errorf("local 适配器需要 endpoint/bucket 指定目录")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &Store{root: root, prefix: strings.Trim(strings.TrimSpace(cfg.Prefix), "/")}, nil
}

func (s *Store) keyPath(key string) (string, error) {
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if key == "" {
		return "", fmt.Errorf("key 不能为空")
	}
	if s.prefix != "" {
		key = s.prefix + "/" + key
	}
	clean := filepath.Clean(key)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return "", fmt.Errorf("非法 key: %q", key)
	}
	return filepath.Join(s.root, clean), nil
}

// Put 写入对象（临时文件 + 原子改名）。
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	path, err := s.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Get 读取对象。
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	path, err := s.keyPath(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// Stat 查询对象元数据。
func (s *Store) Stat(ctx context.Context, key string) (oss.ObjectInfo, error) {
	path, err := s.keyPath(key)
	if err != nil {
		return oss.ObjectInfo{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return oss.ObjectInfo{}, err
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return oss.ObjectInfo{}, err
	}
	return oss.ObjectInfo{
		Key:          key,
		Size:         info.Size(),
		ETag:         sum,
		LastModified: info.ModTime(),
	}, nil
}

// Delete 删除对象（不存在不报错）。
func (s *Store) Delete(ctx context.Context, key string) error {
	path, err := s.keyPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// 确保 time 被使用（ObjectInfo.LastModified 依赖）。
var _ = time.Now
