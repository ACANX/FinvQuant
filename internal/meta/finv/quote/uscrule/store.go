package uscrule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 本文件是 USC 映射规则四表 + 归档表的全部 SQL。
//
// 计算与预演都在**一个事务 + 会话级临时表**里完成（临时表随事务结束消失，互不干扰）：
//
//	_usc_cand    前处理后的候选（P10 闸门短路 → 条目命中 → 动作求值）
//	_usc_dup     候选里大小写不敏感撞名的键
//	_usc_final   冲突升级后的最终结果（撞名改用 ftmm_symbol）
//	_usc_diff    与结果表的差异（insert / update / keep）
//	_usc_keysync 键表 usc 与规则结果不一致的行（手工改值 / 外部同步覆盖 → 需回写）
//	_usc_retire  结果表在册但键表已不存在的行（retire → 标记 RETIRED）
//	_usc_reuse   usc 被新证券接管的行（复用 → 必须先归档下游）
//
// env 与规则版本通过 `set_config(..., true)` 注入（事务级），从而让多语句 DAO 不必逐条传参。

// Store 规则/条目/结果/归档的读写。
type Store struct {
	pool *pgxpool.Pool
}

// NewStore 创建读写器。
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// errConflict 升级后仍撞名（数据问题，需人工处理）。
var errConflict = errors.New("usc 冲突无法自动消解")

// buildCalcSQL 计算候选、升级冲突、生成差异与复用清单（依赖 finv.usc_env 已设置）。
const buildCalcSQL = `
DROP TABLE IF EXISTS _usc_cand, _usc_dup, _usc_final, _usc_diff, _usc_retire, _usc_reuse, _usc_keysync;

CREATE TEMP TABLE _usc_cand AS
WITH c AS (
    SELECT k.ftmm_symbol, k.market, k.code, k.instrument, k.canon_code, k.canon_cnt,
           -- 指数代码去前导点后，富途全码会出现双点（AU..XFL）：折叠成单点（AU.XFL）。
           regexp_replace(k.ftmm_symbol, '^([^.]+)\.\.', '\1.') AS ftmm_collapsed
      FROM futu.f_finv_futu_usc_calc(current_setting('finv.usc_env')) k
)
SELECT c.ftmm_symbol, c.market, c.code, c.instrument, c.canon_code, c.canon_cnt,
       c.ftmm_collapsed,
       CASE WHEN c.canon_cnt = 1 THEN 'P10' ELSE COALESCE(m.rule_no, 'P60') END AS rule_no,
       CASE WHEN c.canon_cnt = 1 THEN c.canon_code
            ELSE COALESCE(
                   CASE COALESCE(m.act, 'FTMM')
                        WHEN 'CODE'    THEN c.canon_code
                        WHEN 'LITERAL' THEN m.usc_literal
                        WHEN 'SH4'     THEN 'SH' || right(c.code, 4)
                        ELSE c.ftmm_collapsed
                   END, c.ftmm_collapsed)
       END AS usc_cand,
       CASE WHEN c.canon_cnt = 1 THEN '天然唯一（P10 闸门命中）' ELSE COALESCE(m.item_desc, '') END AS basis
  FROM c
  LEFT JOIN LATERAL (
        SELECT i.rule_no, i.item_desc,
               COALESCE(i.action, ru.default_action) AS act,
               i.usc_literal
          FROM futu.finv_futu_usc_rule_item i
          JOIN futu.finv_futu_usc_rule ru ON ru.rule_no = i.rule_no
         WHERE ru.flag_enable = '1'
           AND ru.match_scope = 'NON_GATE'
           AND i.status = 'ACTIVE'
           AND i.scope_env IN ('*', current_setting('finv.usc_env'))
           AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
           AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
           AND (i.market     IS NULL OR i.market     = c.market)
           AND (i.instrument IS NULL OR i.instrument = c.instrument)
           AND ( (i.code IS NOT NULL AND lower(i.code) = lower(c.canon_code))
              OR (i.code IS NULL AND i.code_regex IS NOT NULL AND c.code ~* i.code_regex)
              OR (i.code IS NULL AND i.code_regex IS NULL) )
         -- 先分层再比规则优先级：精确 code（P40/P50）→ 正则（市场内特例）→ 市场默认（P20/P21）→ 兜底（P60）。
         -- 只按 ru.priority 排会让市场默认（P20=20）压过例外（P40=40/P50=50），实测 660 条例外里 658 条被抢走。
         ORDER BY CASE WHEN i.code IS NOT NULL THEN 1
                       WHEN i.code_regex IS NOT NULL THEN 2
                       WHEN i.market IS NOT NULL THEN 3
                       ELSE 4 END,
                  ru.priority, i.priority
         LIMIT 1) m ON true;

CREATE TEMP TABLE _usc_dup AS
SELECT lower(usc_cand) AS k, count(*) AS n FROM _usc_cand GROUP BY 1 HAVING count(*) > 1;
CREATE UNIQUE INDEX ON _usc_dup (k);

CREATE TEMP TABLE _usc_final AS
SELECT c.ftmm_symbol, c.market, c.code, c.instrument,
       CASE WHEN d.k IS NULL THEN c.usc_cand ELSE c.ftmm_collapsed END AS usc,
       CASE WHEN d.k IS NULL THEN c.rule_no ELSE 'ESCALATE' END AS rule_no,
       CASE WHEN d.k IS NULL THEN c.basis
            ELSE '冲突升级：候选 usc 与其它证券撞名，改用 ftmm_symbol' END AS basis
  FROM _usc_cand c
  LEFT JOIN _usc_dup d ON d.k = lower(c.usc_cand);

CREATE TEMP TABLE _usc_diff AS
SELECT f.ftmm_symbol, f.market, f.code, f.instrument,
       m.usc AS current_usc, f.usc AS new_usc, f.rule_no, f.basis,
       CASE WHEN m.ftmm_symbol IS NULL THEN 'insert'
            WHEN lower(m.usc) IS DISTINCT FROM lower(f.usc) THEN 'update'
            ELSE 'keep' END AS diff
  FROM _usc_final f
  LEFT JOIN futu.finv_futu_usc_map m
         ON m.ftmm_symbol = f.ftmm_symbol AND m.status = 'ACTIVE';

-- 键表漂移：结果表（或本次算出的值）与该行键表 usc 不一致。
--
-- 关键点：_usc_diff 只比「结果表 vs 本次计算」，**看不到手工改键表**——
-- 手工把 futu.finv_futu_secu_key.usc 改掉后，结果表没变 → diff='keep' → 预演显示「无变更」，
-- 用户以为规则没生效。这里单独把键表漂移抓出来，预演可见、应用时回写。
CREATE TEMP TABLE _usc_keysync AS
SELECT f.ftmm_symbol, f.market, f.code, f.instrument,
       k.usc AS current_usc, f.usc AS new_usc, f.rule_no, f.basis
  FROM _usc_final f
  JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = f.ftmm_symbol
 WHERE k.usc IS DISTINCT FROM f.usc
   -- 结果表本身就要改的行已在 insert/update 里列出，不重复计数
   AND NOT EXISTS (SELECT 1 FROM _usc_diff d
                    WHERE d.ftmm_symbol = f.ftmm_symbol AND d.diff <> 'keep');

CREATE TEMP TABLE _usc_retire AS
SELECT m.ftmm_symbol, m.market, m.code, m.instrument, m.usc
  FROM futu.finv_futu_usc_map m
 WHERE m.status = 'ACTIVE'
   AND NOT EXISTS (SELECT 1 FROM _usc_final f WHERE f.ftmm_symbol = m.ftmm_symbol);

CREATE TEMP TABLE _usc_reuse AS
SELECT d.ftmm_symbol, d.new_usc, p.from_ftmm
  FROM _usc_diff d
  JOIN LATERAL (
        SELECT x.ftmm_symbol AS from_ftmm
          FROM futu.finv_futu_usc_map x
         WHERE x.usc_lower = lower(d.new_usc)
           AND x.ftmm_symbol <> d.ftmm_symbol
           AND ( x.status = 'RETIRED'
                 OR x.ftmm_symbol IN (SELECT r.ftmm_symbol FROM _usc_retire r)
                 OR x.ftmm_symbol IN (SELECT c.ftmm_symbol FROM _usc_diff c WHERE c.diff = 'update') )
         ORDER BY (x.status = 'RETIRED') DESC, x.dt_retire DESC NULLS LAST
         LIMIT 1) p ON true
 WHERE d.diff IN ('insert', 'update');
`

