//go:build windows

package brs

import "errors"

// errDiskUnknown Windows 未实现剩余空间检测（生产部署为 Linux，本地开发仅提示未知）。
var errDiskUnknown = errors.New("当前平台不支持磁盘空间检测（Windows）")

// diskFree 在 Windows 上返回「未知」，不影响备份执行。
func diskFree(path string) (free, total uint64, err error) {
	return 0, 0, errDiskUnknown
}
