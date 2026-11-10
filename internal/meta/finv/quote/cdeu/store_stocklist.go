package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/acnxo/MetaExt/component/quote/moomoo/quoteapiv2"
)

// 本文件是「实时行情列表采集」在本项目侧的落地：配置读写（StockListConfigStore）
// 与逐页落库（StockListSink）。
//
// 采集流程本身在 quote_list_stock_snapshot_sync.go（单文件、可整体迁往 MetaExt 第三方包）；
// 这里只做与 FinvQuant 存储相关的事：读 cdeu.finv_cdeu_secu_realtime_sync_config、
// 把接口条目转成 cdeu.finv_cdeu_secu_realtime_stock 的行并调用既有 upsert。

// tableRealtimeSyncConfig 列表采集配置表（V300121 创建，V300122 更名为 finv_cdeu_secu_realtime_sync_config）。
const tableRealtimeSyncConfig = "cdeu.finv_cdeu_secu_realtime_sync_config"

const (
	// tableFutuQuoteCollect futu 采集配置表：stock_id ↔ usc 的权威映射（含柜台口径）。
	tableFutuQuoteCollect = "futu.finv_futu_quote_collect"
	// tableSecuUsc 证券主数据表：usc → currency / market / timezone。
	tableSecuUsc = "cdeu.finv_quote_cdeu_usc"
)

// stockListSecurityMeta 采集行需要补齐的证券主数据。
type stockListSecurityMeta struct {
	// USC 统一证券代码（港股 5 位、A 股 6 位、美股字母代码）。
	//
	// 空值语义（刻意保留，勿"修复"）：表示该证券尚未录入证券主数据
	//（`futu.finv_futu_quote_collect` 里没有），通常是新上市或未纳入采集范围。
	// 落库后 usc 为 NULL 的行就是「待录入清单」，是后续补录三张表的依据：
	// public.finv_security（证券主数据）+ futu.finv_futu_quote_collect（采集配置）
	// + cdeu.finv_quote_cdeu_usc（usc 主数据）；
	// 禁止用 stock_code 回填，否则这份信号会被抹掉且可能跨市场映射错。
	USC string
	// Currency 结算币种（HK 正股 HKD、R 柜台 CNY、美股 USD、A 股 CNY）。
	Currency string
}

