package brs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acanx/finvquant/internal/meta/base/oss"
)

// testSliceRule 构造一个用于指纹测试的规则（date 模式）。
func testSliceRule() SliceRule {
	return SliceRule{
		DB: "finv", SchemaName: "cdeu", TableName: "finv_quote_secu_kline_min",
		SliceMode: SliceModeDate, Compression: "gzip", FlagUpload: "1", RedoSlices: 2,
	}
}

func testSlice() Slice {
	return Slice{Key: "20261028", Start: "20261028", End: "20261028", Rows: 18574}
}

// TestSliceFingerprintLocalRoundTrip 校验本地指纹 sidecar 的写入与读取。
func TestSliceFingerprintLocalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rule := testSliceRule()
	slice := testSlice()

	path := rule.fingerprintPath(dir, slice)
	if _, ok := loadLocalFingerprint(path); ok {
		t.Fatal("文件不存在时不应读到指纹")
	}

	fp := sliceFingerprint{Rows: 18574, Hash: "-1234567890123", Gmt: "2026-11-02T05:10:40Z"}
	if err := writeLocalFingerprint(path, fp); err != nil {
		t.Fatalf("写入指纹失败: %v", err)
	}
	got, ok := loadLocalFingerprint(path)
	if !ok {
		t.Fatal("应读到刚写入的指纹")
	}
	if !got.Equal(fp) {
		t.Fatalf("指纹不一致: %+v vs %+v", got, fp)
	}
}

