-- =====================================================================
-- FinvQuant PostgreSQL V300157：USC 映射规则表（孪生数据表）+ 引擎 + 初始规则 + 每日同步配置
--
-- 需求：11072（USC 映射规则结构化建表）
-- 授权依据：用户在 2026-11-10 会话中明确点名「按 V300157 落地」并逐条确定以下设计：
--   ① schema 归属 `futu`；② 退市后 `usc` **允许复用**；③ 复用发生时**触发下游归档**；
--   ④ `finv_futu_secu_key.usc` 由**定时任务**同步；⑤ 同步频率**每天一次**；
--   ⑥ 需要 `scope_env` 环境差异；⑦ 要 Web 维护页（含预演 diff）。
--   设计文档：仓库外 `FinvQuantDesign/Mapping/USCMappingRule.Schema.md`（本迁移是其固化）。
--
-- 建了什么：
--   1. futu.finv_futu_usc_rule        规则层（P10/P12/P14/P20/P21/P22/P40/P50/P60 共 9 条）
--   2. futu.finv_futu_usc_rule_item   条目层（52 条：P12 5 / P14 3 / P20 11 / P21 5 / P22 1 / P40 23 / P50 4）
--   3. futu.finv_futu_usc_map         孪生结果表（ftmm_symbol → usc；ACTIVE 行大小写不敏感唯一）
--   4. futu.finv_futu_usc_rule_log    审计日志
--   5. archive.finv_usc_reuse_snapshot 复用归档（usc 被复用时下游各表历史行整行快照）
--   6. futu.f_finv_futu_usc_calc(env)  引擎函数 + futu.finv_futu_usc_calc_view 视图（预演/对账用）
--   7. sys.sys_config 20015            每日同步 cron（默认 0 30 3 * * *，每天 03:30）
--
-- 关键约束（与设计文档一致，不得随意放宽）：
--   - `usc` 大小写不敏感唯一：`usc_lower` 生成列 + **部分唯一索引仅约束 ACTIVE 行**
--     （退市行 RETIRED 不占位，允许复用；复用必须写 `reuse_of_ftmm` 并做下游归档）；
--   - 条目唯一键用 `UNIQUE NULLS NOT DISTINCT (rule_no, scope_env, market, instrument, code, code_regex)`：
--     条目的通配维度大量为 NULL，普通唯一键无法挡住重复灌数（PG 视 NULL 互不相等）；
--   - 生成列 `usc_lower` 禁止显式写入；
--   - 【红线】初始数据不得覆盖用户后来配置/修改的值：全部 `ON CONFLICT DO NOTHING`，本文件无 DO UPDATE。
--
-- 幂等：CREATE TABLE/INDEX/VIEW IF NOT EXISTS、函数 CREATE OR REPLACE、种子 ON CONFLICT DO NOTHING、
--   触发器先 DROP IF EXISTS 再建；重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），失败整体回滚。
-- 前置：public.vq_set_gmt_update()（V200001 起）、schema futu / archive / sys 已存在。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 规则层：futu.finv_futu_usc_rule
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_futu_usc_rule (
    rule_no        TEXT        NOT NULL,                           -- 规则编号（P<数字>）
    rule_name      TEXT        NOT NULL,                           -- 规则名称
    rule_type      TEXT        NOT NULL,                           -- 规则类型
    priority       INTEGER     NOT NULL,                           -- 执行顺序（越小越先）
    match_scope    TEXT        NOT NULL DEFAULT 'NON_GATE',         -- 作用域
    default_action TEXT        NULL,                               -- 默认动作（条目可覆盖）
    flag_enable    CHAR(1)     NOT NULL DEFAULT '1',               -- 启用标志
    rule_desc      TEXT        NOT NULL DEFAULT '',                -- 规则说明
    remark         TEXT        NOT NULL DEFAULT '',                -- 备注
    creator        TEXT        NULL,                               -- 创建人
    gmt_create     TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 创建时间
    gmt_update     TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 更新时间（触发器维护）
    CONSTRAINT finv_futu_usc_rule_pkey PRIMARY KEY (rule_no),
    CONSTRAINT finv_futu_usc_rule_priority_uq UNIQUE (priority),
    CONSTRAINT finv_futu_usc_rule_type_chk CHECK (rule_type IN ('GATE','NORMALIZE','MARKET_DEFAULT','EXCEPTION','FALLBACK')),
    CONSTRAINT finv_futu_usc_rule_scope_chk CHECK (match_scope IN ('ALL','NON_GATE')),
    CONSTRAINT finv_futu_usc_rule_action_chk CHECK (default_action IS NULL OR default_action IN ('CODE','FTMM','LITERAL','REWRITE','SH4')),
    CONSTRAINT finv_futu_usc_rule_flag_chk CHECK (flag_enable IN ('0','1'))
);

