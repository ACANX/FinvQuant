package cdeu

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件实现「缺省汇率」采集口径（需求 10999，11020 去掉 CNY 派生）：
// 一次 moomoo get-fx-overview / get-fx-rate-list → 组装直采行 → 交给 Store 全量 upsert
// 到 public.finv_fxcer（V300124 创建，需求 11002；一行=一个货币对的最新汇率，主键 usc）。
//
// 需求 11020：**不再按 CNH 同价派生 CNY 行**。原因：
//   - 上游只有离岸人民币 CNH，过去为让证券主数据用的 CNY 也有汇率，按同价复制了一份 CNY 行；
//   - 表格模式的 get-fx-rate-list 已直接提供 CNY 各货币对，派生值（等于 CNH 而非真实在岸价）
//     会与真实 CNY 汇率混在一张表里，污染后续换算/回测口径；
//   - 现在只落上游直采行：CNY 有数据就落，没数据就空着（由表格模式采集补齐），
//     不再伪造，也不再写 flag_derive='1' 的行。
//
// 迁移说明（刻意约束）：
//   - 只依赖标准库与 quoteapiv2，不引用本项目其它文件的类型；
//   - 写入通过 FxCERSink 接口注入，后续整体搬到 MetaExt 第三方包时本文件可原样搬走。
//
// 上游口径（2026-11-08 实测，HTTP 200）：
//   - data 为「货币对代码 → 最新价」对象，72 条 = 9 币种两两组合
//     （CNH / USD / HKD / EUR / GBP / AUD / SGD / CAD / JPY）；
//   - 键 AB = 1 A = N B（USDCNH=6.67333）；正反两向都有，且不严格互为倒数
//     （实测 |USDGBP × GBPUSD - 1| 最大 5.6e-4），故两向都落库，不靠倒数推导；
//   - 每对各自带 time（Unix 毫秒），逐对独立；price 与 priceNominal 实测恒等；
//   - 只有离岸人民币 CNH，没有在岸 CNY：需求 11020 起**不再按 CNH 派生 CNY**，
//     CNY 相关货币对改由 get-fx-rate-list 直采补齐。

const (
	// FxCERSrcMoomoo 上游直采行的来源标识。
	FxCERSrcMoomoo = "moomoo.get-fx-overview"
	// FxCERSrcRateList 汇率列表（get-fx-rate-list）直采行的来源标识。
	FxCERSrcRateList = "moomoo.get-fx-rate-list"
	// FxCurrencyCNH 离岸人民币（上游 get-fx-overview 提供的口径）。
	FxCurrencyCNH = "CNH"
	// FxCurrencyCNY 人民币（在岸）：证券主数据（currency）使用的口径；
	// 只由 get-fx-rate-list 直采（需求 11020，不再由 CNH 派生）。
	FxCurrencyCNY = "CNY"

	// FxCERSourceOverview 采集来源：get-fx-overview（9 币种两两完整矩阵，矩阵模式使用）。
	FxCERSourceOverview = "overview"
	// FxCERSourceRateList 采集来源：get-fx-rate-list（全量汇率列表，币种更多，表格模式使用）。
	FxCERSourceRateList = "rateList"
)

// FxCERCurrencyNames 币种简体中文名（展示用，需求 11000）。
//
// 为什么不用 public.finv_currency.currency_name：该字典只有 7 条静态种子
// （CNY/USD/HKD/NTD/JPY/SGD/INR），缺 CNH/EUR/GBP/AUD/CAD，且命名口径与业务不符；
// 汇率表需要「准确、不被误读」的名称，故在此维护（需求 11003）：
//   - CNY = 人民币、CNH = 离岸人民币：两者价格存在基差，仍靠币种代码区分，名称按上游惯例；
//   - 其余取金融行业常用简称。
//
// 未登记的币种由 FxCurrencyName 回落为币种代码本身（前端只显示代码，不编造名称）。
// 将来若把这些名称落进字典表，本映射可退化为「读字典 + 本表兜底」。
var FxCERCurrencyNames = map[string]string{
	FxCurrencyCNY: "人民币",
	FxCurrencyCNH: "离岸人民币",
	"USD":         "美元",
	"HKD":         "港币",
	"EUR":         "欧元",
	"GBP":         "英镑",
	"JPY":         "日元",
	"AUD":         "澳元",
	"CAD":         "加元",
	"SGD":         "新加坡元",
}