// conflictSQL 升级后仍撞名（说明两条规则指向同一个 usc 且都无条件前缀）。
const conflictSQL = `
SELECT lower(usc) || ' ×' || count(*) AS detail
  FROM _usc_final
 GROUP BY lower(usc) HAVING count(*) > 1
 ORDER BY 1 LIMIT 20`

// summarySQL 预演统计。
const summarySQL = `
SELECT COALESCE(count(*) FILTER (WHERE diff = 'insert'), 0)::bigint     AS insert,
       COALESCE(count(*) FILTER (WHERE diff = 'update'), 0)::bigint   AS update,
       COALESCE(count(*) FILTER (WHERE diff = 'keep'), 0)::bigint AS keep,
       (SELECT count(*) FROM _usc_retire)::bigint                      AS retire,
       (SELECT count(*) FROM _usc_reuse)::bigint                       AS reuse,
       (SELECT count(*) FROM _usc_cand)::bigint                        AS total,
       (SELECT count(*) FROM _usc_final WHERE rule_no = 'ESCALATE')::bigint AS escalate,
       (SELECT count(*) FROM _usc_keysync)::bigint                     AS key_update
  FROM _usc_diff`

// diffSummary summarySQL 的扫描目标。
//
// 预演与应用共用同一份统计 SQL：**列序与目标字段必须一一对应**，
// 因此统一走 scanSummary，禁止在调用处各自写 Scan（曾因应用侧少扫 `total` 一列而报
// 「number of field descriptions must equal number of destinations, got 7 and 6」）。
type diffSummary struct {
	Insert    int64
	Update    int64
	Keep      int64
	Retire    int64
	Reuse     int64
	Total     int64
	Escalate  int64
	KeyUpdate int64
}

// scanSummary 按 summarySQL 的固定列序扫描。
func scanSummary(row pgx.Row) (diffSummary, error) {
	var sum diffSummary
	err := row.Scan(&sum.Insert, &sum.Update, &sum.Keep, &sum.Retire,
		&sum.Reuse, &sum.Total, &sum.Escalate, &sum.KeyUpdate)
	return sum, err
}

// tokenSQL 预演令牌：最终结果的整体指纹，应用时校验「预演之后规则/数据是否变过」。
const tokenSQL = `SELECT md5(string_agg(ftmm_symbol || ':' || usc, ',' ORDER BY ftmm_symbol)) FROM _usc_final`

// diffRowsSQL 预演明细全量（变更 + 退市，按类别与 ftmm_symbol 排序）。
//
// 预演响应只回前 MaxPreviewSamples 行；完整明细由 Service 缓存在内存，页面通过
// `Diff/Page` 分页浏览（重新预演会重置缓存）。
const diffRowsSQL = `
SELECT d.ftmm_symbol, d.market, d.code, d.instrument, d.current_usc, d.new_usc,
       d.rule_no, d.basis, d.diff, r.from_ftmm
  FROM _usc_diff d
  LEFT JOIN _usc_reuse r ON r.ftmm_symbol = d.ftmm_symbol
 WHERE d.diff <> 'keep'
 UNION ALL
SELECT t.ftmm_symbol, t.market, t.code, t.instrument, t.usc, NULL,
       'RETIRE', '键表中已不存在该证券，标记退市（usc 释放，允许后续复用）', 'retire', NULL
  FROM _usc_retire t
 UNION ALL
SELECT s.ftmm_symbol, s.market, s.code, s.instrument, s.current_usc, s.new_usc,
       s.rule_no, '键表 usc 与规则结果不一致（手工改值或外部同步覆盖），应用时回写', 'key_update', NULL
  FROM _usc_keysync s
 ORDER BY 9, 1`

