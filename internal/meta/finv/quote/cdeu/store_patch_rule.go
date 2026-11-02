package cdeu

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// 本文件实现 USC 补丁规则的数据访问（需求 10931，方案见 Docs/Proposal/202611-0002）。
//
// 设计要点：
//   - 规则表 cdeu.finv_quote_cdeu_usc_patch_rule 保存「过时 usc → 目标 usc」映射，
//     支持动态增删改与停用，不写死在代码或迁移里；
//   - 执行时按 ts 区间分批，用 INSERT ... ON CONFLICT (ts, usc) DO UPDATE 规避主键冲突，
//     冲突时按 gmt_update 较新者胜；随后删除旧 usc 的行，插入与删除在同一语句内完成；
//   - 分批键是 ts，命中索引 idx_finv_quote_usc_ts，不做全表扫描。

const tableUscPatchRule = "cdeu.finv_quote_cdeu_usc_patch_rule"

// UscPatchRule USC 补丁规则。
type UscPatchRule struct {
	Idn        int64     `db:"idn" json:"idn"`
	UscOld     string    `db:"usc_old" json:"usc_old"`
	UscNew     string    `db:"usc_new" json:"usc_new"`
	MatchMode  string    `db:"match_mode" json:"match_mode"`
	FlagEnable string    `db:"flag_enable" json:"flag_enable"`
	Priority   int       `db:"priority" json:"priority"`
	Note       string    `db:"note" json:"note"`
	GmtCreate  time.Time `db:"gmt_create" json:"gmt_create"`
	GmtUpdate  time.Time `db:"gmt_update" json:"gmt_update"`
}

// UscPatchPreviewItem 试算结果：一条规则当前会影响多少行、其中多少行会与目标 usc 冲突。
type UscPatchPreviewItem struct {
	UscOld       string `json:"usc_old"`
	UscNew       string `json:"usc_new"`
	Rows         int64  `json:"rows"`
	ConflictRows int64  `json:"conflict_rows"`
	MinTs        int64  `json:"min_ts"`
	MaxTs        int64  `json:"max_ts"`
}

// UscPatchStats 一次 patch 执行的累计统计。
type UscPatchStats struct {
	RuleCount    int64 `json:"rule_count"`    // 生效规则数
	Batches      int64 `json:"batches"`       // 执行批次数
	ScannedRows  int64 `json:"scanned_rows"`  // 扫描到的旧 usc 行数
	MovedRows    int64 `json:"moved_rows"`    // 写入目标 usc 的行数
	ConflictRows int64 `json:"conflict_rows"` // 其中与目标 usc 已有行冲突、按较新者胜覆盖的行数
	DeletedRows  int64 `json:"deleted_rows"`  // 删除的旧 usc 行数
	FromTs       int64 `json:"from_ts"`
	ToTs         int64 `json:"to_ts"`

	// 最近一批的明细，供执行器逐批输出日志（不参与汇总）。
	LastBatchFrom int64 `json:"last_batch_from"`
	LastBatchTo   int64 `json:"last_batch_to"`
	LastBatchRows int64 `json:"last_batch_rows"`
	LastBatchMs   int64 `json:"last_batch_ms"`
}

// Enabled 判断规则是否生效。
func (r UscPatchRule) Enabled() bool { return r.FlagEnable == "1" }

// ListUscPatchRules 分页查询补丁规则。
func (s *Store) ListUscPatchRules(ctx context.Context, f AdminFilter) (Page[UscPatchRule], error) {
	conds := []string{}
	args := []any{}
	if f.Keyword != "" {
		args = append(args, "%"+f.Keyword+"%")
		conds = append(conds, fmt.Sprintf("(usc_old ILIKE $%d OR usc_new ILIKE $%d OR note ILIKE $%d)", len(args), len(args), len(args)))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		conds = append(conds, fmt.Sprintf("flag_enable = $%d", len(args)))
	}
	return pagedList[UscPatchRule](ctx, s, tableUscPatchRule,
		`idn, usc_old, usc_new, match_mode, flag_enable, priority, note, gmt_create, gmt_update`,
		conds, args, "flag_enable DESC, usc_old", f)
}

