package syscfg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestParseRepoSpec 覆盖 20004 的合法与非法取值。
func TestParseRepoSpec(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    RepoSpec
		wantErr string
	}{
		{
			name: "合法配置",
			raw:  "ACANX:FinvQuant-Archive:main:Archive/FinvQuant/Monthly",
			want: RepoSpec{
				Owner:      "ACANX",
				Repo:       "FinvQuant-Archive",
				Branch:     "main",
				PathPrefix: "Archive/FinvQuant/Monthly",
				Raw:        "ACANX:FinvQuant-Archive:main:Archive/FinvQuant/Monthly",
			},
		},
		{
			name: "首尾空白与前缀斜杠被归一化",
			raw:  "  ACANX : FinvQuant : main : /Archive/ ",
			want: RepoSpec{
				Owner:      "ACANX",
				Repo:       "FinvQuant",
				Branch:     "main",
				PathPrefix: "Archive",
				Raw:        "ACANX : FinvQuant : main : /Archive/",
			},
		},
		{
			name: "空目录前缀合法（仓库根）",
			raw:  "ACANX:FinvQuant:main:",
			want: RepoSpec{Owner: "ACANX", Repo: "FinvQuant", Branch: "main", Raw: "ACANX:FinvQuant:main:"},
		},
		{name: "空值", raw: "   ", wantErr: "未配置"},
		{name: "段数不足", raw: "ACANX:FinvQuant:main", wantErr: "当前 3 段"},
		{name: "段数过多", raw: "a:b:c:d:e", wantErr: "当前 5 段"},
		{name: "owner 为空", raw: ":FinvQuant:main:p", wantErr: "owner 段为空"},
		{name: "branch 含斜杠", raw: "ACANX:FinvQuant:feature/x:p", wantErr: "branch 段非法"},
		{name: "前缀含 ..", raw: "ACANX:FinvQuant:main:a/../b", wantErr: "不允许 '.' 或 '..'"},
		{name: "前缀含空段", raw: "ACANX:FinvQuant:main:a//b", wantErr: "存在空路径段"},
		{name: "前缀含空白", raw: "ACANX:FinvQuant:main:a b", wantErr: "不能包含空白"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseRepoSpec(testCase.raw)
			if testCase.wantErr != "" {
				if err == nil {
					t.Fatalf("期望错误包含 %q，实际无错误（got=%+v）", testCase.wantErr, got)
				}
				if !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("期望错误包含 %q，实际 %q", testCase.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("期望解析成功，实际错误: %v", err)
			}
			if got.Owner != testCase.want.Owner || got.Repo != testCase.want.Repo ||
				got.Branch != testCase.want.Branch || got.PathPrefix != testCase.want.PathPrefix {
				t.Fatalf("解析结果不符：got=%+v want=%+v", got, testCase.want)
			}
		})
	}
}

// TestRepoSpecJoin 目录前缀与归档文件名拼接。
func TestRepoSpecJoin(t *testing.T) {
	spec := RepoSpec{PathPrefix: "Archive/Monthly"}
	if got := spec.Join("/kline/202610.mvsv"); got != "Archive/Monthly/kline/202610.mvsv" {
		t.Fatalf("Join 结果不符: %q", got)
	}
	root := RepoSpec{}
	if got := root.Join("202610.mvsv"); got != "202610.mvsv" {
		t.Fatalf("空前缀 Join 结果不符: %q", got)
	}
	if got := (RepoSpec{PathPrefix: "Archive"}).Join(""); got != "Archive" {
		t.Fatalf("空文件名 Join 结果不符: %q", got)
	}
}