// reuseStatSQL 复用行（供应用结果与日志使用）。
const reuseSQL = `
SELECT d.ftmm_symbol, d.market, d.code, d.instrument, d.current_usc, d.new_usc,
       d.rule_no, d.basis, d.diff, r.from_ftmm
  FROM _usc_diff d
  JOIN _usc_reuse r ON r.ftmm_symbol = d.ftmm_symbol
 ORDER BY d.ftmm_symbol`

// downstreamTables 复用时要归档的下游表：表名 + 主键表达式 + 关联条件。
//
// 顺序即归档顺序；finv_security 是主数据，放最前面便于排查。
var downstreamTables = []struct {
	Table  string
	SrcKey string // 主键文本表达式（需用别名 x）
}{
	{"public.finv_security", "x.usc"},
	{"futu.finv_futu_quote_collect", "x.usc"},
	{"cdeu.finv_quote_secu_indicator", "x.usc"},
	{"cdeu.finv_quote_secu_kline_day", "x.usc || '@' || x.date::text"},
	{"cdeu.finv_quote_cdeu_usc", "x.usc"},
}

// RuleVersion 规则集版本：规则层 + 条目层最后变更的指纹。
func (s *Store) RuleVersion(ctx context.Context) (string, error) {
	var version string
	err := s.pool.QueryRow(ctx, `
SELECT md5(COALESCE((SELECT string_agg(rule_no || ':' || priority || ':' || flag_enable, ',' ORDER BY priority)
                       FROM futu.finv_futu_usc_rule), '')
           || '|' ||
           COALESCE((SELECT count(*)::text || ':' || COALESCE(max(gmt_update)::text, '')
                       FROM futu.finv_futu_usc_rule_item), ''))`).Scan(&version)
	if err != nil {
		return "", fmt.Errorf("计算规则版本失败: %w", err)
	}
	return version[:12], nil
}

// Prepare 在事务里设置 env 与规则版本，并建好临时表。
func (s *Store) prepare(ctx context.Context, tx pgx.Tx, env string) error {
	if env == "" {
		env = DefaultEnv
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('finv.usc_env', $1, true)`, env); err != nil {
		return fmt.Errorf("设置环境参数失败: %w", err)
	}
	version, err := s.RuleVersion(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('finv.usc_rule_version', $1, true)`, version); err != nil {
		return fmt.Errorf("设置规则版本失败: %w", err)
	}
	if _, err := tx.Exec(ctx, buildCalcSQL); err != nil {
		return fmt.Errorf("计算 USC 候选失败: %w", err)
	}
	rows, err := tx.Query(ctx, conflictSQL)
	if err != nil {
		return fmt.Errorf("检查 usc 冲突失败: %w", err)
	}
	var conflicts []string
	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			rows.Close()
			return fmt.Errorf("读取 usc 冲突失败: %w", err)
		}
		conflicts = append(conflicts, detail)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("读取 usc 冲突失败: %w", err)
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%w：%s", errConflict, strings.Join(conflicts, "；"))
	}
	return nil
}

// Preview 预演：算差异后整体回滚，不写任何表。
//
// 第二个返回值是**全量**差异明细（不含 keep），供 Service 缓存后分页展示。
func (s *Store) Preview(ctx context.Context, env string) (Preview, []DiffRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Preview{}, nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.prepare(ctx, tx, env); err != nil {
		return Preview{}, nil, err
	}

	preview := Preview{Env: normalizeEnv(env), ComputeAt: time.Now().Format(time.RFC3339)}
	sum, err := scanSummary(tx.QueryRow(ctx, summarySQL))
	if err != nil {
		return Preview{}, nil, fmt.Errorf("统计差异失败: %w", err)
	}
	preview.Insert, preview.Update, preview.Keep = sum.Insert, sum.Update, sum.Keep
	preview.Retire, preview.Reuse, preview.Total, preview.Escalate =
		sum.Retire, sum.Reuse, sum.Total, sum.Escalate
	preview.KeyUpdate = sum.KeyUpdate
	if err := tx.QueryRow(ctx, tokenSQL).Scan(&preview.PreviewToken); err != nil {
		return Preview{}, nil, fmt.Errorf("计算预演令牌失败: %w", err)
	}
	if preview.RuleVersion, err = s.RuleVersion(ctx); err != nil {
		return Preview{}, nil, err
	}
	rows, err := s.queryDiffRows(ctx, tx, diffRowsSQL)
	if err != nil {
		return Preview{}, nil, err
	}
	// 响应只带前若干行，避免 7 万行 JSON 拖慢页面；完整明细随 Service 缓存返回。
	if len(rows) > MaxPreviewSamples {
		preview.Samples = rows[:MaxPreviewSamples]
	} else {
		preview.Samples = rows
	}
	preview.Conflicts = []string{}
	return preview, rows, nil
}

