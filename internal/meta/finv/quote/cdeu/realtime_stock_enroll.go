package cdeu

// 待录入证券「补录 / 录入证券元数据」的预填与写库（需求 10996 / 11147）。
//
// 背景：`cdeu.finv_cdeu_secu_realtime_stock` 里 `usc` 为空的行（待录入证券）需要补录到
// 两张主数据表。原实现只做字段展示，全部人工填写，实际补录一条要填十几个字段。
//
// 本文件的规则：
//   - 能由关联得到的**一律自动填**，并标注来源（快照 / 字典 / 参考同市场 / 缺省）；
//   - 确实推不出来的（`security_name_cn` 这类）先用占位值顶上，人工只需核对；
//   - `cdeu.finv_quote_cdeu_usc` **不再录入**：该表数据在运行时由采集链路动态生成，
//     不需要（也不应该）手工补录，需求 11147 起从补录范围移除。
//
// 来源标记（`source`）含义：
//
//	snapshot    快照行直接带过来的值（代码 / 名称 / 市场 / 品种 / 币种）
//	dictionary  查字典表得到的值（finv_exchange、futu 品种映射）
//	reference   参照同市场同品种已配置行的众数（collect 的 market_code 等）
//	default     表默认值或代码里的缺省值，仅供占位
//	manual      推不出来，必须人工确认

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pendingSecuInstrumentTypeExpr 把快照的 instrument_type（TEXT）归一到采集表的整型口径。
//
// 归一规则与 instrumentByInstrumentType 保持一致：`12` 与 `3` 都是 stock；
// 其余按原值映射；落在白名单之外的取 -1（采集表不会有该值 → 匹配不到参考行，
// 前端会把 market_code 标成「必须人工填写」，而不是悄悄写 0）。
const pendingSecuInstrumentTypeExpr = `(CASE btrim(t.instrument_type)
              WHEN '3'  THEN 3
              WHEN '4'  THEN 4
              WHEN '6'  THEN 6
              WHEN '7'  THEN 7
              WHEN '9'  THEN 9
              WHEN '10' THEN 10
              WHEN '11' THEN 11
              WHEN '12' THEN 3
              ELSE -1
            END)`

// 预填来源标记。
const (
	PrefillSourceSnapshot   = "snapshot"
	PrefillSourceDictionary = "dictionary"
	PrefillSourceReference  = "reference"
	PrefillSourceDefault    = "default"
	PrefillSourceManual     = "manual"
)

// PendingSecuPrefillField 一个待填字段的预填结果。
type PendingSecuPrefillField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Source   string `json:"source"`
	Required bool   `json:"required"`
	Note     string `json:"note"`
}

// PendingSecuPrefill 一只待录入证券的完整预填结果（两张表各一组字段）。
type PendingSecuPrefill struct {
	StockID int64  `json:"stock_id"`
	USC     string `json:"usc"`
	Name    string `json:"name"`
	// Security public.finv_security 的字段（usc 由服务端派生，不在列表里）。
	Security []PendingSecuPrefillField `json:"security"`
	// Collect futu.finv_futu_quote_collect 的字段（usc / stock_id / ftmm_symbol 由服务端派生）。
	Collect []PendingSecuPrefillField `json:"collect"`
	// Warnings 需要在界面上提示的风险点（如参考值来自众数、名称中文缺失等）。
	Warnings []string `json:"warnings"`
}

// PendingSecuEnrollSecurity public.finv_security 的可填字段。
type PendingSecuEnrollSecurity struct {
	ExchangeCode     int    `json:"exchange_code"`
	SecurityType     string `json:"security_type"`
	SecurityCode     string `json:"security_code"`
	SecurityName     string `json:"security_name"`
	SecurityNameCN   string `json:"security_name_cn"`
	SecurityNameFull string `json:"security_name_full"`
	CurrencyType     string `json:"currency_type"`
	Timezone         string `json:"timezone"`
	Tz               string `json:"tz"`
}

// PendingSecuEnrollCollect futu.finv_futu_quote_collect 的可填字段。
type PendingSecuEnrollCollect struct {
	FutuSymbol        string `json:"futu_symbol"`
	MarketType        int    `json:"market_type"`
	MarketCode        int    `json:"market_code"`
	InstrumentType    int    `json:"instrument_type"`
	SubInstrumentType int    `json:"sub_instrument_type"`
	MarketLabel       string `json:"market_label"`
	Instrument        string `json:"instrument"`
	ReqSection        string `json:"req_section"`
	FlagEnable        string `json:"flag_enable"`
}