COMMENT ON TABLE  futu.finv_futu_usc_rule IS 'USC 映射规则层：一条 = 一个处理阶段（P10 闸门 / P12 P14 前处理 / P20-P22 市场默认 / P40 P50 例外 / P60 兜底）';
COMMENT ON COLUMN futu.finv_futu_usc_rule.rule_no        IS '规则编号（P<数字>；P1-P9 预留给将来的前置规则）';
COMMENT ON COLUMN futu.finv_futu_usc_rule.rule_name      IS '规则名称';
COMMENT ON COLUMN futu.finv_futu_usc_rule.rule_type      IS '规则类型：GATE=唯一性闸门，NORMALIZE=前处理，MARKET_DEFAULT=市场默认，EXCEPTION=例外，FALLBACK=兜底';
COMMENT ON COLUMN futu.finv_futu_usc_rule.priority       IS '执行顺序，数值越小越先；全局唯一';
COMMENT ON COLUMN futu.finv_futu_usc_rule.match_scope    IS '作用域：ALL=作用于全表，NON_GATE=只作用于 P10 未命中的行';
COMMENT ON COLUMN futu.finv_futu_usc_rule.default_action IS '该层默认动作：CODE=用规范化后的 code，FTMM=用富途全码，LITERAL=用固定字面量，REWRITE=正则改写，SH4=SH+code 后四位；条目可覆盖';
COMMENT ON COLUMN futu.finv_futu_usc_rule.flag_enable    IS '启用标志：1=启用，0=停用（停用后该层不参与计算，数据保留）';
COMMENT ON COLUMN futu.finv_futu_usc_rule.rule_desc      IS '规则说明：语义、前置条件、示例（多行文本，Web 维护页直接展示）';
COMMENT ON COLUMN futu.finv_futu_usc_rule.remark         IS '备注：变更原因、评审单号等';
COMMENT ON COLUMN futu.finv_futu_usc_rule.creator        IS '创建人';
COMMENT ON COLUMN futu.finv_futu_usc_rule.gmt_create     IS '创建时间';
COMMENT ON COLUMN futu.finv_futu_usc_rule.gmt_update     IS '更新时间（由 public.vq_set_gmt_update() 触发器维护）';

-- ---------------------------------------------------------------------
-- 2. 条目层：futu.finv_futu_usc_rule_item
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_futu_usc_rule_item (
    idn            BIGINT      GENERATED BY DEFAULT AS IDENTITY,   -- 自增主键
    rule_no        TEXT        NOT NULL,                           -- 所属规则
    market         TEXT        NULL,                               -- 市场（NULL=任意）
    instrument     TEXT        NULL,                               -- 品种（NULL=任意）
    code           TEXT        NULL,                               -- 精确匹配的交易所代码（NULL=不按 code 匹配）
    code_regex     TEXT        NULL,                               -- 正则匹配（NULL=不按正则匹配）
    action         TEXT        NULL,                               -- 本条动作（NULL=用规则默认）
    usc_literal    TEXT        NULL,                               -- action=LITERAL 时的取值
    rewrite_tpl    TEXT        NULL,                               -- action=REWRITE 时的替换串
    status         TEXT        NOT NULL DEFAULT 'ACTIVE',           -- 条目状态
    effective_from DATE        NULL,                               -- 生效起始日
    effective_to   DATE        NULL,                               -- 失效日（退市/换合约）
    priority       INTEGER     NOT NULL DEFAULT 100,               -- 同层内顺序
    scope_env      TEXT        NOT NULL DEFAULT '*',               -- 环境范围
    item_desc      TEXT        NOT NULL DEFAULT '',                -- 条目说明
    source_ref     TEXT        NOT NULL DEFAULT '',                -- 出处
    remark         TEXT        NOT NULL DEFAULT '',                -- 备注
    creator        TEXT        NULL,                               -- 创建人
    gmt_create     TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 创建时间
    gmt_update     TIMESTAMPTZ NOT NULL DEFAULT now(),             -- 更新时间（触发器维护）
    CONSTRAINT finv_futu_usc_rule_item_pkey PRIMARY KEY (idn),
    CONSTRAINT finv_futu_usc_rule_item_key
        UNIQUE NULLS NOT DISTINCT (rule_no, scope_env, market, instrument, code, code_regex),
    CONSTRAINT finv_futu_usc_rule_item_rule_fk
        FOREIGN KEY (rule_no) REFERENCES futu.finv_futu_usc_rule (rule_no),
    CONSTRAINT finv_futu_usc_rule_item_status_chk CHECK (status IN ('ACTIVE','PENDING','RETIRED')),
    CONSTRAINT finv_futu_usc_rule_item_action_chk CHECK (action IS NULL OR action IN ('CODE','FTMM','LITERAL','REWRITE','SH4')),
    CONSTRAINT finv_futu_usc_rule_item_lit_chk CHECK (action <> 'LITERAL' OR usc_literal IS NOT NULL),
    CONSTRAINT finv_futu_usc_rule_item_rw_chk CHECK (action <> 'REWRITE' OR (code_regex IS NOT NULL AND rewrite_tpl IS NOT NULL)),
    CONSTRAINT finv_futu_usc_rule_item_period_chk CHECK (effective_to IS NULL OR effective_from IS NULL OR effective_to >= effective_from)
);