// queryDiffRows 执行差异查询（列顺序固定）。
func (s *Store) queryDiffRows(ctx context.Context, tx pgx.Tx, query string) ([]DiffRow, error) {
	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询差异明细失败: %w", err)
	}
	defer rows.Close()
	out := make([]DiffRow, 0, 32)
	for rows.Next() {
		var row DiffRow
		if err := rows.Scan(&row.FtmmSymbol, &row.Market, &row.Code, &row.Instrument,
			&row.CurrentUSC, &row.NewUSC, &row.RuleNo, &row.Basis, &row.Diff, &row.ReuseOf); err != nil {
			return nil, fmt.Errorf("读取差异明细失败: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Apply 应用一次映射：归档复用 → 退市 → 写结果表 → 回写键表 → 记账，单事务。
//
// previewToken 非空时校验当前计算结果与预演一致（避免「预演后规则又变了」直接落库）。
func (s *Store) Apply(ctx context.Context, env, previewToken, operator, reason string) (Result, error) {
	started := time.Now()
	result := Result{Env: normalizeEnv(env)}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.prepare(ctx, tx, env); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('finv.usc_operator', $1, true)`, nonEmpty(operator, "finv-scheduler")); err != nil {
		return result, fmt.Errorf("设置操作人失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('finv.usc_reason', $1, true)`, reason); err != nil {
		return result, fmt.Errorf("设置操作原因失败: %w", err)
	}

	// 预演令牌校验：预演与应用的输入必须完全一致。
	var token string
	if err := tx.QueryRow(ctx, tokenSQL).Scan(&token); err != nil {
		return result, fmt.Errorf("计算预演令牌失败: %w", err)
	}
	if previewToken != "" && previewToken != token {
		return result, fmt.Errorf("预演结果已过期（规则或键表在预演之后发生变化），请重新预演后再应用")
	}
	result.PreviewAt = time.Now().Format(time.RFC3339)

	// 统计（与预演同口径，共用 scanSummary）。
	sum, err := scanSummary(tx.QueryRow(ctx, summarySQL))
	if err != nil {
		return result, fmt.Errorf("统计差异失败: %w", err)
	}
	result.Insert, result.Update, result.Keep = sum.Insert, sum.Update, sum.Keep
	result.Retire, result.Reuse, result.Escalate = sum.Retire, sum.Reuse, sum.Escalate

	if err := s.archiveReuse(ctx, tx, &result); err != nil {
		return result, err
	}

	// 先退市（含将要改值的行）：释放 usc 占位，避免部分唯一索引在换值时瞬时冲突。
	if _, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_usc_map m
   SET status = 'RETIRED',
       dt_retire = COALESCE(m.dt_retire, now())
 WHERE m.status = 'ACTIVE'
   AND ( m.ftmm_symbol IN (SELECT ftmm_symbol FROM _usc_retire)
      OR m.ftmm_symbol IN (SELECT ftmm_symbol FROM _usc_diff WHERE diff = 'update') )`); err != nil {
		return result, fmt.Errorf("标记退市失败: %w", err)
	}

	// 写结果表（新增 + 改值；未变行不动）。
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_map
      (ftmm_symbol, market, code, instrument, usc, rule_no, basis, status, dt_retire,
       reuse_of_ftmm, scope_env, rule_version, dt_compute, dt_first_assign)
-- ftmm_symbol 入库统一大写（需求 11143）
SELECT upper(d.ftmm_symbol), d.market, d.code, d.instrument, d.new_usc, d.rule_no, d.basis,
       'ACTIVE', NULL, r.from_ftmm,
       current_setting('finv.usc_env'), current_setting('finv.usc_rule_version'), now(), now()
  FROM _usc_diff d
  LEFT JOIN _usc_reuse r ON r.ftmm_symbol = d.ftmm_symbol
 WHERE d.diff <> 'keep'
ON CONFLICT (ftmm_symbol) DO UPDATE
   SET usc           = EXCLUDED.usc,
       rule_no       = EXCLUDED.rule_no,
       basis         = EXCLUDED.basis,
       status        = 'ACTIVE',
       dt_retire    = NULL,
       reuse_of_ftmm = COALESCE(EXCLUDED.reuse_of_ftmm, futu.finv_futu_usc_map.reuse_of_ftmm),
       scope_env     = EXCLUDED.scope_env,
       rule_version  = EXCLUDED.rule_version,
       market        = EXCLUDED.market,
       code          = EXCLUDED.code,
       instrument    = EXCLUDED.instrument,
       dt_compute   = now()`); err != nil {
		return result, fmt.Errorf("写入结果表失败: %w", err)
	}

	// 在册行的计算时间统一刷新（未变行不改 usc，但整次重算时间应前移）。
	if _, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_usc_map m
   SET dt_compute = now()
 WHERE m.status = 'ACTIVE'
   AND m.ftmm_symbol IN (SELECT ftmm_symbol FROM _usc_final)`); err != nil {
		return result, fmt.Errorf("刷新计算时间失败: %w", err)
	}

	// 回写键表 usc（只写变化行）。
	tag, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_secu_key k
   SET usc = f.usc
  FROM _usc_final f
 WHERE k.ftmm_symbol = f.ftmm_symbol
   AND k.usc IS DISTINCT FROM f.usc`)
	if err != nil {
		return result, fmt.Errorf("回写键表 usc 失败: %w", err)
	}
	result.KeyUpdate = tag.RowsAffected()

	// 记账（两次批量插入：一次汇总、一次逐条复用）。
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
VALUES ('map', 'COMPUTE', current_setting('finv.usc_env'), NULL,
        jsonb_build_object('insert', $1::bigint, 'update', $2::bigint, 'retire', $3::bigint,
                           'reuse', $4::bigint, 'keep', $5::bigint, 'escalate', $6::bigint,
                           'archive', $7::bigint, 'key_update', $8::bigint,
                           'rule_version', current_setting('finv.usc_rule_version'),
                           'preview_at', $9::text),
        current_setting('finv.usc_operator'), current_setting('finv.usc_reason'))`,
		result.Insert, result.Update, result.Retire, result.Reuse, result.Keep,
		result.Escalate, result.Archive, result.KeyUpdate, result.PreviewAt); err != nil {
		return result, fmt.Errorf("写审计日志失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
SELECT 'map', 'COMPUTE', r.ftmm_symbol,
       jsonb_build_object('reuse_of_ftmm', r.from_ftmm, 'usc', r.new_usc),
       NULL, current_setting('finv.usc_operator'), 'usc 复用：已归档下游历史行'
  FROM _usc_reuse r`); err != nil {
		return result, fmt.Errorf("写复用审计日志失败: %w", err)
	}

	result.RuleVersion = ""
	_ = tx.QueryRow(ctx, `SELECT current_setting('finv.usc_rule_version')`).Scan(&result.RuleVersion)
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("提交失败: %w", err)
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result, nil
}

