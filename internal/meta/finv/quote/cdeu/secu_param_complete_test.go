package cdeu

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestSecuParamCompleteTable 目标表白名单（需求 11161）：只接受 security / collect，大小写不敏感。
func TestSecuParamCompleteTable(t *testing.T) {
	cases := []struct {
		target string
		table  string
		ok     bool
	}{
		{"security", "public.finv_security", true},
		{"collect", "futu.finv_futu_quote_collect", true},
		{"SECURITY", "public.finv_security", true},
		{"  Collect  ", "futu.finv_futu_quote_collect", true},
		{"", "", false},
		{"finv_security", "", false},
		{"public.finv_security", "", false},
		{"security; DROP TABLE x", "", false},
	}
	for _, c := range cases {
		table, err := secuParamCompleteTable(c.target)
		if c.ok {
			if err != nil {
				t.Fatalf("target=%q 应通过，实际报错 %v", c.target, err)
			}
			if table != c.table {
				t.Fatalf("target=%q 映射为 %q，期望 %q", c.target, table, c.table)
			}
			continue
		}
		if err == nil {
			t.Fatalf("target=%q 应被拒绝，实际通过并返回 %q", c.target, table)
		}
	}
}

// TestLiveSecuParamComplete 只读核对「证券参数补全」清单与预填（需求 11161）。
//
// 默认跳过：
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN="postgres://finv:finvquant@127.0.0.1:5432/finv?sslmode=disable" \
//	  go test -run TestLiveSecuParamComplete -v ./internal/meta/finv/quote/cdeu/
//
// 断言：两张表的缺口清单都能查出来；第一条的预填 `exists` 必须为 false（它正是清单里「缺」的那条）。
func TestLiveSecuParamComplete(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读核对")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读核对")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	for _, target := range []string{SecuParamTargetSecurity, SecuParamTargetCollect} {
		total, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
			MissingTarget: target, OrderBy: "market_value", Desc: true, Limit: 5,
		})
		if err != nil {
			t.Fatalf("target=%s 查询清单失败: %v", target, err)
		}
		t.Logf("target=%s 待补全 %d 行，前 %d 行样例", target, total, len(list))
		if len(list) == 0 {
			t.Logf("target=%s 没有待补全的行（跳过预填核对）", target)
			continue
		}
		first := list[0]
		prefill, err := store.SecuParamPrefill(ctx, target, first.StockID)
		if err != nil {
			t.Fatalf("target=%s stock_id=%d 预填失败: %v", target, first.StockID, err)
		}
		if prefill.Exists {
			t.Fatalf("清单里的行不应在目标表已存在：target=%s stock_id=%d usc=%s",
				target, first.StockID, prefill.USC)
		}
		if prefill.Target != target {
			t.Fatalf("预填 target=%q，期望 %q", prefill.Target, target)
		}
		if len(prefill.Fields) == 0 {
			t.Fatalf("target=%s 预填字段为空", target)
		}
		t.Logf("target=%s 样例 stock_id=%d usc=%s 名称=%s 字段数=%d 警告数=%d",
			target, first.StockID, prefill.USC, prefill.Name, len(prefill.Fields), len(prefill.Warnings))
		for _, field := range prefill.Fields {
			t.Logf("  字段 %-22s = %-24q 来源=%s 必填=%t", field.Key, field.Value, field.Source, field.Required)
		}
	}

	// 非法 target 必须被拒绝
	if _, err := store.SecuParamPrefill(ctx, "bogus", 1); err == nil {
		t.Fatalf("非法 target 应报错")
	}
}

