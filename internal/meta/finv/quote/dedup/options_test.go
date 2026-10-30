package dedup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestComputeBudget 校验预算公式（方案稿 4.2）与边界收敛。
func TestComputeBudget(t *testing.T) {
	bizDate := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name       string
		baseline   IngestBaseline
		pending    int64
		pressure   float64
		wantFactor float64
		wantRows   int64
	}{
		{
			name:       "基础：无积压无SLA压力",
			baseline:   IngestBaseline{BaseRows: 10000, BaseBytes: 1000000, P75Rows: 0},
			wantFactor: 1.20,
			wantRows:   12000,
		},
		{
			name:       "P75 低于 base*factor 时取 P75",
			baseline:   IngestBaseline{BaseRows: 10000, P75Rows: 5000},
			wantFactor: 1.20,
			wantRows:   5000,
		},
		{
			name:       "积压 5 天",
			baseline:   IngestBaseline{BaseRows: 10000, P75Rows: 100000},
			pending:    50000,
			wantFactor: 1.45,
			wantRows:   14500,
		},
		{
			name:       "SLA 压力拉满：factor 封顶 1.65",
			baseline:   IngestBaseline{BaseRows: 10000, P75Rows: 100000},
			pending:    500000,
			pressure:   1,
			wantFactor: 1.65,
			wantRows:   16500,
		},
		{
			name:       "统计缺失时使用兜底预算",
			baseline:   IngestBaseline{},
			wantFactor: 1.20,
			wantRows:   int64(float64(fallbackBudgetRows) * 1.20),
		},
	}

	for _, item := range cases {
		budget := ComputeBudget(bizDate, item.baseline, item.pending, item.pressure)
		if diff := budget.Factor - item.wantFactor; diff > 0.0001 || diff < -0.0001 {
			t.Fatalf("%s：factor=%.4f，期望 %.4f", item.name, budget.Factor, item.wantFactor)
		}
		if budget.BudgetRows != item.wantRows {
			t.Fatalf("%s：budget_rows=%d，期望 %d", item.name, budget.BudgetRows, item.wantRows)
		}
		if budget.Status != "CALCULATED" {
			t.Fatalf("%s：初始状态应为 CALCULATED，实际 %s", item.name, budget.Status)
		}
		if budget.HardCapRows <= 0 {
			t.Fatalf("%s：hard_cap_rows 应大于 0", item.name)
		}
	}
}

// TestBudgetRemaining 校验剩余预算不为负。
func TestBudgetRemaining(t *testing.T) {
	budget := &Budget{BudgetRows: 100, ConsumedRows: 130}
	if budget.RemainingRows() != 0 {
		t.Fatalf("超额消耗时剩余预算应为 0，实际 %d", budget.RemainingRows())
	}
	var nilBudget *Budget
	if nilBudget.RemainingRows() != 0 {
		t.Fatalf("nil 预算应返回 0")
	}
}

// TestMaxSealedBizMonth 校验收口月份推导（次月 25 日）。
func TestMaxSealedBizMonth(t *testing.T) {
	cases := map[string]int{
		"2026-11-24": 202609,
		"2026-11-25": 202610,
		"2026-12-01": 202610,
		"2026-01-10": 202511,
	}
	for date, want := range cases {
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil {
			t.Fatalf("解析日期失败: %v", err)
		}
		if got := maxSealedBizMonth(parsed); got != want {
			t.Fatalf("maxSealedBizMonth(%s)=%d，期望 %d", date, got, want)
		}
	}
}

// TestMonthHelpers 校验月份加减、日期进位与月首日。
func TestMonthHelpers(t *testing.T) {
	if got := addMonths(202601, -1); got != 202512 {
		t.Fatalf("addMonths(202601,-1)=%d，期望 202512", got)
	}
	if got := addMonths(202612, 1); got != 202701 {
		t.Fatalf("addMonths(202612,1)=%d，期望 202701", got)
	}
	if got := nextDate(20260228); got != 20260301 {
		t.Fatalf("nextDate(20260228)=%d，期望 20260301", got)
	}
	if got := nextDate(20240228); got != 20240229 {
		t.Fatalf("nextDate(20240228)=%d，期望 20240229（闰年）", got)
	}
	if got := firstDayOfMonth(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)); got != 20261001 {
		t.Fatalf("firstDayOfMonth=%d，期望 20261001", got)
	}
}