// archiveReuse 把「usc 被新证券接管」的下游历史行整行快照进 archive.finv_usc_reuse_snapshot。
//
// 顺序硬约束：必须先归档再让旧行让位/写新行，否则历史数据不可恢复。
// 表可能不存在（新环境 / 历史版本），用 to_regclass 守卫并跳过。
func (s *Store) archiveReuse(ctx context.Context, tx pgx.Tx, result *Result) error {
	var total int64
	for _, item := range downstreamTables {
		var exists *string
		if err := tx.QueryRow(ctx, `SELECT to_regclass($1)::text`, item.Table).Scan(&exists); err != nil {
			return fmt.Errorf("检查表 %s 是否存在失败: %w", item.Table, err)
		}
		if exists == nil {
			continue
		}
		query := fmt.Sprintf(`
INSERT INTO archive.finv_usc_reuse_snapshot
      (usc, src_table, src_key, row_json, from_ftmm, to_ftmm, reason)
SELECT r.new_usc, '%s', %s, to_jsonb(x), r.from_ftmm, r.ftmm_symbol, 'usc 复用归档'
  FROM _usc_reuse r
  JOIN %s x ON lower(x.usc) = lower(r.new_usc)
ON CONFLICT DO NOTHING`, item.Table, item.SrcKey, item.Table)
		tag, err := tx.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("归档 %s 失败: %w", item.Table, err)
		}
		total += tag.RowsAffected()
	}
	result.Archive = total
	return nil
}