// FxCurrencyName 返回币种简体中文名；未登记时回落为币种代码。
func FxCurrencyName(code string) string {
	if name, ok := FxCERCurrencyNames[code]; ok {
		return name
	}
	return code
}

// FxCERFetcher 汇率概览拉取能力（*quoteapiv2.Client 已实现；单测可注入假实现）。
type FxCERFetcher interface {
	GetFxOverview(ctx context.Context, req quoteapiv2.GetFxOverviewRequest) (map[string]quoteapiv2.FxOverviewQuote, error)
}

// FxCERRateListFetcher 汇率列表拉取能力（*quoteapiv2.Client 已实现；单测可注入假实现）。
type FxCERRateListFetcher interface {
	GetFxRateList(ctx context.Context, req quoteapiv2.GetFxRateListRequest) (quoteapiv2.FxRateListPage, error)
}

// FxCERSink 缺省汇率写入能力（由 *Store 实现；采集逻辑不关心 SQL 细节）。
type FxCERSink interface {
	UpsertFxCER(ctx context.Context, rows []FxCERRow) (FxCERUpsertStat, error)
}

// FxCERRow 一行缺省汇率（public.finv_fxcer 的镜像，字段名与列名一致）。
type FxCERRow struct {
	// USC 上游标识：get-fx-rate-list 的 stockCode / get-fx-overview 的 map key（如 USDCNH）。
	USC string `json:"usc"`
	// Name 展示名称（简体中文）：列表接口原样使用其 name（如「美元/日元」），
	// 概览按币种中文名合成（如「美元/离岸人民币」）。
	Name string `json:"name"`
	// CurrencySrc 源币种（ISO 4217 alpha-3）。
	CurrencySrc string `json:"currency_src"`
	// CurrencyDest 目标币种。
	CurrencyDest string `json:"currency_dest"`
	// Rate 缺省汇率原文：1 单位 CurrencySrc = N CurrencyDest。
	// 保留上游字符串形式，避免经 float64 丢精度（NUMERIC(24,10) 由数据库解析）。
	Rate string `json:"rate"`
	// TsQuote 报价时间（Unix 毫秒）：overview 取上游逐对 time，
	// rateList 接口不返回时间，取本次采集时刻。
	TsQuote int64 `json:"ts_quote"`
	// Source 来源标识：FxCERSrcMoomoo / FxCERSrcRateList（需求 11020 起不再产生派生来源）。
	Source string `json:"source"`
	// ChangePrice 涨跌额（带符号，定点文本）：moomoo get-fx-rate-list 的 changeAmount（需求 11016）；
	// 概览来源不提供该字段，空串 → 落库为 NULL（store 侧 COALESCE，不清空已有值）。
	ChangePrice string `json:"change_price,omitempty"`
	// ChangeRatio 涨跌幅（百分数数值文本，-0.35 表示 -0.35%）：同接口的 changePercent，
	// 落库前去掉了「%」；概览来源不提供，空串 → NULL。
	ChangeRatio string `json:"change_ratio,omitempty"`
	// FlagDerive 0=上游直采、1=派生。需求 11020 起采集只写 '0'；
	// 字段保留是因为表结构与历史数据仍在（库里若有 '1' 的旧行，由使用者决定如何处置）。
	FlagDerive string `json:"flag_derive"`
	// Remark 备注（口径说明，可空）。
	Remark string `json:"remark,omitempty"`
	// GmtUpdate 最近更新时间（仅查询返回，写入忽略）。
	GmtUpdate time.Time `json:"gmt_update,omitempty"`
}

// FxCERUpsertStat 一次写入的统计（来自 RETURNING (xmax = 0)）。
type FxCERUpsertStat struct {
	// Insert 新增行数。
	Insert int64
	// Update 覆盖行数。
	Update int64
}

