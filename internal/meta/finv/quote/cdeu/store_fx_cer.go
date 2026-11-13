package cdeu

import (
	"context"
	"encoding/json"
	"fmt"
)

// 本文件实现缺省汇率表 public.finv_fxcer（V300124 创建，需求 11002）的写入与查询。
//
// 表语义：一行 = 一个货币对的最新汇率，主键 usc（上游 stockCode / map key），
// rate = 1 单位 currency_src 折合的 currency_dest 数量；只保最新、不建历史表。
// 需求 11020 起不再派生 CNY 行，采集只写 flag_derive='0'（字段保留以兼容历史数据）。

// tableFxCER 缺省汇率表（public 域，显式写 schema，避免 search_path 命中歧义）。
const tableFxCER = "public.finv_fxcer"

// UpsertFxCER 批量写入缺省汇率（整批一条语句，无部分成功）。
//
// 语义：
//   - 主键 usc 不存在 → 新增；
//   - 已存在时的覆盖条件（两个都必须满足，否则该行原样保留）：
//   - (EXCLUDED.dt_quote >= 现有 dt_quote OR EXCLUDED.src <> 现有 src)：
//     同一来源内旧报价不得覆盖新报价；**跨来源以最后一次采集为准**——
//     overview 的 ts_quote 是上游给的时间、rateList 的是采集时刻，两个时钟不可比，
//     只比时间会让先跑的 rateList 永久压住 overview（实测其本地时刻更大）；
//   - EXCLUDED.flag_derive = '0' OR 现有 flag_derive = '1'：派生行不得覆盖直采行
//     （上游将来直供 CNY 时，直采行会把旧派生行升级为 flag_derive='0'）。
//   - 因此「本次实际改动行数」可能小于传入行数，统计以 RETURNING 为准。
func (s *Store) UpsertFxCER(ctx context.Context, rows []FxCERRow) (FxCERUpsertStat, error) {
	var stat FxCERUpsertStat
	if len(rows) == 0 {
		return stat, nil
	}

	payload := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		payload = append(payload, map[string]any{
			"usc":           row.USC,
			"name":          row.Name,
			"currency_src":  row.CurrencySrc,
			"currency_dest": row.CurrencyDest,
			"rate":          row.Rate,
			"change_price":  row.ChangePrice,
			"change_ratio":  row.ChangeRatio,
			"ts_quote":      row.TsQuote,
			"source":        row.Source,
			"flag_derive":   row.FlagDerive,
			"remark":        row.Remark,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return stat, fmt.Errorf("UpsertFxCER：序列化失败: %w", err)
	}

	// cer 先取 text 再由数据库转 numeric：不经过 float64，定点精度由 NUMERIC(24,10) 负责。
	err = s.pool.QueryRow(ctx, `
WITH src AS (
    SELECT nullif(btrim(x.usc), '')            AS usc,
           COALESCE(btrim(x.name), '')         AS name,
           nullif(btrim(x.currency_src), '')   AS currency_src,
           nullif(btrim(x.currency_dest), '')  AS currency_dest,
           nullif(btrim(x.rate), '')::numeric  AS rate,
           nullif(btrim(x.change_price), '')::numeric AS change_price,
           nullif(btrim(x.change_ratio), '')::numeric AS change_ratio,
           x.ts_quote::bigint                  AS ts_quote,
           nullif(btrim(x.source), '')         AS source,
           COALESCE(nullif(btrim(x.flag_derive), ''), '0') AS flag_derive,
           nullif(btrim(x.remark), '')         AS remark
      FROM jsonb_to_recordset($1::jsonb) AS x(
               usc text, name text, currency_src text, currency_dest text, rate text,
               change_price text, change_ratio text,
               ts_quote bigint, source text, flag_derive text, remark text)
), upserted AS (
    INSERT INTO public.finv_fxcer
        (usc, name, currency_src, currency_dest, rate, change_price, change_ratio,
         ts_quote, source, flag_derive, remark)
    SELECT usc, name, currency_src, currency_dest, rate, change_price, change_ratio,
           ts_quote, source, flag_derive, remark
      FROM src
    ON CONFLICT (usc) DO UPDATE
       SET name          = EXCLUDED.name,
           currency_src  = EXCLUDED.currency_src,
           currency_dest = EXCLUDED.currency_dest,
           rate          = EXCLUDED.rate,
           -- 涨跌额/涨跌幅只有 rateList 来源提供；overview 写入时为 NULL，
           -- 这里用 COALESCE 保留已有值，避免矩阵模式一轮采集把表格模式采到的涨跌数据清空（需求 11016）
           change_price  = COALESCE(EXCLUDED.change_price, public.finv_fxcer.change_price),
           change_ratio  = COALESCE(EXCLUDED.change_ratio, public.finv_fxcer.change_ratio),
           ts_quote      = EXCLUDED.ts_quote,
           source        = EXCLUDED.source,
           flag_derive   = EXCLUDED.flag_derive,
           remark        = EXCLUDED.remark
     WHERE (EXCLUDED.ts_quote >= public.finv_fxcer.ts_quote
            OR EXCLUDED.source <> public.finv_fxcer.source)
       AND (EXCLUDED.flag_derive = '0' OR public.finv_fxcer.flag_derive = '1')
    RETURNING (xmax = 0) AS inserted
)
SELECT count(*) FILTER (WHERE inserted)::bigint,
       count(*) FILTER (WHERE NOT inserted)::bigint
  FROM upserted`, raw).
		Scan(&stat.Insert, &stat.Update)
	if err != nil {
		return stat, fmt.Errorf("UpsertFxCER：写入缺省汇率失败: %w", err)
	}
	return stat, nil
}

// LoadFxCER 全量读取缺省汇率（88 行量级，不分页），按源/目标币种排序。
func (s *Store) LoadFxCER(ctx context.Context) ([]FxCERRow, error) {
	rows, err := s.pool.Query(ctx, `
SELECT usc,
       name,
       currency_src,
       currency_dest,
       rate::text,
       COALESCE(change_price::text, ''),
       COALESCE(change_ratio::text, ''),
       ts_quote,
       source,
       flag_derive,
       COALESCE(remark, ''),
       gmt_update
  FROM public.finv_fxcer
 ORDER BY usc`)
	if err != nil {
		return nil, fmt.Errorf("LoadFxCER：查询缺省汇率失败: %w", err)
	}
	defer rows.Close()

	list := make([]FxCERRow, 0, 96)
	for rows.Next() {
		var row FxCERRow
		if err := rows.Scan(&row.USC, &row.Name, &row.CurrencySrc, &row.CurrencyDest, &row.Rate,
			&row.ChangePrice, &row.ChangeRatio,
			&row.TsQuote, &row.Source, &row.FlagDerive, &row.Remark, &row.GmtUpdate); err != nil {
			return nil, fmt.Errorf("LoadFxCER：读取行失败: %w", err)
		}
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("LoadFxCER：遍历结果失败: %w", err)
	}
	return list, nil
}