// PendingSecuEnrollRequest 录入请求体（`usc` / `stock_id` / `ftmm_symbol` 一律服务端派生，
// 避免前端把 `[object Object]` 之类的东西写进主键）。
type PendingSecuEnrollRequest struct {
	StockID  int64                     `json:"stock_id"`
	Remark   string                    `json:"remark"`
	Security PendingSecuEnrollSecurity `json:"security"`
	Collect  PendingSecuEnrollCollect  `json:"collect"`
}

// PendingSecuEnrollResult 录入结果。
type PendingSecuEnrollResult struct {
	USC string `json:"usc"`
	// SecurityCreated / CollectCreated 本次是否新建（重跑已存在则为 false，幂等）。
	SecurityCreated bool `json:"security_created"`
	CollectCreated  bool `json:"collect_created"`
	// SnapshotUpdated 是否把 usc 回写到了快照表（已写过则为 false）。
	SnapshotUpdated bool `json:"snapshot_updated"`
	// Remark 实际写入两张表的 remark 值。
	Remark string `json:"remark"`
}

// pendingSecuAutoRemarkPrefix 补录程序写入 remark 的固定标记，便于后续检索溯源。
const pendingSecuAutoRemarkPrefix = "[AUTO-FILL]"

// buildAutoRemark 生成写库用的 remark：固定标记 + 时间 + 人工备注（可为空）。
//
// 形如 `[AUTO-FILL] 待录入证券补录 2026-01-15T10:20:30+08:00`；
// 人工备注非空时追加 `；备注：<人工备注>`。
func buildAutoRemark(manual string, at time.Time) string {
	remark := fmt.Sprintf("%s 待录入证券补录 %s", pendingSecuAutoRemarkPrefix, at.Format(time.RFC3339))
	if m := strings.TrimSpace(manual); m != "" {
		remark += "；备注：" + m
	}
	return remark
}

// ---- 小工具（本文件内部用，避免到处重复写法）----

func itoa(v int) string     { return strconv.Itoa(v) }
func itoa64(v int64) string { return strconv.FormatInt(v, 10) }

// parseIntOr 解析整数字符串，失败或为空时返回 fallback（快照里的类型字段是 TEXT）。
func parseIntOr(raw string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		return v
	}
	return fallback
}

// sourceOf 条件成立时给出指定来源，否则标为 manual（需要人工确认）。
func sourceOf(ok bool, source string) string {
	if ok {
		return source
	}
	return PrefillSourceManual
}

// explainUniqueViolation 把 23505（唯一约束冲突）翻译成能指导人工处理的错误。
//
// 典型场景：老库 `public.finv_security` 里同一交易所+代码还存在旧口径 usc 的行，
// 新口径 `HK.09999` 插不进去。这种情况**不能自动改旧行的 usc**（主键变更须人工裁决）。
func explainUniqueViolation(err error, table string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return err
	}
	detail := pgErr.Detail
	if detail == "" {
		detail = pgErr.ConstraintName
	}
	return fmt.Errorf("%s 唯一约束冲突（%s）——同一 交易所/代码 或 市场/代码 已有其它 usc 的行，"+
		"需人工确认后再录入", table, detail)
}

// dictNote 组装「来源：字典表」的说明文字。
func dictNote(detail string) string {
	if detail == "" {
		return "来源：字典表"
	}
	return "来源：字典表（" + detail + "）"
}

// securityTypeByInstrumentType futu instrument_type → public.finv_security.security_type。
//
// 取值口径以 `public.finv_security` 既有数据为准（Stock / ETF / StockIndex / Crypto / LOF / Fund）。
func securityTypeByInstrumentType(instrumentType int) string {
	switch instrumentType {
	case 3, 12:
		return "Stock"
	case 4:
		return "ETF"
	case 6:
		return "StockIndex"
	case 11:
		return "Crypto"
	default:
		return "Stock"
	}
}