// FxCERSyncResult 一次手动更新的结果（接口返回与前端展示用）。
type FxCERSyncResult struct {
	// Source 本次采集来源：overview / rateList。
	Source string `json:"source"`
	// Pages 本次实际请求的页数（overview 固定 1；rateList 单页成功为 1，降级分页时 > 1）。
	Pages int `json:"pages"`
	// RowsReceive 本次组装出的行数（需求 11020 起只有直采行）。
	RowsReceive int `json:"rows_receive"`
	// Skipped 上游返回但被跳过的条目数（非货币对如 USDindex、缺价条目；overview 恒为 0）。
	Skipped int `json:"skipped"`
	// Derive 派生行数。需求 11020 起不再派生 CNY 行，恒为 0；
	// 字段保留是为了接口向后兼容（前端仍按该字段展示统计）。
	Derive int `json:"derive"`
	// Insert 新增行数。
	Insert int64 `json:"insert"`
	// Update 覆盖行数。
	Update int64 `json:"update"`
	// TsQuoteMax 本次最大报价时间（Unix 毫秒）。
	TsQuoteMax int64 `json:"ts_quote_max"`
	// ElapsedMs 耗时（毫秒）。
	ElapsedMs int64 `json:"elapsed_ms"`
}

// ErrFxCERInvalid 汇率数据非法（上游返回了无法解析的键或价格）。
var ErrFxCERInvalid = errors.New("汇率数据非法")

// BuildFxCERRows 把 GetFxOverview 的返回组装成待写入行（只有直采行）。
//
// 纯函数（不做网络与数据库访问），便于单测固化口径：
//   - 键必须是大写三位币种拼成的 6 位代码，且源/目标不同；
//   - 价格优先取 priceNominal，缺失回落 price，必须是正数；
//   - 报价时间必须为正（Unix 毫秒）；
//   - 不再派生 CNY 行（需求 11020）：上游只给 CNH，就只落 CNH 的行；
//     CNY 相关货币对由表格模式的 get-fx-rate-list 直采补齐，不按同价伪造。
func BuildFxCERRows(quotes map[string]quoteapiv2.FxOverviewQuote) ([]FxCERRow, error) {
	if len(quotes) == 0 {
		return nil, fmt.Errorf("%w: 汇率概览返回为空", ErrFxCERInvalid)
	}

	direct := make([]FxCERRow, 0, len(quotes))
	for rawPair, quote := range quotes {
		pair := strings.ToUpper(strings.TrimSpace(rawPair))
		if !isFxCurrencyPair(pair) {
			return nil, fmt.Errorf("%w: 货币对键非法（%s）", ErrFxCERInvalid, rawPair)
		}
		src, dest := pair[:3], pair[3:]
		if src == dest {
			return nil, fmt.Errorf("%w: 货币对源目标相同（%s）", ErrFxCERInvalid, pair)
		}
		price := strings.TrimSpace(quote.PriceNominal)
		if price == "" {
			price = strings.TrimSpace(quote.Price)
		}
		rate, err := strconv.ParseFloat(price, 64)
		if err != nil || rate <= 0 {
			return nil, fmt.Errorf("%w: 货币对 %s 价格非法（%q）", ErrFxCERInvalid, pair, price)
		}
		if quote.Time <= 0 {
			return nil, fmt.Errorf("%w: 货币对 %s 报价时间非法（%d）", ErrFxCERInvalid, pair, quote.Time)
		}
		direct = append(direct, FxCERRow{
			USC:          pair,
			Name:         fxPairName(src, dest),
			CurrencySrc:  src,
			CurrencyDest: dest,
			Rate:         price,
			TsQuote:      quote.Time,
			Source:       FxCERSrcMoomoo,
			FlagDerive:   "0",
		})
	}
	sortFxCERRows(direct)
	return direct, nil
}

