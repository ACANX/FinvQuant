package logging

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSetupWritesFile 校验日志同时写入文件，并在关闭后可读取。
func TestSetupWritesFile(t *testing.T) {
	original := log.Writer()
	t.Cleanup(func() { log.SetOutput(original) })

	dir := filepath.Join(t.TempDir(), "log")
	closer, err := Setup(dir, "test.log", 0)
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
	closer, err := Setup("", "", 0)
	if err != nil {
		t.Fatalf("空目录不应报错: %v", err)
	}
	if closer != nil {
		t.Fatal("空目录应返回 nil closer")
	}
}

// TestDefaultFileName 校验默认活动文件名为 FinvQuant.log。
func TestDefaultFileName(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewWriter(Options{Dir: dir})
	if err != nil {
		t.Fatalf("NewWriter 失败: %v", err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, DefaultFile)); err != nil {
		t.Fatalf("默认日志文件不存在: %v", err)
	}
}

// TestRotateBySize 校验超过上限后切分为 FinvQuant-yyyyMMdd-NN.log（NN 从 01）。
func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 28, 9, 0, 0, 0, time.Local)
	writer, err := NewWriter(Options{
		Dir: dir, File: "FinvQuant.log", MaxBytes: 100,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewWriter 失败: %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte(strings.Repeat("a", 60))); err != nil {
		t.Fatalf("第一次写入失败: %v", err)
	}
	if _, err := writer.Write([]byte(strings.Repeat("b", 60))); err != nil {
		t.Fatalf("第二次写入失败: %v", err)
	}

	split := filepath.Join(dir, "FinvQuant-20261028-01.log")
	content, err := os.ReadFile(split)
	if err != nil {
		t.Fatalf("切分文件不存在: %v", err)
	}
	if len(content) != 60 || !strings.HasPrefix(string(content), "a") {
		t.Fatalf("切分文件内容错误: len=%d", len(content))
	}
	active, err := os.ReadFile(filepath.Join(dir, "FinvQuant.log"))
	if err != nil {
		t.Fatalf("活动文件不存在: %v", err)
	}
	if len(active) != 60 || !strings.HasPrefix(string(active), "b") {
		t.Fatalf("活动文件内容错误: len=%d", len(active))
	}
}

// TestArchiveOnNextDay 校验跨日把前一天切分文件打包为 tar.gz 并删除源文件。
func TestArchiveOnNextDay(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 28, 23, 59, 0, 0, time.Local)
	writer, err := NewWriter(Options{
		Dir: dir, File: "FinvQuant.log", MaxBytes: 10,
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewWriter 失败: %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte("day-one-entry\n")); err != nil {
		t.Fatalf("day1 写入失败: %v", err)
	}
	// 切分一次，确保 day1 存在 -01 文件。
	if _, err := writer.Write([]byte("day-one-rotate\n")); err != nil {
		t.Fatalf("day1 第二次写入失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "FinvQuant-20261028-01.log")); err != nil {
		t.Fatalf("day1 切分文件不存在: %v", err)
	}

	now = time.Date(2026, 10, 29, 0, 1, 0, 0, time.Local)
	if _, err := writer.Write([]byte("day-two-entry\n")); err != nil {
		t.Fatalf("day2 写入失败: %v", err)
	}

	archive := filepath.Join(dir, "FinvQuant-20261028.tar.gz")
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("按日归档不存在: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "FinvQuant-20261028-") {
			t.Fatalf("归档后不应保留切分文件: %s", entry.Name())
		}
	}

	// 校验 tar.gz 内至少包含一个切分文件。
	file, err := os.Open(archive)
	if err != nil {
		t.Fatalf("打开归档失败: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("读取 gzip 失败: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	for {
		_, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("读取 tar 失败: %v", err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("归档内没有文件")
	}
}