// instrumentByInstrumentType futu instrument_type → 采集表的 instrument（富途列表接口的字符串口径）。
func instrumentByInstrumentType(instrumentType int) string {
	switch instrumentType {
	case 4:
		return "etf"
	case 6:
		return "index"
	case 7:
		return "sectors"
	case 11:
		return "crypto"
	default:
		return "stock"
	}
}

// subInstrumentType 由 instrument_type 推缺省子类型：富途口径为 `类型 * 1000 + 2`（3→3002、4→4002）。
func subInstrumentType(instrumentType int) int {
	if instrumentType <= 0 {
		return 3002
	}
	return instrumentType*1000 + 2
}

// PendingSecuPrefill 按快照行 + 字典表 + 同市场参考值，生成一只证券的补录预填（需求 11147）。
func (s *Store) PendingSecuPrefill(ctx context.Context, stockID int64) (PendingSecuPrefill, error) {
	var result PendingSecuPrefill
	result.StockID = stockID

	// 一次查询取齐：快照行 + finv_exchange 字典 + 同市场同品种的参考行（market_code 等众数）。
	const stmt = `
SELECT t.stock_id,
       upper(btrim(t.market_label) || '.' || btrim(t.stock_code))          AS usc,
       COALESCE(btrim(t.stock_code), '')                                   AS stock_code,
       COALESCE(btrim(t.name), '')                                         AS name,
       COALESCE(upper(btrim(t.market_label)), '')                          AS market_label,
       COALESCE(NULLIF(btrim(t.currency), ''), e.base_currency, '')        AS currency,
       COALESCE(e.exchange_code, 0)                                        AS exchange_code,
       COALESCE(e.base_currency, '')                                       AS base_currency,
       COALESCE(t.instrument_type, '')                                     AS instrument_type_raw,
       COALESCE(t.market_type, '')                                         AS market_type_raw,
       ref.market_code, ref.sub_instrument_type, ref.instrument,
       sec_conflict.usc  AS security_conflict_usc,
       col_conflict.usc  AS collect_conflict_usc,
       ref_sec.req_section AS ref_req_section,
       ref_mt.market_type  AS ref_market_type,
       tz_ex.timezone    AS exchange_tz_offset,
       tz_ex.tz          AS exchange_tz_name,
       tz_rg.timezone    AS region_tz_offset,
       tz_rg.tz          AS region_tz_name,
       COALESCE(e.region, '') AS exchange_region
  FROM ` + tableRealtimeStock + ` t
  LEFT JOIN public.finv_exchange e
         ON upper(btrim(e.exchange_flag)) = upper(btrim(t.market_label))
  LEFT JOIN LATERAL (
        SELECT q.market_code,
               q.sub_instrument_type,
               q.instrument
          FROM futu.finv_futu_quote_collect q
         WHERE q.market_label = btrim(t.market_label)
           -- 快照的 instrument_type 是 TEXT，且 12 与 3 都表示「股票」；
           -- 采集表里只有 3。「12 的行」直接按 text 比对永远匹配不到参考行，
           -- 结果 market_code 取不到值（=0）、必填校验必然失败（需求 11164 修复）。
           AND q.instrument_type = ` + pendingSecuInstrumentTypeExpr + `
         GROUP BY 1, 2, 3
         ORDER BY count(*) DESC, 1
         LIMIT 1
  ) ref ON true
  -- 采集参数缺省推断（需求 11166）：同市场同品种的 req_section 众数
  LEFT JOIN LATERAL (
        SELECT COALESCE(btrim(q.req_section), '') AS req_section
          FROM futu.finv_futu_quote_collect q
         WHERE q.market_label = btrim(t.market_label)
           AND q.instrument_type = ` + pendingSecuInstrumentTypeExpr + `
         GROUP BY 1
         ORDER BY count(*) DESC, 1
         LIMIT 1
  ) ref_sec ON true
  -- 采集参数缺省推断（需求 11166）：同市场的 market_type 众数
  LEFT JOIN LATERAL (
        SELECT q.market_type
          FROM futu.finv_futu_quote_collect q
         WHERE q.market_label = btrim(t.market_label)
         GROUP BY 1
         ORDER BY count(*) DESC, 1
         LIMIT 1
  ) ref_mt ON true
  -- 时区缺省值（需求 11165）：同交易所已有行的 (timezone, tz) 众数
  LEFT JOIN LATERAL (
        SELECT s.timezone, s.tz
          FROM public.finv_security s
         WHERE s.exchange_code = e.exchange_code
           AND NULLIF(btrim(s.timezone), '') IS NOT NULL
           AND NULLIF(btrim(s.tz), '') IS NOT NULL
         GROUP BY 1, 2
         ORDER BY count(*) DESC, 1, 2
         LIMIT 1
  ) tz_ex ON true
  -- 时区缺省值兜底：同区域（finv_exchange.region）已有行的众数
  LEFT JOIN LATERAL (
        SELECT s.timezone, s.tz
          FROM public.finv_security s
          JOIN public.finv_exchange se ON se.exchange_code = s.exchange_code
         WHERE se.region = e.region
           AND btrim(COALESCE(e.region, '')) NOT IN ('', 'N/A')
           AND NULLIF(btrim(s.timezone), '') IS NOT NULL
           AND NULLIF(btrim(s.tz), '') IS NOT NULL
         GROUP BY 1, 2
         ORDER BY count(*) DESC, 1, 2
         LIMIT 1
  ) tz_rg ON true
  -- 唯一约束预检：finv_security 的 (exchange_code, security_code) 已存在但 usc 不同 → 录入会 23505
  LEFT JOIN LATERAL (
        SELECT s.usc
          FROM public.finv_security s
         WHERE s.exchange_code = e.exchange_code
           AND s.security_code = btrim(t.stock_code)
           AND s.usc <> upper(btrim(t.market_label) || '.' || btrim(t.stock_code))
         LIMIT 1
  ) sec_conflict ON true
  -- 采集表的唯一键 (market_code, futu_symbol) 预检
  LEFT JOIN LATERAL (
        SELECT q.usc
          FROM futu.finv_futu_quote_collect q
         WHERE ref.market_code IS NOT NULL
           AND q.market_code = ref.market_code
           AND q.futu_symbol = btrim(t.stock_code)
           AND q.usc <> upper(btrim(t.market_label) || '.' || btrim(t.stock_code))
         LIMIT 1
  ) col_conflict ON true
 WHERE t.stock_id = $1`

	var (
		usc, stockCode, name, marketLabel, currency string
		exchangeCode                                int
		baseCurrency, instrumentTypeRaw             string
		marketTypeRaw                               *string
		securityConflictUSC, collectConflictUSC     *string
		refMarketCode, refSubInstrumentType         *int
		refInstrument                               *string
		exchangeTZOffset, exchangeTZName            *string
		regionTZOffset, regionTZName                *string
		exchangeRegion                              string
		refReqSection                               *string
		refMarketType                               *int
	)
	if err := s.pool.QueryRow(ctx, stmt, stockID).Scan(
		&result.StockID, &usc, &stockCode, &name, &marketLabel, &currency,
		&exchangeCode, &baseCurrency, &instrumentTypeRaw, &marketTypeRaw,
		&refMarketCode, &refSubInstrumentType, &refInstrument,
		&securityConflictUSC, &collectConflictUSC,
		&refReqSection, &refMarketType,
		&exchangeTZOffset, &exchangeTZName,
		&regionTZOffset, &regionTZName,
		&exchangeRegion,
	); err != nil {
		if err == pgx.ErrNoRows {
			return result, fmt.Errorf("%w: stock_id=%d 不存在", ErrRealtimeStockInvalid, stockID)
		}
		return result, fmt.Errorf("PendingSecuPrefill：查询失败: %w", err)
	}
	result.USC = usc
	result.Name = name

	snapshotMarketType := 0
	if marketTypeRaw != nil {
		snapshotMarketType = parseIntOr(*marketTypeRaw, 0)
	}
	instrumentType := parseIntOr(instrumentTypeRaw, 3)

	// ---- public.finv_security ----
	securityNameCN := name
	securityNameCNNote := "快照没有中文名，先用英文名占位，请核对"
	result.Security = []PendingSecuPrefillField{
		{Key: "usc", Label: "usc（主键，派生）", Value: usc, Source: PrefillSourceDictionary, Required: true,
			Note: "由 `市场标签.代码` 派生并统一大写，不可改"},
		{Key: "exchange_code", Label: "exchange_code 交易所代码", Value: itoa(exchangeCode),
			Source: sourceOf(exchangeCode > 0, PrefillSourceDictionary), Required: true,
			Note: dictNote("public.finv_exchange.exchange_flag = " + marketLabel)},
		{Key: "security_type", Label: "security_type 证券类型", Value: securityTypeByInstrumentType(instrumentType),
			Source: PrefillSourceDictionary, Required: true, Note: "按 instrument_type=" + itoa(instrumentType) + " 映射"},
		{Key: "security_code", Label: "security_code 证券代码", Value: stockCode,
			Source: PrefillSourceSnapshot, Required: true, Note: "取自快照 stock_code"},
		{Key: "security_name", Label: "security_name 名称（英文）", Value: name,
			Source: PrefillSourceSnapshot, Required: true},
		{Key: "security_name_cn", Label: "security_name_cn 名称（中文）", Value: securityNameCN,
			Source: PrefillSourceDefault, Required: true, Note: securityNameCNNote},
		{Key: "security_name_full", Label: "security_name_full 全称", Value: name,
			Source: PrefillSourceDefault, Note: "默认取英文名占位"},
		{Key: "currency_type", Label: "currency_type 币种", Value: currency,
			Source: sourceOf(currency != "", PrefillSourceDictionary), Required: true,
			Note: "快照币种优先，缺失时用 finv_exchange.base_currency（" + baseCurrency + "）"},
	}
	// 时区：按「交易所 → 区域 → 内置区域映射」三层推断（需求 11165）
	timezone := inferSecurityTZ(
		stringOrEmpty(exchangeTZOffset), stringOrEmpty(exchangeTZName),
		stringOrEmpty(regionTZOffset), stringOrEmpty(regionTZName),
		exchangeRegion, time.Now(),
	)
	result.Security = append(result.Security, timezone.fields()...)
	result.Warnings = append(result.Warnings, timezone.warnings(exchangeRegion)...)

	// ---- futu.finv_futu_quote_collect ----
	marketCode, marketCodeSource := 0, PrefillSourceManual
	if refMarketCode != nil && *refMarketCode > 0 {
		marketCode, marketCodeSource = *refMarketCode, PrefillSourceReference
	}
	// instrument_type 归一口径只用于**推缺省值**（12 与 3 同为股票；采集表存量以 3 为主）：
	// 写库仍用快照原值，避免擅自改动下游（日K同步）实际拿到的品种码。
	normalizedType := normalizeInstrumentType(instrumentType)
	subType, subTypeSource := subInstrumentType(normalizedType), PrefillSourceDefault
	if refSubInstrumentType != nil && *refSubInstrumentType > 0 {
		subType, subTypeSource = *refSubInstrumentType, PrefillSourceReference
	}
	// market_type：快照优先 → 同市场已有行众数（需求 11166）→ 内置市场映射
	marketType, marketTypeSource := deriveMarketType(snapshotMarketType, refMarketType, marketLabel)
	// req_section：同市场同品种众数（需求 11166），没有参考行时留空（选填）
	reqSection, reqSectionSource, reqSectionNote := deriveReqSection(refReqSection)
	instrument, instrumentSource := instrumentByInstrumentType(instrumentType), PrefillSourceDictionary
	if refInstrument != nil && strings.TrimSpace(*refInstrument) != "" {
		instrument, instrumentSource = strings.TrimSpace(*refInstrument), PrefillSourceReference
	}

	// market_code 取不到参考值时**留空串**（而不是 "0"）：0 会被当成「已填」，
	// 用户看到的是「必填都齐了」，点写入才被后端拒（需求 11164 的踩坑点）。
	marketCodeValue := ""
	marketCodeNote := "参考同市场同品种已配置行的众数，请核对（不同板块取值不同）"
	if marketCode > 0 {
		marketCodeValue = itoa(marketCode)
	} else {
		marketCodeNote = "同市场同品种没有可参照的采集配置行，必须人工填写（正整数）"
	}

	result.Collect = []PendingSecuPrefillField{
		{Key: "usc", Label: "usc（与 finv_security 同键）", Value: usc, Source: PrefillSourceDictionary,
			Required: true, Note: "派生键，与证券主数据保持一致"},
		{Key: "stock_id", Label: "stock_id", Value: itoa64(result.StockID), Source: PrefillSourceSnapshot,
			Required: true, Note: "取自快照行"},
		{Key: "futu_symbol", Label: "futu_symbol 富途代码", Value: stockCode, Source: PrefillSourceSnapshot,
			Required: true, Note: "取自快照 stock_code（非字母字符原样保留）"},
		{Key: "market_type", Label: "market_type 市场类型", Value: itoa(marketType),
			Source: marketTypeSource, Required: true,
			Note: marketTypeNote(marketTypeSource, snapshotMarketType, marketLabel)},
		{Key: "market_code", Label: "market_code 市场细码", Value: marketCodeValue,
			Source: marketCodeSource, Required: true, Note: marketCodeNote},
		{Key: "instrument_type", Label: "instrument_type 品种类型", Value: itoa(instrumentType),
			Source: PrefillSourceSnapshot, Required: true,
			Note: instrumentTypeNote(instrumentType)},
		{Key: "sub_instrument_type", Label: "sub_instrument_type 子类型", Value: itoa(subType),
			Source: subTypeSource, Required: true, Note: "缺省为 品种类型*1000+2"},
		{Key: "market_label", Label: "market_label 市场标签", Value: marketLabel, Source: PrefillSourceSnapshot,
			Required: true, Note: "与 ftmm_symbol 前缀一致"},
		{Key: "instrument", Label: "instrument 品种名", Value: instrument, Source: instrumentSource,
			Required: true, Note: "富途列表接口的小写字符串口径"},
		{Key: "req_section", Label: "req_section 请求分段", Value: reqSection,
			Source: reqSectionSource, Note: reqSectionNote},
		{Key: "flag_enable", Label: "flag_enable 启用标志", Value: "0", Source: PrefillSourceDefault,
			Required: true, Note: "默认停用，人工核对后再打开采集"},
	}

	// ---- 风险提示 ----
	if exchangeCode <= 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("市场标签 %q 在 public.finv_exchange 里没有对应交易所，exchange_code 需人工填写", marketLabel))
	}
	if currency == "" {
		result.Warnings = append(result.Warnings, "快照没有币种、字典也没有基准币种，currency_type 需人工填写")
	}
	if name == "" {
		result.Warnings = append(result.Warnings, "快照没有名称，security_name 需人工填写")
	}
	if marketCode == 0 {
		result.Warnings = append(result.Warnings,
			"同市场同品种没有可参照的采集配置行，market_code 需人工填写")
	}
	if securityConflictUSC != nil && *securityConflictUSC != "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"public.finv_security 里 交易所 %d + 代码 %s 已存在（usc=%s），直接录入会撞唯一约束，需先人工处理旧行",
			exchangeCode, stockCode, *securityConflictUSC))
	}
	if collectConflictUSC != nil && *collectConflictUSC != "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"futu.finv_futu_quote_collect 里 市场细码 %d + 代码 %s 已存在（usc=%s），直接录入会撞唯一约束",
			marketCode, stockCode, *collectConflictUSC))
	}
	return result, nil
}