// loadStockListSecurityMeta 批量补齐 usc 与币种（需求 10993）。
//
// 关联顺序：
//
//  1. usc：只认 stock_id → futu.finv_futu_quote_collect.usc（最准确，带柜台口径）。
//     **查不到就保持 NULL，不允许用 stock_code 冒充 usc**——两者并非等价，
//     直接挪用会把不同市场的同码证券映射错（如 SZ/SH 均有 000001）。
//
//     这个 NULL 是**业务信号，不是缺陷**：采集会把所有在售证券都写进来，
//     usc 为 NULL 的那些就是「新上市 / 尚未录入证券主数据」的证券。
//     它是后续补录 `public.finv_security` + `futu.finv_futu_quote_collect`
//     + `cdeu.finv_quote_cdeu_usc` 三张表的依据，
//     因此不得回填、不得兜底、不得在写入链路里自动建主数据（需求 10994 的刻意约束）。
//
//  2. currency：有 usc 时先查 cdeu.finv_quote_cdeu_usc（主数据），
//     再查 public.finv_security.currency_type（覆盖面更广）。
//
//  3. 仍查不到时按 market_label + 代码规则推导（见 stockListFallbackCurrency）。
//
// 背景：moomoo 列表接口不返回币种，usc 也不在响应里，早期实现只写行情字段，
// 导致采集新增的证券 usc / currency 长期为空（生产环境实测约 17% 行为空）。
func (s *Store) loadStockListSecurityMeta(ctx context.Context, items []quoteapiv2.StockListItem) (map[int64]stockListSecurityMeta, error) {
	metas := make(map[int64]stockListSecurityMeta, len(items))
	if len(items) == 0 {
		return metas, nil
	}

	ids := make([]int64, 0, len(items))
	uscByID := make(map[int64]string, len(items))
	for _, item := range items {
		if item.StockID > 0 {
			ids = append(ids, item.StockID)
		}
	}
	if len(ids) > 0 {
		rows, err := s.pool.Query(ctx, `
SELECT stock_id, usc
  FROM `+tableFutuQuoteCollect+`
 WHERE stock_id = ANY($1::bigint[]) AND usc IS NOT NULL AND usc <> ''`, ids)
		if err != nil {
			return nil, fmt.Errorf("查询 stock_id→usc 映射失败（page 数据可能缺 usc）: %w", err)
		}
		for rows.Next() {
			var stockID int64
			var code string
			if err := rows.Scan(&stockID, &code); err != nil {
				rows.Close()
				return nil, fmt.Errorf("解析 stock_id→usc 映射失败: %w", err)
			}
			uscByID[stockID] = strings.TrimSpace(code)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("查询 stock_id→usc 映射失败: %w", err)
		}
	}

	// 收集待查币种的 usc 集合：只用真正映射到的 usc（不拿 stock_code 顶替）。
	uscSet := make(map[string]struct{}, len(items))
	for _, item := range items {
		if code := uscByID[item.StockID]; code != "" {
			uscSet[code] = struct{}{}
		}
	}
	currencyByUSC := make(map[string]string, len(uscSet))
	if len(uscSet) > 0 {
		uscs := make([]string, 0, len(uscSet))
		for code := range uscSet {
			uscs = append(uscs, code)
		}
		rows, err := s.pool.Query(ctx, `
SELECT usc, currency
  FROM `+tableSecuUsc+`
 WHERE usc = ANY($1::text[]) AND currency IS NOT NULL AND currency <> ''`, uscs)
		if err != nil {
			return nil, fmt.Errorf("查询 usc→currency 映射失败: %w", err)
		}
		for rows.Next() {
			var code, currency string
			if err := rows.Scan(&code, &currency); err != nil {
				rows.Close()
				return nil, fmt.Errorf("解析 usc→currency 映射失败: %w", err)
			}
			currencyByUSC[code] = strings.TrimSpace(currency)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("查询 usc→currency 映射失败: %w", err)
		}
	}

	// 第二数据源：public.finv_security（security_code → currency_type），覆盖 usc 主数据未收录的证券。
	currencyByCode := make(map[string]string, len(uscSet))
	if len(uscSet) > 0 {
		codes := make([]string, 0, len(uscSet))
		for code := range uscSet {
			codes = append(codes, code)
		}
		rows, err := s.pool.Query(ctx, `
SELECT DISTINCT security_code, currency_type
  FROM public.finv_security
 WHERE security_code = ANY($1::text[]) AND currency_type IS NOT NULL AND currency_type <> ''`, codes)
		if err != nil {
			return nil, fmt.Errorf("查询 security_code→currency_type 映射失败: %w", err)
		}
		for rows.Next() {
			var code, currency string
			if err := rows.Scan(&code, &currency); err != nil {
				rows.Close()
				return nil, fmt.Errorf("解析 security_code→currency_type 映射失败: %w", err)
			}
			currencyByCode[code] = strings.TrimSpace(currency)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("查询 security_code→currency_type 映射失败: %w", err)
		}
	}

	for _, item := range items {
		// usc 只认 futu 采集表；查不到就是空（NULL），不做 stock_code 兜底。
		code := uscByID[item.StockID]
		currency := ""
		if code != "" {
			currency = currencyByUSC[code]
			if currency == "" {
				currency = currencyByCode[code]
			}
		}
		if currency == "" {
			currency = stockListFallbackCurrency(item.MarketLabel, item.StockCode)
		}
		metas[item.StockID] = stockListSecurityMeta{USC: code, Currency: currency}
	}
	return metas, nil
}

// stockListFallbackCurrency 按 market_label + 代码推导币种，只在主数据表都查不到时使用。
//
// 依据（实测 2026-11-05，库内港股快照零例外）：
//   - HK：5 位代码中 8 开头（80000–89999）为人民币柜台（R 股）→ CNY，其余 → HKD；
//   - US：→ USD；
//   - SH / SZ / BJ（沪深京）：→ CNY（兼容历史数据里的 CN）。
//
// 只看 market_label（采集响应直接给出），不依赖 marketType，避免两套口径打架。
func stockListFallbackCurrency(marketLabel, stockCode string) string {
	switch strings.ToUpper(strings.TrimSpace(marketLabel)) {
	case "HK":
		return stockListHKCurrency(stockCode)
	case "US":
		return "USD"
	case "SH", "SZ", "BJ", "CN":
		return "CNY"
	}
	return ""
}

