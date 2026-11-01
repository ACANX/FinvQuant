// 本地产物清理：对象存储上传成功后，把过期的小表全量 dump 从本地删掉，
// 避免 <BackupDir>/Full/<yyyyMMdd>/ 目录每天累积、持续占用文件系统空间。
//
// 设计原则（宁可不删，不可误删）：
//   - 只清理**确认已在对象存储中存在**的文件（逐个 Stat 远端 key），远端没有就不删；
//   - 对象存储未启用 / 运行环境标识缺失时**整体跳过**（本地是唯一副本）；
//   - 只处理 Full/ 与历史根目录下形如 yyyyMMdd 的日期目录里的 *.dump，
//     不碰 Incr/ 分片目录与其它文件；
//   - 保留天数由 FINV_BRS_LOCAL_KEEP_DAYS 控制（默认 7，<=0 表示不清理）。
//
// 大表分片（Incr/）不在清理范围：分片按数据日期固定一份、重跑覆盖，
// 文件数等于分片数而非天数，不会随时间无限增长。
package brs

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

// dayDirPattern 本地产物日期目录名（yyyyMMdd）。
var dayDirPattern = regexp.MustCompile(`^\d{8}$`)

// defaultLocalKeepDays 未配置 FINV_BRS_LOCAL_KEEP_DAYS 时的默认本地保留天数。
const defaultLocalKeepDays = 7

// CleanupResult 一轮本地清理的结果。
type CleanupResult struct {
	RemovedFiles int   // 删除的文件数
	RemovedBytes int64 // 释放的字节数
	RemovedDirs  int   // 删除的空目录数
	KeptFiles    int   // 因远端不存在而保留的文件数
}

// cleanupLocalDumps 清理早于 keepDays 的本地小表全量 dump。
//
// 只会删除「远端对象存在」的文件；store 为 nil 或 env 为空时直接返回（不清理）。
func cleanupLocalDumps(ctx context.Context, opts Options, store oss.Store, env string, keepDays int, hooks Hooks) (CleanupResult, error) {
	var result CleanupResult
	if keepDays <= 0 {
		hooks.emit("info", "本地备份清理未启用（FINV_BRS_LOCAL_KEEP_DAYS=%d），保留全部本地产物", keepDays)
		return result, nil
	}
	if store == nil || env == "" {
		hooks.emit("warn", "对象存储或运行环境标识不可用，跳过本地清理（本地文件是唯一副本）")
		return result, nil
	}

	cutoff := time.Now().AddDate(0, 0, -keepDays).Format("20060102")

	// 新位置：<BackupDir>/Full/<yyyyMMdd>/；同时兼容 10915 之前的旧位置 <BackupDir>/<yyyyMMdd>/。
	roots := []string{filepath.Join(opts.BackupDir, FullDirName), opts.BackupDir}
	for _, root := range roots {
		cleanupDayDirs(ctx, root, env, cutoff, store, hooks, &result)
	}

	if result.RemovedFiles > 0 || result.KeptFiles > 0 {
		hooks.emit("info", "本地清理完成：删除 %d 个文件（释放 %s）、%d 个空目录；%d 个文件因远端不存在而保留（保留期 %d 天）",
			result.RemovedFiles, humanSize(result.RemovedBytes), result.RemovedDirs, result.KeptFiles, keepDays)
	}
	return result, nil
}

// cleanupDayDirs 清理 root 下早于 cutoff 的 yyyyMMdd 日期目录。
//
// 远端 key 的子目录段与本地一致：Full/ 下的日期目录带 `Full/`，
// 历史根目录下的产物不带（10915 之前上传的 key 没有 Full/ 段）。
func cleanupDayDirs(ctx context.Context, root, env, cutoff string, store oss.Store, hooks Hooks, result *CleanupResult) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	keyPrefix := ""
	if filepath.Base(root) == FullDirName {
		keyPrefix = FullDirName + "/"
	}

	for _, entry := range entries {
		if !entry.IsDir() || !dayDirPattern.MatchString(entry.Name()) {
			continue
		}
		day := entry.Name()
		if day >= cutoff {
			continue
		}
		dir := filepath.Join(root, day)
		files, err := os.ReadDir(dir)
		if err != nil {
			hooks.emit("warn", "读取目录失败，跳过 %s: %v", dir, err)
			continue
		}
		remaining := 0
		for _, file := range files {
			if file.IsDir() || filepath.Ext(file.Name()) != ".dump" {
				continue
			}
			path := filepath.Join(dir, file.Name())
			key := keyPrefix + day + "/" + file.Name()
			if env != "" {
				key = env + "/" + key
			}
			if _, statErr := store.Stat(ctx, key); statErr != nil {
				// 远端没有 → 本地是唯一副本，必须保留。
				result.KeptFiles++
				remaining++
				continue
			}
			info, statErr := os.Stat(path)
			var size int64
			if statErr == nil {
				size = info.Size()
			}
			if removeErr := os.Remove(path); removeErr != nil {
				hooks.emit("warn", "删除本地文件失败 %s: %v", path, removeErr)
				remaining++
				continue
			}
			result.RemovedFiles++
			result.RemovedBytes += size
		}
		if remaining == 0 {
			if removeErr := os.Remove(dir); removeErr == nil {
				result.RemovedDirs++
			}
		}
	}
}