COMMENT ON TABLE  futu.finv_futu_usc_rule_item IS 'USC 映射条目层：一行 = 一条点对点映射（市场/品种/代码 → 取值）；新增上市、退市、规则修正只动这张表';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.idn            IS '自增主键';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.rule_no        IS '所属规则编号（外键 futu.finv_futu_usc_rule.rule_no）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.market         IS '市场限定（如 US / HK / CC）；NULL=不限市场';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.instrument     IS '品种限定（如 futures / etf）；NULL=不限品种';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.code           IS '精确匹配的交易所代码（比较时按 lower()，大小写不敏感）；NULL=不按 code 匹配';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.code_regex     IS '正则匹配表达式（如 ^000、(main)$）；NULL=不按正则匹配';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.action         IS '本条动作：CODE/FTMM/LITERAL/REWRITE/SH4；NULL=沿用规则层 default_action';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.usc_literal    IS 'action=LITERAL 时的固定取值（如 HSI / USDIndex）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.rewrite_tpl    IS 'action=REWRITE 时的正则替换串（如 Main）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.status         IS '条目状态：ACTIVE=生效，PENDING=已登记未生效，RETIRED=已停用/退市（保留痕迹不参与命中）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.effective_from IS '生效起始日（NULL=不限）；周期性品种/新上市用';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.effective_to   IS '失效日（NULL=长期有效）；退市/换合约用';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.priority       IS '同层内多条命中时的顺序，数值越小越先';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.scope_env      IS '环境范围：*=全部环境；也可填 prod/dev 做环境差异';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.item_desc      IS '条目说明：为什么这么映射（如"ETF 与正股重名"）；Web 维护页保存时必填';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.source_ref     IS '出处：文档章节/附录条号，如 USCMappingRule.md 附录 B';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.remark         IS '备注：变更原因、评审单号等';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.creator        IS '创建人';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.gmt_create     IS '创建时间';
COMMENT ON COLUMN futu.finv_futu_usc_rule_item.gmt_update     IS '更新时间（由 public.vq_set_gmt_update() 触发器维护）';

-- ---------------------------------------------------------------------
-- 3. 孪生结果表：futu.finv_futu_usc_map
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_futu_usc_map (
    ftmm_symbol       TEXT        NOT NULL,                        -- 富途全码（主键）
    market            TEXT        NOT NULL,                        -- 市场
    code              TEXT        NOT NULL,                        -- 交易所代码
    instrument        TEXT        NOT NULL,                        -- 品种
    usc               TEXT        NOT NULL,                        -- 映射结果
    usc_lower         TEXT        GENERATED ALWAYS AS (lower(usc)) STORED,  -- 大小写不敏感检索列（生成列，禁止显式写入）
    rule_no           TEXT        NOT NULL,                        -- 依据的规则编号
    basis             TEXT        NOT NULL DEFAULT '',             -- 具体依据
    status            TEXT        NOT NULL DEFAULT 'ACTIVE',        -- 证券生命周期状态
    retired_at        TIMESTAMPTZ NULL,                            -- 退市时间
    reuse_of_ftmm     TEXT        NULL,                            -- 复用来源
    scope_env         TEXT        NOT NULL DEFAULT '*',            -- 环境范围
    rule_version      TEXT        NULL,                            -- 规则集版本
    computed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),          -- 最近计算时间
    first_assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),          -- 首次分配时间
    remark            TEXT        NOT NULL DEFAULT '',             -- 备注
    CONSTRAINT finv_futu_usc_map_pkey PRIMARY KEY (ftmm_symbol),
    CONSTRAINT finv_futu_usc_map_status_chk CHECK (status IN ('ACTIVE','RETIRED')),
    CONSTRAINT finv_futu_usc_map_retired_chk CHECK (status <> 'RETIRED' OR retired_at IS NOT NULL)
);