// stockListHKCurrency 港股币种：8 开头的 5 位代码是人民币柜台（R 股），其余为港币。
func stockListHKCurrency(stockCode string) string {
	code := strings.TrimSpace(stockCode)
	if len(code) == 5 && strings.HasPrefix(code, "8") {
		return "CNY"
	}
	return "HKD"
}

// LoadStockListConfigs 读取启用中的采集配置，按市场类型、品种排序（决定采集顺序）。
func (s *Store) LoadStockListConfigs(ctx context.Context) ([]StockListCollectConfig, error) {
	rows, err := s.pool.Query(ctx, `
SELECT instrument_symbol, market_type, count_plan,
       COALESCE(count_exists, 0)    AS count_exists,
       COALESCE(total_available, 0) AS total_available
  FROM `+tableRealtimeSyncConfig+`
 WHERE flag_enable = '1'
 ORDER BY market_type, instrument_symbol`)
	if err != nil {
		return nil, fmt.Errorf("读取采集配置失败: %w", err)
	}
	defer rows.Close()

	configs := make([]StockListCollectConfig, 0, 8)
	for rows.Next() {
		var cfg StockListCollectConfig
		if err := rows.Scan(&cfg.InstrumentSymbol, &cfg.MarketType, &cfg.CountPlan, &cfg.CountExists, &cfg.TotalAvailable); err != nil {
			return nil, fmt.Errorf("解析采集配置失败: %w", err)
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("读取采集配置失败: %w", err)
	}
	return configs, nil
}

// SaveStockListProgress 回写采集进度：count_exists / total_available 只在拿到正数时覆盖，
// 避免接口异常返回 0 把已观测到的市场总数抹掉。
func (s *Store) SaveStockListProgress(ctx context.Context, cfg StockListCollectConfig, progress StockListProgress) error {
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableRealtimeSyncConfig+`
   SET count_exists    = CASE WHEN $3 > 0 THEN $3 ELSE count_exists END,
       total_available = CASE WHEN $4 > 0 THEN $4 ELSE total_available END,
       gmt_update      = now()
 WHERE market_type = $1
   AND instrument_symbol = $2`,
		cfg.MarketType, cfg.InstrumentSymbol, progress.CountExists, progress.TotalAvailable)
	if err != nil {
		return fmt.Errorf("回写采集进度失败（market_type=%s instrument_symbol=%s）: %w",
			cfg.MarketType, cfg.InstrumentSymbol, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("回写采集进度失败：配置不存在（market_type=%s instrument_symbol=%s）",
			cfg.MarketType, cfg.InstrumentSymbol)
	}
	return nil
}

// SaveStockListPage 把一页列表条目写入证券实时行情快照表，按行返回新增/更新标记。
//
// 数值字段的口径与既有快照一致：
//   - 金额/股本/成交额/成交量：接口带「万/亿/万亿/B/M」单位，换算成元/股后入库；
//   - 百分比：去掉 `%`，按百分点数值入库（-4.30% → -4.30）；
//   - 方向：up/down/flat → `+`/`-`/`0`；
//   - 解析失败的字段一律不写（NULL），不猜值。
func (s *Store) SaveStockListPage(ctx context.Context, cfg StockListCollectConfig, page int, items []quoteapiv2.StockListItem) ([]StockListSavedRow, error) {
	if len(items) == 0 {
		return nil, nil
	}
	now := time.Now().UnixMilli()
	// 补齐 usc 与币种：接口不返回币种、usc 也不在响应里，必须按 stock_id/stock_code 关联主数据表。
	securityMeta, err := s.loadStockListSecurityMeta(ctx, items)
	if err != nil {
		return nil, err
	}
	batch := make([]RealtimeStockItem, 0, len(items))
	meta := make([]StockListSavedRow, 0, len(items))
	for _, it := range items {
		if it.StockID <= 0 {
			continue
		}
		stockID := it.StockID
		dtUpdate := now
		pageNo := int64(page)
		marketType := cfg.MarketType
		marketLabel := it.MarketLabel
		instrumentType := strconv.Itoa(it.InstrumentType)

		secuMeta := securityMeta[stockID]
		batch = append(batch, RealtimeStockItem{
			USC:                    stockListText(secuMeta.USC),
			StockID:                &stockID,
			Name:                   stockListText(it.Name),
			StockCode:              stockListText(it.StockCode),
			MarketType:             stockListText(marketType),
			MarketLabel:            stockListText(marketLabel),
			InstrumentType:         stockListText(instrumentType),
			Currency:               stockListText(secuMeta.Currency),
			MarketValue:            stockListNumberText(stockListAmount(it.MarketValue)),
			TotalShares:            stockListNumberText(stockListAmount(it.TotalShares)),
			CirculationTotalShares: stockListNumberText(stockListAmount(it.CirculationTotalShares)),
			TradeTrunover:          stockListNumberText(stockListAmount(it.TradeTurnover)),
			TradeVolumn:            stockListNumberText(stockListAmount(it.TradeVolume)),
			CirculationMarketValue: stockListNumberText(stockListAmount(it.CirculationMarketValue)),
			DtUpdate:               &dtUpdate,
			PriceNominal:           stockListNumberText(stockListPlainNumber(it.PriceNominal)),
			ChangeRatio:            stockListNumberText(stockListPlainNumber(it.ChangeRatio)),
			TrustAum:               stockListNumberText(stockListAmount(it.TrustAUM)),
			TradeChangeRatio:       stockListNumberText(stockListPlainNumber(it.TradeChangeRatio)),
			PriceAmplitude:         stockListNumberText(stockListPlainNumber(it.PriceAmplitude)),
			VolumnRatio:            stockListNumberText(stockListPlainNumber(it.VolumeRatio)),
			BuysellRatio:           stockListNumberText(stockListPlainNumber(it.BuySellRatio)),
			ChangePrice:            stockListNumberText(stockListPlainNumber(it.Change)),
			PriceDirect:            stockListTextValue(stockListPriceDirect(it.PriceDirect)),
			Leverage:               stockListNumberText(stockListPlainNumber(stockListFlexString(it.Leverage))),
			Page:                   &pageNo,
		})
		meta = append(meta, StockListSavedRow{
			StockID:   stockID,
			StockCode: it.StockCode,
			Name:      it.Name,
		})
	}
	if len(batch) == 0 {
		return nil, nil
	}

	// 先查这批 stock_id 是否已存在，用于在日志里区分「新增」与「更新」
	// （upsert 语句本身只返回汇总计数，拿不到逐行标记）。
	stockIDs := make([]int64, 0, len(batch))
	for _, item := range batch {
		stockIDs = append(stockIDs, *item.StockID)
	}
	existing := make(map[int64]struct{}, len(batch))
	rows, err := s.pool.Query(ctx, `
SELECT stock_id FROM `+tableRealtimeStock+` WHERE stock_id = ANY($1::bigint[])`, stockIDs)
	if err != nil {
		return nil, fmt.Errorf("查询已有证券失败（marker=%s/%s page=%d）: %w",
			cfg.InstrumentSymbol, cfg.MarketType, page, err)
	}
	for rows.Next() {
		var stockID int64
		if err := rows.Scan(&stockID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("解析已有证券失败: %w", err)
		}
		existing[stockID] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("查询已有证券失败: %w", err)
	}

	if _, err := s.UpsertRealtimeStocks(ctx, batch); err != nil {
		return nil, fmt.Errorf("写入列表页失败（marker=%s/%s page=%d）: %w",
			cfg.InstrumentSymbol, cfg.MarketType, page, err)
	}

	saved := make([]StockListSavedRow, 0, len(meta))
	for _, row := range meta {
		_, ok := existing[row.StockID]
		row.Inserted = !ok
		saved = append(saved, row)
	}
	return saved, nil
}

// stockListText 文本字段：空串按未提供处理（不入库、不覆盖旧值）。
func stockListText(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// stockListTextValue 把 (值, 是否有效) 转成可空文本。
func stockListTextValue(value string, ok bool) *string {
	if !ok || value == "" {
		return nil
	}
	return &value
}

// stockListNumberText 把已解析的数值文本转成 json.Number（保留原文，交给 NUMERIC 定点转换）。
func stockListNumberText(value string, ok bool) *json.Number {
	if !ok || value == "" {
		return nil
	}
	number := json.Number(value)
	return &number
}

// stockListFlexString 取 FlexNum 的原始文本。
func stockListFlexString(value *quoteapiv2.FlexNum) string {
	if value == nil {
		return ""
	}
	return value.String()
}

// stockListPriceDirect 涨跌方向：up/+ → `+`、down/- → `-`、flat/0 → `0`。
func stockListPriceDirect(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "up", "+":
		return "+", true
	case "down", "-":
		return "-", true
	case "flat", "0":
		return "0", true
	}
	return "", false
}

// stockListPlainNumber 归一化为纯十进制文本：去千分位逗号、空白、正号、百分号。
//
// 仅做文本清洗，不用 float64 参与计算，避免精度漂移；合法性用 ParseFloat 校验。
func stockListPlainNumber(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, " ", "")
	text = strings.TrimSuffix(text, "%")
	text = strings.TrimPrefix(text, "+")
	if text == "" || text == "-" || text == "." || text == "-." {
		return "", false
	}
	if _, err := strconv.ParseFloat(text, 64); err != nil {
		return "", false
	}
	return text, true
}