// ListRules 规则层列表（带条目数）。
func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.pool.Query(ctx, `
SELECT r.rule_no, r.rule_name, r.rule_type, r.priority, r.match_scope, r.default_action,
       r.flag_enable, r.rule_desc, r.remark,
       to_char(r.gmt_update, 'YYYY-MM-DD HH24:MI:SS'),
       COALESCE(i.cnt, 0),
       COALESCE(i.cnt_active, 0)
  FROM futu.finv_futu_usc_rule r
  LEFT JOIN (SELECT rule_no,
                    count(*)::int AS cnt,
                    count(*) FILTER (WHERE status = 'ACTIVE')::int AS cnt_active
               FROM futu.finv_futu_usc_rule_item GROUP BY 1) i
         ON i.rule_no = r.rule_no
 ORDER BY r.priority`)
	if err != nil {
		return nil, fmt.Errorf("查询规则失败: %w", err)
	}
	defer rows.Close()
	out := make([]Rule, 0, 16)
	for rows.Next() {
		var rule Rule
		if err := rows.Scan(&rule.RuleNo, &rule.RuleName, &rule.RuleType, &rule.Priority,
			&rule.MatchScope, &rule.DefaultAction, &rule.FlagEnable, &rule.RuleDesc,
			&rule.Remark, &rule.GmtUpdate, &rule.ItemCount, &rule.ItemActive); err != nil {
			return nil, fmt.Errorf("读取规则失败: %w", err)
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// ListMapRows 结果表分页查询：关键字同时匹配 ftmm_symbol / code / usc（大小写不敏感）。
//
// 「预演与应用」页要能在几万条差异里直接定位某一只证券，不必翻页。
func (s *Store) ListMapRows(ctx context.Context, filter MapFilter) ([]MapRow, int64, error) {
	page, pageSize := normalizePager(filter.Page, filter.PageSize)
	args := make([]any, 0, 4)
	where := make([]string, 0, 4)
	if kw := strings.TrimSpace(filter.Keyword); kw != "" {
		args = append(args, "%"+kw+"%")
		n := len(args)
		where = append(where, fmt.Sprintf(
			"(ftmm_symbol ILIKE $%d OR code ILIKE $%d OR usc ILIKE $%d)", n, n, n))
	}
	if v := strings.TrimSpace(filter.Market); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("market = $%d", len(args)))
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		args = append(args, v)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	clause := "TRUE"
	if len(where) > 0 {
		clause = strings.Join(where, " AND ")
	}

	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM futu.finv_futu_usc_map WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计结果表失败: %w", err)
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
SELECT ftmm_symbol, market, code, instrument, usc, rule_no, basis, status, scope_env, rule_version,
       to_char(dt_compute, 'YYYY-MM-DD HH24:MI:SS'),
       to_char(dt_retire, 'YYYY-MM-DD HH24:MI:SS'),
       reuse_of_ftmm
  FROM futu.finv_futu_usc_map
 WHERE %s
 ORDER BY ftmm_symbol
 LIMIT $%d OFFSET $%d`, clause, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询结果表失败: %w", err)
	}
	defer rows.Close()

	out := make([]MapRow, 0, pageSize)
	for rows.Next() {
		var row MapRow
		if err := rows.Scan(&row.FtmmSymbol, &row.Market, &row.Code, &row.Instrument, &row.USC,
			&row.RuleNo, &row.Basis, &row.Status, &row.ScopeEnv, &row.RuleVersion,
			&row.DtCompute, &row.DtRetire, &row.ReuseOfFtmm); err != nil {
			return nil, 0, fmt.Errorf("读取结果表行失败: %w", err)
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// ListItems 条目分页查询。
func (s *Store) ListItems(ctx context.Context, filter ItemFilter) ([]Item, int64, error) {
	page, pageSize := normalizePager(filter.Page, filter.PageSize)
	args := []any{}
	where := make([]string, 0, 4)
	if filter.RuleNo != "" {
		args = append(args, filter.RuleNo)
		where = append(where, fmt.Sprintf("rule_no = $%d", len(args)))
	}
	if filter.Market != "" {
		args = append(args, filter.Market)
		where = append(where, fmt.Sprintf("market = $%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		args = append(args, "%"+strings.TrimSpace(filter.Keyword)+"%")
		where = append(where, fmt.Sprintf("(code ILIKE $%d OR item_desc ILIKE $%d OR usc_literal ILIKE $%d)", len(args), len(args), len(args)))
	}
	clause := "TRUE"
	if len(where) > 0 {
		clause = strings.Join(where, " AND ")
	}

	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_usc_rule_item WHERE `+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计条目失败: %w", err)
	}

	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
SELECT idn, rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
       status, to_char(effective_from, 'YYYY-MM-DD'), to_char(effective_to, 'YYYY-MM-DD'),
       priority, scope_env, item_desc, source_ref, remark,
       to_char(gmt_update, 'YYYY-MM-DD HH24:MI:SS')
  FROM futu.finv_futu_usc_rule_item
 WHERE %s
 ORDER BY rule_no, market NULLS FIRST, instrument NULLS FIRST, code NULLS FIRST, code_regex NULLS FIRST
 LIMIT $%d OFFSET $%d`, clause, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询条目失败: %w", err)
	}
	defer rows.Close()
	out := make([]Item, 0, pageSize)
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.IDN, &item.RuleNo, &item.Market, &item.Instrument, &item.Code,
			&item.CodeRegex, &item.Action, &item.USCLiteral, &item.RewriteTpl, &item.Status,
			&item.EffectiveFrom, &item.EffectiveTo, &item.Priority, &item.ScopeEnv,
			&item.ItemDesc, &item.SourceRef, &item.Remark, &item.GmtUpdate); err != nil {
			return nil, 0, fmt.Errorf("读取条目失败: %w", err)
		}
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// ErrItemExists 条目自然键（rule_no + scope_env + market + instrument + code + code_regex）
// 已被占用。保存路径据此返回 409 与可读提示，而不是把 23505 原始报错抛给前端。
var ErrItemExists = errors.New("条目已存在")

// SaveItem 新增或更新条目（idn=0 新增），返回 idn。
//
// 校验口径与迁移里的 CHECK 一致：LITERAL 必须有 usc_literal、REWRITE 必须有 code_regex+rewrite_tpl、
// 且 item_desc 必填（Web 维护页要展示「为什么这么映射」）。
func (s *Store) SaveItem(ctx context.Context, item Item, operator string) (int64, error) {
	item.RuleNo = strings.TrimSpace(item.RuleNo)
	if item.RuleNo == "" {
		return 0, fmt.Errorf("rule_no 不能为空")
	}
	item.Status = nonEmpty(item.Status, "ACTIVE")
	item.ScopeEnv = nonEmpty(item.ScopeEnv, DefaultEnv)
	if item.Priority <= 0 {
		item.Priority = 100
	}
	if strings.TrimSpace(item.ItemDesc) == "" {
		return 0, fmt.Errorf("item_desc（条目说明）不能为空")
	}
	action := ""
	if item.Action != nil {
		action = strings.TrimSpace(*item.Action)
	}
	switch action {
	case ActionLiteral:
		if item.USCLiteral == nil || strings.TrimSpace(*item.USCLiteral) == "" {
			return 0, fmt.Errorf("action=LITERAL 时 usc_literal 不能为空")
		}
	case ActionRewrite:
		if item.CodeRegex == nil || item.RewriteTpl == nil {
			return 0, fmt.Errorf("action=REWRITE 时 code_regex 与 rewrite_tpl 都不能为空")
		}
	}
	if item.Code != nil && item.CodeRegex != nil {
		return 0, fmt.Errorf("code 与 code_regex 只能二选一")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before *string
	if item.IDN > 0 {
		var snapshot string
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule_item t WHERE idn = $1`, item.IDN).Scan(&snapshot); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, fmt.Errorf("条目不存在: idn=%d", item.IDN)
			}
			return 0, fmt.Errorf("读取原条目失败: %w", err)
		}
		before = &snapshot
	} else {
		item.IDN = 0
		// 自然键唯一约束 finv_futu_usc_rule_item_key 是 (rule_no, scope_env, market,
		// instrument, code, code_regex) NULLS NOT DISTINCT —— 与主键 idn 无关。
		// 新增前先查重，命中就给出「改编辑哪个条目」的提示，避免抛裸的 23505。
		var existIDN int64
		var existStatus, existDesc string
		err := tx.QueryRow(ctx, `
SELECT idn, status, item_desc FROM futu.finv_futu_usc_rule_item
 WHERE rule_no = $1
   AND scope_env IS NOT DISTINCT FROM $2
   AND market IS NOT DISTINCT FROM $3
   AND instrument IS NOT DISTINCT FROM $4
   AND code IS NOT DISTINCT FROM $5
   AND code_regex IS NOT DISTINCT FROM $6`,
			item.RuleNo, item.ScopeEnv, item.Market, item.Instrument, item.Code, item.CodeRegex).
			Scan(&existIDN, &existStatus, &existDesc)
		switch {
		case err == nil:
			return 0, fmt.Errorf("%w：%s（idn=%d，状态 %s，说明「%s」）。请直接编辑该条目，或先把它停用后再新增同键条目",
				ErrItemExists, itemKeyText(item), existIDN, existStatus, existDesc)
		case !errors.Is(err, pgx.ErrNoRows):
			return 0, fmt.Errorf("检查条目是否已存在失败: %w", err)
		}
	}

	// 编辑与新增走两条语句。
	//
	// 历史实现是「一条 INSERT ... ON CONFLICT (idn) DO UPDATE」——但 INSERT 的列里**没有 idn**，
	// 冲突目标 idn 永远命不中，编辑因此必然落到自然键唯一约束 `finv_futu_usc_rule_item_key`
	// 上爆 23505（表现为「保存条目失败: duplicate key value violates unique constraint」）。
	// 现在 idn>0 走 UPDATE（改键也算编辑），idn=0 走 INSERT。
	var idn int64
	if item.IDN > 0 {
		tag, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_usc_rule_item
   SET rule_no        = $2,
       market         = $3,
       instrument     = $4,
       code           = $5,
       code_regex     = $6,
       action         = $7,
       usc_literal    = $8,
       rewrite_tpl    = $9,
       status         = $10,
       effective_from = $11::date,
       effective_to   = $12::date,
       priority       = $13,
       scope_env      = $14,
       item_desc      = $15,
       source_ref     = $16,
       remark         = $17
 WHERE idn = $1`,
			item.IDN, item.RuleNo, item.Market, item.Instrument, item.Code, item.CodeRegex,
			item.Action, item.USCLiteral, item.RewriteTpl, item.Status, item.EffectiveFrom, item.EffectiveTo,
			item.Priority, item.ScopeEnv, item.ItemDesc, item.SourceRef, item.Remark)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
				pgErr.ConstraintName == "finv_futu_usc_rule_item_key" {
				return 0, fmt.Errorf("%w：%s。该键已被另一条目占用，请改回原键或先停用占用条目",
					ErrItemExists, itemKeyText(item))
			}
			return 0, fmt.Errorf("保存条目失败: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return 0, fmt.Errorf("条目不存在: idn=%d", item.IDN)
		}
		idn = item.IDN
	} else {
		err = tx.QueryRow(ctx, `
INSERT INTO futu.finv_futu_usc_rule_item
      (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
       status, effective_from, effective_to, priority, scope_env, item_desc, source_ref,
       remark, creator)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date, $11::date, $12, $13, $14, $15, $16, $17)