COMMENT ON TABLE  futu.finv_futu_usc_map IS 'USC 映射结果表（孪生表）：一行 = 一只证券的最终 usc；在册行大小写不敏感唯一，退市行保留作历史且允许 usc 复用';
COMMENT ON COLUMN futu.finv_futu_usc_map.ftmm_symbol       IS '富途全码（market.code）';
COMMENT ON COLUMN futu.finv_futu_usc_map.market            IS '市场（如 US / HK / CC）';
COMMENT ON COLUMN futu.finv_futu_usc_map.code              IS '交易所代码';
COMMENT ON COLUMN futu.finv_futu_usc_map.instrument        IS '品种（如 stock / etf / futures / index）';
COMMENT ON COLUMN futu.finv_futu_usc_map.usc               IS '映射结果（Finv 内部证券统一代码）';
COMMENT ON COLUMN futu.finv_futu_usc_map.usc_lower         IS '生成列 lower(usc)：供大小写不敏感唯一索引与检索使用，禁止显式写入';
COMMENT ON COLUMN futu.finv_futu_usc_map.rule_no           IS '本次结果依据的规则编号（P10/P12/P20/P40/P50…）';
COMMENT ON COLUMN futu.finv_futu_usc_map.basis             IS '具体依据说明（如 P50 附录 B、P12 指数别名）';
COMMENT ON COLUMN futu.finv_futu_usc_map.status            IS '生命周期：ACTIVE=在册（占位 usc），RETIRED=已退市（不占位，允许被复用）';
COMMENT ON COLUMN futu.finv_futu_usc_map.retired_at        IS '退市时间（status=RETIRED 时必填）';
COMMENT ON COLUMN futu.finv_futu_usc_map.reuse_of_ftmm     IS '复用来源：本行 usc 的上一个占用者（退市证券的 ftmm_symbol）；非复用为 NULL';
COMMENT ON COLUMN futu.finv_futu_usc_map.scope_env         IS '环境范围：*=全部环境；也可填 prod/dev';
COMMENT ON COLUMN futu.finv_futu_usc_map.rule_version      IS '生成时使用的规则集版本标识';
COMMENT ON COLUMN futu.finv_futu_usc_map.computed_at       IS '最近一次计算时间';
COMMENT ON COLUMN futu.finv_futu_usc_map.first_assigned_at IS '该 usc 首次分配给本证券的时间（跨期统计的时间下界）';
COMMENT ON COLUMN futu.finv_futu_usc_map.remark            IS '备注';

-- 在册（ACTIVE）行 US C 大小写不敏感唯一；RETIRED 行不占位（允许复用）
CREATE UNIQUE INDEX IF NOT EXISTS uq_finv_futu_usc_map_active
    ON futu.finv_futu_usc_map (usc_lower) WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_finv_futu_usc_map_market_code
    ON futu.finv_futu_usc_map (market, code);
CREATE INDEX IF NOT EXISTS idx_finv_futu_usc_map_rule
    ON futu.finv_futu_usc_map (rule_no);
CREATE INDEX IF NOT EXISTS idx_finv_futu_usc_map_usc_lower
    ON futu.finv_futu_usc_map (usc_lower);

-- ---------------------------------------------------------------------
-- 4. 审计日志：futu.finv_futu_usc_rule_log
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS futu.finv_futu_usc_rule_log (
    idn         BIGINT      GENERATED BY DEFAULT AS IDENTITY,      -- 自增主键
    src_table   TEXT        NOT NULL,                              -- 来源对象
    action      TEXT        NOT NULL,                              -- 动作
    key_text    TEXT        NOT NULL,                              -- 对象主键文本
    before_json JSONB       NULL,                                  -- 变更前
    after_json  JSONB       NULL,                                  -- 变更后
    operator    TEXT        NULL,                                  -- 操作人
    reason      TEXT        NOT NULL DEFAULT '',                   -- 原因
    gmt_create  TIMESTAMPTZ NOT NULL DEFAULT now(),                -- 记录时间
    CONSTRAINT finv_futu_usc_rule_log_pkey PRIMARY KEY (idn),
    CONSTRAINT finv_futu_usc_rule_log_action_chk CHECK (action IN ('INSERT','UPDATE','DELETE','COMPUTE'))
);

COMMENT ON TABLE  futu.finv_futu_usc_rule_log IS 'USC 映射审计日志：规则/条目/结果集的增删改与每次重算的 diff 留痕';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.idn         IS '自增主键';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.src_table   IS '来源对象：rule / rule_item / map / sync';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.action      IS '动作：INSERT/UPDATE/DELETE=规则或条目变更，COMPUTE=重算 diff';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.key_text    IS '对象主键文本（rule_no 或 ftmm_symbol）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.before_json IS '变更前快照（jsonb）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.after_json  IS '变更后快照（jsonb）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.operator    IS '操作人（Web 页操作为登录账号，定时任务为 finv-scheduler）';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.reason      IS '变更/计算原因';
COMMENT ON COLUMN futu.finv_futu_usc_rule_log.gmt_create  IS '记录时间';

-- ---------------------------------------------------------------------
-- 5. 复用归档：archive.finv_usc_reuse_snapshot
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_usc_reuse_snapshot (
    idn         BIGINT      GENERATED BY DEFAULT AS IDENTITY,      -- 自增主键
    usc         TEXT        NOT NULL,                              -- 被复用的 usc
    usc_lower   TEXT        GENERATED ALWAYS AS (lower(usc)) STORED, -- 大小写不敏感检索列（生成列）
    src_table   TEXT        NOT NULL,                              -- 来源表
    src_key     TEXT        NOT NULL,                              -- 源行主键文本
    row_json    JSONB       NOT NULL,                              -- 整行快照
    from_ftmm   TEXT        NOT NULL,                              -- 原占用者
    to_ftmm     TEXT        NOT NULL,                              -- 复用者
    reason      TEXT        NOT NULL DEFAULT 'usc 复用归档',        -- 归档原因
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT now(),                -- 归档时间
    CONSTRAINT finv_usc_reuse_snapshot_pkey PRIMARY KEY (idn),
    CONSTRAINT finv_usc_reuse_snapshot_uq UNIQUE (usc_lower, src_table, src_key, from_ftmm)
);

