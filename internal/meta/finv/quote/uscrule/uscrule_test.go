package uscrule

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestNormalizeEnv 空/空白环境一律按 * 处理（与条目层 scope_env 约定一致）。
func TestNormalizeEnv(t *testing.T) {
	cases := map[string]string{
		"":      DefaultEnv,
		"   ":   DefaultEnv,
		"prod":  "prod",
		" dev ": "dev",
	}
	for in, want := range cases {
		if got := normalizeEnv(in); got != want {
			t.Errorf("normalizeEnv(%q)=%q，期望 %q", in, got, want)
		}
	}
}

// TestNormalizePager 分页兜底：页码/页大小非法时回落，页大小上限 200。
func TestNormalizePager(t *testing.T) {
	cases := []struct{ page, size, wantPage, wantSize int }{
		{0, 0, 1, 20},
		{-1, -5, 1, 20},
		{3, 50, 3, 50},
		{1, 9999, 1, 200},
	}
	for _, c := range cases {
		page, size := normalizePager(c.page, c.size)
		if page != c.wantPage || size != c.wantSize {
			t.Errorf("normalizePager(%d,%d)=(%d,%d)，期望 (%d,%d)",
				c.page, c.size, page, size, c.wantPage, c.wantSize)
		}
	}
}

// TestNextRun 6 段 cron 的下一次执行时间；空串与非法表达式返回空。
func TestNextRun(t *testing.T) {
	from := time.Date(2026, 11, 10, 4, 0, 0, 0, time.Local)
	if got := NextRun("0 30 3 * * *", from); got != "2026-11-11T03:30:00+08:00" && got != "2026-11-11T03:30:00+09:00" {
		// 时区随服务器，只校验「日期是次日、时间为 03:30」。
		parsed, err := time.Parse(time.RFC3339, got)
		if err != nil {
			t.Fatalf("NextRun 返回非法时间 %q: %v", got, err)
		}
		if parsed.Hour() != 3 || parsed.Minute() != 30 || parsed.Day() != 11 {
			t.Errorf("NextRun 期望次日 03:30，实际 %q", got)
		}
	}
	if got := NextRun("", from); got != "" {
		t.Errorf("空表达式应返回空串，实际 %q", got)
	}
	if got := NextRun("不是cron", from); got != "" {
		t.Errorf("非法表达式应返回空串，实际 %q", got)
	}
	if ParseSchedule("0 30 3 * * *") == nil {
		t.Error("合法表达式应解析成功")
	}
}

// TestPreviewLive 对**真实库**做只读预演（不写任何表）。
//
// 默认跳过；需要只读验证时：
//
//	FINV_USC_LIVE_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test ./internal/meta/finv/quote/uscrule -run TestPreviewLive -v
//
// 断言口径与设计文档一致：预演行数 = 键表行数、冲突为空、令牌非空、分类计数之和 = 总行数。
func TestPreviewLive(t *testing.T) {
	dsn := os.Getenv("FINV_USC_LIVE_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_USC_LIVE_DSN，跳过只读预演测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()

	var keyRows int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_secu_key`).Scan(&keyRows); err != nil {
		t.Fatalf("统计键表失败: %v", err)
	}

	service := NewService(pool)
	preview, err := service.Preview(ctx, DefaultEnv)
	if err != nil {
		t.Fatalf("预演失败: %v", err)
	}
	t.Logf("预演：total=%d added=%d changed=%d removed=%d reused=%d unchanged=%d escalated=%d token=%s",
		preview.Total, preview.Added, preview.Changed, preview.Removed, preview.Reused,
		preview.Unchanged, preview.Escalated, preview.PreviewToken)

	if preview.Total != keyRows {
		t.Errorf("预演行数 %d 与键表行数 %d 不一致", preview.Total, keyRows)
	}
	if preview.Added+preview.Changed+preview.Unchanged != preview.Total {
		t.Errorf("分类计数之和 %d 与总行数 %d 不一致",
			preview.Added+preview.Changed+preview.Unchanged, preview.Total)
	}
	if len(preview.Conflicts) != 0 {
		t.Errorf("预演报告了 usc 冲突: %v", preview.Conflicts)
	}
	if preview.PreviewToken == "" {
		t.Error("预演令牌为空")
	}
	if len(preview.Samples) > MaxPreviewSamples {
		t.Errorf("预演明细 %d 行超过上限 %d", len(preview.Samples), MaxPreviewSamples)
	}
	if preview.RuleVersion == "" {
		t.Error("规则版本为空")
	}
}