RETURNING idn`,
			item.RuleNo, item.Market, item.Instrument, item.Code, item.CodeRegex,
			item.Action, item.USCLiteral, item.RewriteTpl, item.Status, item.EffectiveFrom, item.EffectiveTo,
			item.Priority, item.ScopeEnv, item.ItemDesc, item.SourceRef, item.Remark,
			nonEmpty(operator, "finv")).Scan(&idn)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
				pgErr.ConstraintName == "finv_futu_usc_rule_item_key" {
				return 0, fmt.Errorf("%w：%s。请直接编辑已存在的该条目，或先停用后再新增同键条目",
					ErrItemExists, itemKeyText(item))
			}
			return 0, fmt.Errorf("保存条目失败: %w", err)
		}
	}

	var after string
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule_item t WHERE idn = $1`, idn).Scan(&after); err != nil {
		return 0, fmt.Errorf("读取新条目失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
VALUES ('rule_item', $1, $2, $3::jsonb, $4::jsonb, $5, $6)`,
		map[bool]string{true: "UPDATE", false: "INSERT"}[before != nil],
		fmt.Sprintf("idn=%d %s", idn, item.RuleNo), before, after,
		nonEmpty(operator, "finv"), nonEmpty(item.Remark, "Web 维护页保存条目")); err != nil {
		return 0, fmt.Errorf("写审计日志失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("提交失败: %w", err)
	}
	return idn, nil
}

// itemKeyText 把条目的自然键拼成人可读的一段（用于冲突提示）。
//
// 形如 `P40 / market=CC / code=XRP`，空字段显示 `*`（表示不限定）。
func itemKeyText(item Item) string {
	dash := func(p *string) string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return "*"
		}
		return strings.TrimSpace(*p)
	}
	parts := []string{item.RuleNo}
	if item.Market != nil && strings.TrimSpace(*item.Market) != "" {
		parts = append(parts, "market="+dash(item.Market))
	}
	if item.Instrument != nil && strings.TrimSpace(*item.Instrument) != "" {
		parts = append(parts, "instrument="+dash(item.Instrument))
	}
	if item.Code != nil && strings.TrimSpace(*item.Code) != "" {
		parts = append(parts, "code="+dash(item.Code))
	}
	if item.CodeRegex != nil && strings.TrimSpace(*item.CodeRegex) != "" {
		parts = append(parts, "code_regex="+dash(item.CodeRegex))
	}
	if item.ScopeEnv != "" && item.ScopeEnv != DefaultEnv {
		parts = append(parts, "scope_env="+item.ScopeEnv)
	}
	return strings.Join(parts, " / ")
}

// RetireItem 停用条目（status=RETIRED + 记失效日），保留痕迹。
func (s *Store) RetireItem(ctx context.Context, idn int64, reason, operator string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before string
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule_item t WHERE idn = $1`, idn).Scan(&before); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("条目不存在: idn=%d", idn)
		}
		return fmt.Errorf("读取原条目失败: %w", err)
	}
	tag, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_usc_rule_item
   SET status = 'RETIRED',
       effective_to = COALESCE(effective_to, CURRENT_DATE),
       remark = CASE WHEN $2 = '' THEN remark ELSE trim(both ' ' FROM remark || ' | ' || $2) END
 WHERE idn = $1 AND status <> 'RETIRED'`, idn, strings.TrimSpace(reason))
	if err != nil {
		return fmt.Errorf("停用条目失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("条目已停用: idn=%d", idn)
	}
	var after string
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule_item t WHERE idn = $1`, idn).Scan(&after); err != nil {
		return fmt.Errorf("读取新条目失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
VALUES ('rule_item', 'UPDATE', $1, $2::jsonb, $3::jsonb, $4, $5)`,
		fmt.Sprintf("idn=%d", idn), before, after, nonEmpty(operator, "finv"),
		nonEmpty(reason, "停用条目")); err != nil {
		return fmt.Errorf("写审计日志失败: %w", err)
	}
	return tx.Commit(ctx)
}

// Status 页面只读快照。
func (s *Store) Status(ctx context.Context, env string) (Status, error) {
	status := Status{Env: normalizeEnv(env), TargetTables: []string{TableMap, TableKey}}
	var lastComputed *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT (SELECT count(*) FROM futu.finv_futu_usc_rule),
       (SELECT count(*) FROM futu.finv_futu_usc_rule_item),
       -- 只统计「所在规则层启用」的生效条目：P50 停层后条目仍为 ACTIVE，但已不参与计算
       (SELECT count(*) FROM futu.finv_futu_usc_rule_item i
          JOIN futu.finv_futu_usc_rule r ON r.rule_no = i.rule_no
         WHERE i.status = 'ACTIVE' AND r.flag_enable = '1'),
       (SELECT count(*) FROM futu.finv_futu_usc_map WHERE status = 'ACTIVE'),
       (SELECT count(*) FROM futu.finv_futu_usc_map WHERE status = 'RETIRED'),
       (SELECT count(*) FROM futu.finv_futu_usc_map WHERE reuse_of_ftmm IS NOT NULL),
       (SELECT count(*) FROM futu.finv_futu_secu_key),
       (SELECT count(*) FROM futu.finv_futu_secu_key k
          JOIN futu.finv_futu_usc_map m ON m.status = 'ACTIVE' AND m.usc_lower = lower(k.usc)),
       (SELECT count(*) FROM futu.finv_futu_secu_key k
         LEFT JOIN futu.finv_futu_usc_map m ON m.status = 'ACTIVE' AND m.usc_lower = lower(k.usc)
         WHERE m.ftmm_symbol IS NULL),
       (SELECT max(dt_compute) FROM futu.finv_futu_usc_map),
       (SELECT count(*) FROM archive.finv_usc_reuse_snapshot)`).Scan(
		&status.RuleCount, &status.ItemCount, &status.ItemActive, &status.MapActive,
		&status.MapRetire, &status.MapReuse, &status.KeyTotal, &status.KeyMatch,
		&status.KeyMiss, &lastComputed, &status.ArchiveRows)
	if err != nil {
		return Status{}, fmt.Errorf("统计状态失败: %w", err)
	}
	if lastComputed != nil {
		status.LastCompute = lastComputed.Format(time.RFC3339)
	}
	if status.RuleVersion, err = s.RuleVersion(ctx); err != nil {
		return Status{}, err
	}
	var expr *string
	if err := s.pool.QueryRow(ctx, `SELECT str_value FROM sys.sys_config WHERE config_no = $1`, ConfigCronNo).Scan(&expr); err == nil {
		status.CronExpr = strings.TrimSpace(derefString(expr))
		status.CronNextRun = NextRun(status.CronExpr, time.Now())
	}
	return status, nil
}

