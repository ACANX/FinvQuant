package brs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCheckDiskReportsFreeSpace 校验剩余空间与阈值判定（Windows 平台跳过：无检测实现）。
func TestCheckDiskReportsFreeSpace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 未实现磁盘空间检测")
	}
	dir := t.TempDir()
	status := checkDisk(dir, 1)
	if status.Error != "" {
		t.Fatalf("检测失败: %s", status.Error)
	}
	if status.AvailableBytes == 0 || status.TotalBytes == 0 {
		t.Fatalf("空间数据异常: %+v", status)
	}
	if status.AvailableBytes > status.TotalBytes {
		t.Fatalf("可用空间不应大于总空间: %+v", status)
	}
	if status.MinFreeBytes != 1*1024*1024*1024 {
		t.Fatalf("阈值换算错误: %d", status.MinFreeBytes)
	}
	// 阈值 10 TB → 必然低于阈值，Low 应为 true（验证判定方向）。
	huge := checkDisk(dir, 10*1024)
	if huge.Error != "" {
		t.Fatalf("检测失败: %s", huge.Error)
	}
	if !huge.Low {
		t.Fatalf("阈值远大于可用空间时应判定为低水位: %+v", huge)
	}
}

// TestCheckDiskDisabled 校验阈值 <=0 时关闭告警（Low 恒为 false）。
func TestCheckDiskDisabled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 未实现磁盘空间检测")
	}
	dir := t.TempDir()
	status := checkDisk(dir, 0)
	if status.Error != "" {
		t.Fatalf("检测失败: %s", status.Error)
	}
	if status.Low {
		t.Fatalf("阈值 0 时不应告警: %+v", status)
	}
	if status.MinFreeBytes != 0 {
		t.Fatalf("阈值 0 时 MinFreeBytes 应为 0: %d", status.MinFreeBytes)
	}
}

// TestCheckDiskUsesExistingAncestor 校验目录尚不存在时沿父目录检测。
func TestCheckDiskUsesExistingAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 未实现磁盘空间检测")
	}
	root := t.TempDir()
	missing := filepath.Join(root, "not", "created", "yet")
	status := checkDisk(missing, 1)
	if status.Error != "" {
		t.Fatalf("检测失败: %s", status.Error)
	}
	if status.Path != root {
		t.Fatalf("应回落到最近的已存在祖先 %s，实际 %s", root, status.Path)
	}
}

// TestExistingAncestorFallback 校验空路径回落到临时目录。
func TestExistingAncestorFallback(t *testing.T) {
	if got := existingAncestor("   "); got != os.TempDir() {
		t.Fatalf("空路径应回落到临时目录，实际 %s", got)
	}
	root := string(filepath.Separator)
	if got := existingAncestor(root); got != root {
		t.Fatalf("根目录应原样返回，实际 %s", got)
	}
}
