package indicator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestLiveIndicatorTypeSecuSub 关联 public.finv_security 取 type_secu_sub 与按它筛选（需求 11305，默认跳过）。
//
// 需要可读 PG：
//
//	FINV_INDICATOR_LIVE=1 \
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveIndicatorTypeSecuSub -v ./internal/meta/finv/quote/indicator/
func TestLiveIndicatorTypeSecuSub(t *testing.T) {
	if os.Getenv("FINV_INDICATOR_LIVE") != "1" {
		t.Skip("未设置 FINV_INDICATOR_LIVE=1，跳过指标关联列真实库测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("FINV_TEST_PG_DSN"))
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	defer pool.Close()
	reader := NewReader(pool)

	// 1) /Columns 末尾必须带虚拟列
	columns, err := reader.Columns(ctx)
	if err != nil {
		t.Fatalf("读取列元数据失败: %v", err)
	}
	found := false
	for _, column := range columns {
		if column.Name == IndicatorVirtualTypeSecuSub {
			found = true
			if !column.Sortable {
				t.Fatalf("虚拟列应可排序")
			}
		}
	}
	if !found {
		t.Fatalf("列元数据缺少虚拟列 %s", IndicatorVirtualTypeSecuSub)
	}

	// 2) 选项接口有数据，且 Stock.US 行数与筛选 total 一致
	options, err := reader.TypeSecuSub(ctx)
	if err != nil {
		t.Fatalf("读取子类选项失败: %v", err)
	}
	if len(options) < 10 {
		t.Fatalf("子类选项过少（%d），疑似关联失效", len(options))
	}
	var stockUSRows int64
	for _, option := range options {
		if option.Value == "Stock.US" {
			stockUSRows = option.Rows
		}
	}
	if stockUSRows == 0 {
		t.Skip("库里没有 Stock.US 行，跳过后半段断言")
	}

	total, list, err := reader.Page(ctx, IndicatorPageFilter{
		TypeSecuSub: "Stock.US",
		Columns:     []string{IndicatorVirtualTypeSecuSub},
		OrderBy:     IndicatorVirtualTypeSecuSub,
		Desc:        true,
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("按子类筛选失败: %v", err)
	}
	if total != stockUSRows {
		t.Fatalf("筛选 total=%d，选项 rows=%d，两者应一致", total, stockUSRows)
	}
	for _, row := range list {
		if row[IndicatorVirtualTypeSecuSub] != "Stock.US" {
			t.Fatalf("返回行的子类不符: %+v", row)
		}
	}

	// 3) 空值筛选（__empty__）：未关联 / 未维护
	emptyTotal, _, err := reader.Page(ctx, IndicatorPageFilter{
		TypeSecuSub: IndicatorTypeSecuSubEmpty,
		Columns:     []string{IndicatorVirtualTypeSecuSub},
		Limit:       1,
	})
	if err != nil {
		t.Fatalf("按空子类筛选失败: %v", err)
	}
	if emptyTotal == 0 {
		t.Fatalf("库里应有未维护/未关联的行")
	}
	t.Logf("验证通过：%d 个子类选项，Stock.US=%d 行，空子类=%d 行",
		len(options), stockUSRows, emptyTotal)
}