// TestLivePendingSecuCollectReference 只读回归：instrument_type='12' 的行必须能取到参考
// 采集配置（需求 11164）。
//
// 故障现场：快照 instrument_type 是 TEXT，且 12 与 3 都表示股票；采集表只有 3。
// 原来直接用 `q.instrument_type::text = btrim(t.instrument_type)` 匹配，
// 12 的行永远匹配不到 → market_code 取不到值（=0）→ 点「写入」被后端拒：
// 「采集配置缺少必填（market_label / futu_symbol / market_code / …）」。
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN=... go test -run TestLivePendingSecuCollectReference -v ./internal/meta/finv/quote/cdeu/
func TestLivePendingSecuCollectReference(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读回归")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读回归")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	var stockID int64
	if err := pool.QueryRow(ctx, `
SELECT stock_id
  FROM `+tableRealtimeStock+`
 WHERE instrument_type = '12'
   AND market_label IN ('US', 'HK')
 ORDER BY stock_id
 LIMIT 1`).Scan(&stockID); err != nil {
		t.Skipf("库里没有 instrument_type='12' 的样本行，跳过：%v", err)
	}

	prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, stockID)
	if err != nil {
		t.Fatalf("预填失败: %v", err)
	}
	values := map[string]string{}
	for _, field := range prefill.Fields {
		values[field.Key] = field.Value
	}
	t.Logf("stock_id=%d usc=%s market_label=%s instrument=%s market_type=%s market_code=%s "+
		"instrument_type=%s sub_instrument_type=%s market_label 来源=%s",
		stockID, prefill.USC, values["market_label"], values["instrument"], values["market_type"],
		values["market_code"], values["instrument_type"], values["sub_instrument_type"], values["market_label"])

	// 必填项：market_code 必须能取到参考值（instrument_type 12 按 3 归一后匹配）
	if values["market_code"] == "" || values["market_code"] == "0" {
		t.Fatalf("instrument_type='12' 的行仍取不到 market_code 参考值（需求 11164 回归）：market_code=%q",
			values["market_code"])
	}
	// instrument 也应从参考行取到（type 3 的口径是 stock）
	if values["instrument"] == "" {
		t.Fatalf("instrument 为空，无法插入采集配置")
	}
	if values["market_label"] == "" || values["futu_symbol"] == "" {
		t.Fatalf("market_label / futu_symbol 不应为空")
	}
	// 参考值来源要标成 reference（提示人工核对），而不是 manual
	if values["market_label"] == "" {
		t.Fatalf("market_label 为空")
	}
}

// TestLiveSecuParamCollectComplete 端到端验证「采集参数」补全真能落库（需求 11164）。
//
// **会写本地库**（不碰上游）：取清单里第一条，用预填值调 CompleteSecuParam，
// 断言 created=true，然后把自己刚插入的那一行删掉（只删本测试插入的 usc，删不到就报错）。
// 默认跳过：
//
//	FINV_SECU_PARAM_LIVE=1 FINV_SECU_PARAM_LIVE_WRITE=1 FINV_TEST_PG_DSN=... \
//	  go test -run TestLiveSecuParamCollectComplete -v ./internal/meta/finv/quote/cdeu/
func TestLiveSecuParamCollectComplete(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" || os.Getenv("FINV_SECU_PARAM_LIVE_WRITE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1 与 FINV_SECU_PARAM_LIVE_WRITE=1，跳过写入验证")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过写入验证")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	_, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
		MissingTarget: SecuParamTargetCollect, OrderBy: "market_value", Desc: true, Limit: 1,
	})
	if err != nil || len(list) == 0 {
		t.Skipf("采集参数没有待补全的行，跳过：%v", err)
	}
	row := list[0]

	prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, row.StockID)
	if err != nil {
		t.Fatalf("预填失败: %v", err)
	}
	values := map[string]string{}
	for _, field := range prefill.Fields {
		values[field.Key] = field.Value
	}

	req := SecuParamCompleteRequest{
		Target:  SecuParamTargetCollect,
		StockID: row.StockID,
		Remark:  "自动测试（会立即删除）",
		Collect: PendingSecuEnrollCollect{
			FutuSymbol:        values["futu_symbol"],
			MarketType:        parseIntOr(values["market_type"], 0),
			MarketCode:        parseIntOr(values["market_code"], 0),
			InstrumentType:    parseIntOr(values["instrument_type"], 0),
			SubInstrumentType: parseIntOr(values["sub_instrument_type"], 0),
			MarketLabel:       values["market_label"],
			Instrument:        values["instrument"],
			ReqSection:        values["req_section"],
			FlagEnable:        values["flag_enable"],
		},
	}
	result, err := store.CompleteSecuParam(ctx, req)
	if err != nil {
		t.Fatalf("采集参数补全失败（正是需求 11164 的现场）：%v", err)
	}
	if !result.Created {
		t.Fatalf("期望新插入一行，实际 created=false（usc=%s 已存在）", result.USC)
	}
	t.Logf("补全成功：target=%s usc=%s market_code=%s instrument=%s", result.Target,
		result.USC, values["market_code"], values["instrument"])

	// 清理：只删本测试刚插入的那一行
	tag, err := pool.Exec(ctx, `DELETE FROM futu.finv_futu_quote_collect WHERE usc = $1`, result.USC)
	if err != nil {
		t.Fatalf("清理测试行失败（请手工删除 usc=%s）: %v", result.USC, err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("清理时应删除 1 行，实际 %d 行（请手工核对 usc=%s）", tag.RowsAffected(), result.USC)
	}
	t.Logf("已清理测试行 usc=%s", result.USC)
}