// stockListAmount 把带单位的金额/股数/成交量换算成基准单位（元/股）：
//
// 中文口径：万亿=1e12、亿=1e8、万=1e4；
// 英文口径：T=1e12、B=1e9、M=1e6、K=1e3（大小写均可）；无单位则原样返回。
//
// 英文后缀是必须支持的：同一接口在未锁定语言（Accept-Language / IP 地区）时
// 返回英文单位（如 `4.07T`、`895.92B`），只认中文口径会把万亿级证券整条丢掉。
// 解析失败返回 false，调用方按「字段缺失」处理（不猜值）。
func stockListAmount(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}
	text = strings.ReplaceAll(text, ",", "")

	pow := 0
	switch {
	case strings.HasSuffix(text, "万亿"):
		pow, text = 12, strings.TrimSuffix(text, "万亿")
	case strings.HasSuffix(text, "亿"):
		pow, text = 8, strings.TrimSuffix(text, "亿")
	case strings.HasSuffix(text, "万"):
		pow, text = 4, strings.TrimSuffix(text, "万")
	case strings.HasSuffix(text, "T"), strings.HasSuffix(text, "t"):
		pow, text = 12, text[:len(text)-1]
	case strings.HasSuffix(text, "B"), strings.HasSuffix(text, "b"):
		pow, text = 9, text[:len(text)-1]
	case strings.HasSuffix(text, "M"), strings.HasSuffix(text, "m"):
		pow, text = 6, text[:len(text)-1]
	case strings.HasSuffix(text, "K"), strings.HasSuffix(text, "k"):
		pow, text = 3, text[:len(text)-1]
	}

	plain, ok := stockListPlainNumber(text)
	if !ok {
		return "", false
	}
	return stockListScaleDecimal(plain, pow)
}

