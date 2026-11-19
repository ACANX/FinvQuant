package secuft

// 本文件实现「新增」对话框的元数据补全（需求 11345）：
// 用户只输入 usc，其余 31 列由本地 4 张表联查补出来。
//
// 映射口径不是猜的：2026-11-19 抓取该表 **465 行真实数据**，与本地 4 张表逐列比对得到
// （`cmd` 里生成的对照统计）：
//
//	列               本地来源                                     一致率
//	name             finv_security.security_name_cn               456/459（3 行人工改过）
//	region           market_label 映射（BK 按 exchange_code 兜底）  457/459
//	market           quote_collect.market_label                   254/459（205 行是细分市场，见下）
//	market_label     quote_collect.market_label                   447/459（12 行 BK* 板块，表里用交易所）
//	symbol           quote_collect.ftmm_symbol                    447/459（同上 12 行 BK*）
//	currency         finv_security.currency_type                  429/459（外汇对取报价货币）
//	timezone         finv_security.tz（IANA）                      347/459
//	provider         ——（表里恒 FTMM）                             459/459
//	type_secu        finv_security.type_secu                      367/459（表里更细：ADR/FX/Future/Crypto/LOF）
//	type_secu_sub    finv_security.type_secu_sub                   40/459（本地多为空）
//	stock_id / market_type / market_code / instrument_type /
//	sub_instrument_type / req_section / futu_symbol / flag_enable
//	                 quote_collect.*                              457/459
//	instrument       quote_collect.instrument（etfs→etf、
//	                 sectors→index、currency→forex）               457/459
//
// `market` 的 205 行差异是**无法自动补全**的：表里存的是细分市场
// （NASDAQ 62 / NYSE 29 / CER 27 / ARCA 25 / COMP 19 / USOTC 12 / HKFE 9 /
// COMEX 8 / NYMEX 3 / CME 3 / CBOT 1），本地 `finv_market` 只有「港股主板 /
// 美股 OTC 场外交易」这种粗粒度名称，没有交易所级细分，因此这里只能给
// `market_label` 作为建议值，并在 notes 里提示核对。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/acanx/finvquant/internal/meta/finv/quote/secukey"
)

// 新增行的默认值（用户确认：新增后先停采）。
const (
	// DefaultFlagEnable 采集开关默认值：0 = 停用，由人工确认后改成 1。
	DefaultFlagEnable = 0
	// DefaultProvider provider 默认值（该表现网 465 行全部是 FTMM）。
	DefaultProvider = "FTMM"
	// DefaultWeightPriority 采集权重默认值（现网 437/465 行是 50）。
	DefaultWeightPriority = 50
	// DefaultWeightFrequency 采集频率权重默认值（现网 465 行全部是 10）。
	DefaultWeightFrequency = 10
)

// collectStatColumns 采集统计列：新增时留空，由采集链路自己维护，不进 missing。
var collectStatColumns = map[string]bool{
	"ts_latest_data":   true,
	"count_last_fetch": true,
	"count_fail":       true,
	"count_stale":      true,
	"dt_last_check":    true,
	"dt_last_fetch":    true,
}

// LookupResult 一次补全的结果。
type LookupResult struct {
	// Usc 输入的统一编号。
	Usc string `json:"usc"`
	// Fields 32 列的建议值（键为与源表一致的列名）；补不出来的列为 nil。
	Fields map[string]any `json:"fields"`
	// Sources 每个字段的取值来源，前端悬浮提示用。
	Sources map[string]string `json:"sources"`
	// Missing 补不出来、需要人工填写或核对的列（按 columns 顺序，不含时间列）。
	Missing []string `json:"missing"`
	// Notes 补全过程中的提示。
	Notes []string `json:"notes"`
	// InSecurity 该 usc 是否在本地证券主数据里（false 时不允许新增）。
	InSecurity bool `json:"in_security"`
	// HasIndicator 该 usc 是否已有指标数据（仅提示，不填字段）。
	HasIndicator bool `json:"has_indicator"`
	// ExistsInSupabase Supabase 里是否已有该 usc（true 时前端直接拦下，避免提交后 409）。
	ExistsInSupabase bool `json:"exists_in_supabase"`
}