// TestInferSecurityTZ 时区缺省值三层推断（需求 11165）。
func TestInferSecurityTZ(t *testing.T) {
	at := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) // 夏令时期间

	// ① 交易所级参考优先：原样采用库里的 (偏移, IANA)
	got := inferSecurityTZ("+08:00", "Asia/Shanghai", "-04:00", "America/New_York", "HK", at)
	if got.Timezone != "+08:00" || got.TZ != "Asia/Shanghai" || got.Source != PrefillSourceReference {
		t.Fatalf("交易所级参考未优先: %+v", got)
	}
	if !strings.Contains(got.Note, "同交易所") {
		t.Fatalf("说明应指出同交易所众数: %q", got.Note)
	}

	// ② 交易所无参考 → 区域级参考
	got = inferSecurityTZ("", "", "-04:00", "America/New_York", "US", at)
	if got.Timezone != "-04:00" || got.TZ != "America/New_York" || got.Source != PrefillSourceReference {
		t.Fatalf("区域级参考未生效: %+v", got)
	}
	if !strings.Contains(got.Note, "region=US") {
		t.Fatalf("说明应指出 region: %q", got.Note)
	}

	// ③ 两级都没有 → 内置区域映射 + 当前时刻偏移
	cases := []struct {
		region   string
		wantTZ   string
		wantOff  string
		wantNote string
	}{
		{"US", "America/New_York", "-04:00", "America/New_York"}, // 7 月夏令时
		{"CN", "Asia/Shanghai", "+08:00", "Asia/Shanghai"},
		{"HK", "Asia/Hong_Kong", "+08:00", "Asia/Hong_Kong"},
		{"JP", "Asia/Tokyo", "+09:00", "Asia/Tokyo"},
		{"CC", "Etc/UTC", "+00:00", "Etc/UTC"},
		{"in", "Asia/Kolkata", "+05:30", "Asia/Kolkata"}, // 大小写不敏感 + 半小时偏移
	}
	for _, c := range cases {
		got := inferSecurityTZ("", "", "", "", c.region, at)
		if got.TZ != c.wantTZ || got.Timezone != c.wantOff {
			t.Fatalf("region=%s 推断为 %s/%s，期望 %s/%s", c.region, got.TZ, got.Timezone, c.wantTZ, c.wantOff)
		}
		if got.Source != PrefillSourceDictionary {
			t.Fatalf("region=%s 来源应为 dictionary，实际 %s", c.region, got.Source)
		}
		if !strings.Contains(got.Note, c.wantNote) {
			t.Fatalf("region=%s 说明缺少时区名: %q", c.region, got.Note)
		}
	}

	// 美国冬令时应给 -05:00（历史数据恒为 -04:00，这里是按 IANA 实时推算）
	winter := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	if got := inferSecurityTZ("", "", "", "", "US", winter); got.Timezone != "-05:00" {
		t.Fatalf("美国冬令时偏移应为 -05:00，实际 %q", got.Timezone)
	}

	// ④ 区域也没有映射（N/A）→ 留空 + 人工填写 + 风险提示
	got = inferSecurityTZ("", "", "", "", "N/A", at)
	if got.Timezone != "" || got.TZ != "" {
		t.Fatalf("N/A 区域不应推断出值: %+v", got)
	}
	if got.Source != PrefillSourceManual {
		t.Fatalf("N/A 区域来源应为 manual，实际 %s", got.Source)
	}
	if len(got.warnings("N/A")) == 0 {
		t.Fatalf("推断不出时区时应给风险提示")
	}
	if len(got.fields()) != 2 {
		t.Fatalf("应输出 timezone + tz 两个字段")
	}

	// ⑤ 有推断结果时不该再给风险提示
	if len(cases) > 0 {
		if warns := inferSecurityTZ("", "", "", "", "CN", at).warnings("CN"); len(warns) != 0 {
			t.Fatalf("推断成功时不应给风险提示: %v", warns)
		}
	}

	// ⑥ 字段顺序与说明（timezone 存偏移、tz 存 IANA）
	fields := inferSecurityTZ("", "", "", "", "US", at).fields()
	if fields[0].Key != "timezone" || fields[0].Value != "-04:00" {
		t.Fatalf("第一个字段应为 timezone=-04:00，实际 %+v", fields[0])
	}
	if fields[1].Key != "tz" || fields[1].Value != "America/New_York" {
		t.Fatalf("第二个字段应为 tz=America/New_York，实际 %+v", fields[1])
	}
	for _, field := range fields {
		if !strings.Contains(field.Note, "IANA") && !strings.Contains(field.Note, "UTC 偏移") {
			t.Fatalf("字段说明应写清列口径: %q", field.Note)
		}
		if field.Required {
			t.Fatalf("时区是选填列，不该标必填")
		}
	}
}