// TestBackoffFor 校验指数退避与上限。
func TestBackoffFor(t *testing.T) {
	cases := map[int]time.Duration{
		0:  time.Minute,
		1:  time.Minute,
		2:  2 * time.Minute,
		3:  4 * time.Minute,
		7:  60 * time.Minute,
		20: 60 * time.Minute,
	}
	for attempt, want := range cases {
		if got := backoffFor(attempt); got != want {
			t.Fatalf("backoffFor(%d)=%s，期望 %s", attempt, got, want)
		}
	}
}

// TestDirUploader 校验目录型上传适配器：上传、远端查重、校验与目录穿越防护。
func TestDirUploader(t *testing.T) {
	root := t.TempDir()
	uploader, err := newDirUploader(root)
	if err != nil {
		t.Fatalf("创建 dir uploader 失败: %v", err)
	}

	source := filepath.Join(t.TempDir(), "CN_SZ_159934_Min_FTMM_Mon_202610.mvsv")
	content := []byte("# header\n\n1792546200|20261021|093000\n")
	if err := os.WriteFile(source, content, 0o644); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}
	checksum, size, err := FileSHA256(source)
	if err != nil {
		t.Fatalf("计算 checksum 失败: %v", err)
	}

	targetPath := "archive/CN_SZ_159934_Min_FTMM_Mon_202610.mvsv"
	written, err := uploader.Put(context.Background(), source, targetPath)
	if err != nil {
		t.Fatalf("Put 失败: %v", err)
	}
	if written != size {
		t.Fatalf("上传字节数不符：%d != %d", written, size)
	}

	exists, remoteSize, remoteChecksum, err := uploader.Stat(context.Background(), targetPath)
	if err != nil || !exists {
		t.Fatalf("Stat 应命中远端文件: exists=%v err=%v", exists, err)
	}
	if remoteSize != size || remoteChecksum != checksum {
		t.Fatalf("远端 size/checksum 不符：%d/%s", remoteSize, remoteChecksum)
	}
	if err := uploader.Verify(context.Background(), targetPath, size, checksum); err != nil {
		t.Fatalf("Verify 失败: %v", err)
	}
	if err := uploader.Verify(context.Background(), targetPath, size+1, checksum); err == nil {
		t.Fatalf("大小不一致时 Verify 应失败")
	}
	if err := uploader.Verify(context.Background(), targetPath, size, "deadbeef"); err == nil {
		t.Fatalf("checksum 不一致时 Verify 应失败")
	}

	for _, bad := range []string{"../escape.mvsv", "/abs/escape.mvsv", "a/../../escape.mvsv"} {
		if _, err := uploader.Put(context.Background(), source, bad); err == nil {
			t.Fatalf("目标路径 %q 应被拒绝", bad)
		}
	}
}

// TestNoneUploader 校验未启用上传时的行为。
func TestNoneUploader(t *testing.T) {
	uploader, err := NewUploader(Options{UploadMode: UploadModeNone})
	if err != nil {
		t.Fatalf("创建 none uploader 失败: %v", err)
	}
	if uploader.Mode() != UploadModeNone {
		t.Fatalf("Mode 应为 none，实际 %s", uploader.Mode())
	}
	if _, err := uploader.Put(context.Background(), "x", "y"); err == nil {
		t.Fatalf("none 模式下 Put 应返回错误")
	}
}