// TestValuesArchiveTarget 校验上传目标的三重前置条件。
func TestValuesArchiveTarget(t *testing.T) {
	spec, err := ParseRepoSpec("ACANX:FinvQuant-Archive:main:Archive")
	if err != nil {
		t.Fatalf("准备仓库配置失败: %v", err)
	}

	if _, err := (Values{}).ArchiveTarget(); !errors.Is(err, ErrSwitchOff) {
		t.Fatalf("开关关闭应返回 ErrSwitchOff，实际 %v", err)
	}
	if _, err := (Values{ArchiveUploadEnabled: true, ArchiveRepo: spec}).ArchiveTarget(); !errors.Is(err, ErrTokenMissing) {
		t.Fatalf("缺 PAT 应返回 ErrTokenMissing，实际 %v", err)
	}
	if _, err := (Values{ArchiveUploadEnabled: true, ArchiveToken: "ghp_x",
		ArchiveRepoErr: ErrRepoSpecEmpty}).ArchiveTarget(); !errors.Is(err, ErrRepoSpecEmpty) {
		t.Fatalf("仓库配置缺失应返回 ErrRepoSpecEmpty，实际 %v", err)
	}

	target, err := (Values{ArchiveUploadEnabled: true, ArchiveToken: " ghp_x ", ArchiveRepo: spec}).ArchiveTarget()
	if err != nil {
		t.Fatalf("配置完整时应成功，实际 %v", err)
	}
	if target.Token != "ghp_x" {
		t.Fatalf("PAT 应去除首尾空白，实际 %q", target.Token)
	}
	if target.Repo.Owner != "ACANX" {
		t.Fatalf("仓库所有者不符: %q", target.Repo.Owner)
	}
}

// TestStoreCachesWithinTTL 断言同一 TTL 内只读库一次，过期后重新读取。
func TestStoreCachesWithinTTL(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	loads := 0
	store := newStoreWithLoader(func(context.Context) (Values, error) {
		loads++
		return Values{ArchiveUploadEnabled: true}, nil
	}, time.Minute)
	store.now = func() time.Time { return now }

	ctx := context.Background()
	if _, err := store.Load(ctx); err != nil {
		t.Fatalf("首次读取失败: %v", err)
	}
	if _, err := store.Load(ctx); err != nil {
		t.Fatalf("二次读取失败: %v", err)
	}
	if loads != 1 {
		t.Fatalf("TTL 内应命中缓存，实际读库 %d 次", loads)
	}

	now = now.Add(2 * time.Minute)
	if _, err := store.Load(ctx); err != nil {
		t.Fatalf("过期后读取失败: %v", err)
	}
	if loads != 2 {
		t.Fatalf("TTL 过期后应重新读库，实际读库 %d 次", loads)
	}
}

// TestStoreInvalidate 断言主动失效缓存后立即重新读库（在线改配置场景）。
func TestStoreInvalidate(t *testing.T) {
	loads := 0
	enabled := false
	store := newStoreWithLoader(func(context.Context) (Values, error) {
		loads++
		return Values{ArchiveUploadEnabled: enabled}, nil
	}, time.Hour)

	ctx := context.Background()
	values, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("首次读取失败: %v", err)
	}
	if values.ArchiveUploadEnabled {
		t.Fatal("初始不应启用")
	}

	enabled = true
	store.Invalidate()
	values, err = store.Load(ctx)
	if err != nil {
		t.Fatalf("失效后读取失败: %v", err)
	}
	if !values.ArchiveUploadEnabled {
		t.Fatal("失效后应读到最新配置（已启用）")
	}
	if loads != 2 {
		t.Fatalf("失效后应重新读库，实际读库 %d 次", loads)
	}
}

// TestStoreLoadError 断言读库失败时返回错误而非静默回退。
func TestStoreLoadError(t *testing.T) {
	sentinel := errors.New("boom")
	store := newStoreWithLoader(func(context.Context) (Values, error) {
		return Values{}, sentinel
	}, time.Minute)
	if _, err := store.Load(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("应透传读库错误，实际 %v", err)
	}
}

// TestNewStoreNilPool 断言空连接池给出可读错误而不是 panic。
func TestNewStoreNilPool(t *testing.T) {
	store := NewStore(nil)
	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("空连接池应返回错误")
	}
}

