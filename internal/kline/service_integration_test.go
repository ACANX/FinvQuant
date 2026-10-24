package kline

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestServiceIntegration 真实数据库集成测试（默认跳过）。
//
// 运行方式（需要可建表的测试库）：
//
//	FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test ./internal/kline -run TestServiceIntegration -v
//
// 测试使用独立的临时表 dedup.finv_quote_secu_kline_min_it，结束后删除，不影响业务表。
func TestServiceIntegration(t *testing.T) {
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过数据库集成测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("测试库不可达: %v", err)
	}

	const testTable = "dedup.finv_quote_secu_kline_min_it"
	if _, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS dedup`); err != nil {
		t.Fatalf("创建 dedup schema 失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS `+testTable); err != nil {
		t.Fatalf("清理旧测试表失败: %v", err)
	}
	if _, err := pool.Exec(ctx, `
CREATE TABLE `+testTable+` (
    usc           text NOT NULL,
    ts            bigint NOT NULL,
    date          integer,
    "time"        integer,
    open          numeric(20,6),
    high          numeric(20,6),
    low           numeric(20,6),
    close         numeric(20,6),
    volume        numeric(20,6),
    turnover      numeric(30,8),
    change_price  numeric(20,6),
    change_ratio  numeric(20,6),
    paocd         numeric(20,6),
    ext_field     text,
    remark        text,
    provider      varchar(255),
    gmt_create    timestamptz NOT NULL DEFAULT now(),
    gmt_update    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, ts)
)`); err != nil {
		t.Fatalf("创建测试表失败: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS `+testTable)
	})

	service := NewService(pool, Options{MaxBatchRows: 100, MaxListSize: 100, TSUnit: "s"})
	service.table = testTable

	items := func(raw string) []ItemIn {
		t.Helper()
		var in BatchIn
		if err := DecodeJSON([]byte(raw), &in); err != nil {
			t.Fatalf("解析测试数据失败: %v", err)
		}
		return in.Items
	}

	// 单条插入 + 重复冲突
	row, err := service.Add(ctx, items(`{"items":[{"usc":"IT001","ts":1792373400,"close":10.5,"provider":"IT"}]}`)[0])
	if err != nil {
		t.Fatalf("Add 失败: %v", err)
	}
	if row.Usc != "IT001" || row.Ts != 1792373400 {
		t.Fatalf("Add 返回值异常: %+v", row)
	}
	if _, err := service.Add(ctx, items(`{"items":[{"usc":"IT001","ts":1792373400}]}`)[0]); err == nil {
		t.Fatal("重复键应返回错误")
	} else if AsError(err).Code != CodeDuplicateKey {
		t.Fatalf("重复键错误码期望 %d，实际 %d", CodeDuplicateKey, AsError(err).Code)
	}

	// 批量插入 + 请求内重复拦截
	batch := items(`{"items":[
		{"usc":"IT002","ts":1792373400,"close":1},
		{"usc":"IT003","ts":1792373400,"close":2}]}`)
	if n, err := service.BatchInsert(ctx, batch); err != nil || n != 2 {
		t.Fatalf("BatchInsert 失败: n=%d err=%v", n, err)
	}
	if _, err := service.BatchInsert(ctx, []ItemIn{batch[0], batch[0]}); err == nil {
		t.Fatal("请求内重复应返回错误")
	} else if AsError(err).Code != CodeDuplicateInRequest {
		t.Fatalf("请求内重复错误码期望 %d，实际 %d", CodeDuplicateInRequest, AsError(err).Code)
	}

	// upsert skip / update
	if n, err := service.Upsert(ctx, batch, "skip"); err != nil || n != 0 {
		t.Fatalf("upsert skip 期望 0，实际 n=%d err=%v", n, err)
	}
	update := items(`{"items":[{"usc":"IT002","ts":1792373400,"close":9.9}]}`)
	if n, err := service.Upsert(ctx, update, "update"); err != nil || n != 1 {
		t.Fatalf("upsert update 期望 1，实际 n=%d err=%v", n, err)
	}

	// 分页 / 行数组 / 批量查询
	usc := "IT002"
	cnt, rows, err := service.Page(ctx, ListFilter{Usc: &usc}, 10, 0)
	if err != nil || cnt != 1 || len(rows) != 1 {
		t.Fatalf("Page 异常: cnt=%d len=%d err=%v", cnt, len(rows), err)
	}
	if rows[0].Close == nil || cleanNumericLiteral(*rows[0].Close) != "9.9" {
		t.Fatalf("upsert 后 close 异常: %v", rows[0].Close)
	}
	uni := rows[0].UniRow()
	if len(uni) != len(CompactFields()) {
		t.Fatalf("UniRow 列数异常: %d", len(uni))
	}

	qCount, qRows, err := service.Query(ctx, QuerySpec{
		Keys: []Key{{Usc: "IT001", Ts: 1792373400}}, Limit: 10,
	})
	if err != nil || qCount != 1 || len(qRows) != 1 {
		t.Fatalf("Query 异常: cnt=%d len=%d err=%v", qCount, len(qRows), err)
	}

	// 增量更新（只改 volume）
	patchItems := items(`{"items":[{"usc":"IT001","ts":1792373400,"volume":8888}]}`)
	if n, err := service.PatchBatch(ctx, patchItems); err != nil || n != 1 {
		t.Fatalf("PatchBatch 期望 1，实际 n=%d err=%v", n, err)
	}

	// 计数 + 删除
	if total, err := service.Count(ctx, CountFilter{Provider: strPtr("IT")}); err != nil || total != 1 {
		t.Fatalf("Count 异常: total=%d err=%v", total, err)
	}
	if n, err := service.DeleteBatch(ctx, nil, []string{"IT002", "IT003"}); err != nil || n != 2 {
		t.Fatalf("DeleteBatch 期望 2，实际 n=%d err=%v", n, err)
	}
	if total, err := service.Count(ctx, CountFilter{}); err != nil || total != 1 {
		t.Fatalf("收尾 Count 期望 1，实际 total=%d err=%v", total, err)
	}
}
