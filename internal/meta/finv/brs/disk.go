// 备份目录所在文件系统的剩余空间检测（磁盘水位告警）。
//
// 用途：备份产物（小表全量 dump、大表分片 MVSV-2）都落在 <BackupDir>，
// 磁盘写满会让备份静默失败或产生截断文件，因此每轮备份前后检测剩余空间，
// 低于阈值时写 warn 日志并在页面上红字提示。
//
// 平台差异：Linux/macOS 走 syscall.Statfs（见 disk_unix.go）；
// Windows 没有等价的标准库实现，返回「未知」（disk_windows.go），不影响 Linux 生产环境。
package brs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultDiskMinFreeGB 未配置 FINV_BRS_DISK_MIN_FREE_GB 时的默认阈值（GB）。
const defaultDiskMinFreeGB = 5

// DiskStatus 备份目录所在文件系统的空间状态。
type DiskStatus struct {
	Path           string `json:"disk_path"`           // 检测路径（备份目录或其最近的已存在祖先）
	AvailableBytes uint64 `json:"disk_free_bytes"`     // 可用空间（字节）
	TotalBytes     uint64 `json:"disk_total_bytes"`    // 总空间（字节）
	MinFreeBytes   uint64 `json:"disk_min_free_bytes"` // 告警阈值（字节）
	Low            bool   `json:"disk_low"`            // 是否低于阈值
	Error          string `json:"disk_error"`          // 检测失败原因（成功时为空）
}

// checkDisk 检测 path 所在文件系统的剩余空间，并与 minFreeGB 比较。
//
// path 不存在时沿父目录向上找最近的已存在目录（<BackupDir> 可能尚未创建）。
// minFreeGB <= 0 表示关闭告警（仍返回剩余空间，Low 恒为 false）。
func checkDisk(path string, minFreeGB int) DiskStatus {
	status := DiskStatus{Path: path}
	if minFreeGB <= 0 {
		minFreeGB = 0
	}
	status.MinFreeBytes = uint64(minFreeGB) * 1024 * 1024 * 1024

	target := existingAncestor(path)
	status.Path = target
	free, total, err := diskFree(target)
	if err != nil {
		status.Error = fmt.Sprintf("检测磁盘剩余空间失败（%s）: %v", target, err)
		return status
	}
	status.AvailableBytes = free
	status.TotalBytes = total
	status.Low = status.MinFreeBytes > 0 && free < status.MinFreeBytes
	return status
}

// existingAncestor 返回 path 或其最近的已存在祖先目录。
func existingAncestor(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return os.TempDir()
	}
	current := filepath.Clean(trimmed)
	for {
		if info, err := os.Stat(current); err == nil && info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return os.TempDir()
		}
		current = parent
	}
}

// lowDiskMessage 生成告警文案（低于阈值时使用）。
func lowDiskMessage(status DiskStatus) string {
	return fmt.Sprintf("磁盘剩余空间偏低：%s 可用 %s，低于阈值 %s；请清理旧备份或扩容（可用 FINV_BRS_LOCAL_KEEP_DAYS 调整本地保留天数）",
		status.Path, humanSize(int64(status.AvailableBytes)), humanSize(int64(status.MinFreeBytes)))
}
