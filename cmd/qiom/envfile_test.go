package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadEnvFile 校验 KEY=VALUE 解析、注释/空行/引号处理。
func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "finvquant.env")
	content := "# comment\nFINV_PG_HOST=127.0.0.1\nFINV_PG_PORT=5432\nFINV_PG_PASSWORD=\"p@ss=word\"\nexport FINV_PG_DATABASE='finv'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入 env 失败: %v", err)
	}
	// 清理测试用环境变量，避免其他测试污染
	for _, key := range []string{"FINV_PG_HOST", "FINV_PG_PORT", "FINV_PG_PASSWORD", "FINV_PG_DATABASE"} {
		t.Setenv(key, "")
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}
	if err := loadEnvFile(path); err != nil {
		t.Fatalf("loadEnvFile 失败: %v", err)
	}
	if got := os.Getenv("FINV_PG_HOST"); got != "127.0.0.1" {
		t.Fatalf("host=%q", got)
	}
	if got := os.Getenv("FINV_PG_PASSWORD"); got != "p@ss=word" {
		t.Fatalf("password=%q", got)
	}
	if got := os.Getenv("FINV_PG_DATABASE"); got != "finv" {
		t.Fatalf("database=%q", got)
	}
}