// EnrollPendingSecu 把一只待录入证券写入两张主数据表，并把 usc 回写快照（需求 10996 / 11147）。
//
// 单事务；三处写入都按主键幂等：
//   - `public.finv_security` / `futu.finv_futu_quote_collect`：`ON CONFLICT DO NOTHING`（已存在不覆盖人工数据）
//   - `cdeu.finv_cdeu_secu_realtime_stock`：只写 `usc` 为空的行
//
// `remark` 由服务端生成，带 `[AUTO-FILL]` 标记便于溯源；人工备注附在后面。
// `usc` / `stock_id` / `ftmm_symbol` 一律以快照行为准，不采信请求体（防止写进奇怪的值）。
func (s *Store) EnrollPendingSecu(ctx context.Context, req PendingSecuEnrollRequest) (PendingSecuEnrollResult, error) {
	var result PendingSecuEnrollResult
	if req.StockID <= 0 {
		return result, fmt.Errorf("%w: stock_id 必须为正整数", ErrRealtimeStockInvalid)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("EnrollPendingSecu：开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) 取快照行（派生键 / 代码 / 市场标签以它为准）
	var (
		stockID                     int64
		usc, stockCode, marketLabel string
	)
	if err := tx.QueryRow(ctx, `
SELECT stock_id,
       upper(btrim(market_label) || '.' || btrim(stock_code)),
       COALESCE(btrim(stock_code), ''),
       COALESCE(upper(btrim(market_label)), '')
  FROM `+tableRealtimeStock+`
 WHERE stock_id = $1`, req.StockID).Scan(&stockID, &usc, &stockCode, &marketLabel); err != nil {
		if err == pgx.ErrNoRows {
			return result, fmt.Errorf("%w: stock_id=%d 不存在", ErrRealtimeStockInvalid, req.StockID)
		}
		return result, fmt.Errorf("EnrollPendingSecu：读取快照失败: %w", err)
	}
	if usc == "." || usc == "" {
		return result, fmt.Errorf("%w: 快照行的 市场标签/代码 为空，无法派生 usc", ErrRealtimeStockInvalid)
	}
	result.USC = usc

	sec := req.Security
	col := req.Collect
	if err := normalizePendingSecuSecurity(&sec, stockCode); err != nil {
		return result, err
	}
	if err := normalizePendingSecuCollect(&col, stockCode, marketLabel); err != nil {
		return result, err
	}

	result.Remark = buildAutoRemark(req.Remark, time.Now())

	// 2) public.finv_security（已存在则不覆盖）
	securityCreated, err := insertSecuSecurity(ctx, tx, usc, sec, result.Remark)
	if err != nil {
		return result, fmt.Errorf("EnrollPendingSecu：%w", err)
	}
	result.SecurityCreated = securityCreated

	// 3) futu.finv_futu_quote_collect（ftmm_symbol 是生成列，不能写）
	collectCreated, err := insertSecuCollect(ctx, tx, usc, stockID, col, result.Remark)
	if err != nil {
		return result, fmt.Errorf("EnrollPendingSecu：%w", err)
	}
	result.CollectCreated = collectCreated

	// 4) 回写快照 usc：只填空值，不覆盖既有值（幂等）
	tag, err := tx.Exec(ctx, `
UPDATE `+tableRealtimeStock+`
   SET usc = $2
 WHERE stock_id = $1
   AND NULLIF(btrim(usc), '') IS NULL`, stockID, usc)
	if err != nil {
		return result, fmt.Errorf("EnrollPendingSecu：回写快照 usc 失败: %w", err)
	}
	result.SnapshotUpdated = tag.RowsAffected() > 0

	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("EnrollPendingSecu：提交事务失败: %w", err)
	}
	return result, nil
}

// ---- 归一与写入（需求 11161 起，供「整体录入」与「单表补全」共用）----

// normalizePendingSecuSecurity 校验并补齐证券主数据的必填项。
//
// 服务端兜底，不采信前端传来的空值：`security_code` 缺省用快照代码，
// `security_name_cn` 缺省用英文名占位，其余必填缺失直接拒绝（避免把空串写进主数据）。
func normalizePendingSecuSecurity(sec *PendingSecuEnrollSecurity, stockCode string) error {
	if sec.SecurityCode == "" {
		sec.SecurityCode = stockCode
	}
	bad := make([]string, 0, 4)
	if sec.SecurityName == "" {
		bad = append(bad, "security_name 为空")
	}
	if sec.CurrencyType == "" {
		bad = append(bad, "currency_type 为空")
	}
	if sec.SecurityType == "" {
		bad = append(bad, "security_type 为空")
	}
	if sec.ExchangeCode <= 0 {
		bad = append(bad, fmt.Sprintf("exchange_code=%d（必须 >0）", sec.ExchangeCode))
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: 证券主数据缺少必填 —— %s", ErrRealtimeStockInvalid, strings.Join(bad, "；"))
	}
	if sec.SecurityNameCN == "" {
		sec.SecurityNameCN = sec.SecurityName
	}
	return nil
}

// normalizePendingSecuCollect 校验并补齐采集配置的必填项与缺省值。
//
// 缺省口径：`futu_symbol` 用快照代码、`market_label` 用快照市场、`instrument` 由
// `instrument_type` 映射、`sub_instrument_type` 为 `类型*1000+2`、`flag_enable` 默认停用（0）。
func normalizePendingSecuCollect(col *PendingSecuEnrollCollect, stockCode, marketLabel string) error {
	if col.FutuSymbol == "" {
		col.FutuSymbol = stockCode
	}
	if col.MarketLabel == "" {
		col.MarketLabel = marketLabel
	}
	if col.Instrument == "" {
		col.Instrument = instrumentByInstrumentType(col.InstrumentType)
	}
	if col.SubInstrumentType <= 0 {
		col.SubInstrumentType = subInstrumentType(col.InstrumentType)
	}
	if col.FlagEnable == "" {
		col.FlagEnable = "0"
	}
	// 逐项列出到底哪个字段不合格（以前只回一句「缺 6 项里的某些」，用户不知道要改哪个）
	bad := make([]string, 0, 6)
	if col.MarketLabel == "" {
		bad = append(bad, "market_label 为空")
	}
	if col.FutuSymbol == "" {
		bad = append(bad, "futu_symbol 为空")
	}
	if col.MarketType <= 0 {
		bad = append(bad, fmt.Sprintf("market_type=%d（必须 >0）", col.MarketType))
	}
	if col.MarketCode <= 0 {
		bad = append(bad, fmt.Sprintf("market_code=%d（必须 >0，同市场同品种没有参考行时需人工填）", col.MarketCode))
	}
	if col.InstrumentType <= 0 {
		bad = append(bad, fmt.Sprintf("instrument_type=%d（必须 >0）", col.InstrumentType))
	}
	if col.Instrument == "" {
		bad = append(bad, "instrument 为空")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: 采集配置缺少必填 —— %s", ErrRealtimeStockInvalid, strings.Join(bad, "；"))
	}
	return nil
}

// insertSecuSecurity 写 `public.finv_security`，按主键 `ON CONFLICT (usc) DO NOTHING`。
//
// 返回是否真正插入了新行（false = 该 usc 已存在，人工数据不被覆盖）。
// 23505 会被翻译成可读提示：老库同一 交易所+代码 可能还挂着旧口径的 usc，这种冲突需人工裁决，
// **不自动改旧行主键**。
func insertSecuSecurity(ctx context.Context, tx pgx.Tx, usc string,
	sec PendingSecuEnrollSecurity, remark string) (bool, error) {
	var created bool
	if err := tx.QueryRow(ctx, `
INSERT INTO public.finv_security AS t
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, timezone, tz, remark)
VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, NULLIF($9, ''), NULLIF($10, ''), $11)
ON CONFLICT (usc) DO NOTHING
RETURNING true`,
		usc, sec.ExchangeCode, sec.SecurityType, sec.SecurityCode, sec.SecurityName, sec.SecurityNameCN,
		sec.SecurityNameFull, sec.CurrencyType, sec.Timezone, sec.Tz, remark,
	).Scan(&created); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("写入 finv_security 失败: %w",
			explainUniqueViolation(err, "public.finv_security"))
	}
	return created, nil
}

