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

// TestCleanupLocalDumpsFullDir 校验新位置 Full/<yyyyMMdd>/ 的清理：
// 远端 key 必须带 Full/ 段；远端存在才删，不存在则保留。
func TestCleanupLocalDumpsFullDir(t *testing.T) {
	backupDir := t.TempDir()
	mirrorDir := t.TempDir()
	store := newLocalStore(t, mirrorDir)

	day := "20260101"
	dir := filepath.Join(backupDir, FullDirName, day)
	uploaded := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "dump-uploaded")
	localOnly := writeLocalDump(t, dir, "finv.sys.sys_config.dump", "dump-local-only")
	// 远端 key 带 Full/（与 upload() 一致）
	putObject(t, store, "prod/Full/"+day+"/finv.sys.sys_user.dump", "dump-uploaded")
	// 反例：旧的不带 Full/ 的 key 不应被当成「已上传」
	putObject(t, store, "prod/"+day+"/finv.sys.sys_config.dump", "dump-local-only")

	result, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", defaultLocalKeepDays, Hooks{})
	if err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	if _, statErr := os.Stat(uploaded); !os.IsNotExist(statErr) {
		t.Fatalf("远端带 Full/ 的旧文件应被删除: %s", uploaded)
	}
	if _, statErr := os.Stat(localOnly); statErr != nil {
		t.Fatalf("远端 key 无 Full/ 段时必须保留（不能误判为已上传）: %v", statErr)
	}
	if result.RemovedFiles != 1 || result.KeptFiles != 1 {
		t.Fatalf("统计不符: %+v", result)
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Fatalf("目录仍有未删文件时应保留: %v", statErr)
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

// TestCleanupLocalDumpsKeepsIncrFingerprint 校验分片指纹 sidecar 不被清理：
// 清理只扫 Full/ 与顶层 yyyyMMdd 目录里的 *.dump，Incr/ 完全不在范围内。
func TestCleanupLocalDumpsKeepsIncrFingerprint(t *testing.T) {
	backupDir := t.TempDir()
	store := newLocalStore(t, t.TempDir())

	tableDir := filepath.Join(backupDir, IncrDirName, "finv.cdeu.finv_quote_secu_kline_min")
	sliceFile := writeLocalDump(t, tableDir, "20260101.mvsv.gz", "slice")
	fpFile := writeLocalDump(t, tableDir, "20260101.mvsv.gz.fp", `{"rows":10,"hash":"1"}`)

	if _, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", defaultLocalKeepDays, Hooks{}); err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	for _, path := range []string{sliceFile, fpFile} {
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("Incr/ 下的产物与指纹不应被清理: %s（%v）", path, statErr)
		}
	}
}

// TestCleanupLocalDumpsRemovesPairedFingerprint 校验删除过期 dump 时成对清理同名 .fp，
// 而孤立的 .fp（没有对应 dump）保持不动。
func TestCleanupLocalDumpsRemovesPairedFingerprint(t *testing.T) {
	backupDir := t.TempDir()
	store := newLocalStore(t, t.TempDir())
	day := "20260101"
	dir := filepath.Join(backupDir, FullDirName, day)

	dumpPath := writeLocalDump(t, dir, "finv.sys.sys_user.dump", "d")
	fpPath := writeLocalDump(t, dir, "finv.sys.sys_user.dump.fp", `{"rows":1,"hash":"1"}`)
	orphanFP := writeLocalDump(t, dir, "keep.dump.fp", `{"rows":2,"hash":"2"}`)
	putObject(t, store, "prod/Full/"+day+"/finv.sys.sys_user.dump", "d")

	if _, err := cleanupLocalDumps(context.Background(),
		Options{BackupDir: backupDir}, store, "prod", defaultLocalKeepDays, Hooks{}); err != nil {
		t.Fatalf("cleanupLocalDumps 失败: %v", err)
	}
	for _, path := range []string{dumpPath, fpPath} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("应成对删除: %s", path)
		}
	}
	if _, statErr := os.Stat(orphanFP); statErr != nil {
		t.Fatalf("孤立 .fp 不应被删除: %v", statErr)
	}
}
