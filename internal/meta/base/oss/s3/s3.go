// Package s3 提供对象存储的 S3 兼容适配器（AWS S3 / Cloudflare R2 / MinIO / B2 / OSS 等）。
//
// 使用 aws-sdk-go-v2（非 MinIO 官方库）；通过 oss.Store 抽象接入，
// 后续切换客户端或厂商只替换本适配器，不改业务代码。
package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

func init() {
	oss.Register("s3", New)
}

// Store S3 兼容实现。
type Store struct {
	client *awss3.Client
	bucket string
	prefix string
}

// New 创建 S3 适配器；endpoint 为空时使用 AWS 默认地址。
func New(cfg oss.Config) (oss.Store, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 适配器需要 bucket")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if cfg.AccessKey != "" || cfg.SecretKey != "" {
		loadOpts = append(loadOpts,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("加载 S3 配置失败: %w", err)
	}
	client := awss3.NewFromConfig(awsCfg, func(options *awss3.Options) {
		if endpoint := strings.TrimSpace(cfg.Endpoint); endpoint != "" {
			options.BaseEndpoint = aws.String(endpoint)
		}
		// R2 / MinIO 等兼容存储通常使用 path-style，避免虚拟主机域名问题
		options.UsePathStyle = true
	})
	return &Store{
		client: client,
		bucket: strings.TrimSpace(cfg.Bucket),
		prefix: strings.Trim(strings.TrimSpace(cfg.Prefix), "/"),
	}, nil
}

func (s *Store) objectKey(key string) string {
	key = strings.TrimPrefix(strings.TrimSpace(key), "/")
	if s.prefix != "" {
		return s.prefix + "/" + key
	}
	return key
}

// Put 上传对象。
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	input := &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
		Body:   body,
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("S3 PutObject 失败: %w", err)
	}
	return nil
}

// Get 下载对象。
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return nil, fmt.Errorf("S3 GetObject 失败: %w", err)
	}
	return output.Body, nil
}

// Stat 查询对象元数据。
func (s *Store) Stat(ctx context.Context, key string) (oss.ObjectInfo, error) {
	output, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return oss.ObjectInfo{}, fmt.Errorf("S3 HeadObject 失败: %w", err)
	}
	info := oss.ObjectInfo{Key: key}
	if output.ContentLength != nil {
		info.Size = *output.ContentLength
	}
	if output.ETag != nil {
		info.ETag = strings.Trim(*output.ETag, `"`)
	}
	if output.LastModified != nil {
		info.LastModified = *output.LastModified
	}
	return info, nil
}

// Delete 删除对象（不存在不报错）。
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(key)),
	})
	if err != nil {
		return fmt.Errorf("S3 DeleteObject 失败: %w", err)
	}
	return nil
}