// TestDescribeUploadTarget 校验上传目标描述（Job 过程日志里「传到哪」那段文本）。
func TestDescribeUploadTarget(t *testing.T) {
	root := t.TempDir()
	dirUp, err := newDirUploader(root)
	if err != nil {
		t.Fatalf("创建 dir uploader 失败: %v", err)
	}
	target := "CN_SZ_159934/CN_SZ_159934_Min_FTMM_Mon_202610_Base.mvsv"
	description := describeUploadTarget(context.Background(), dirUp, target)
	if !strings.Contains(description, "mode=dir") || !strings.Contains(description, root) ||
		!strings.Contains(description, "Min_FTMM_Mon_202610_Base.mvsv") {
		t.Fatalf("dir 目标描述不合格：%s", description)
	}

	noneUp, err := NewUploader(Options{UploadMode: UploadModeNone})
	if err != nil {
		t.Fatalf("创建 none uploader 失败: %v", err)
	}
	if got := describeUploadTarget(context.Background(), noneUp, target); !strings.Contains(got, "mode=none") {
		t.Fatalf("none 目标描述不合格：%s", got)
	}

	// 越界路径不阻断日志：退回「模式 + 路径」字样。
	if got := describeUploadTarget(context.Background(), dirUp, "../escape.mvsv"); !strings.Contains(got, "mode=dir") {
		t.Fatalf("非法路径应退回模式描述：%s", got)
	}
}

// TestOptionsWindow 校验低峰窗口判断（含跨零点窗口）。
func TestOptionsWindow(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Skipf("缺少时区数据: %v", err)
	}
	options := Options{Location: location, WindowStart: 30 * 60, WindowEnd: 6*3600 + 30*60}

	inside := []string{"2026-10-24 00:30", "2026-10-24 03:00", "2026-10-24 06:29"}
	outside := []string{"2026-10-24 00:29", "2026-10-24 06:30", "2026-10-24 12:00"}
	for _, text := range inside {
		parsed, _ := time.ParseInLocation("2006-01-02 15:04", text, location)
		if !options.InWindow(parsed) {
			t.Fatalf("%s 应在窗口内", text)
		}
	}
	for _, text := range outside {
		parsed, _ := time.ParseInLocation("2006-01-02 15:04", text, location)
		if options.InWindow(parsed) {
			t.Fatalf("%s 不应在窗口内", text)
		}
	}

	crossMidnight := Options{Location: location, WindowStart: 22 * 3600, WindowEnd: 2 * 3600}
	night, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-24 23:30", location)
	early, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-24 01:00", location)
	noon, _ := time.ParseInLocation("2006-01-02 15:04", "2026-10-24 12:00", location)
	if !crossMidnight.InWindow(night) || !crossMidnight.InWindow(early) {
		t.Fatalf("跨零点窗口判断错误")
	}
	if crossMidnight.InWindow(noon) {
		t.Fatalf("跨零点窗口不应包含正午")
	}

	fullDay := Options{Location: location, WindowStart: 0, WindowEnd: 23*3600 + 59*60 + 59}
	for _, text := range []string{"2026-10-24 00:00", "2026-10-24 12:00", "2026-10-24 23:59"} {
		parsed, _ := time.ParseInLocation("2006-01-02 15:04", text, location)
		if !fullDay.InWindow(parsed) {
			t.Fatalf("%s 应在全天窗口内", text)
		}
	}
}

// TestParseClock 校验在线修改窗口时的解析：返回值统一为自 0 点起的**秒数**。
func TestParseClock(t *testing.T) {
	for _, item := range []struct {
		input string
		want  int
	}{
		{"00:00", 0},
		{"00:00:00", 0},
		{"01:15", 4500}, // HH:MM 按秒补齐（1h15m = 4500s）
		{"01:15:30", 4530},
		{"23:59", 86340},
		{"23:59:59", 86399},
	} {
		got, err := ParseClock(item.input)
		if err != nil || got != item.want {
			t.Fatalf("ParseClock(%q)=(%d,%v)，期望 (%d,nil)", item.input, got, err, item.want)
		}
	}
	for _, input := range []string{"", "bad", "24:00", "12:60", "1", "12:00:60"} {
		if _, err := ParseClock(input); err == nil {
			t.Fatalf("ParseClock(%q) 应返回错误", input)
		}
	}
}