COMMENT ON TABLE  archive.finv_usc_reuse_snapshot IS 'USC 复用归档：usc 被新证券复用时，下游各表（finv_security/quote_collect/indicator/kline_day/cdeu_usc）历史行整行快照';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.usc         IS '被复用的 usc（原始大小写）';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.usc_lower   IS '生成列 lower(usc)：大小写不敏感检索，禁止显式写入';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.src_table   IS '来源表（如 public.finv_security）';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.src_key     IS '源行主键文本（usc / ftmm_symbol / stock_id）';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.row_json    IS '整行快照 to_jsonb(row)';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.from_ftmm   IS '该 usc 的原占用者 ftmm_symbol';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.to_ftmm     IS '复用的新证券 ftmm_symbol';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.reason      IS '归档原因（固定口径：usc 复用归档）';
COMMENT ON COLUMN archive.finv_usc_reuse_snapshot.snapshot_at IS '归档时间';

CREATE INDEX IF NOT EXISTS idx_finv_usc_reuse_snapshot_usc
    ON archive.finv_usc_reuse_snapshot (usc_lower);

-- ---------------------------------------------------------------------
-- 6. gmt_update 触发器（复用 public.vq_set_gmt_update()）
-- ---------------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_finv_futu_usc_rule_gmt_update ON futu.finv_futu_usc_rule;
CREATE TRIGGER trg_finv_futu_usc_rule_gmt_update
    BEFORE UPDATE ON futu.finv_futu_usc_rule
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

DROP TRIGGER IF EXISTS trg_finv_futu_usc_rule_item_gmt_update ON futu.finv_futu_usc_rule_item;
CREATE TRIGGER trg_finv_futu_usc_rule_item_gmt_update
    BEFORE UPDATE ON futu.finv_futu_usc_rule_item
    FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update();

-- ---------------------------------------------------------------------
-- 7. 引擎：函数（带 env 参数）+ 视图（env='*'）
--    前处理（P12 别名 / P14 正则改写）→ canon_code → 大小写不敏感计数 → 输出 canon_cnt
--    规则条目、闸门与例外由应用侧按 rule_no/priority 应用，冲突升级（回落 ftmm_symbol）也在应用侧
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION futu.f_finv_futu_usc_calc(p_env TEXT DEFAULT '*')
RETURNS TABLE (
    ftmm_symbol TEXT,
    market      TEXT,
    code        TEXT,
    instrument  TEXT,
    canon_code  TEXT,
    canon_cnt   BIGINT
) AS $$
    WITH lit AS (
        SELECT i.market, i.code, i.usc_literal
          FROM futu.finv_futu_usc_rule_item i
         WHERE i.rule_no = 'P12'
           AND i.action = 'LITERAL'
           AND i.status = 'ACTIVE'
           AND i.scope_env IN ('*', p_env)
           AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
           AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
    ), rw AS (
        SELECT i.instrument, i.code_regex, i.rewrite_tpl, i.priority
          FROM futu.finv_futu_usc_rule_item i
         WHERE i.rule_no = 'P14'
           AND i.action = 'REWRITE'
           AND i.status = 'ACTIVE'
           AND i.scope_env IN ('*', p_env)
           AND (i.effective_from IS NULL OR i.effective_from <= CURRENT_DATE)
           AND (i.effective_to   IS NULL OR i.effective_to   >= CURRENT_DATE)
    ), pre AS (
        SELECT k.ftmm_symbol, k.market, k.code, k.instrument,
               COALESCE(l.usc_literal,
                        (SELECT regexp_replace(k.code, r.code_regex, r.rewrite_tpl, 'i')
                           FROM rw r
                          WHERE (r.instrument IS NULL OR r.instrument = k.instrument)
                            AND k.code ~* r.code_regex
                          ORDER BY r.priority
                          LIMIT 1),
                        k.code) AS canon_code
          FROM futu.finv_futu_secu_key k
          LEFT JOIN lit l ON l.market = k.market AND lower(l.code) = lower(k.code)
    ), cnt AS (
        SELECT lower(p.canon_code) AS k, count(*) AS n FROM pre p GROUP BY 1
    )
    SELECT p.ftmm_symbol, p.market, p.code, p.instrument, p.canon_code, c.n
      FROM pre p
      JOIN cnt c ON c.k = lower(p.canon_code);
$$ LANGUAGE sql STABLE;

