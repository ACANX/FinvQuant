package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func trimSpace(value string) string { return strings.TrimSpace(value) }

func checksumFile(path string) string { return path + ".sha256" }

// writeChecksum 写入 `sha256sum` 兼容的校验文件。
func writeChecksum(path, checksumPath string) error {
	sum, err := fileSHA256(path)
	if err != nil {
		return err
	}
	content := fmt.Sprintf("%s  %s\n", sum, filepath.Base(path))
	return os.WriteFile(checksumPath, []byte(content), 0o640)
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