// SaveRule 保存规则层可改字段（启用开关 / 默认动作 / 说明 / 备注），并写一条审计日志。
//
// 幂等：带目标值比较，无变化时 UPDATE 影响 0 行，仍按原值落审计（便于留痕）。
func (s *Store) SaveRule(ctx context.Context, u RuleUpdate, operator string) error {
	ruleNo := strings.TrimSpace(u.RuleNo)
	if ruleNo == "" {
		return fmt.Errorf("rule_no 不能为空")
	}
	flag := strings.TrimSpace(u.FlagEnable)
	if flag != "0" && flag != "1" {
		return fmt.Errorf("flag_enable 只能是 0 或 1，实际 %q", u.FlagEnable)
	}
	var action *string
	if u.DefaultAction != nil {
		val := strings.TrimSpace(*u.DefaultAction)
		if val != "" {
			switch val {
			case "CODE", "FTMM", "LITERAL", "REWRITE", "SH4":
				action = &val
			default:
				return fmt.Errorf("default_action 只能是 CODE/FTMM/LITERAL/REWRITE/SH4，实际 %q", val)
			}
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var before string
	if err := tx.QueryRow(ctx,
		`SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule t WHERE rule_no = $1`, ruleNo).Scan(&before); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("规则不存在: %s", ruleNo)
		}
		return fmt.Errorf("读取规则失败: %w", err)
	}
	if _, err := tx.Exec(ctx, `
UPDATE futu.finv_futu_usc_rule
   SET flag_enable    = $2,
       default_action = $3,
       rule_desc      = $4,
       remark         = $5
 WHERE rule_no = $1`, ruleNo, flag, action, strings.TrimSpace(u.RuleDesc), strings.TrimSpace(u.Remark)); err != nil {
		return fmt.Errorf("保存规则失败: %w", err)
	}
	var after string
	if err := tx.QueryRow(ctx,
		`SELECT to_jsonb(t)::text FROM futu.finv_futu_usc_rule t WHERE rule_no = $1`, ruleNo).Scan(&after); err != nil {
		return fmt.Errorf("读取新规则失败: %w", err)
	}
	reason := strings.TrimSpace(u.Reason)
	if reason == "" {
		reason = "Web 维护页保存规则"
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO futu.finv_futu_usc_rule_log (src_table, action, key_text, before_json, after_json, operator, reason)
VALUES ('rule', 'UPDATE', $1, $2::jsonb, $3::jsonb, $4, $5)`,
		ruleNo, before, after, nonEmpty(operator, "finv"), reason); err != nil {
		return fmt.Errorf("写审计日志失败: %w", err)
	}
	return tx.Commit(ctx)
}

// ListLogs 审计日志分页（倒序）。
func (s *Store) ListLogs(ctx context.Context, page, pageSize int) ([]LogRow, int64, error) {
	page, pageSize = normalizePager(page, pageSize)
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM futu.finv_futu_usc_rule_log`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计审计日志失败: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
SELECT idn, src_table, action, key_text,
       -- 主键与所属规则拆成两列返回（需求 11137）；历史行同样能从快照里取到
       COALESCE(after_json->>'idn',         before_json->>'idn')         AS src_idn,
       COALESCE(after_json->>'rule_no',     before_json->>'rule_no')     AS src_rule_no,
       COALESCE(after_json->>'ftmm_symbol', before_json->>'ftmm_symbol') AS src_ftmm,
       before_json::text, after_json::text,
       COALESCE(operator, ''), reason, to_char(gmt_create, 'YYYY-MM-DD HH24:MI:SS')
  FROM futu.finv_futu_usc_rule_log
 ORDER BY idn DESC
 LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("查询审计日志失败: %w", err)
	}
	defer rows.Close()
	out := make([]LogRow, 0, pageSize)
	for rows.Next() {
		var row LogRow
		if err := rows.Scan(&row.IDN, &row.SrcTable, &row.Action, &row.KeyText,
			&row.SrcIDN, &row.SrcRuleNo, &row.SrcFtmm, &row.Before,
			&row.After, &row.Operator, &row.Reason, &row.GmtCreate); err != nil {
			return nil, 0, fmt.Errorf("读取审计日志失败: %w", err)
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// normalizeEnv 空环境按 * 处理。
func normalizeEnv(env string) string {
	if strings.TrimSpace(env) == "" {
		return DefaultEnv
	}
	return strings.TrimSpace(env)
}

// normalizePager 分页参数兜底（与 ApiSpec 的默认单页大小一致）。
func normalizePager(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

// nonEmpty 空串取默认值。
func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

// derefString 解引用字符串指针（nil → 空串）。
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
