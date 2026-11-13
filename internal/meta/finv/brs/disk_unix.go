//go:build !windows

package brs

import "syscall"

// diskFree 返回 path 所在文件系统的可用字节数与总字节数（Linux/macOS）。
func diskFree(path string) (free, total uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	blockSize := uint64(stat.Bsize)
	return stat.Bavail * blockSize, stat.Blocks * blockSize, nil
}
