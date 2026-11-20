package secukey

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// 本文件实现本地目标表 futu.finv_futu_secu_key 的批量增量写入（需求 11043）。
//
// 增量口径：
//   - ftmm_symbol（主键）不存在 → INSERT；
//   - 已存在且 9 个同步列中任一与源不同 → UPDATE；
//   - 已存在且完全一致 → 不改写（DO UPDATE 的 WHERE 条件为假，不返回该行，
//     因此不会触发 gmt_update 的 BEFORE UPDATE 触发器）。
//
// stock_id / usc 自上游补列后也纳入同步（需求 11062），但取 COALESCE(源, 本地) 语义：
// 上游这两列仍大量为空，直接覆盖会把本地已有的值清掉；`url` 出于同样原因也用 COALESCE
//（url 主要由「FT证券指标数据同步」从 secu_url 补全）。

// Store futu.finv_futu_secu_key 的写入与统计。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建写入器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// upsertBatchSQL 一批写入一条语句：jsonb 展开 → ON CONFLICT DO UPDATE（仅变化行）。
//
// `RETURNING (xmax = 0)` 区分本次是插入还是更新：新插入的行 xmax 为 0。
const upsertBatchSQL = `
WITH src AS (
    -- ftmm_symbol 入库统一大写（需求 11143）：此处是最后一道闸，任何调用方都过它
    SELECT upper(btrim(x.ftmm_symbol)) AS ftmm_symbol,
           btrim(x.market)      AS market,
           btrim(x.code)        AS code,
           btrim(x.instrument)  AS instrument,
           x.flag_enable::smallint AS flag_enable,
           x.priority::int         AS priority,
           btrim(x.status)      AS status,
           nullif(btrim(x.url), '') AS url,
           x.stock_id::bigint   AS stock_id,
           nullif(btrim(x.usc), '') AS usc
      FROM jsonb_to_recordset($1::jsonb) AS x(
             ftmm_symbol text, market text, code text, instrument text,
             flag_enable int, priority int, status text, url text,
             stock_id bigint, usc text)
), ins AS (
    INSERT INTO futu.finv_futu_secu_key AS t
          (instrument, market, code, ftmm_symbol, flag_enable, priority, status, url,
           stock_id, usc)
    SELECT instrument, market, code, ftmm_symbol, flag_enable, priority, status, url,
           stock_id, usc
      FROM src
    ON CONFLICT (ftmm_symbol) DO UPDATE
       SET instrument  = EXCLUDED.instrument,
           market      = EXCLUDED.market,
           code        = EXCLUDED.code,
           flag_enable = EXCLUDED.flag_enable,
           priority    = EXCLUDED.priority,
           status      = EXCLUDED.status,
           -- 源 url 为 NULL 时保留本地已有值：url 由「FT证券指标数据同步」（源 secu_url）补全，
           -- 源键表的 url 全为空，若直接覆盖会把指标同步刚补好的 url 清掉。
           url         = COALESCE(EXCLUDED.url, t.url),
           -- 同上：源为 NULL 时保留本地值（上游这两列仍有大量空值）
           stock_id    = COALESCE(EXCLUDED.stock_id, t.stock_id),
           usc         = COALESCE(EXCLUDED.usc, t.usc)
     WHERE (t.instrument, t.market, t.code, t.flag_enable, t.priority, t.status, t.url,
            t.stock_id, t.usc)
           IS DISTINCT FROM
           (EXCLUDED.instrument, EXCLUDED.market, EXCLUDED.code, EXCLUDED.flag_enable,
            EXCLUDED.priority, EXCLUDED.status, COALESCE(EXCLUDED.url, t.url),
            COALESCE(EXCLUDED.stock_id, t.stock_id), COALESCE(EXCLUDED.usc, t.usc))
    RETURNING (xmax = 0) AS inserted
)
SELECT COALESCE(count(*) FILTER (WHERE inserted), 0)::int          AS inserted,
       COALESCE(count(*) FILTER (WHERE NOT inserted), 0)::int     AS updated
  FROM ins`

// UpsertBatch 批量写入一批已归一化的行，返回「实际新增 / 实际更新」计数。
func (s *Store) UpsertBatch(ctx context.Context, rows []KeyRow) (UpsertStat, error) {
	var stat UpsertStat
	if len(rows) == 0 {
		return stat, nil
	}

	payload := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"ftmm_symbol": row.FtmmSymbol,
			"market":      row.Market,
			"code":        row.Code,
			"instrument":  row.Instrument,
			"flag_enable": row.FlagEnable,
			"priority":    row.Priority,
			"status":      row.Status,
		}
		if row.URL != nil {
			item["url"] = *row.URL
		}
		if row.StockID != nil {
			item["stock_id"] = *row.StockID
		}
		if row.USC != nil {
			item["usc"] = *row.USC
		}
		payload = append(payload, item)
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return stat, fmt.Errorf("序列化待写入行失败: %w", err)
	}

	if err := s.pool.QueryRow(ctx, upsertBatchSQL, string(raw)).Scan(&stat.Insert, &stat.Update); err != nil {
		return UpsertStat{}, fmt.Errorf("写入 %s 失败: %w", TargetTable, err)
	}
	return stat, nil
}

// TargetSummary 目标表当前行数与最近写入时间（RFC3339；空表时为空串）。
//
// 「最近写入时间」取 max(gmt_update)：一致行不改写，所以它等于最后一次真正发生变更的时刻。
func (s *Store) TargetSummary(ctx context.Context) (int64, string, error) {
	var (
		rows       int64
		lastUpdate *time.Time
	)
	err := s.pool.QueryRow(ctx, `
SELECT count(*)::bigint, max(gmt_update)
  FROM futu.finv_futu_secu_key`).Scan(&rows, &lastUpdate)
	if err != nil {
		return 0, "", fmt.Errorf("统计 %s 失败: %w", TargetTable, err)
	}
	if lastUpdate == nil {
		return rows, "", nil
	}
	return rows, lastUpdate.Format(time.RFC3339), nil
}