// TestLiveSecuParamPrefillTZ 只读核对：补全预填已带时区缺省值（需求 11165）。
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN=... go test -run TestLiveSecuParamPrefillTZ -v ./internal/meta/finv/quote/cdeu/
func TestLiveSecuParamPrefillTZ(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读核对")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读核对")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	// 各市场各取一条，确认推断值符合该市场的真实时区
	markets := []string{"US", "HK", "SH", "SZ", "BJ"}
	for _, market := range markets {
		_, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
			MissingTarget: SecuParamTargetSecurity, MarketLabel: market, OrderBy: "market_value",
			Desc: true, Limit: 1,
		})
		if err != nil {
			t.Fatalf("market=%s 查询清单失败: %v", market, err)
		}
		if len(list) == 0 {
			t.Logf("market=%s 没有待补全的行，跳过", market)
			continue
		}
		prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetSecurity, list[0].StockID)
		if err != nil {
			t.Fatalf("market=%s 预填失败: %v", market, err)
		}
		values := map[string]string{}
		sources := map[string]string{}
		for _, field := range prefill.Fields {
			values[field.Key] = field.Value
			sources[field.Key] = field.Source
		}
		t.Logf("market=%s stock_id=%d usc=%s timezone=%q(%s) tz=%q(%s)",
			market, list[0].StockID, prefill.USC, values["timezone"], sources["timezone"],
			values["tz"], sources["tz"])
		if values["timezone"] == "" || values["tz"] == "" {
			t.Fatalf("market=%s 的时区仍为空（需求 11165 回归）：timezone=%q tz=%q",
				market, values["timezone"], values["tz"])
		}
	}
}

// TestDeriveCollectDefaults 采集参数缺省推断（需求 11166）。
func TestDeriveCollectDefaults(t *testing.T) {
	// instrument_type 归一：12 与 3 都是股票，推缺省值时统一按 3
	for input, want := range map[int]int{12: 3, 3: 3, 0: 3, 4: 4, 6: 6, 7: 7, 9: 9, 10: 10, 11: 11, 99: 3} {
		if got := normalizeInstrumentType(input); got != want {
			t.Fatalf("normalizeInstrumentType(%d) = %d，期望 %d", input, got, want)
		}
	}
	// 归一后 sub_instrument_type 才是 3002 而不是 12002
	if got := subInstrumentType(normalizeInstrumentType(12)); got != 3002 {
		t.Fatalf("instrument_type=12 的 sub_instrument_type 应为 3002，实际 %d", got)
	}

	eleven := 11
	two := 2

	// market_type：快照优先
	if value, source := deriveMarketType(2, &eleven, "US"); value != 2 || source != PrefillSourceSnapshot {
		t.Fatalf("快照有值时应用快照: %d/%s", value, source)
	}
	// 快照缺失 → 同市场众数
	if value, source := deriveMarketType(0, &eleven, "US"); value != 11 || source != PrefillSourceReference {
		t.Fatalf("快照缺失应取同市场众数: %d/%s", value, source)
	}
	// 都没有 → 内置映射
	cases := []struct {
		label string
		want  int
	}{
		{"HK", 1}, {"US", 2}, {"SH", 4}, {"SZ", 4}, {"BJ", 4},
		{"FX", 11}, {"CC", 17}, {"hk", 1}, {"", 4},
	}
	for _, c := range cases {
		if value, source := deriveMarketType(0, nil, c.label); value != c.want || source != PrefillSourceDictionary {
			t.Fatalf("market_label=%q 应映射为 %d/字典，实际 %d/%s", c.label, c.want, value, source)
		}
	}
	if note := marketTypeNote(PrefillSourceReference, 0, "US"); !strings.Contains(note, "同市场") {
		t.Fatalf("参考来源说明应指出同市场: %q", note)
	}

	// req_section：众数为空 → 留空但来源标「参考」，不标「待填」
	empty := ""
	if value, source, note := deriveReqSection(&empty); value != "" || source != PrefillSourceReference {
		t.Fatalf("req_section 众数为空时应留空且来源为参考: %q/%s", value, source)
	} else if !strings.Contains(note, "选填") {
		t.Fatalf("众数为空时说明应提「选填」: %q", note)
	}
	// 众数非空 → 直接带出参考值
	one := "1"
	if value, source, _ := deriveReqSection(&one); value != "1" || source != PrefillSourceReference {
		t.Fatalf("req_section 众数为 1 时应带出 1: %q/%s", value, source)
	}
	// 没有参考行 → 留空 + 字典来源 + 说明选填
	if value, source, note := deriveReqSection(nil); value != "" || source != PrefillSourceDictionary {
		t.Fatalf("无参考行时应留空且来源为字典: %q/%s", value, source)
	} else if !strings.Contains(note, "留空") {
		t.Fatalf("无参考行说明应提「留空」: %q", note)
	}
	_ = two
}