// lookupRow 本地 4 表联查的原始行。
type lookupRow struct {
	usc              string
	securityName     string
	securityNameCn   string
	typeSecu         string
	typeSecuSub      string
	provider         string
	currencyType     string
	tz               string
	exchangeCode     int32
	collectFtmm      *string
	futuSymbol       *string
	stockID          *int64
	marketLabel      *string
	marketType       *int32
	marketCode       *int32
	instrument       *string
	instrumentType   *int32
	subInstrumentTyp *int32
	reqSection       *string
	keyMarket        *string
	keyFtmm          *string
	indFtmm          *string
}

// Lookup 按 usc 补全 32 列（需求 11345）。只读本地库，不访问 Supabase。
func (s *Store) Lookup(ctx context.Context, usc string) (*LookupResult, error) {
	usc = strings.TrimSpace(usc)
	if usc == "" {
		return nil, inputError("usc 不能为空")
	}

	const query = `
SELECT s.usc, s.security_name, s.security_name_cn,
       s.type_secu, s.type_secu_sub, s.provider,
       s.currency_type, COALESCE(s.tz, ''), s.exchange_code,
       c.ftmm_symbol, c.futu_symbol, c.stock_id,
       c.market_label, c.market_type, c.market_code,
       c.instrument, c.instrument_type, c.sub_instrument_type, c.req_section,
       k.market, k.ftmm_symbol,
       i.ftmm_symbol
FROM public.finv_security s
LEFT JOIN LATERAL (
    SELECT ftmm_symbol, futu_symbol, stock_id, market_label, market_type, market_code,
           instrument, instrument_type, sub_instrument_type, req_section
    FROM futu.finv_futu_quote_collect WHERE usc = s.usc
    ORDER BY gmt_update DESC LIMIT 1
) c ON TRUE
LEFT JOIN LATERAL (
    SELECT market, ftmm_symbol FROM futu.finv_futu_secu_key WHERE usc = s.usc
    ORDER BY gmt_update DESC LIMIT 1
) k ON TRUE
LEFT JOIN LATERAL (
    SELECT ftmm_symbol FROM cdeu.finv_quote_secu_indicator WHERE usc = s.usc
    ORDER BY dt_update DESC NULLS LAST LIMIT 1
) i ON TRUE
WHERE s.usc = $1`

	var row lookupRow
	err := s.pool.QueryRow(ctx, query, usc).Scan(
		&row.usc, &row.securityName, &row.securityNameCn,
		&row.typeSecu, &row.typeSecuSub, &row.provider,
		&row.currencyType, &row.tz, &row.exchangeCode,
		&row.collectFtmm, &row.futuSymbol, &row.stockID,
		&row.marketLabel, &row.marketType, &row.marketCode,
		&row.instrument, &row.instrumentType, &row.subInstrumentTyp, &row.reqSection,
		&row.keyMarket, &row.keyFtmm,
		&row.indFtmm,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// 不在证券主数据里：仍返回结构，但 InSecurity=false，前端据此禁止新增。
			return &LookupResult{
				Usc:        usc,
				Fields:     map[string]any{"usc": usc},
				Sources:    map[string]string{"usc": "输入"},
				Missing:    creatableColumnNames(),
				Notes:      []string{"该 usc 不在本地证券主数据（public.finv_security）里，无法补全元数据；请先登记证券主数据"},
				InSecurity: false,
			}, nil
		}
		return nil, fmt.Errorf("查询证券元数据失败: %w", err)
	}

	return buildLookupResult(row), nil
}

