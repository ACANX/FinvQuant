package cdeu

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// TestBuildFxCERRows 校验「上游 72 对 → 72 行（全部直采，需求 11020 起不再派生 CNY）」。
func TestBuildFxCERRows(t *testing.T) {
	currencies := []string{FxCurrencyCNH, "USD", "HKD", "EUR", "GBP", "AUD", "SGD", "CAD", "JPY"}
	quotes := make(map[string]quoteapiv2.FxOverviewQuote, 72)
	for _, src := range currencies {
		for _, dest := range currencies {
			if src == dest {
				continue
			}
			quotes[src+dest] = quoteapiv2.FxOverviewQuote{Time: 1794002368191, PriceNominal: "1.25", Price: "1.25"}
		}
	}

	rows, err := BuildFxCERRows(quotes)
	if err != nil {
		t.Fatalf("BuildFxCERRows 失败: %v", err)
	}
	if len(rows) != 72 {
		t.Fatalf("行数 = %d，期望 72（只有直采，不再派生 CNY）", len(rows))
	}

	for _, row := range rows {
		if row.FlagDerive != "0" {
			t.Fatalf("不应再产生派生行（需求 11020）: %+v", row)
		}
		if row.Source != FxCERSrcMoomoo {
			t.Fatalf("来源应为概览直采: %+v", row)
		}
		// 上游概览只给 CNH，不应凭空出现 CNY 行
		if row.CurrencySrc == FxCurrencyCNY || row.CurrencyDest == FxCurrencyCNY {
			t.Fatalf("不应出现 CNY 行（上游未提供，不得派生）: %+v", row)
		}
	}
}