COMMENT ON FUNCTION futu.f_finv_futu_usc_calc(TEXT) IS 'USC 映射引擎：按规则条目计算每只证券的前处理代码 canon_code 与大小写不敏感计数 canon_cnt（canon_cnt=1 即 P10 闸门命中）；p_env 为环境标识（* 表示全部环境）';

CREATE OR REPLACE VIEW futu.finv_futu_usc_calc_view AS
SELECT * FROM futu.f_finv_futu_usc_calc('*');

COMMENT ON VIEW futu.finv_futu_usc_calc_view IS 'USC 映射引擎视图（env=*）：预演与对账用，不落库；实际写入见 futu.finv_futu_usc_map';

-- ---------------------------------------------------------------------
-- 8. 初始规则（9 条）
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule
    (rule_no, rule_name, rule_type, priority, match_scope, default_action, flag_enable, rule_desc, creator)
VALUES
 ('P10', '天然唯一探测（闸门）', 'GATE', 10, 'ALL', NULL, '1',
  E'对前处理后的 canon_code 做大小写不敏感计数，计数=1 即定稿，不再走下续规则。\n这是终局语义：命中即定稿，P20~P60 只作用于 P10 未命中的重名 code。\n示例：HK.00700 → 00700；US.AAPL → AAPL。', 'migration'),
 ('P12', '前处理：指数别名与特殊指数', 'NORMALIZE', 12, 'ALL', 'LITERAL', '1',
  E'把上游的指数别名/特殊写法改写成业界通用写法（固定字面量）。\n示例：HK.800000 → HSI；FX.USDINDEX → USDIndex。\n前处理只改写法、不改加不加前缀：加前缀仍由 P20~P60 决定。', 'migration'),
 ('P14', '前处理：期货合约后缀', 'NORMALIZE', 14, 'ALL', 'REWRITE', '1',
  E'期货主力/当月/次月合约的后缀统一为 Main / Current / Next（大小写不敏感的正则改写）。\n示例：HK.AACMAIN → AACMain。', 'migration'),
 ('P20', '市场默认·可读型', 'MARKET_DEFAULT', 20, 'NON_GATE', 'CODE', '1',
  E'这些市场的交易所代码天然唯一率高，usc 直接取 code（可读性最好）。\n适用：HK/SZ/SH/BJ/CC/US/BK/GL/MY/FX/THA。', 'migration'),
 ('P21', '市场默认·全码型', 'MARKET_DEFAULT', 21, 'NON_GATE', 'FTMM', '1',
  E'这些市场的代码天然唯一率不足，usc 取富途全码 ftmm_symbol（CA.XXX）兜住重名。\n适用：CA/AU/SG/JP/BMS。', 'migration'),
 ('P22', '市场默认·上交所指数', 'MARKET_DEFAULT', 22, 'NON_GATE', 'SH4', '1',
  E'上交所 000 开头的指数，usc 取 SH + 交易所代码后四位。\n示例：SH.000001 → SH0001。', 'migration'),
 ('P40', '单点例外', 'EXCEPTION', 40, 'NON_GATE', 'FTMM', '1',
  E'逐条出现的跨市场重名，按附录口径一条一条钉死（23 条）。\n加密币 BTC/ETH/PAXG/USDC/USDT/SOL 为权威主体取 code；其余同名方加市场前缀。', 'migration'),
 ('P50', '批量例外', 'EXCEPTION', 50, 'NON_GATE', 'FTMM', '1',
  E'附录 A/B 的批量例外清单（CC 69 + US 571 + THA 1 = 641 条），统一用富途全码区分。\n这些 code 与其它市场/品种同名，直接用 code 会造成 usc 冲突。', 'migration'),
 ('P60', '兜底', 'FALLBACK', 60, 'NON_GATE', 'FTMM', '1',
  E'前序规则都未命中时的最后兜底：usc 取富途全码 ftmm_symbol，保证一定有值且唯一。', 'migration')
