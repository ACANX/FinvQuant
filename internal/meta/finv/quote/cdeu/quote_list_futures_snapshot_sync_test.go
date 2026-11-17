package cdeu

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件覆盖「期货列表快照同步」（需求 11274）的纯逻辑：
// 分页/计划数量、失败重试、分类过滤、下架标记时机、字段映射口径。
// 全部用假 fetcher / 假 sink / 假配置，不依赖网络与数据库。

// fakeFutureConfigStore 假配置存储：读固定配置，记录每次回写的进度。
type fakeFutureConfigStore struct {
	configs  []FutureListCollectConfig
	progress []FutureListProgress
}

func (f *fakeFutureConfigStore) LoadFuturesConfigs(context.Context) ([]FutureListCollectConfig, error) {
	return f.configs, nil
}

func (f *fakeFutureConfigStore) SaveFuturesProgress(_ context.Context, _ FutureListCollectConfig, progress FutureListProgress) error {
	f.progress = append(f.progress, progress)
	return nil
}

// fakeFutureFetcher 假接口：按「分类/页码」返回预设页；failPages 里的页前 N 次失败。
type fakeFutureFetcher struct {
	pages     map[string]quoteapiv2.FuturesListPage
	failPages map[string]int
	calls     []quoteapiv2.GetFuturesListRequest
}

func (f *fakeFutureFetcher) GetFuturesList(_ context.Context, req quoteapiv2.GetFuturesListRequest) (quoteapiv2.FuturesListPage, error) {
	f.calls = append(f.calls, req)
	key := fmt.Sprintf("%s/%s", req.FuturesType, req.Page)
	if left, ok := f.failPages[key]; ok && left > 0 {
		f.failPages[key] = left - 1
		return quoteapiv2.FuturesListPage{}, errors.New("模拟接口失败")
	}
	page, ok := f.pages[key]
	if !ok {
		return quoteapiv2.FuturesListPage{}, fmt.Errorf("未预置的页: %s", key)
	}
	return page, nil
}

// fakeFutureSink 假落库：记录调用与行数，可注入下架返回行数。
type fakeFutureSink struct {
	saved        []FutureListSavedRow
	offlineCalls int
	offlineMs    []int64
	offlineRows  int64
	saveErrOn    map[int]error
}

func (f *fakeFutureSink) SaveFuturesListPage(_ context.Context, _ FutureListCollectConfig, page int,
	items []quoteapiv2.FuturesListItem) ([]FutureListSavedRow, error) {
	if err, ok := f.saveErrOn[page]; ok {
		return nil, err
	}
	rows := make([]FutureListSavedRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, FutureListSavedRow{
			StockID:      item.StockID,
			ContractCode: item.Fields[futureFieldContractCode],
		})
	}
	f.saved = append(f.saved, rows...)
	return rows, nil
}

func (f *fakeFutureSink) MarkFuturesOffline(_ context.Context, notSeenBeforeMs int64) (int64, error) {
	f.offlineCalls++
	f.offlineMs = append(f.offlineMs, notSeenBeforeMs)
	return f.offlineRows, nil
}

// futureListTestItem 构造一条最小可用的期货条目。
func futureListTestItem(stockID int64, code, market string) quoteapiv2.FuturesListItem {
	return quoteapiv2.FuturesListItem{
		StockID:     stockID,
		PriceDirect: "up",
		FieldList: []quoteapiv2.FuturesField{
			{FieldID: 1, FieldName: "合约代码", Text: code, ContentType: 4},
			{FieldID: 3, FieldName: "价格", Value: "4413800000000", Precision: 1, ContentType: 2},
		},
		Fields: map[string]string{
			"1":  code,
			"2":  "黄金",
			"3":  "4,413.8",
			"4":  "-0.25%",
			"19": market,
		},
	}
}

