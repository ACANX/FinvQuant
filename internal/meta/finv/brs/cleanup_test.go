package brs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/base/oss"
	"github.com/acanx/finvquant/internal/meta/base/oss/local"
)

// newLocalStore 构造 local 适配器（root 为镜像根目录，prefix 为空）。
func newLocalStore(t *testing.T, root string) oss.Store {
	t.Helper()
	store, err := local.New(oss.Config{Provider: "local", Endpoint: root})
	if err != nil {
		t.Fatalf("创建 local 适配器失败: %v", err)
	}
	return store
}

// putObject 在镜像里写一个对象（模拟已上传）。
func putObject(t *testing.T, store oss.Store, key, content string) {
	t.Helper()
	if err := store.Put(context.Background(), key, strings.NewReader(content), int64(len(content)), "text/plain"); err != nil {
		t.Fatalf("写入对象 %s 失败: %v", key, err)
	}
}

// writeLocalDump 在本地备份目录写一个 dump 文件。
func writeLocalDump(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	return path
}

// TestCleanupLocalDumpsRemovesUploadedOnly 校验：远端存在的旧文件被删，远端不存在的保留。
func TestCleanupLocalDumpsRemovesUploadedOnly(t *testing.T) {
	backupDir := t.TempDir()
	mirrorDir := t.TempDir()
	store := newLocalStore(t, mirrorDir)

	day := "20260101" // 远早于默认 7 天保留期
	dir := filepath.Join(backupDir, day)
	uploaded := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "dump-uploaded")
	notUploaded := writeLocalDump(t, dir, "finv.sys.sys_config.dump", "dump-local-only")
	putObject(t, store, "prod/"+day+"/finv.sys.sys_user.dump", "dump-uploaded")

	hooks := Hooks{OnLog: func(level, message string) {
		if level == "error" {
			t.Fatalf("不应产生 error 日志: %s", message)
		}
	}}
	result, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", defaultLocalKeepDays, hooks)
	if err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}

	if _, statErr := os.Stat(uploaded); !os.IsNotExist(statErr) {
		t.Fatalf("已上传的旧文件应被删除: %s", uploaded)
	}
	if _, statErr := os.Stat(notUploaded); statErr != nil {
		t.Fatalf("远端不存在的文件必须保留: %s（%v）", notUploaded, statErr)
	}
	if result.RemovedFiles != 1 || result.KeptFiles != 1 {
		t.Fatalf("统计不符: %+v", result)
	}
	if result.RemovedBytes != int64(len("dump-uploaded")) {
		t.Fatalf("释放字节数不符: %d", result.RemovedBytes)
	}
}

// TestCleanupLocalDumpsKeepWindow 校验保留期内的目录不被清理。
func TestCleanupLocalDumpsKeepWindow(t *testing.T) {
	backupDir := t.TempDir()
	mirrorDir := t.TempDir()
	store := newLocalStore(t, mirrorDir)

	today := "20260101"
	dir := filepath.Join(backupDir, today)
	path := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "x")
	putObject(t, store, "prod/"+today+"/finv.sys.sys_user.dump", "x")

	// 保留期设成 36500 天（约 100 年）→ cutoff 远早于 today，文件必须保留。
	result, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", 36500, Hooks{})
	if err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	if result.RemovedFiles != 0 {
		t.Fatalf("保留期内不应删除: %+v", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("保留期内的文件应存在: %v", statErr)
	}
}

// TestCleanupLocalDumpsSkipsWithoutStore 校验无对象存储时完全不清理（本地是唯一副本）。
func TestCleanupLocalDumpsSkipsWithoutStore(t *testing.T) {
	backupDir := t.TempDir()
	dir := filepath.Join(backupDir, "20260101")
	path := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "x")

	var warned bool
	hooks := Hooks{OnLog: func(level, message string) {
		if level == "warn" && strings.Contains(message, "跳过本地清理") {
			warned = true
		}
	}}
	result, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, nil, "", defaultLocalKeepDays, hooks)
	if err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	if result.RemovedFiles != 0 {
		t.Fatalf("无 store 时不应删除: %+v", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("无 store 时文件必须保留: %v", statErr)
	}
	if !warned {
		t.Fatal("应输出「跳过本地清理」告警")
	}
}

// TestCleanupLocalDumpsDisabled 校验 keepDays<=0 时不清理。
func TestCleanupLocalDumpsDisabled(t *testing.T) {
	backupDir := t.TempDir()
	dir := filepath.Join(backupDir, "20260101")
	path := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "x")

	var info string
	hooks := Hooks{OnLog: func(level, message string) {
		if level == "info" && strings.Contains(message, "未启用") {
			info = message
		}
	}}
	result, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, newLocalStore(t, t.TempDir()), "prod", 0, hooks)
	if err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	if result.RemovedFiles != 0 {
		t.Fatalf("keepDays=0 时不应删除: %+v", result)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("keepDays=0 时文件必须保留: %v", statErr)
	}
	if info == "" {
		t.Fatal("应输出「未启用」提示")
	}
}

// TestCleanupLocalDumpsIgnoresIncrDir 校验 Incr/ 分片目录不受清理影响。
func TestCleanupLocalDumpsIgnoresIncrDir(t *testing.T) {
	backupDir := t.TempDir()
	store := newLocalStore(t, t.TempDir())

	incrDir := filepath.Join(backupDir, IncrDirName, "finv.cdeu.finv_quote_secu_kline_min")
	path := writeLocalDump(t, incrDir, "20260101.mvsv.gz", "slice")
	// 即使远端存在同名 key，Incr/ 也不应被清理（分片按分片键覆盖，不随时间增长）。
	putObject(t, store, "prod/Incr/finv.cdeu.finv_quote_secu_kline_min/20260101.mvsv.gz", "slice")

	if _, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", defaultLocalKeepDays, Hooks{}); err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("Incr/ 下的分片不应被删除: %v", statErr)
	}
}