// TestSliceFingerprintLocalCorrupted 校验损坏的 sidecar 视为「无指纹」而不是报错。
func TestSliceFingerprintLocalCorrupted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.fp")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadLocalFingerprint(path); ok {
		t.Fatal("损坏文件不应读出指纹")
	}
	if err := os.WriteFile(path, []byte(`{"rows":10}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := loadLocalFingerprint(path); ok {
		t.Fatal("缺少 hash 的记录视为无效")
	}
}

// TestSliceFingerprintRemote 校验远端 sidecar 的读取（用 local 适配器模拟对象存储）。
func TestSliceFingerprintRemote(t *testing.T) {
	mirror := t.TempDir()
	store := newLocalStore(t, mirror)
	rule := testSliceRule()
	slice := testSlice()

	key := rule.remoteFingerprintKey("Prod", slice)
	if !strings.HasPrefix(key, "Prod/Incr/finv.cdeu.finv_quote_secu_kline_min/") {
		t.Fatalf("远端指纹 key 结构不符: %s", key)
	}
	if _, ok := loadRemoteFingerprint(context.Background(), store, key); ok {
		t.Fatal("远端无对象时不应读到指纹")
	}

	payload := `{"rows":18574,"hash":"42","gmt":"2026-11-02T05:10:40Z"}`
	if err := store.Put(context.Background(), key, strings.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		t.Fatalf("写入远端指纹失败: %v", err)
	}
	fp, ok := loadRemoteFingerprint(context.Background(), store, key)
	if !ok || fp.Rows != 18574 || fp.Hash != "42" {
		t.Fatalf("远端指纹读取不符: %+v ok=%v", fp, ok)
	}
}

// TestSliceFingerprintEqual 校验指纹比较只认「行数 + 哈希」。
func TestSliceFingerprintEqual(t *testing.T) {
	a := sliceFingerprint{Rows: 10, Hash: "100", Gmt: "t1"}
	if !a.Equal(sliceFingerprint{Rows: 10, Hash: "100", Gmt: "t2"}) {
		t.Fatal("行数与哈希相同应判等（Gmt 不参与比较）")
	}
	if a.Equal(sliceFingerprint{Rows: 11, Hash: "100"}) {
		t.Fatal("行数不同不应判等")
	}
	if a.Equal(sliceFingerprint{Rows: 10, Hash: "101"}) {
		t.Fatal("哈希不同不应判等")
	}
}

// TestDecideSkipSliceFingerprint 校验跳过判定的四条分支。
func TestDecideSkipSliceFingerprint(t *testing.T) {
	mirror := t.TempDir()
	store := newLocalStore(t, mirror)
	dir := t.TempDir()
	rule := testSliceRule()
	slice := testSlice()
	opts := Options{BackupDir: dir}
	src := sliceFingerprint{Rows: 18574, Hash: "999"}
	path := rule.fingerprintPath(dir, slice)

	// 1) 源端指纹不可用 → 必须重做
	if skip, _ := decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, src, false, true, Hooks{}); skip {
		t.Fatal("算不出源端指纹时不应跳过")
	}

	// 2) 无指纹 + 自愈开启 → 重做一次建立基线，并输出 info 说明
	var healed bool
	hooks := Hooks{OnLog: func(level, message string) {
		if level == "info" && strings.Contains(message, "重做一次建立基线") {
			healed = true
		}
	}}
	if skip, _ := decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, src, true, true, hooks); skip {
		t.Fatal("无指纹且自愈开启时应重做")
	}
	if !healed {
		t.Fatal("应输出「重做一次建立基线」日志")
	}

	// 3) 无指纹 + 自愈关闭（redo_slices=0）→ 跳过并告警
	var warned bool
	closedHooks := Hooks{OnLog: func(level, message string) {
		if level == "warn" && strings.Contains(message, "自愈关闭") {
			warned = true
		}
	}}
	skip, reason := decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, src, true, false, closedHooks)
	if !skip || !strings.Contains(reason, "无指纹") {
		t.Fatalf("自愈关闭且无指纹应跳过，得到 skip=%v reason=%q", skip, reason)
	}
	if !warned {
		t.Fatal("应输出「自愈关闭」告警")
	}

	// 4) 本地指纹与源端一致 → 跳过（且提示内容无变化）
	if err := writeLocalFingerprint(path, src); err != nil {
		t.Fatal(err)
	}
	skip, reason = decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, src, true, false, Hooks{})
	if !skip || !strings.Contains(reason, "内容无变化") {
		t.Fatalf("指纹一致应跳过，得到 skip=%v reason=%q", skip, reason)
	}

	// 5) 本地指纹与源端不同 → 重做
	changed := sliceFingerprint{Rows: 18600, Hash: "999"}
	if skip, _ := decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, changed, true, false, Hooks{}); skip {
		t.Fatal("指纹不同（行数变化）应重做")
	}

	// 6) 本地无指纹但远端有且一致 → 跳过（本地文件被手工清理的场景）
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	payload := `{"rows":18574,"hash":"999"}`
	key := rule.remoteFingerprintKey("Prod", slice)
	if err := store.Put(context.Background(), key, strings.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		t.Fatal(err)
	}
	skip, reason = decideSkipSlice(context.Background(), store, rule, opts, "Prod", slice, src, true, false, Hooks{})
	if !skip || !strings.Contains(reason, "内容无变化") {
		t.Fatalf("远端指纹一致应跳过，得到 skip=%v reason=%q", skip, reason)
	}
}

// TestSaveFingerprintsWritesBoth 校验备份成功后本地与远端指纹都会写入。
func TestSaveFingerprintsWritesBoth(t *testing.T) {
	mirror := t.TempDir()
	store := newLocalStore(t, mirror)
	dir := t.TempDir()
	rule := testSliceRule()
	slice := testSlice()
	fp := sliceFingerprint{Rows: 18574, Hash: "12345"}

	saveFingerprints(context.Background(), store, rule, Options{BackupDir: dir}, "Prod", slice, fp, Hooks{})

	if _, ok := loadLocalFingerprint(rule.fingerprintPath(dir, slice)); !ok {
		t.Fatal("应写入本地指纹")
	}
	if _, ok := loadRemoteFingerprint(context.Background(), store, rule.remoteFingerprintKey("Prod", slice)); !ok {
		t.Fatal("应写入远端指纹")
	}

	// flag_upload=0 时只写本地
	rule2 := rule
	rule2.FlagUpload = "0"
	slice2 := Slice{Key: "20261029", Start: "20261029", End: "20261029"}
	saveFingerprints(context.Background(), store, rule2, Options{BackupDir: dir}, "Prod", slice2, fp, Hooks{})
	if _, ok := loadLocalFingerprint(rule2.fingerprintPath(dir, slice2)); !ok {
		t.Fatal("关闭上传时仍应写本地指纹")
	}
	if _, ok := loadRemoteFingerprint(context.Background(), store, rule2.remoteFingerprintKey("Prod", slice2)); ok {
		t.Fatal("关闭上传时不应写远端指纹")
	}
}

// 保证 oss 包被使用（newLocalStore 依赖 oss.Config）。
var _ = oss.Config{}