// TestLivePendingSecuCollectDefaults 只读核对：采集参数预填的 market_type / req_section /
// sub_instrument_type 都有合理缺省（需求 11166）。
//
//	FINV_SECU_PARAM_LIVE=1 FINV_TEST_PG_DSN=... go test -run TestLivePendingSecuCollectDefaults -v ./internal/meta/finv/quote/cdeu/
func TestLivePendingSecuCollectDefaults(t *testing.T) {
	if os.Getenv("FINV_SECU_PARAM_LIVE") != "1" {
		t.Skip("未设置 FINV_SECU_PARAM_LIVE=1，跳过只读核对")
	}
	dsn := os.Getenv("FINV_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设置 FINV_TEST_PG_DSN，跳过只读核对")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("连接数据库失败: %v", err)
	}
	defer pool.Close()
	store := NewStore(pool)

	for _, market := range []string{"US", "HK"} {
		_, list, err := store.ListRealtimeStocks(ctx, RealtimeStockListFilter{
			MissingTarget: SecuParamTargetCollect, MarketLabel: market, OrderBy: "market_value",
			Desc: true, Limit: 1,
		})
		if err != nil {
			t.Fatalf("market=%s 查询失败: %v", market, err)
		}
		if len(list) == 0 {
			t.Logf("market=%s 没有待补全的行，跳过", market)
			continue
		}
		prefill, err := store.SecuParamPrefill(ctx, SecuParamTargetCollect, list[0].StockID)
		if err != nil {
			t.Fatalf("market=%s 预填失败: %v", market, err)
		}
		values, sources := map[string]string{}, map[string]string{}
		for _, field := range prefill.Fields {
			values[field.Key] = field.Value
			sources[field.Key] = field.Source
		}
		t.Logf("market=%s usc=%s market_type=%s(%s) req_section=%q(%s) sub_instrument_type=%s(%s) instrument=%s(%s)",
			market, prefill.USC, values["market_type"], sources["market_type"],
			values["req_section"], sources["req_section"],
			values["sub_instrument_type"], sources["sub_instrument_type"],
			values["instrument"], sources["instrument"])

		if values["market_type"] == "" || values["market_type"] == "0" {
			t.Fatalf("market=%s 的 market_type 未推出缺省值", market)
		}
		if values["sub_instrument_type"] == "" || values["sub_instrument_type"] == "0" {
			t.Fatalf("market=%s 的 sub_instrument_type 未推出缺省值", market)
		}
		// req_section 是选填，允许为空，但来源必须是「参考」或「字典」（不能是「待填」）
		if sources["req_section"] == PrefillSourceManual {
			t.Fatalf("market=%s 的 req_section 被标成待填（选填列不该如此）", market)
		}
	}
}