// insertSecuCollect 写 `futu.finv_futu_quote_collect`，按主键 `ON CONFLICT (usc) DO NOTHING`。
//
// `ftmm_symbol` 是生成列（market_label || '.' || futu_symbol），不能显式写入。
func insertSecuCollect(ctx context.Context, tx pgx.Tx, usc string, stockID int64,
	col PendingSecuEnrollCollect, remark string) (bool, error) {
	var created bool
	if err := tx.QueryRow(ctx, `
INSERT INTO futu.finv_futu_quote_collect AS t
       (usc, stock_id, futu_symbol, market_type, market_code, instrument_type,
        sub_instrument_type, req_section, market_label, instrument, flag_enable, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9, $10, $11, $12)
ON CONFLICT (usc) DO NOTHING
RETURNING true`,
		usc, stockID, col.FutuSymbol, col.MarketType, col.MarketCode, col.InstrumentType,
		col.SubInstrumentType, col.ReqSection, col.MarketLabel, col.Instrument, col.FlagEnable, remark,
	).Scan(&created); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("写入 finv_futu_quote_collect 失败: %w",
			explainUniqueViolation(err, "futu.finv_futu_quote_collect"))
	}
	return created, nil
}

// stringOrEmpty 解引用可空字符串（NULL → 空串），用于时区参考值等可选列。
func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