// futureListTestPage 构造一页（total/pageCount 由参数给出）。
func futureListTestPage(total, pageCount int, items ...quoteapiv2.FuturesListItem) quoteapiv2.FuturesListPage {
	return quoteapiv2.FuturesListPage{
		Pagination: quoteapiv2.Pagination{PageSize: len(items), PageCount: pageCount, Total: total},
		Items:      items,
	}
}

// TestFutureListCollectTwoCategories 两个分类各一页：按分类逐个采集、回写进度、整轮成功后标记下架。
func TestFutureListCollectTwoCategories(t *testing.T) {
	store := &fakeFutureConfigStore{configs: []FutureListCollectConfig{
		{MarketType: "0", FuturesType: "1", FuturesTypeName: "股指期货", CountExists: 2},
		{MarketType: "0", FuturesType: "5"},
	}}
	fetcher := &fakeFutureFetcher{pages: map[string]quoteapiv2.FuturesListPage{
		"1/0": futureListTestPage(2, 1, futureListTestItem(101, "ESmain", "US"), futureListTestItem(102, "CNCmain", "HK")),
		"5/0": futureListTestPage(1, 1, futureListTestItem(201, "GCmain", "US")),
	}}
	sink := &fakeFutureSink{offlineRows: 3}
	collector := NewFutureListCollector(store, fetcher, sink, nil)

	var stages []string
	collector.WithHook(func(event StockListCollectEvent) { stages = append(stages, event.Stage) })

	results, err := collector.Collect(context.Background(), FutureListCollectOptions{
		PageSize:     40,
		MarkOffline:  true,
		MaxRetries:   1,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("结果数 = %d，期望 2", len(results))
	}
	if results[0].FuturesType != "1" || results[0].RowsReceived != 2 || results[0].Pages != 1 {
		t.Fatalf("分类 1 结果不符: %+v", results[0])
	}
	if results[0].CountExists != 2 || results[0].TotalAvailable != 2 {
		t.Fatalf("分类 1 总数/可采数不符: %+v", results[0])
	}
	if results[1].FuturesType != "5" || results[1].RowsReceived != 1 {
		t.Fatalf("分类 5 结果不符: %+v", results[1])
	}
	if len(sink.saved) != 3 {
		t.Fatalf("落库行数 = %d，期望 3", len(sink.saved))
	}
	if sink.offlineCalls != 1 {
		t.Fatalf("整轮成功后应标记下架 1 次，实际 %d", sink.offlineCalls)
	}
	if len(sink.offlineMs) != 1 || sink.offlineMs[0] <= 0 {
		t.Fatalf("下架水位应为本轮开始时刻的毫秒时间戳: %+v", sink.offlineMs)
	}
	if len(store.progress) == 0 {
		t.Fatal("应逐页回写进度")
	}
	// 事件序列：每个分类 begin → page → done，整轮结束后一条 round（下架汇总）
	want := []string{"begin", "page", "done", "begin", "page", "done", "round"}
	if fmt.Sprint(stages) != fmt.Sprint(want) {
		t.Fatalf("事件序列不符：期望 %v，实际 %v", want, stages)
	}
}

// TestFutureListFilterSingleCategorySkipsOffline 只采单个分类时不标记下架（避免误标其它分类）。
func TestFutureListFilterSingleCategorySkipsOffline(t *testing.T) {
	store := &fakeFutureConfigStore{configs: []FutureListCollectConfig{
		{MarketType: "0", FuturesType: "1"},
		{MarketType: "0", FuturesType: "2"},
	}}
	fetcher := &fakeFutureFetcher{pages: map[string]quoteapiv2.FuturesListPage{
		"2/0": futureListTestPage(1, 1, futureListTestItem(201, "GCmain", "US")),
	}}
	sink := &fakeFutureSink{}
	collector := NewFutureListCollector(store, fetcher, sink, nil)

	results, err := collector.Collect(context.Background(), FutureListCollectOptions{
		PageSize:          40,
		FilterFuturesType: "2",
		MarkOffline:       true,
	})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if len(results) != 1 || results[0].FuturesType != "2" {
		t.Fatalf("单分类过滤结果不符: %+v", results)
	}
	if sink.offlineCalls != 0 {
		t.Fatalf("单分类采集不应标记下架，实际调用 %d 次", sink.offlineCalls)
	}
}

// TestFutureListPartialFailureSkipsOffline 有分类失败时不标记下架（接口抖动不能把在售合约标下架）。
func TestFutureListPartialFailureSkipsOffline(t *testing.T) {
	store := &fakeFutureConfigStore{configs: []FutureListCollectConfig{
		{MarketType: "0", FuturesType: "1"},
		{MarketType: "0", FuturesType: "2"},
	}}
	fetcher := &fakeFutureFetcher{
		pages: map[string]quoteapiv2.FuturesListPage{
			"1/0": futureListTestPage(1, 1, futureListTestItem(101, "ESmain", "US")),
			// 分类 2 的页全部失败（未预置 → 返回错误）
		},
	}
	sink := &fakeFutureSink{}
	collector := NewFutureListCollector(store, fetcher, sink, nil)

	results, err := collector.Collect(context.Background(), FutureListCollectOptions{
		PageSize:        40,
		MaxRetries:      -1, // 归一为 0，不重试
		RetryBackoff:    time.Millisecond,
		ContinueOnError: true,
		MarkOffline:     true,
	})
	if err == nil {
		t.Fatal("部分失败时应返回首个失败原因")
	}
	if len(results) != 2 || results[1].Error == "" {
		t.Fatalf("应保留成功分类并记录失败原因: %+v", results)
	}
	if sink.offlineCalls != 0 {
		t.Fatalf("部分失败时不应标记下架，实际调用 %d 次", sink.offlineCalls)
	}
}

// TestFutureListPlanTruncation 计划数量截断最后一页，并只采够计划页数。
func TestFutureListPlanTruncation(t *testing.T) {
	plan := 5
	store := &fakeFutureConfigStore{configs: []FutureListCollectConfig{
		{MarketType: "0", FuturesType: "3"},
	}}
	page1 := futureListTestPage(120, 3,
		futureListTestItem(1, "A1", "US"), futureListTestItem(2, "A2", "US"),
		futureListTestItem(3, "A3", "US"), futureListTestItem(4, "A4", "US"))
	page2 := futureListTestPage(120, 3,
		futureListTestItem(5, "A5", "US"), futureListTestItem(6, "A6", "US"),
		futureListTestItem(7, "A7", "US"), futureListTestItem(8, "A8", "US"))
	fetcher := &fakeFutureFetcher{pages: map[string]quoteapiv2.FuturesListPage{
		"3/0": page1,
		"3/1": page2,
	}}
	sink := &fakeFutureSink{}
	collector := NewFutureListCollector(store, fetcher, sink, nil)

	results, err := collector.Collect(context.Background(), FutureListCollectOptions{
		PageSize:          4,
		Interval:          time.Millisecond,
		OverrideCountPlan: &plan,
	})
	if err != nil {
		t.Fatalf("采集失败: %v", err)
	}
	if results[0].TotalAvailable != 5 {
		t.Fatalf("可采数 = %d，期望 5", results[0].TotalAvailable)
	}
	if results[0].RowsReceived != 5 {
		t.Fatalf("收到行数 = %d，期望 5（第二页截断）", results[0].RowsReceived)
	}
	if results[0].Pages != 2 {
		t.Fatalf("页数 = %d，期望 2", results[0].Pages)
	}
}

// TestFutureListRetryThenSuccess 单页失败后按退避重试，最终成功。
func TestFutureListRetryThenSuccess(t *testing.T) {
	store := &fakeFutureConfigStore{configs: []FutureListCollectConfig{{MarketType: "0", FuturesType: "4"}}}
	fetcher := &fakeFutureFetcher{
		pages: map[string]quoteapiv2.FuturesListPage{
			"4/0": futureListTestPage(1, 1, futureListTestItem(9, "CLmain", "US")),
		},
		failPages: map[string]int{"4/0": 1},
	}
	sink := &fakeFutureSink{}
	collector := NewFutureListCollector(store, fetcher, sink, nil)
	var retries int
	collector.WithHook(func(event StockListCollectEvent) {
		if event.Stage == "retry" {
			retries++
		}
	})

	results, err := collector.Collect(context.Background(), FutureListCollectOptions{
		PageSize:     40,
		MaxRetries:   2,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("重试后应成功: %v", err)
	}
	if retries != 1 {
		t.Fatalf("重试事件数 = %d，期望 1", retries)
	}
	if results[0].RowsReceived != 1 {
		t.Fatalf("收到行数 = %d，期望 1", results[0].RowsReceived)
	}
}

// TestBuildFutureListRows 校验字段映射口径：已验证券 1/2/3/4/19 提列，
// 百分比去号、千分位去逗号、方向 up→+、usc 解析、缺失 stock_id 跳过。
func TestBuildFutureListRows(t *testing.T) {
	item := futureListTestItem(70000294, "GCmain", "US")
	noID := futureListTestItem(0, "ESmain", "US")

	cfg := FutureListCollectConfig{MarketType: "0", FuturesType: "5"}
	rows := BuildFutureListRows(cfg, 2,
		[]quoteapiv2.FuturesListItem{item, noID},
		map[int64]string{70000294: "GC0001"},
		nil, 1767225600000)
	if len(rows) != 1 {
		t.Fatalf("行数 = %d，期望 1（stock_id=0 的条目应跳过）", len(rows))
	}
	row := rows[0]
	if row.StockID != 70000294 || row.FuturesType != "5" || row.USC == nil || *row.USC != "GC0001" {
		t.Fatalf("身份列不符: %+v", row)
	}
	if row.ContractCode == nil || *row.ContractCode != "GCmain" {
		t.Fatalf("合约代码不符: %+v", row.ContractCode)
	}
	if row.Name == nil || *row.Name != "黄金" {
		t.Fatalf("名称不符: %+v", row.Name)
	}
	if row.PriceNominal == nil || row.PriceNominal.String() != "4413.8" {
		t.Fatalf("价格应为去千分位的 4413.8，实际 %+v", row.PriceNominal)
	}
	if row.ChangeRatio == nil || row.ChangeRatio.String() != "-0.25" {
		t.Fatalf("涨跌幅应为去百分号的 -0.25，实际 %+v", row.ChangeRatio)
	}
	if row.PriceDirect == nil || *row.PriceDirect != "+" {
		t.Fatalf("方向应为 +，实际 %+v", row.PriceDirect)
	}
	if row.MarketLabel == nil || *row.MarketLabel != "US" {
		t.Fatalf("市场标签不符: %+v", row.MarketLabel)
	}
	if row.Page == nil || *row.Page != 2 || row.DtUpdate == nil || *row.DtUpdate != 1767225600000 {
		t.Fatalf("页码/采集时刻不符: %+v %+v", row.Page, row.DtUpdate)
	}
	if row.FlagOnline != "1" || row.Provider != FutureListProvider {
		t.Fatalf("在线标志/来源不符: %+v", row)
	}
	if row.FieldsJSON == "" || row.FieldListJSON == "" {
		t.Fatalf("原始字段应留档: fields=%q fieldList=%q", row.FieldsJSON, row.FieldListJSON)
	}
}

// TestBuildFutureListRowsFallbackFtmm 派生键兜底解析 usc；都查不到时 usc 为 NULL（不用代码冒充）。
func TestBuildFutureListRowsFallbackFtmm(t *testing.T) {
	cfg := FutureListCollectConfig{MarketType: "0", FuturesType: "8"}
	item := futureListTestItem(70000300, "CNCmain", "hk")

	rows := BuildFutureListRows(cfg, 1, []quoteapiv2.FuturesListItem{item}, nil,
		map[string]string{"HK.CNCmain": "CNC0001"}, 1767225600000)
	if len(rows) != 1 || rows[0].USC == nil || *rows[0].USC != "CNC0001" {
		t.Fatalf("派生键应解析出 usc: %+v", rows[0].USC)
	}

	rows = BuildFutureListRows(cfg, 1, []quoteapiv2.FuturesListItem{item}, nil, nil, 1767225600000)
	if rows[0].USC != nil {
		t.Fatalf("两级都查不到时 usc 应为 NULL（不得用合约代码冒充）: %+v", *rows[0].USC)
	}
}

// TestFutureListPriceDirectMapping 方向映射：up/down/flat/空/其它。
func TestFutureListPriceDirectMapping(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"up", "+"}, {"DOWN", "-"}, {"+", "+"}, {"-", "-"}, {"flat", "="}, {"", ""},
	}
	for _, c := range cases {
		got := futureListPriceDirect(c.raw)
		if c.want == "" {
			if got != nil {
				t.Fatalf("raw=%q 期望 nil，实际 %q", c.raw, *got)
			}
			continue
		}
		if got == nil || *got != c.want {
			t.Fatalf("raw=%q 期望 %q，实际 %+v", c.raw, c.want, got)
		}
	}
}

