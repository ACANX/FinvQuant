package dedup

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/acanx/finvquant/internal/meta/finv/sys"
)

// archiveConfigStore 抽象 syscfg.Store 的读取能力（生产实现为 *syscfg.Store，测试可注入 fake）。
type archiveConfigStore interface {
	Load(ctx context.Context) (syscfg.Values, error)
}

// dynamicUploader 按 sys.sys_config 实时选择上传实现：
//
//	20002 = '1'  → GitHub Contents API（20003 PAT + 20004 {owner}:{repo}:{branch}:{pre_path_key}）
//	20002 = '0'  → 环境变量派生的兜底实现（none / dir / http），保持既有部署不变
//
// 之所以做成「路由器」而不是启动时一次性选择：运维在线把 20002 改为 '1' 后，
// 下一轮上传 Job 即按新配置执行，无需重启进程。
type dynamicUploader struct {
	store    archiveConfigStore
	fallback Uploader
	github   *githubUploader

	mu       sync.Mutex
	lastMode string
}

func newDynamicUploader(store archiveConfigStore, fallback Uploader, apiBase string) *dynamicUploader {
	return &dynamicUploader{
		store:    store,
		fallback: fallback,
		github:   newGitHubUploader(store, apiBase),
	}
}

// Mode 返回当前生效的上传模式（供 Job 判定是否需要上传、供日志展示）。
//
// 注意：开关打开但 PAT / 仓库配置不完整时仍返回 github —— 让上传以明确错误
// 失败并写入审计日志，而不是静默回退到其它目标造成「以为传了其实没传」。
func (u *dynamicUploader) Mode() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	values, err := u.store.Load(ctx)
	if err != nil {
		log.Printf("[cdeu] 读取系统配置失败，回退兜底上传模式 %s: %v", u.fallback.Mode(), err)
		mode := u.fallback.Mode()
		u.noteMode(mode)
		return mode
	}
	if values.ArchiveUploadEnabled {
		u.noteMode(UploadModeGitHub)
		return UploadModeGitHub
	}
	mode := u.fallback.Mode()
	u.noteMode(mode)
	return mode
}

// active 选择本次调用使用的上传实现。
func (u *dynamicUploader) active(ctx context.Context) (Uploader, error) {
	values, err := u.store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取系统配置失败: %w", err)
	}
	if values.ArchiveUploadEnabled {
		u.noteMode(UploadModeGitHub)
		return u.github, nil
	}
	u.noteMode(u.fallback.Mode())
	return u.fallback, nil
}

func (u *dynamicUploader) Stat(ctx context.Context, targetPath string) (bool, int64, string, error) {
	uploader, err := u.active(ctx)
	if err != nil {
		return false, 0, "", err
	}
	return uploader.Stat(ctx, targetPath)
}

func (u *dynamicUploader) Put(ctx context.Context, localPath, targetPath string) (int64, error) {
	uploader, err := u.active(ctx)
	if err != nil {
		return 0, err
	}
	return uploader.Put(ctx, localPath, targetPath)
}

func (u *dynamicUploader) Verify(ctx context.Context, targetPath string, size int64, checksum string) error {
	uploader, err := u.active(ctx)
	if err != nil {
		return err
	}
	return uploader.Verify(ctx, targetPath, size, checksum)
}

// DescribeTarget 转发给当前生效的上传实现，供 Job 过程日志展示「传到哪」。
//
// 不转发的话，外层 dynamicUploader 不实现 UploadTargetDescriber，日志只能
// 退化成「mode=github path=…」，看不清具体是哪个组织/仓库/分支。
func (u *dynamicUploader) DescribeTarget(ctx context.Context, targetPath string) (string, error) {
	uploader, err := u.active(ctx)
	if err != nil {
		return "", err
	}
	return describeUploadTarget(ctx, uploader, targetPath), nil
}

// noteMode 在模式发生切换时打一条日志，便于运维确认配置是否生效。
func (u *dynamicUploader) noteMode(mode string) {
	u.mu.Lock()
	changed := u.lastMode != mode
	u.lastMode = mode
	u.mu.Unlock()
	if changed {
		log.Printf("[cdeu] 上传模式切换为 %s", mode)
	}
}