// BuildFxCERRowsFromRateList 把 get-fx-rate-list 的条目组装成待写入行（只有直采行）。
//
// 与 Overview 的差异：
//   - 币种更多（listId=1 为全部汇率，实测 282 条），可补上 Overview 只有 9 币种的缺口；
//   - 该接口**不返回报价时间**，`dt_quote` 统一取本次采集时刻（参数 nowMs）；
//   - 条目里混有非货币对（美元指数 `USDindex`、欧元指数 `EURindex` 等），
//     以及可能缺价的条目，这些**逐条跳过并计数**（skipped），不因单条异常丢掉整批。
//
// 只有「全部条目都不可用」或采集时刻非法时才返回错误。
func BuildFxCERRowsFromRateList(items []quoteapiv2.FxRateListItem, nowMs int64) ([]FxCERRow, int, error) {
	if nowMs <= 0 {
		return nil, 0, fmt.Errorf("%w: 采集时刻非法（%d）", ErrFxCERInvalid, nowMs)
	}

	direct := make([]FxCERRow, 0, len(items))
	skipped := 0
	for _, item := range items {
		pair := strings.ToUpper(strings.TrimSpace(item.StockCode))
		if !isFxCurrencyPair(pair) { // USDindex / EURindex 等指数序列不是货币对
			skipped++
			continue
		}
		src, dest := pair[:3], pair[3:]
		if src == dest {
			skipped++
			continue
		}
		price := strings.TrimSpace(item.PriceNominal)
		rate, err := strconv.ParseFloat(price, 64)
		if err != nil || rate <= 0 {
			skipped++
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fxPairName(src, dest)
		}
		direct = append(direct, FxCERRow{
			USC:          pair,
			Name:         name,
			CurrencySrc:  src,
			CurrencyDest: dest,
			Rate:         price,
			TsQuote:      nowMs,
			Source:       FxCERSrcRateList,
			ChangePrice:  normalizeChangeAmount(item.ChangeAmount),
			ChangeRatio:  normalizeChangeRatio(item.ChangePercent),
			FlagDerive:   "0",
		})
	}
	if len(direct) == 0 {
		return nil, skipped, fmt.Errorf("%w: 汇率列表没有可用货币对（跳过 %d 条）", ErrFxCERInvalid, skipped)
	}
	sortFxCERRows(direct)
	return direct, skipped, nil
}

// sortFxCERRows 按「源币种 + 目标币种」升序排序（输出稳定，便于比对与落库顺序一致）。
func sortFxCERRows(rows []FxCERRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CurrencySrc != rows[j].CurrencySrc {
			return rows[i].CurrencySrc < rows[j].CurrencySrc
		}
		return rows[i].CurrencyDest < rows[j].CurrencyDest
	})
}

// normalizeChangeAmount 归一化涨跌额：去空白、去千分位逗号，校验可解析为有限数字。
//
// 上游 `changeAmount` 形如 "-0.0012" / "+0.0025"；解析失败（空、"--"、非数字）返回空串，
// 由 store 落库为 NULL——涨跌额缺失不影响汇率主口径，不因单条异常丢弃整条记录。
func normalizeChangeAmount(raw string) string {
	value := strings.ReplaceAll(strings.TrimSpace(raw), ",", "")
	if value == "" {
		return ""
	}
	if n, err := strconv.ParseFloat(value, 64); err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return ""
	}
	return value
}

// normalizeChangeRatio 归一化涨跌幅：去掉尾随「%」，其余同 normalizeChangeAmount。
//
// 上游 `changePercent` 形如 "-0.35%" / "+1.08%"；落库存数值（-0.35），百分号由前端补。
func normalizeChangeRatio(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimSuffix(value, "%")
	return normalizeChangeAmount(value)
}

// fxPairName 生成货币对展示名称：「源中文名/目标中文名」，未登记币种回落代码。
func fxPairName(src, dest string) string {
	return FxCurrencyName(src) + "/" + FxCurrencyName(dest)
}

// isFxCurrencyPair 判断是否为「三位大写字母 + 三位大写字母」的货币对键。
func isFxCurrencyPair(pair string) bool {
	if len(pair) != 6 {
		return false
	}
	for _, r := range pair {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