// TestFutureListDisplayName 分类显示名：有中文名用中文名，否则「分类 N」。
func TestFutureListDisplayName(t *testing.T) {
	if got := (FutureListCollectConfig{FuturesType: "5"}).DisplayName(); got != "分类 5" {
		t.Fatalf("缺中文名应降级为「分类 5」，实际 %q", got)
	}
	if got := (FutureListCollectConfig{FuturesType: "5", FuturesTypeName: "商品期货"}).DisplayName(); got != "商品期货" {
		t.Fatalf("有中文名应直接用，实际 %q", got)
	}
}

// TestFutureListDetailFormat 明细排版：每行 10 个、固定宽度补空格。
func TestFutureListDetailFormat(t *testing.T) {
	rows := make([]FutureListSavedRow, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, FutureListSavedRow{ContractCode: fmt.Sprintf("C%02d", i), Inserted: i%2 == 0})
	}
	detail := futureListFormatDetail(rows)
	lines := 0
	for _, line := range strings.Split(detail, "\n") {
		if line == "" {
			continue
		}
		lines++
	}
	if lines != 3 {
		t.Fatalf("12 行明细应排成 3 行（每行 5 个），实际 %d 行:\n%s", lines, detail)
	}
	// 每格 20 显示列：11 字符的合约代码加 `[I] ` 前缀必须完整显示，不得截断
	longDetail := futureListFormatDetail([]FutureListSavedRow{{ContractCode: "NK225MCmain"}})
	if !strings.Contains(longDetail, "NK225MCmain") {
		t.Fatalf("最长合约代码应完整显示，实际 %q", longDetail)
	}
}

// TestFutureListSampleFields 字段字典采样日志包含 fieldId=fieldName/type/prec，
// 并列出只出现在 fields 里的独有键（19=市场标签）。
func TestFutureListSampleFields(t *testing.T) {
	text := futureListSampleFields([]quoteapiv2.FuturesListItem{futureListTestItem(1, "GCmain", "US")})
	if !strings.Contains(text, "1=合约代码/type4/prec0") {
		t.Fatalf("采样应含字段 1 的定义，实际 %q", text)
	}
	if !strings.Contains(text, "fields 独有键：") || !strings.Contains(text, "19") {
		t.Fatalf("采样应列出 fields 独有键（含 19=市场标签），实际 %q", text)
	}
}
