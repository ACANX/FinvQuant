package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupWritesFile 校验日志同时写入文件，并在关闭后可读取。
func TestSetupWritesFile(t *testing.T) {
	original := log.Writer()
	t.Cleanup(func() { log.SetOutput(original) })

	dir := filepath.Join(t.TempDir(), "logs")
	closer, err := Setup(dir, "test.log")
	if err != nil {
		t.Fatalf("Setup 失败: %v", err)
	}
	if closer == nil {
		t.Fatal("closer 不应为 nil")
	}

	log.Println("hello-file-log")
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatalf("读取日志失败: %v", err)
	}
	if !strings.Contains(string(content), "hello-file-log") {
		t.Fatalf("日志文件缺少内容: %q", string(content))
	}
}

// TestSetupEmptyDir 校验空目录时不做任何事（容器方式仅标准输出）。
func TestSetupEmptyDir(t *testing.T) {
	closer, err := Setup("", "")
	if err != nil {
		t.Fatalf("空目录不应报错: %v", err)
	}
	if closer != nil {
		t.Fatal("空目录应返回 nil closer")
	}
}

// TestSetupDefaultFileName 校验文件名为空时使用默认值。
func TestSetupDefaultFileName(t *testing.T) {
	original := log.Writer()
	t.Cleanup(func() { log.SetOutput(original) })

	dir := t.TempDir()
	closer, err := Setup(dir, "")
	if err != nil {
		t.Fatalf("Setup 失败: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, DefaultFile)); err != nil {
		t.Fatalf("默认日志文件不存在: %v", err)
	}
}