// SaveUscPatchRule 新增或更新规则（idn 为空表示新增）。
func (s *Store) SaveUscPatchRule(ctx context.Context, rule UscPatchRule) (int64, error) {
	uscOld := strings.TrimSpace(rule.UscOld)
	uscNew := strings.TrimSpace(rule.UscNew)
	if uscOld == "" || uscNew == "" {
		return 0, fmt.Errorf("usc_old 与 usc_new 不能为空")
	}
	if uscOld == uscNew {
		return 0, fmt.Errorf("usc_old 与 usc_new 不能相同")
	}
	mode := strings.TrimSpace(rule.MatchMode)
	if mode == "" {
		mode = "exact"
	}
	if mode != "exact" && mode != "prefix" {
		return 0, fmt.Errorf("match_mode 只能是 exact 或 prefix")
	}
	if rule.FlagEnable != "0" && rule.FlagEnable != "1" {
		rule.FlagEnable = "0"
	}
	if rule.Priority < 0 {
		rule.Priority = 0
	}
	note := strings.TrimSpace(rule.Note)

	if rule.Idn > 0 {
		tag, err := s.pool.Exec(ctx, `
UPDATE `+tableUscPatchRule+`
   SET usc_old = $2, usc_new = $3, match_mode = $4, flag_enable = $5,
       priority = $6, note = $7, gmt_update = now()
 WHERE idn = $1`, rule.Idn, uscOld, uscNew, mode, rule.FlagEnable, rule.Priority, note)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 0 {
			return 0, fmt.Errorf("规则不存在: idn=%d", rule.Idn)
		}
		return rule.Idn, nil
	}

	var idn int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO `+tableUscPatchRule+` (usc_old, usc_new, match_mode, flag_enable, priority, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING idn`, uscOld, uscNew, mode, rule.FlagEnable, rule.Priority, note).Scan(&idn)
	return idn, err
}

// SetUscPatchRuleEnable 启用或停用规则；停用不删除，便于追溯。
func (s *Store) SetUscPatchRuleEnable(ctx context.Context, idn int64, enable bool) error {
	flag := "0"
	if enable {
		flag = "1"
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE `+tableUscPatchRule+` SET flag_enable = $2, gmt_update = now() WHERE idn = $1`, idn, flag)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("规则不存在: idn=%d", idn)
	}
	return nil
}

// DeleteUscPatchRule 删除规则（建议优先停用，保留追溯）。
func (s *Store) DeleteUscPatchRule(ctx context.Context, idn int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM `+tableUscPatchRule+` WHERE idn = $1`, idn)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("规则不存在: idn=%d", idn)
	}
	return nil
}

// countEnabledUscPatchRules 返回生效规则数，为 0 时调用方直接跳过执行。
func (s *Store) countEnabledUscPatchRules(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM `+tableUscPatchRule+` WHERE flag_enable = '1'`).Scan(&count)
	return count, err
}