ON CONFLICT (rule_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 9. 初始条目（52 条：P12 5 / P14 3 / P20 11 / P21 5 / P22 1 / P40 23 / P50 4）
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule_item
    (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
     priority, status, scope_env, item_desc, source_ref, creator)
VALUES
-- 【需求 11134】P50 只保留 4 条单点例外（CC.ANT / CC.ATOM / US .SPTSX / THA .SET）：
--   P50 整层已按需求 11079 停用（flag_enable='0'），其余 637 条批量例外不再随脚本发放，
--   避免全新库凭空灌入一批从不参与计算的条目；存量库里这些行保持 RETIRED 不动。
 ('P12', 'HK', NULL, '800000', NULL, 'LITERAL', 'HSI', NULL, 100, 'ACTIVE', '*', '恒生指数：上游用 800000 作别名，usc 取业界通用写法 HSI', 'USCMappingRule.md §5', 'migration'),
 ('P12', 'HK', NULL, '800700', NULL, 'LITERAL', 'HTI', NULL, 100, 'ACTIVE', '*', '恒生科技指数：usc 取 HTI；注意与美股 ETF US.HSTI 无关', 'USCMappingRule.md §5', 'migration'),
 ('P12', 'HK', NULL, '800806', NULL, 'LITERAL', 'HSIII', NULL, 100, 'ACTIVE', '*', '恒生综合小型股指数：usc 取 HSIII', 'USCMappingRule.md §5', 'migration'),
 ('P12', 'FX', NULL, 'USDINDEX', NULL, 'LITERAL', 'USDIndex', NULL, 100, 'ACTIVE', '*', '美元指数：usc 保留驼峰写法作唯一权威形态', 'USCMappingRule.md §5', 'migration'),
 ('P12', 'FX', NULL, 'EURINDEX', NULL, 'LITERAL', 'EURIndex', NULL, 100, 'ACTIVE', '*', '欧元指数：usc 保留驼峰写法作唯一权威形态', 'USCMappingRule.md §5', 'migration'),
 ('P14', NULL, 'futures', NULL, '(main)$', 'REWRITE', NULL, 'Main', 100, 'ACTIVE', '*', '期货主力合约后缀 main → Main（大小写不敏感）', 'USCMappingRule.md §3.4', 'migration'),
 ('P14', NULL, 'futures', NULL, '(current)$', 'REWRITE', NULL, 'Current', 100, 'ACTIVE', '*', '期货当月合约后缀 current → Current', 'USCMappingRule.md §3.4', 'migration'),
 ('P14', NULL, 'futures', NULL, '(next)$', 'REWRITE', NULL, 'Next', 100, 'ACTIVE', '*', '期货次月合约后缀 next → Next', 'USCMappingRule.md §3.4', 'migration'),
 ('P20', 'HK', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'HK 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'SZ', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'SZ 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'SH', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'SH 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'BJ', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'BJ 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'CC', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'CC 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'US', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'US 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'BK', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'BK 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'GL', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'GL 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'MY', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'MY 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'FX', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'FX 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P20', 'THA', NULL, NULL, NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', 'THA 市场天然唯一率高，usc 直接用交易所代码（可读性最好）', 'USCMappingRule.md §3.2', 'migration'),
 ('P21', 'CA', NULL, NULL, NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'CA 市场代码天然唯一率不足，usc 用富途全码 ftmm_symbol 兜住', 'USCMappingRule.md §3.2', 'migration'),
 ('P21', 'AU', NULL, NULL, NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'AU 市场代码天然唯一率不足，usc 用富途全码 ftmm_symbol 兜住', 'USCMappingRule.md §3.2', 'migration'),
 ('P21', 'SG', NULL, NULL, NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'SG 市场代码天然唯一率不足，usc 用富途全码 ftmm_symbol 兜住', 'USCMappingRule.md §3.2', 'migration'),
 ('P21', 'JP', NULL, NULL, NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'JP 市场代码天然唯一率不足，usc 用富途全码 ftmm_symbol 兜住', 'USCMappingRule.md §3.2', 'migration'),
 ('P21', 'BMS', NULL, NULL, NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'BMS 市场代码天然唯一率不足，usc 用富途全码 ftmm_symbol 兜住', 'USCMappingRule.md §3.2', 'migration'),
 ('P22', 'SH', NULL, NULL, '^000', 'SH4', NULL, NULL, 100, 'ACTIVE', '*', '上交所 000 开头的指数，usc 取 SH + 交易所代码后四位', 'USCMappingRule.md §3.2', 'migration'),
 ('P40', 'CA', NULL, 'HSI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', 'CA.HSI 与港股 HSI 规范化后同名，CA 侧必须用全码区分', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'SZ', NULL, '045860', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '深市该代码与其它市场同名，加市场前缀', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'SZ', NULL, '045833', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '深市该代码与其它市场同名，加市场前缀', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'SH', NULL, '045860', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '沪市该代码与其它市场同名，加市场前缀', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'SH', NULL, '045833', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '沪市该代码与其它市场同名，加市场前缀', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, '.SET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 .SET 与泰股 THA..SET 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, 'ETH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ETH 与加密币 ETH 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, 'BTC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BTC 与加密币 BTC 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, 'PAXG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PAXG 与加密币 PAXG 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, 'USDC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 USDC 与加密币 USDC 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'US', NULL, 'SOL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SOL 与加密币 SOL 同名，美股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'AU', NULL, 'SET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '澳股 SET 与泰股 THA..SET 同名，澳股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CA', NULL, 'SOL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加股 SOL 与加密币 SOL 同名，加股侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'ANT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ANT 与美股 ANT 同名，加密币侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'UNI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 UNI 与美股 UNI 同名，加密币侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'ATOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ATOM 与美股 ATOM 同名，加密币侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'XRP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 XRP 与美股 XRP 同名，加密币侧用全码', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'BTC', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BTC 为权威主体，usc 直接用 BTC', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'ETH', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ETH 为权威主体，usc 直接用 ETH', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'PAXG', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 PAXG 为权威主体，usc 直接用 PAXG', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'USDC', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 USDC 为权威主体，usc 直接用 USDC', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'USDT', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 USDT 为权威主体，usc 直接用 USDT', 'USCMappingRule.md §5', 'migration'),
 ('P40', 'CC', NULL, 'SOL', NULL, 'CODE', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SOL 为权威主体，usc 直接用 SOL', 'USCMappingRule.md §5', 'migration'),
 ('P50', 'CC', NULL, 'ANT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ANT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ATOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ATOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'US', NULL, '.SPTSX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 .SPTSX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'THA', NULL, '.SET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '泰股 .SET 与美股 .SET 同名，泰股侧用富途全码区分', 'USCMappingRule.md 附录 B', 'migration')
ON CONFLICT (rule_no, scope_env, market, instrument, code, code_regex) DO NOTHING;

-- ---------------------------------------------------------------------
-- 10. 调度配置：每日同步 cron（20015；20014 已被「Supabase 证券键源表名」占用）
-- ---------------------------------------------------------------------
INSERT INTO sys.sys_config
    (config_no, config_name, data_type, int_value, char_value, str_value,
     json_value, access_level, create_by, gmt_create, gmt_modify, modify_by, remark)
VALUES
    (20015, 'USC 映射规则每日同步cron（6段）', '3', 0, '', '0 30 3 * * *', NULL, NULL, 'finv',
     CURRENT_TIMESTAMP(6), CURRENT_TIMESTAMP(6), NULL,
     'USC 映射规则（futu.finv_futu_usc_rule_item → futu.finv_futu_usc_map → futu.finv_futu_secu_key.usc）每日同步 cron，6 段格式（秒 分 时 日 月 周）。默认 0 30 3 * * *（每天 03:30 一次）。留空或非法则关闭定时同步，仍可在维护页手动触发')
ON CONFLICT (config_no) DO NOTHING;

-- ---------------------------------------------------------------------
-- 11. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rule   BIGINT;
    v_item   BIGINT;
    v_cfg    BIGINT;
    v_view   BIGINT;
    v_func   BIGINT;
    v_dup    BIGINT;
    v_prio   BIGINT;
    v_active BIGINT;
BEGIN
    SELECT count(*) INTO v_rule FROM futu.finv_futu_usc_rule;
    IF v_rule <> 9 THEN
        RAISE EXCEPTION 'V300157 校验失败：规则应为 9 条，实际 %', v_rule;
    END IF;

    -- 条目 52 条（P50 只发 4 条单点例外，其余 637 条批量例外已按需求 11079 停用，不再随脚本发放）
    SELECT count(*) INTO v_item FROM futu.finv_futu_usc_rule_item;
    IF v_item <> 52 THEN
        RAISE EXCEPTION 'V300157 校验失败：条目应为 52 条，实际 %', v_item;
    END IF;

    -- 同一条规则内同一匹配维度不得重复（NULLS NOT DISTINCT 唯一键的兜底复核）
    SELECT count(*) INTO v_dup FROM (
        SELECT rule_no, scope_env, market, instrument, code, code_regex
          FROM futu.finv_futu_usc_rule_item
         GROUP BY 1,2,3,4,5,6 HAVING count(*) > 1) t;
    IF v_dup <> 0 THEN
        RAISE EXCEPTION 'V300157 校验失败：条目存在 % 组重复', v_dup;
    END IF;

    SELECT count(DISTINCT priority) INTO v_prio FROM futu.finv_futu_usc_rule;
    IF v_prio <> v_rule THEN
        RAISE EXCEPTION 'V300157 校验失败：规则 priority 不唯一（% vs %）', v_prio, v_rule;
    END IF;

    SELECT count(*) INTO v_active FROM futu.finv_futu_usc_rule_item WHERE status = 'ACTIVE';
    IF v_active <> v_item THEN
        RAISE EXCEPTION 'V300157 校验失败：初始条目应全部 ACTIVE，实际 ACTIVE=% 总数=%', v_active, v_item;
    END IF;

    SELECT count(*) INTO v_cfg FROM sys.sys_config WHERE config_no = 20015;
    IF v_cfg <> 1 THEN
        RAISE EXCEPTION 'V300157 校验失败：sys_config 20015 未插入';
    END IF;

    SELECT count(*) INTO v_view FROM pg_views WHERE schemaname = 'futu' AND viewname = 'finv_futu_usc_calc_view';
    SELECT count(*) INTO v_func FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
     WHERE n.nspname = 'futu' AND p.proname = 'f_finv_futu_usc_calc';
    IF v_view <> 1 OR v_func < 1 THEN
        RAISE EXCEPTION 'V300157 校验失败：引擎视图/函数缺失（view=%, func=%）', v_view, v_func;
    END IF;

    RAISE NOTICE 'V300157 就绪：规则 % 条、条目 % 条、配置 20015=%',
        v_rule, v_item,
        (SELECT str_value FROM sys.sys_config WHERE config_no = 20015);
END $$;

COMMIT;