// TestNormalizeCDEUMode 断言 20005 取值归一化：只有 auto 视为自动，其余一律 Manual。
func TestNormalizeCDEUMode(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"Auto", CDEUModeAuto},
		{"auto", CDEUModeAuto},
		{"  AUTO  ", CDEUModeAuto},
		{"Manual", CDEUModeManual},
		{"manual", CDEUModeManual},
		{"", CDEUModeManual},
		{"   ", CDEUModeManual},
		{"AutoX", CDEUModeManual}, // 未知取值按 Manual（安全侧）
		{"1", CDEUModeManual},
	}
	for _, c := range cases {
		if got := NormalizeCDEUMode(c.raw); got != c.want {
			t.Errorf("NormalizeCDEUMode(%q) = %q，期望 %q", c.raw, got, c.want)
		}
	}
}

// TestValuesCDEUScheduleEnabled 断言只有 Auto 才允许调度器自动触发。
func TestValuesCDEUScheduleEnabled(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"Auto", true},
		{"auto", true},
		{"Manual", false},
		{"", false}, // 未配置：安全侧不自动跑
		{"garbage", false},
	}
	for _, c := range cases {
		v := Values{CDEUModeRaw: c.raw, CDEUMode: NormalizeCDEUMode(c.raw)}
		if got := v.CDEUScheduleEnabled(); got != c.want {
			t.Errorf("CDEUScheduleEnabled(raw=%q) = %v，期望 %v", c.raw, got, c.want)
		}
	}
}

// TestParseExportWindow 断言 20006 取值解析：必须 HH:MM:SS|HH:MM:SS。
func TestParseExportWindow(t *testing.T) {
	ok := []struct {
		raw        string
		start, end int
	}{
		{"00:30:00|06:30:00", 1800, 23400},
		{"00:00:00|23:59:59", 0, 86399},
		{"23:00:00|06:00:00", 82800, 21600}, // 跨零点
		{" 01:15:30 | 02:45:15 ", 4530, 9915},
	}
	for _, item := range ok {
		window, err := ParseExportWindow(item.raw)
		if err != nil {
			t.Fatalf("ParseExportWindow(%q) 意外报错: %v", item.raw, err)
		}
		if window.Start != item.start || window.End != item.end {
			t.Fatalf("ParseExportWindow(%q)=(%d,%d)，期望 (%d,%d)",
				item.raw, window.Start, window.End, item.start, item.end)
		}
	}

	bad := []string{
		"",
		"00:30|06:30",                // 缺秒
		"00:30:00",                   // 缺分隔符
		"00:30:00|06:30:00|07:00:00", // 段数过多
		"24:00:00|06:30:00",          // 小时越界
		"00:60:00|06:30:00",          // 分钟越界
		"00:30:60|06:30:00",          // 秒越界
		"bad|bad",
	}
	for _, raw := range bad {
		if _, err := ParseExportWindow(raw); err == nil {
			t.Fatalf("ParseExportWindow(%q) 应返回错误", raw)
		}
	}
}

// TestFormatExportWindow 断言窗口格式化回 HH:MM:SS|HH:MM:SS。
func TestFormatExportWindow(t *testing.T) {
	cases := []struct {
		window ExportWindow
		want   string
	}{
		{ExportWindow{Start: 0, End: 86399}, "00:00:00|23:59:59"},
		{ExportWindow{Start: 1800, End: 23400}, "00:30:00|06:30:00"},
		{ExportWindow{Start: 4530, End: 9915}, "01:15:30|02:45:15"},
	}
	for _, item := range cases {
		if got := FormatExportWindow(item.window); got != item.want {
			t.Fatalf("FormatExportWindow(%+v)=%q，期望 %q", item.window, got, item.want)
		}
	}
	// 格式化后可再解析（往返一致）
	window, err := ParseExportWindow(FormatExportWindow(ExportWindow{Start: 82800, End: 21600}))
	if err != nil || window.Start != 82800 || window.End != 21600 {
		t.Fatalf("往返解析不一致: %+v, %v", window, err)
	}
}