// PreviewUscPatch 试算：统计每条生效规则当前能命中多少行、其中多少行会与目标 usc 冲突。
// 只读，不写库。
func (s *Store) PreviewUscPatch(ctx context.Context) ([]UscPatchPreviewItem, error) {
	rows, err := s.pool.Query(ctx, `
SELECT p.usc_old,
       p.usc_new,
       count(m.ts) AS rows,
       count(*) FILTER (
           WHERE EXISTS (SELECT 1 FROM `+tableMain+` t
                          WHERE t.usc = p.usc_new AND t.ts = m.ts)
       ) AS conflict_rows,
       COALESCE(min(m.ts), 0) AS min_ts,
       COALESCE(max(m.ts), 0) AS max_ts
  FROM `+tableUscPatchRule+` p
  LEFT JOIN `+tableMain+` m ON m.usc = p.usc_old
 WHERE p.flag_enable = '1'
 GROUP BY p.usc_old, p.usc_new
 ORDER BY p.usc_old`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []UscPatchPreviewItem{}
	for rows.Next() {
		var item UscPatchPreviewItem
		if err := rows.Scan(&item.UscOld, &item.UscNew, &item.Rows,
			&item.ConflictRows, &item.MinTs, &item.MaxTs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// PatchOutdatedUsc 按生效规则把旧 usc 的行情迁移到新 usc。
//
// 参数：
//   - batchSize：单批扫描行数上限（<=0 时取 5000）；
//   - progress：可选回调，每批结束后调用，用于写 JOB 进度。
//
// 冲突处理：目标 usc 在同一 ts 已有行时，用 ON CONFLICT DO UPDATE 覆盖，
// 且只在「来源行更新更晚」（gmt_update 更大）时才覆盖，即较新者胜。
func (s *Store) PatchOutdatedUsc(ctx context.Context, batchSize int, progress func(stats UscPatchStats)) (UscPatchStats, error) {
	if batchSize <= 0 {
		batchSize = 5000
	}
	var stats UscPatchStats

	ruleCount, err := s.countEnabledUscPatchRules(ctx)
	if err != nil {
		return stats, err
	}
	stats.RuleCount = ruleCount
	if ruleCount == 0 {
		return stats, nil
	}

	// 待处理区间：只统计能命中生效规则的旧 usc
	var minTs, maxTs int64
	if err := s.pool.QueryRow(ctx, `
SELECT COALESCE(min(m.ts), 0), COALESCE(max(m.ts), 0)
  FROM `+tableMain+` m
  JOIN `+tableUscPatchRule+` p ON p.usc_old = m.usc AND p.flag_enable = '1'`).Scan(&minTs, &maxTs); err != nil {
		return stats, err
	}
	if maxTs == 0 {
		return stats, nil
	}
	stats.FromTs = minTs
	stats.ToTs = maxTs

	cursor := minTs - 1
	for cursor < maxTs {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		// 取下一批的 ts 上界
		var batchFrom, batchTo int64
		var batchRows int64
		if err := s.pool.QueryRow(ctx, `
SELECT COALESCE(min(t.ts), 0), COALESCE(max(t.ts), 0), count(*)
  FROM (SELECT m.ts
          FROM `+tableMain+` m
          JOIN `+tableUscPatchRule+` p ON p.usc_old = m.usc AND p.flag_enable = '1'
         WHERE m.ts > $1
         ORDER BY m.ts
         LIMIT $2) t`, cursor, batchSize).Scan(&batchFrom, &batchTo, &batchRows); err != nil {
			return stats, err
		}
		if batchRows == 0 || batchTo == 0 {
			break
		}

		batchStarted := time.Now()
		var scanned, moved, conflicts, deleted int64
		// 一条语句完成「写入新 usc → 删除旧 usc」：
		//   · DISTINCT ON 避免同一 ts 多条映射时触发 ON CONFLICT 二次影响同一行的错误；
		//   · ON CONFLICT 只在来源行更新更晚时覆盖；
		//   · DELETE 只匹配旧 usc，刚写入的新 usc 行不受影响。
		err := s.pool.QueryRow(ctx, `
WITH src AS (
    SELECT DISTINCT ON (p.usc_new, m.ts)
           p.usc_new AS usc, m.ts, m.date, m.time, m.open, m.high, m.low, m.close,
           m.vwap, m.volume, m.turnover, m.change_price, m.change_ratio,
           m.ext_field, m.last_close, m.remark, m.provider, m.gmt_update
      FROM `+tableMain+` m
      JOIN `+tableUscPatchRule+` p ON p.usc_old = m.usc AND p.flag_enable = '1'
     WHERE m.ts >= $1 AND m.ts <= $2
     ORDER BY p.usc_new, m.ts, p.priority
), moved AS (
    INSERT INTO `+tableMain+`
           (usc, ts, date, time, open, high, low, close, vwap, volume, turnover,
            change_price, change_ratio, ext_field, last_close, remark, provider)
    SELECT usc, ts, date, time, open, high, low, close, vwap, volume, turnover,
           change_price, change_ratio, ext_field, last_close, remark, provider
      FROM src
    ON CONFLICT (ts, usc) DO UPDATE
       SET open = EXCLUDED.open,
           high = EXCLUDED.high,
           low = EXCLUDED.low,
           close = EXCLUDED.close,
           vwap = EXCLUDED.vwap,
           volume = EXCLUDED.volume,
           turnover = EXCLUDED.turnover,
           change_price = EXCLUDED.change_price,
           change_ratio = EXCLUDED.change_ratio,
           ext_field = EXCLUDED.ext_field,
           last_close = EXCLUDED.last_close,
           remark = EXCLUDED.remark,
           provider = EXCLUDED.provider,
           gmt_update = now()
     WHERE `+tableMain+`.gmt_update < EXCLUDED.gmt_update
    RETURNING (xmax::text <> '0') AS conflict
), del AS (
    DELETE FROM `+tableMain+` m
     USING `+tableUscPatchRule+` p
     WHERE m.usc = p.usc_old
       AND p.flag_enable = '1'
       AND m.ts >= $1 AND m.ts <= $2
    RETURNING 1
)
SELECT (SELECT count(*) FROM src),
       (SELECT count(*) FROM moved),
       (SELECT count(*) FROM moved WHERE conflict),
       (SELECT count(*) FROM del)`, batchFrom, batchTo).
			Scan(&scanned, &moved, &conflicts, &deleted)
		if err != nil {
			return stats, err
		}

		stats.Batches++
		stats.LastBatchFrom = batchFrom
		stats.LastBatchTo = batchTo
		stats.LastBatchRows = scanned
		stats.LastBatchMs = time.Since(batchStarted).Milliseconds()
		stats.ScannedRows += scanned
		stats.MovedRows += moved
		stats.ConflictRows += conflicts
		stats.DeletedRows += deleted
		if progress != nil {
			progress(stats)
		}
		cursor = batchTo
	}
	return stats, nil
}