// TestTargetPath 校验上传目标路径拼接。
func TestTargetPath(t *testing.T) {
	options := Options{}
	if got := options.TargetPath("a.mvsv"); got != "a.mvsv" {
		t.Fatalf("无前缀时 TargetPath=%q", got)
	}
	options.UploadPrefix = "cn/quotes"
	if got := options.TargetPath("a.mvsv"); got != "cn/quotes/a.mvsv" {
		t.Fatalf("有前缀时 TargetPath=%q", got)
	}
}

// TestArchiveTargetPath 校验归档目标的分层布局（默认 by_security）。
//
// 约定：最终远端位置 = sys_config 20004 的 pre_path_key + 本函数返回值。
func TestArchiveTargetPath(t *testing.T) {
	const fileName = "USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607.mvsv"

	cases := []struct {
		name   string
		layout string
		prefix string
		want   string
	}{
		{
			name:   "默认按证券分层",
			layout: UploadLayoutBySecurity,
			want:   "USA_US-NSDQ/NVDA/" + fileName,
		},
		{
			name:   "分层 + 外层前缀",
			layout: UploadLayoutBySecurity,
			prefix: "cn/quotes",
			want:   "cn/quotes/USA_US-NSDQ/NVDA/" + fileName,
		},
		{
			name:   "flat 兼容旧行为",
			layout: UploadLayoutFlat,
			want:   fileName,
		},
		{
			name:   "flat + 外层前缀",
			layout: UploadLayoutFlat,
			prefix: "cn/quotes",
			want:   "cn/quotes/" + fileName,
		},
		{
			name:   "空 layout 按默认分层",
			layout: "",
			want:   "USA_US-NSDQ/NVDA/" + fileName,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			options := Options{UploadLayout: testCase.layout, UploadPrefix: testCase.prefix}
			if got := options.ArchiveTargetPath("USA", "US-NSDQ", "NVDA", fileName); got != testCase.want {
				t.Fatalf("ArchiveTargetPath=%q，期望 %q", got, testCase.want)
			}
		})
	}

	// 空 region/market 退回 UNKNOWN，保证目录段不为空（与文件名清洗规则一致）。
	options := Options{UploadLayout: UploadLayoutBySecurity}
	if got := options.ArchiveTargetPath("", "", "NVDA", fileName); got != "UNKNOWN_UNKNOWN/NVDA/"+fileName {
		t.Fatalf("空元数据时应使用 UNKNOWN 占位，实际 %q", got)
	}
	// 含路径分隔符的元数据被清洗，不会跳目录。
	if got := options.ArchiveTargetPath("US/A", "M\\B", "../NVDA", fileName); got != "US_A_M_B/.._NVDA/"+fileName {
		t.Fatalf("路径分隔符应被清洗，实际 %q", got)
	}
	// 段本身恰为 '.' / '..' 时替换为 UNKNOWN（防目录越界）。
	if got := options.ArchiveTargetPath("", "", "..", fileName); got != "UNKNOWN_UNKNOWN/UNKNOWN/"+fileName {
		t.Fatalf("'.'/'..' 段应被替换为 UNKNOWN，实际 %q", got)
	}
}

// TestNormalizedLayout 校验布局取值收敛（非法值回退默认分层）。
func TestNormalizedLayout(t *testing.T) {
	cases := map[string]string{
		"by_security": UploadLayoutBySecurity,
		"BY_SECURITY": UploadLayoutBySecurity,
		"flat":        UploadLayoutFlat,
		" FLAT ":      UploadLayoutFlat,
		"":            UploadLayoutBySecurity,
		"unknown":     UploadLayoutBySecurity,
	}
	for input, want := range cases {
		if got := normalizedLayout(input); got != want {
			t.Fatalf("normalizedLayout(%q)=%q，期望 %q", input, got, want)
		}
	}
}