// buildLookupResult 把联查行映射成 32 列的建议值（纯函数，便于单测）。
func buildLookupResult(row lookupRow) *LookupResult {
	fields := make(map[string]any, len(columns))
	sources := make(map[string]string, len(columns))
	notes := make([]string, 0, 4)

	// set(列名, 候选值1, 来源1, 候选值2, 来源2, ...)：取第一个非空候选。
	set := func(column string, pairs ...any) {
		value, source := pickSource(pairs...)
		fields[column] = value
		if source != "" {
			sources[column] = source
		}
	}
	setFixed := func(column string, value any, source string) {
		fields[column] = value
		sources[column] = source
	}

	marketLabel := strings.TrimSpace(stringValue(row.marketLabel))

	setFixed("usc", row.usc, "输入")
	set(
		"name",
		strings.TrimSpace(row.securityNameCn), "public.finv_security.security_name_cn",
		strings.TrimSpace(row.securityName), "public.finv_security.security_name",
	)
	set("region", regionOf(marketLabel, row.exchangeCode),
		"public.finv_security.market_label/exchange_code 映射")
	// market 只给 coarse 建议值：本地没有交易所级细分市场（NASDAQ/NYSE/ARCA/USOTC/...）。
	set("market", marketLabel, "futu.finv_futu_quote_collect.market_label（细分市场需核对）")
	set("market_label", marketLabel, "futu.finv_futu_quote_collect.market_label")
	set("symbol",
		valueOf(row.collectFtmm), "futu.finv_futu_quote_collect.ftmm_symbol",
		valueOf(row.keyFtmm), "futu.finv_futu_secu_key.ftmm_symbol",
		valueOf(row.indFtmm), "cdeu.finv_quote_secu_indicator.ftmm_symbol",
	)
	set("currency", strings.TrimSpace(row.currencyType), "public.finv_security.currency_type")
	set("timezone", strings.TrimSpace(row.tz), "public.finv_security.tz")
	setFixed("provider", DefaultProvider, "默认值")
	set("type_secu", strings.TrimSpace(row.typeSecu), "public.finv_security.type_secu")
	set("type_secu_sub", strings.TrimSpace(row.typeSecuSub), "public.finv_security.type_secu_sub")
	set("stock_id", int64Of(row.stockID), "futu.finv_futu_quote_collect.stock_id")
	set("market_type", int32Of(row.marketType), "futu.finv_futu_quote_collect.market_type")
	set("market_code", int32Of(row.marketCode), "futu.finv_futu_quote_collect.market_code")
	set("instrument_type", int32Of(row.instrumentType), "futu.finv_futu_quote_collect.instrument_type")
	set("sub_instrument_type", int32Of(row.subInstrumentTyp), "futu.finv_futu_quote_collect.sub_instrument_type")

	// req_section：源列是 varchar（实测非空仅 2 行、非数字 0 行），目标列是 integer；空/非数字 → 0。
	reqSection, reqSectionSource := reqSectionInt(valueOf(row.reqSection))
	setFixed("req_section", reqSection, reqSectionSource)

	setFixed("weight_priority", DefaultWeightPriority, "默认值")
	setFixed("weight_frequency", DefaultWeightFrequency, "默认值")
	// 采集统计列：新增时留空（由采集链路自己维护），不进 missing。
	for column := range collectStatColumns {
		setFixed(column, nil, "新增时留空")
	}
	set("futu_symbol", valueOf(row.futuSymbol), "futu.finv_futu_quote_collect.futu_symbol")
	setFixed("flag_enable", DefaultFlagEnable, "默认值（新增后先停采）")
	set("instrument", normalizeInstrument(stringValue(row.instrument)),
		"futu.finv_futu_quote_collect.instrument（etfs→etf / sectors→index / currency→forex）")
	setFixed("remark", "", "默认值")

	if row.collectFtmm == nil {
		notes = append(notes, "该证券在富途采集配置（futu.finv_futu_quote_collect）里没有记录，"+
			"富途编码类字段无法自动补全，需要手工填写")
	}
	if marketLabel != "" && row.collectFtmm != nil {
		notes = append(notes, "「市场」列在现网数据里有 44% 是细分市场"+
			"（NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX/CME/CBOT/NYMEX），本地没有该粒度数据，"+
			"当前按行情市场（"+marketLabel+"）填的建议值，请核对后再提交")
	}
	if strings.TrimSpace(row.typeSecuSub) == "" {
		notes = append(notes, "证券主数据的 type_secu_sub 为空（本地该列缺失较多），建议手工补全")
	}
	if row.indFtmm != nil {
		notes = append(notes, "该证券已有指标数据（cdeu.finv_quote_secu_indicator），注意避免重复采集")
	}

	missing := make([]string, 0, 6)
	for _, column := range columns {
		if column.Kind == KindTimestamp {
			continue // 时间列由服务端写 now()
		}
		if column.Name == "market" || collectStatColumns[column.Name] {
			continue // market 已给建议值（用 notes 提示核对）；采集统计列新增时留空
		}
		if isEmptyValue(fields[column.Name]) && column.Name != "remark" {
			missing = append(missing, column.Name)
		}
	}

	return &LookupResult{
		Usc:          row.usc,
		Fields:       fields,
		Sources:      sources,
		Missing:      missing,
		Notes:        notes,
		InSecurity:   true,
		HasIndicator: row.indFtmm != nil,
	}
}

