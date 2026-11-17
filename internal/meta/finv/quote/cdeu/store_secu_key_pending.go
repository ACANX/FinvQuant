package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// 本文件处理「采集到但 usc 未解析」的证券（需求 11271 D1）：
// 新表 cdeu.finv_quote_secu_realtime 主键是 usc，usc 为空的行无法写入，
// 改为增量 upsert 到键表 futu.finv_futu_secu_key（status='PENDING'），
// 后续在键表补全 usc 后，下一次采集即可解析到 usc、不再告警。
//
// 语义约定：
//   - 只新增/更新「usc 仍为空」的键；已有 usc 的行一律不覆盖（保护人工补录结果）；
//   - ftmm_symbol = upper(market_label) || '.' || stock_code，与派生键口径一致；
//   - stock_id 为富途 64 位证券编号（键表该列是 BIGINT）。

// PendingSecuKeyRow 一条待补 usc 的证券键。
type PendingSecuKeyRow struct {
	// FtmmSymbol 派生键 `市场标签.代码`（大写）。
	FtmmSymbol string
	// Instrument 品种：stock / etf。
	Instrument string
	// Market 市场标签（BJ / SH / SZ / US / HK）。
	Market string
	// Code 证券代码。
	Code string
	// StockID 源系统证券编号（富途 stock_id，可选）。
	StockID int64
}

// UpsertPendingSecuKeys 把 usc 未解析的采集行增量写入键表，返回新增 / 更新 / 跳过数量。
//
// 跳过的是「键已存在且 usc 已非空」的行（已有确定 usc，不再当待办）。
func (s *Store) UpsertPendingSecuKeys(ctx context.Context, rows []PendingSecuKeyRow) (inserted int64, updated int64, skipped int64, err error) {
	if len(rows) == 0 {
		return 0, 0, 0, nil
	}
	payload := make([]map[string]any, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		symbol := strings.ToUpper(strings.TrimSpace(row.FtmmSymbol))
		if symbol == "" || symbol == "." {
			skipped++
			continue
		}
		if _, ok := seen[symbol]; ok {
			skipped++
			continue
		}
		seen[symbol] = struct{}{}
		payload = append(payload, map[string]any{
			"ftmm_symbol": symbol,
			"instrument":  strings.TrimSpace(row.Instrument),
			"market":      strings.ToUpper(strings.TrimSpace(row.Market)),
			"code":        strings.TrimSpace(row.Code),
			"stock_id":    row.StockID,
		})
	}
	if len(payload) == 0 {
		return 0, 0, skipped, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, 0, skipped, fmt.Errorf("UpsertPendingSecuKeys：序列化失败: %w", err)
	}

	rowsOut, err := s.pool.Query(ctx, `
WITH src AS (
    SELECT upper(btrim(x.ftmm_symbol)) AS ftmm_symbol,
           NULLIF(lower(btrim(x.instrument)), '') AS instrument,
           NULLIF(upper(btrim(x.market)), '')     AS market,
           NULLIF(btrim(x.code), '')              AS code,
           NULLIF(x.stock_id, 0)                  AS stock_id
      FROM jsonb_to_recordset($1::jsonb) AS x(
             ftmm_symbol text, instrument text, market text, code text, stock_id bigint)
), done AS (
    INSERT INTO `+tableSecuKey+` AS k
        (ftmm_symbol, instrument, market, code, stock_id, status, flag_enable)
    SELECT src.ftmm_symbol,
           COALESCE(src.instrument, 'stock'),
           COALESCE(src.market, ''),
           COALESCE(src.code, ''),
           src.stock_id,
           'PENDING',
           1
      FROM src
    ON CONFLICT (ftmm_symbol) DO UPDATE SET
        stock_id   = COALESCE(EXCLUDED.stock_id, k.stock_id),
        gmt_update = now()
    -- 已有 usc 的键不再当待办；交由人工/上游维护
    WHERE k.usc IS NULL OR btrim(k.usc) = ''
    RETURNING (xmax = 0) AS inserted
)
SELECT count(*) FILTER (WHERE inserted)::bigint,
       count(*) FILTER (WHERE NOT inserted)::bigint
  FROM done`, raw)
	if err != nil {
		return 0, 0, skipped, fmt.Errorf("UpsertPendingSecuKeys：写入 %s 失败: %w", tableSecuKey, err)
	}
	defer rowsOut.Close()
	for rowsOut.Next() {
		var ins, upd int64
		if err := rowsOut.Scan(&ins, &upd); err != nil {
			return 0, 0, skipped, fmt.Errorf("UpsertPendingSecuKeys：解析结果失败: %w", err)
		}
		inserted, updated = ins, upd
	}
	if err := rowsOut.Err(); err != nil {
		return 0, 0, skipped, fmt.Errorf("UpsertPendingSecuKeys：写入 %s 失败: %w", tableSecuKey, err)
	}
	return inserted, updated, skipped, nil
}

// LoadUscByFtmm 从键表批量取 `(market, code) → usc`，用于列表采集的 usc 兜底解析。
//
// 只返回 usc 非空的键；查不到即视为仍未解析。
func (s *Store) LoadUscByFtmm(ctx context.Context, keys []PendingSecuKeyRow) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	symbols := make([]string, 0, len(keys))
	for _, key := range keys {
		symbol := strings.ToUpper(strings.TrimSpace(key.FtmmSymbol))
		if symbol != "" && symbol != "." {
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
SELECT ftmm_symbol, btrim(usc)
  FROM `+tableSecuKey+`
 WHERE ftmm_symbol = ANY($1::text[])
   AND usc IS NOT NULL AND btrim(usc) <> ''`, symbols)
	if err != nil {
		return nil, fmt.Errorf("LoadUscByFtmm：查询 %s 失败: %w", tableSecuKey, err)
	}
	defer rows.Close()
	for rows.Next() {
		var symbol, usc string
		if err := rows.Scan(&symbol, &usc); err != nil {
			return nil, fmt.Errorf("LoadUscByFtmm：解析结果失败: %w", err)
		}
		out[symbol] = usc
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("LoadUscByFtmm：查询 %s 失败: %w", tableSecuKey, err)
	}
	return out, nil
}

// ftmmSymbolOf 派生 `市场标签.代码`（大写），与键表口径一致。
func ftmmSymbolOf(marketLabel, stockCode string) string {
	market := strings.ToUpper(strings.TrimSpace(marketLabel))
	code := strings.TrimSpace(stockCode)
	if market == "" || code == "" {
		return ""
	}
	return market + "." + code
}