// stockListScaleDecimal 把十进制文本乘以 10^pow（pow ≥ 0），全程字符串移位，不经过浮点。
func stockListScaleDecimal(text string, pow int) (string, bool) {
	if pow <= 0 {
		return text, true
	}

	negative := strings.HasPrefix(text, "-")
	body := strings.TrimPrefix(text, "-")

	intPart, fracPart := body, ""
	if index := strings.IndexByte(body, '.'); index >= 0 {
		intPart, fracPart = body[:index], body[index+1:]
	}
	digits := intPart + fracPart
	point := len(intPart) + pow

	var out string
	if point >= len(digits) {
		out = digits + strings.Repeat("0", point-len(digits))
	} else {
		out = digits[:point] + "." + digits[point:]
		out = strings.TrimRight(out, "0")
		out = strings.TrimSuffix(out, ".")
	}
	if out == "" {
		return "0", true
	}

	integerPart, decimalPart := out, ""
	hasDecimal := false
	if index := strings.IndexByte(out, '.'); index >= 0 {
		integerPart, decimalPart, hasDecimal = out[:index], out[index+1:], true
	}
	integerPart = strings.TrimLeft(integerPart, "0")
	if integerPart == "" {
		integerPart = "0"
	}
	if hasDecimal && decimalPart != "" {
		out = integerPart + "." + decimalPart
	} else {
		out = integerPart
	}
	if negative && out != "0" {
		out = "-" + out
	}
	return out, true
}