// Exists 检查 Supabase 源表里是否已有该 usc（供对话框提前拦截，避免提交后 409）。
func (s *Store) Exists(ctx context.Context, usc string) (bool, error) {
	usc = strings.TrimSpace(usc)
	if usc == "" {
		return false, inputError("usc 不能为空")
	}
	conn, err := s.supabaseConn(ctx)
	if err != nil {
		return false, err
	}
	client := secukey.NewClient(conn)
	rows, total, err := client.FetchQuery(ctx, TableName, buildExistsQuery(usc))
	if err != nil {
		return false, err
	}
	return total > 0 || len(rows) > 0, nil
}

// regionOf 由行情市场推区域；板块（BK）用交易所代码兜底。
//
// 实测 457/459 行吻合（差异 2 行：1 个 BK 板块的交易所归属、1 个本地无采集记录的 UNI）。
func regionOf(marketLabel string, exchangeCode int32) any {
	label := strings.ToUpper(strings.TrimSpace(marketLabel))
	switch label {
	case "SH", "SZ", "BJ":
		return "CN"
	case "HK":
		return "HK"
	case "US":
		return "US"
	case "FX":
		return "FX"
	case "CC":
		return "CC"
	case "AU":
		return "AU"
	case "BK":
		// 板块本身没有区域，按交易所归属：11/13 → CN（上交所/北交所），12/21 → HK。
		switch exchangeCode {
		case 11, 13:
			return "CN"
		case 12, 21:
			return "HK"
		case 30:
			return "US"
		}
	}
	return nil
}

// normalizeInstrument 把富途的品种写法归一到该表的取值。
//
// 实测：只归一这三个就 457/459 吻合（`futures` 在该表里保留原样，不能改成 `future`）。
func normalizeInstrument(instrument string) any {
	value := strings.TrimSpace(instrument)
	switch value {
	case "":
		return nil
	case "etfs":
		return "etf"
	case "sectors":
		return "index"
	case "currency":
		return "forex"
	}
	return value
}

// pickSource 按「值 + 来源」候选序列取第一个非空值。
func pickSource(pairs ...any) (any, string) {
	for i := 0; i+1 < len(pairs); i += 2 {
		value := pairs[i]
		source, _ := pairs[i+1].(string)
		if !isEmptyValue(value) {
			return value, source
		}
	}
	return nil, ""
}

// stringValue 解引用可选文本列。
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// valueOf 解引用可选文本列并去空白（空串返回 nil）。
func valueOf(value *string) any {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(*value)
	if text == "" {
		return nil
	}
	return text
}

// int64Of 解引用可选 bigint 列。
func int64Of(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

// int32Of 解引用可选 integer 列。
func int32Of(value *int32) any {
	if value == nil {
		return nil
	}
	return *value
}

// reqSectionInt 把 varchar 的 req_section 转成 integer（空/非数字 → 0）。
func reqSectionInt(raw any) (int, string) {
	text, _ := raw.(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, "默认值（源为空）"
	}
	number := 0
	for _, char := range text {
		if char < '0' || char > '9' {
			return 0, "默认值（源非数字）"
		}
		number = number*10 + int(char-'0')
	}
	return number, "futu.finv_futu_quote_collect.req_section"
}

// isEmptyValue 判断候选值是否为空（nil / 空串）。
func isEmptyValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

// creatableColumnNames 返回「新增时需要人工提供」的列（去掉时间列与采集统计列）。
func creatableColumnNames() []string {
	out := make([]string, 0, len(columns))
	for _, column := range columns {
		if column.Kind == KindTimestamp || collectStatColumns[column.Name] || !isCreatableColumn(column.Name) {
			continue
		}
		out = append(out, column.Name)
	}
	return out
}

// inputError 构造 InputError（handler 映射为 HTTP 400）。
func inputError(message string) error {
	return &InputError{msg: message}
}
