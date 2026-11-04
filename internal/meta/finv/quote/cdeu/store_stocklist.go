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

		batch = append(batch, RealtimeStockItem{
			StockID:                &stockID,
			Name:                   stockListText(it.Name),
			StockCode:              stockListText(it.StockCode),
			MarketType:             stockListText(marketType),
			MarketLabel:            stockListText(marketLabel),
			InstrumentType:         stockListText(instrumentType),
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
// 万亿=1e12、亿=1e8、万=1e4、B=1e9、M=1e6；无单位则原样返回。
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
	case strings.HasSuffix(text, "B"), strings.HasSuffix(text, "b"):
		pow, text = 9, text[:len(text)-1]
	case strings.HasSuffix(text, "M"), strings.HasSuffix(text, "m"):
		pow, text = 6, text[:len(text)-1]
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