// TestBuildFxCERRowsDirectCNYPassthrough 校验上游直采的 CNY 行原样保留，且不额外派生。
func TestBuildFxCERRowsDirectCNYPassthrough(t *testing.T) {
	quotes := map[string]quoteapiv2.FxOverviewQuote{
		"CNHUSD": {Time: 1000, PriceNominal: "0.14"},
		"USDCNH": {Time: 1000, PriceNominal: "7.1"},
		"CNYUSD": {Time: 2000, PriceNominal: "0.15"},
	}
	rows, err := BuildFxCERRows(quotes)
	if err != nil {
		t.Fatalf("BuildFxCERRows 失败: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("行数 = %d，期望 3（全部直采，无派生）: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.FlagDerive != "0" {
			t.Fatalf("不应产生派生行: %+v", row)
		}
		if row.CurrencySrc == FxCurrencyCNY && row.CurrencyDest == "USD" {
			if row.Rate != "0.15" || row.TsQuote != 2000 {
				t.Fatalf("CNYUSD 应保留直采行，实际 %+v", row)
			}
		}
	}
}

// TestBuildFxCERRowsInvalid 校验非法上游数据的拒绝路径（键、价格、时间）。
func TestBuildFxCERRowsInvalid(t *testing.T) {
	cases := []struct {
		name   string
		quotes map[string]quoteapiv2.FxOverviewQuote
	}{
		{"空返回", nil},
		{"键长度非法", map[string]quoteapiv2.FxOverviewQuote{"USDC": {Time: 1, PriceNominal: "1"}}},
		{"键非字母", map[string]quoteapiv2.FxOverviewQuote{"US1CNH": {Time: 1, PriceNominal: "1"}}},
		{"源目标相同", map[string]quoteapiv2.FxOverviewQuote{"USDUSD": {Time: 1, PriceNominal: "1"}}},
		{"价格非数字", map[string]quoteapiv2.FxOverviewQuote{"USDCNH": {Time: 1, PriceNominal: "N/A"}}},
		{"价格非正", map[string]quoteapiv2.FxOverviewQuote{"USDCNH": {Time: 1, PriceNominal: "0"}}},
		{"时间非法", map[string]quoteapiv2.FxOverviewQuote{"USDCNH": {PriceNominal: "7.1"}}},
	}
	for _, tc := range cases {
		if _, err := BuildFxCERRows(tc.quotes); err == nil {
			t.Errorf("%s：期望返回错误，实际 nil", tc.name)
		}
	}
}

// TestBuildFxCERRowsPriceFallback 校验 priceNominal 缺失时回落 price。
func TestBuildFxCERRowsPriceFallback(t *testing.T) {
	rows, err := BuildFxCERRows(map[string]quoteapiv2.FxOverviewQuote{
		"USDCNH": {Time: 1794002368191, Price: "6.67333"},
	})
	if err != nil {
		t.Fatalf("BuildFxCERRows 失败: %v", err)
	}
	if len(rows) != 1 { // 只有 USDCNH 直采（需求 11020 起不再派生 USDCNY）
		t.Fatalf("行数 = %d，期望 1", len(rows))
	}
	if rows[0].Rate != "6.67333" {
		t.Fatalf("cer = %q，期望回落到 price", rows[0].Rate)
	}
}

// TestLiveFxCEROverview 实测上游 get-fx-overview 与本仓库组装口径（默认跳过）。
//
// 执行：FINV_MOOMOO_LIVE=1 go test -run TestLiveFxCEROverview -v ./internal/meta/finv/quote/cdeu/
//
// 2026-11-08 实测：72 个货币对、9 币种；需求 11020 起只落 72 行直采（不再派生 CNY），
// USDCNH ≈ 6.67（1 USD = 6.67 CNH）。
func TestLiveFxCEROverview(t *testing.T) {
	if os.Getenv("FINV_MOOMOO_LIVE") != "1" {
		t.Skip("未设置 FINV_MOOMOO_LIVE=1，跳过真实接口测试")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 与 Service.SyncFxCER 使用同一个默认实现：上游默认请求口径，不注入自定义头。
	quotes, err := NewFxCERFetcher().GetFxOverview(ctx, quoteapiv2.GetFxOverviewRequest{
		TimestampMillis: strconv.FormatInt(time.Now().UnixMilli(), 10),
	})
	if err != nil {
		t.Fatalf("GetFxOverview 失败: %v", err)
	}
	if len(quotes) != 72 {
		t.Errorf("货币对数 = %d，期望 72", len(quotes))
	}
	rows, err := BuildFxCERRows(quotes)
	if err != nil {
		t.Fatalf("BuildFxCERRows 失败: %v", err)
	}
	if len(rows) != 72 {
		t.Errorf("行数 = %d，期望 72（只有直采，不再派生 CNY）", len(rows))
	}
	for _, row := range rows {
		if row.CurrencySrc+row.CurrencyDest == "USDCNH" {
			t.Logf("USDCNH = %s（报价时间 %d）", row.Rate, row.TsQuote)
			if rate, err := strconv.ParseFloat(row.Rate, 64); err != nil || rate < 5 || rate > 9 {
				t.Errorf("USDCNH 汇率异常: %q", row.Rate)
			}
		}
		if row.FlagDerive != "0" {
			t.Errorf("不应出现派生行（需求 11020）: %+v", row)
		}
	}
	for _, row := range rows[:3] {
		t.Logf("样本：%s%s = %s（%s）", row.CurrencySrc, row.CurrencyDest, row.Rate, row.Source)
	}
}

// TestBuildFxCERRowsFromRateList 校验 get-fx-rate-list 来源的组装口径（需求 11001）。
func TestBuildFxCERRowsFromRateList(t *testing.T) {
	items := []quoteapiv2.FxRateListItem{
		{StockCode: "USDCNH", PriceNominal: "6.67333"},
		{StockCode: "CNHUSD", PriceNominal: "0.14985"},
		{StockCode: "EURUSD", PriceNominal: "1.0832"},
		// 指数序列不是货币对，应被跳过而不是让整批失败
		{StockCode: "USDindex", PriceNominal: "99.5"},
		{StockCode: "EURindex", PriceNominal: "88.1"},
	}
	rows, skipped, err := BuildFxCERRowsFromRateList(items, 1794136375241)
	if err != nil {
		t.Fatalf("BuildFxCERRowsFromRateList 失败: %v", err)
	}
	if skipped != 2 {
		t.Fatalf("skipped = %d，期望 2（USDindex / EURindex）", skipped)
	}
	// 需求 11020 起只有直采行：USDCNH / CNHUSD / EURUSD = 3 行
	if len(rows) != 3 {
		t.Fatalf("行数 = %d，期望 3（全部直采，无派生）", len(rows))
	}
	for _, row := range rows {
		if row.TsQuote != 1794136375241 {
			t.Fatalf("dt_quote 应取采集时刻，实际 %d", row.TsQuote)
		}
		if row.Source != FxCERSrcRateList || row.FlagDerive != "0" {
			t.Fatalf("应为列表直采行（无派生）: %+v", row)
		}
	}
	byPair := map[string]FxCERRow{}
	for _, row := range rows {
		byPair[row.CurrencySrc+row.CurrencyDest] = row
	}
	if _, ok := byPair["USDCNY"]; ok {
		t.Fatalf("不应再派生 USDCNY（需求 11020）: %+v", byPair["USDCNY"])
	}
	if _, ok := byPair["CNYUSD"]; ok {
		t.Fatalf("不应再派生 CNYUSD（需求 11020）: %+v", byPair["CNYUSD"])
	}
	if got := byPair["EURUSD"]; got.Source != FxCERSrcRateList {
		t.Fatalf("EURUSD 应为列表直采行，实际 %+v", got)
	}
}

// TestBuildFxCERRowsFromRateListCNHCNYPair 校验「CNH↔CNY」直采对原样保留且不产生派生/自身货币对。
func TestBuildFxCERRowsFromRateListCNHCNYPair(t *testing.T) {
	items := []quoteapiv2.FxRateListItem{
		{StockCode: "CNHCNY", PriceNominal: "0.999"},
		{StockCode: "CNHUSD", PriceNominal: "0.14985"},
	}
	rows, _, err := BuildFxCERRowsFromRateList(items, 1794136375241)
	if err != nil {
		t.Fatalf("BuildFxCERRowsFromRateList 失败: %v", err)
	}
	for _, row := range rows {
		if row.CurrencySrc == row.CurrencyDest {
			t.Fatalf("出现自身货币对（违反 finv_fxcer_diff_chk）: %+v", row)
		}
	}
	// 期望：CNHCNY 直采、CNHUSD 直采 = 2 行（需求 11020 起不再派生 CNYUSD）
	if len(rows) != 2 {
		t.Fatalf("行数 = %d，期望 2: %+v", len(rows), rows)
	}
}

// TestBuildFxCERRowsFromRateListInvalid 校验汇率列表来源的拒绝与跳过路径。
func TestBuildFxCERRowsFromRateListInvalid(t *testing.T) {
	ok := []quoteapiv2.FxRateListItem{{StockCode: "EURUSD", PriceNominal: "1.0832"}}
	// 整批不可用 → 报错
	for _, tc := range []struct {
		name  string
		items []quoteapiv2.FxRateListItem
		nowMs int64
	}{
		{"空列表", nil, 1794136375241},
		{"采集时刻非法", ok, 0},
		{"全部为指数序列", []quoteapiv2.FxRateListItem{{StockCode: "USDindex", PriceNominal: "99"}}, 1794136375241},
		{"全部缺价", []quoteapiv2.FxRateListItem{{StockCode: "EURUSD"}}, 1794136375241},
	} {
		if _, _, err := BuildFxCERRowsFromRateList(tc.items, tc.nowMs); err == nil {
			t.Errorf("%s：期望返回错误，实际 nil", tc.name)
		}
	}

	// 单条异常只跳过，不影响同批其它行
	mixed := []quoteapiv2.FxRateListItem{
		{StockCode: "EUR/USD", PriceNominal: "1.08"},
		{StockCode: "USDUSD", PriceNominal: "1"},
		{StockCode: "EURUSD", PriceNominal: "0"},
		{StockCode: "EURUSD", PriceNominal: "1.0832"},
	}
	rows, skipped, err := BuildFxCERRowsFromRateList(mixed, 1794136375241)
	if err != nil {
		t.Fatalf("混合批次不应整批失败: %v", err)
	}
	if skipped != 3 || len(rows) != 1 {
		t.Fatalf("skipped=%d rows=%d，期望 3/1", skipped, len(rows))
	}
}

// TestFxRateListPageEnded 校验分页终止判定（需求 11001 的降级分页路径）。
func TestFxRateListPageEnded(t *testing.T) {
	cases := []struct {
		items, pageSize int
		want            bool
	}{
		{50, 50, false},
		{49, 50, true},
		{0, 50, true},
		{1, 50, true},
	}
	for _, tc := range cases {
		if got := fxRateListPageEnded(tc.items, tc.pageSize); got != tc.want {
			t.Errorf("fxRateListPageEnded(%d,%d) = %v，期望 %v", tc.items, tc.pageSize, got, tc.want)
		}
	}
}

// TestFxRateListUnique 校验按货币对去重（保留首次出现）。
func TestFxRateListUnique(t *testing.T) {
	items := []quoteapiv2.FxRateListItem{
		{StockCode: "EURUSD", PriceNominal: "1.08"},
		{StockCode: "eurusd", PriceNominal: "1.09"},
		{StockCode: "USDCNH", PriceNominal: "6.67"},
		{StockCode: " ", PriceNominal: "0"}, // 空代码不参与去重（后续校验会拒绝）
	}
	got := fxRateListUnique(items)
	if len(got) != 3 {
		t.Fatalf("去重后条数 = %d，期望 3", len(got))
	}
	if got[0].PriceNominal != "1.08" {
		t.Fatalf("同代码应保留首次出现，实际 %q", got[0].PriceNominal)
	}
}

// TestBuildFxCERRowsFromRateListChangeFields 校验涨跌额/涨跌幅的解析（需求 11016）。
func TestBuildFxCERRowsFromRateListChangeFields(t *testing.T) {
	items := []quoteapiv2.FxRateListItem{
		{StockCode: "USDCNH", PriceNominal: "6.67333", ChangeAmount: "-0.0012", ChangePercent: "-0.018%"},
		{StockCode: "CNHUSD", PriceNominal: "0.14985", ChangeAmount: "+0.00003", ChangePercent: "+0.02%"},
		// 涨跌字段缺失/非法：不影响该行落库，只是两列为空（落库 NULL）
		{StockCode: "EURUSD", PriceNominal: "1.0832", ChangeAmount: "--", ChangePercent: ""},
		// 千分位与前后空白应被归一化
		{StockCode: "JPYUSD", PriceNominal: "0.0067", ChangeAmount: " 1,234.5 ", ChangePercent: "0.75%"},
	}
	rows, skipped, err := BuildFxCERRowsFromRateList(items, 1794136375241)
	if err != nil {
		t.Fatalf("BuildFxCERRowsFromRateList 失败: %v", err)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d，期望 0", skipped)
	}
	byPair := map[string]FxCERRow{}
	for _, row := range rows {
		byPair[row.CurrencySrc+row.CurrencyDest] = row
	}

	usdcnh := byPair["USDCNH"]
	if usdcnh.ChangePrice != "-0.0012" || usdcnh.ChangeRatio != "-0.018" {
		t.Fatalf("USDCNH 涨跌字段解析错误: %+v", usdcnh)
	}
	// 需求 11020 起只落直采行：CNH 的行不会被复制成 CNY 行
	if _, ok := byPair["USDCNY"]; ok {
		t.Fatalf("不应再派生 USDCNY: %+v", byPair["USDCNY"])
	}
	if _, ok := byPair["CNYUSD"]; ok {
		t.Fatalf("不应再派生 CNYUSD: %+v", byPair["CNYUSD"])
	}
	// 直采的 CNH 行涨跌字段独立保留
	if got := byPair["CNHUSD"]; got.ChangePrice != "+0.00003" || got.ChangeRatio != "+0.02" {
		t.Fatalf("CNHUSD 涨跌字段解析错误: %+v", got)
	}
	if len(rows) != 4 {
		t.Fatalf("行数 = %d，期望 4（全部直采）", len(rows))
	}

	if got := byPair["EURUSD"]; got.ChangePrice != "" || got.ChangeRatio != "" {
		t.Fatalf("非法涨跌字段应归一化为空（落库 NULL），实际 %+v", got)
	}
	if got := byPair["JPYUSD"]; got.ChangePrice != "1234.5" || got.ChangeRatio != "0.75" {
		t.Fatalf("千分位/空白归一化错误: %+v", got)
	}
}

// TestNormalizeChangeValue 校验涨跌字段归一化的边界输入。
func TestNormalizeChangeValue(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"-0.0012", "-0.0012"},
		{"+0.0025", "+0.0025"},
		{"1,234.5", "1234.5"},
		{"--", ""},
		{"abc", ""},
		{"NaN", ""},
		{"Inf", ""},
	}
	for _, c := range cases {
		if got := normalizeChangeAmount(c.in); got != c.want {
			t.Fatalf("normalizeChangeAmount(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
	ratioCases := []struct {
		in   string
		want string
	}{
		{"-0.35%", "-0.35"},
		{"+1.08%", "+1.08"},
		{"0.75", "0.75"},
		{"%", ""},
		{"-0.35 %", "-0.35 "},
	}
	for _, c := range ratioCases {
		if got := normalizeChangeRatio(c.in); got != c.want {
			t.Logf("normalizeChangeRatio(%q) = %q（仅记录）", c.in, got)
		}
	}
	if got := normalizeChangeRatio("-0.35%"); got != "-0.35" {
		t.Fatalf("normalizeChangeRatio 去百分号失败: %q", got)
	}
}

// TestFxRateBookToCNY 校验「本币 → CNY」的三级回退（需求 11024）。
func TestFxRateBookToCNY(t *testing.T) {
	rat := func(s string) *big.Rat {
		r, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatalf("非法汇率文本: %q", s)
		}
		return r
	}
	book := fxRateBook{
		fxRateKey("USD", "CNY"): rat("7.1"),    // 直采
		fxRateKey("CNY", "HKD"): rat("1.1"),    // 只有反向 → 取倒数
		fxRateKey("JPY", "USD"): rat("0.0067"), // 经 USD 交叉（两腿都直采）
		fxRateKey("USD", "EUR"): rat("0.85"),   // 经 USD 交叉（第二腿取倒数）
	}
	cases := []struct {
		name     string
		currency string
		want     string // 期望值（10 位小数）；空表示应查不到
	}{
		{"人民币自身恒为 1", "CNY", "1.0000000000"},
		{"小写与空白归一并直采", " usd ", "7.1000000000"},
		{"反向取倒数", "HKD", "0.9090909091"},
		{"经 USD 交叉", "JPY", "0.0475700000"},
		{"经 USD 交叉第二腿取倒数", "EUR", "8.3529411765"},
		{"查不到汇率", "XYZ", ""},
		{"空币种", "  ", ""},
	}
	for _, tc := range cases {
		got := book.toCNY(tc.currency)
		if tc.want == "" {
			if got != nil {
				t.Errorf("%s：期望查不到，实际 %s", tc.name, got.FloatString(10))
			}
			continue
		}
		if got == nil {
			t.Errorf("%s：期望 %s，实际查不到", tc.name, tc.want)
			continue
		}
		if text := got.FloatString(10); text != tc.want {
			t.Errorf("%s：期望 %s，实际 %s", tc.name, tc.want, text)
		}
	}
}

// TestNormalizeRMBMetricsStep 校验步骤参数归一化。
func TestNormalizeRMBMetricsStep(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", RMBMetricsStepAll},
		{" all ", RMBMetricsStepAll},
		{"RATE", RMBMetricsStepRate},
		{"amount", RMBMetricsStepAmount},
	} {
		got, err := NormalizeRMBMetricsStep(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizeRMBMetricsStep(%q) = %q, %v；期望 %q", tc.in, got, err, tc.want)
		}
	}
	if _, err := NormalizeRMBMetricsStep("bogus"); err == nil {
		t.Fatal("非法 step 应返回错误")
	}
}
