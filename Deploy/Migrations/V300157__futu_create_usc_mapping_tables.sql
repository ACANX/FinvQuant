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
--   2. futu.finv_futu_usc_rule_item   条目层（689 条：P12 5 / P14 3 / P20 11 / P21 5 / P22 1 / P40 23 / P50 641）
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
-- 9. 初始条目（689 条：P12 5 / P14 3 / P20 11 / P21 5 / P22 1 / P40 23 / P50 641）
-- ---------------------------------------------------------------------
INSERT INTO futu.finv_futu_usc_rule_item
    (rule_no, market, instrument, code, code_regex, action, usc_literal, rewrite_tpl,
     priority, status, scope_env, item_desc, source_ref, creator)
VALUES
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
 ('P50', 'CC', NULL, 'UNI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 UNI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ALGO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ALGO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ANT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ANT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ARB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ARB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ATOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ATOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BAND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BAND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BAT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BAT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BSV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BSV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BTG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BTG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BTM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BTM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'CMT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 CMT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'COMP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 COMP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'CRV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 CRV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'DAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 DAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'DASH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 DASH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ELA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ELA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ELF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ELF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ENS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ENS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'EOS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 EOS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ETP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ETP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'FET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 FET 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'FLR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 FLR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'FTT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 FTT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'GAS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 GAS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'HYPE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 HYPE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'LINK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 LINK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'LTC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 LTC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'MANA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 MANA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'MKR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 MKR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'MNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 MNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'NEAR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 NEAR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'OGN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 OGN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'NMR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 NMR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'OCN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 OCN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'PAX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 PAX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'PAY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 PAY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'POL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 POL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'QNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 QNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'QTUM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 QTUM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'REP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 REP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'SEI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SEI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'SNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'SNX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SNX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'STRK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 STRK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'STX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 STX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'SUI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SUI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'SXP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 SXP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'TIA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 TIA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'TON', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 TON 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'TRX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 TRX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'WTC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 WTC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'XEM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 XEM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'XVG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 XVG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'YFI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 YFI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ZEN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ZEN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'GRAM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 GRAM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'MON', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 MON 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ONT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ONT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'VET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 VET 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'OMG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 OMG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'XRP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 XRP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'KSM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 KSM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'ETC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 ETC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'CVC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 CVC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'APT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 APT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'NEO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 NEO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'BCH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 BCH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'CC', NULL, 'AMPL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '加密币 AMPL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 A', 'migration'),
 ('P50', 'US', NULL, 'AESG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AESG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ACHP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ACHP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AEMC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AEMC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AINF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AINF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ALA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ALA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AMA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AMA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AMGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AMGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ANG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ANG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'APMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 APMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ARMY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ARMY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ATC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ATC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUAU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUAU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUGC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUGC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BAY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BAY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BBBB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BBBB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BCOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BCOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BCV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BCV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BEC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BEC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BEX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BEX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BEZ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BEZ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BGX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BGX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BLCK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BLCK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BMN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BMN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BSA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BSA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BSL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BSL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CAF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CAF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CAGE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CAGE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CBIL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CBIL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CCOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CCOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CGO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CGO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CIEI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CIEI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CMAG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CMAG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CPU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CPU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CRD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CRD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CTMA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CTMA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CWY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CWY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DACL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DACL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DVGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DVGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EARN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EARN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EASY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EASY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ECF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ECF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EEE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EEE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EMET', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EMET 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EMKT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EMKT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ENHI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ENHI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ENRG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ENRG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ETX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ETX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EVG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EVG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FFF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FFF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IBX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IBX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FLC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FLC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FMN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FMN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FSML', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FSML 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FTG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FTG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FTN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FTN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GCV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GCV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GDL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GDL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GDXD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GDXD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GEMG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GEMG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GLDN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GLDN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GLO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GLO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GPT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GPT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRNI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRNI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HBIL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HBIL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HBIT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HBIT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HBRD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HBRD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HLTH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HLTH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HWO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HWO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HXE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HXE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HXF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HXF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IAF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IAF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IGB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IGB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IRE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IRE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'JIDE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 JIDE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KPO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KPO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LCTO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LCTO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LGI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LGI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LIFT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LIFT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORBX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORBX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MATE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MATE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MAUG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MAUG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MCN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MCN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'METG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 METG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MPG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MPG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MSR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MSR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NBIE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NBIE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QAU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QAU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NICO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NICO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NIM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NIM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NMBL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NMBL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NMT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NMT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NPV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NPV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NUG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NUG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NYSX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NYSX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OCT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OCT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OILD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OILD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OILU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OILU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ONX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ONX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORAC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORAC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OVQ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OVQ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OWN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OWN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PAYM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PAYM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PGP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PGP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PGZ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PGZ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PIM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PIM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PINC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PINC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PLA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PLA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'POW', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 POW 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PRIZ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PRIZ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PUL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PUL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PUR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PUR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GLU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GLU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QMY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QMY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QQI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QQI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QSE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QSE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RCLO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RCLO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RESM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RESM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RGT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RGT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RISE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RISE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RSF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RSF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SABA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SABA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TMGN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TMGN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SBI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SBI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SLVX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SLVX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SNAG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SNAG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SPMC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SPMC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SRV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SRV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SSS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SSS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'STLR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 STLR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SVTN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SVTN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TCAN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TCAN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TDNA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TDNA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TECY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TECY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TEST', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TEST 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TLA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TLA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TLG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TLG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TPUT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TPUT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TRIL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TRIL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TRUN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TRUN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'USSX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 USSX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VGRO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VGRO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VRC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VRC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VUS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VUS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WIA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WIA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XSEM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XSEM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XSPC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XSPC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZJAN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZJAN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZJUL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZJUL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZOCT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZOCT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZSB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZSB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MATR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MATR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PNI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PNI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, '.SPTSX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 .SPTSX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EQ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EQ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EQS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EQS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ESP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ESP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SCM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SCM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SCNA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SCNA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EVGN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EVGN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FCAP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FCAP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FCUL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FCUL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FCUV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FCUV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SCND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SCND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FFR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FFR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FGC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FGC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SEED', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SEED 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SEM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SEM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FLNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FLNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FLX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FLX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SER', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SER 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FSI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FSI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SERV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SERV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SEV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SEV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SGA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SGA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SGC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SGC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FURY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FURY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GAME', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GAME 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GBR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GBR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GCEI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GCEI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SGLD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SGLD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GDC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GDC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SGMD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SGMD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'UG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 UG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SILO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SILO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GGL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GGL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GHG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GHG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SINT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SINT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GLE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GLE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SITC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SITC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'UMAX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 UMAX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GOGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GOGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GOGY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GOGY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GOW', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GOW 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SJ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SJ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GPUS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GPUS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRAM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRAM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HGH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HGH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SKK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SKK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WPRT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WPRT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GRO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GRO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GROW', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GROW 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'UUU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 UUU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GTCH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GTCH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GTII', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GTII 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GXAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GXAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HBIE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HBIE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SMNR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SMNR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SMX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SMX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WTER', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WTER 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HERB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HERB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HESG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HESG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HGBL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HGBL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HHS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HHS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HIND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HIND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HIT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HIT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HLX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HLX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HMR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HMR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HURA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HURA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'HWH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 HWH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SOLN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SOLN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SPAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SPAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WXM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WXM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IBG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IBG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IBO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IBO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ICG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ICG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ICON', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ICON 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ICU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ICU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IDEA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IDEA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SRG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SRG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MFB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MFB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MGMA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MGMA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IMA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IMA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MHH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MHH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SRI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SRI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INDO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INDO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INEO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INEO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INFI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INFI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INTE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INTE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INTG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INTG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'INV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 INV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IPB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IPB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MMMM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MMMM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SRL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SRL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MNBO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MNBO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ITP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ITP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IVG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IVG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'IVRO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 IVRO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'JG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 JG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XCH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XCH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'JSDA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 JSDA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'JUSH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 JUSH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'JWEL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 JWEL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KALY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KALY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KAPA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KAPA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KCLI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KCLI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KDC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KDC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KDK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KDK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SSM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SSM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CDT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CDT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'KTN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 KTN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CGRA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CGRA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XRTX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XRTX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LCU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LCU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LDI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LDI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LFS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LFS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LFT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LFT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SST', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SST 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LGL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LGL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LGO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LGO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'STCK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 STCK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LITH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LITH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LITS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LITS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LIVE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LIVE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LOCL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LOCL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LODE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LODE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LOOP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LOOP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LOT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LOT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LSF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LSF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'STG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 STG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MACK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MACK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MANA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MANA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MAPS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MAPS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'XRX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 XRX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'STRK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 STRK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MBIO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MBIO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MCAP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MCAP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SUNPF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SUNPF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MDAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MDAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MDCX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MDCX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MED', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MED 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MERC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MERC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MERG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MERG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SURG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SURG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MGLD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MGLD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MGX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MGX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MIMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MIMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MIND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MIND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MINE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MINE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MIRA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MIRA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PHOS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PHOS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MMA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MMA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SVCC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SVCC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MNY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MNY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MONI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MONI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MOT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MOT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WHG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WHG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MRM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MRM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MRMD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MRMD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MRT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MRT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MSC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MSC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TACT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TACT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MTEK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MTEK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PSYC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PSYC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PTN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PTN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MYND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MYND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NARA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NARA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TBA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TBA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NBND', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NBND 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TBC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TBC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NCI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NCI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DPRO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DPRO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QNC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QNC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NEXM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NEXM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NGLD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NGLD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RAY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RAY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CCM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CCM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NINE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NINE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NEON', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NEON 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZEO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZEO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TELO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TELO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ECOR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ECOR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NPT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NPT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TERA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TERA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NSAV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NSAV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DCM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DCM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NVA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NVA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ENRT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ENRT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NVX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NVX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NXH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NXH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'NXL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 NXL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TGL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TGL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OCC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OCC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TII', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TII 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OGI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OGI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TKMO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TKMO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZJK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZJK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OMH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OMH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ONCO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ONCO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TLF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TLF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OPTI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OPTI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ELC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ELC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORBT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORBT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORGN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORGN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ORIO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ORIO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TMXN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TMXN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OSS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OSS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TONOF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TONOF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FINN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FINN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OZ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OZ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TRAD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TRAD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PAL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PAL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PALI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PALI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ZONE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ZONE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PBK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PBK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PBM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PBM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SGU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SGU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PDC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PDC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PEMC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PEMC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PGOL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PGOL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TRT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TRT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'OCG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 OCG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PHRX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PHRX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TUSK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TUSK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TVE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TVE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'UPPR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 UPPR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PLG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PLG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'MYX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 MYX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BHM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BHM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PLUR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PLUR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AEAE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AEAE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AEC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AEC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AEI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AEI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PMAX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PMAX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AEON', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AEON 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'TGRO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 TGRO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AETR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AETR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AHI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AHI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIDX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIDX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AIOS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AIOS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PMN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PMN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AKA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AKA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PMNT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PMNT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ALAR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ALAR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ALD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ALD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ALLR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ALLR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ALTB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ALTB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'POM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 POM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AMS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AMS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ANDC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ANDC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PPX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PPX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ANT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ANT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PRB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PRB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'APT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 APT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'APXC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 APXC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PRH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PRH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ARQ', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ARQ 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ASTI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ASTI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'UZF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 UZF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PROP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PROP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ATHR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ATHR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ATOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ATOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PRS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PRS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PDIV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PDIV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUMC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUMC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUMN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUMN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AURX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AURX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AUST', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AUST 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AVD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AVD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AVG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AVG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AVX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AVX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AWX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AWX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AXR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AXR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AZI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AZI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AZIO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AZIO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'AZR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 AZR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BABY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BABY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BAMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BAMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BARK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BARK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VIP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VIP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PURE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PURE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BCBC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BCBC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BCHT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BCHT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'PXS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 PXS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VISM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VISM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'GMTL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 GMTL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BEEP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BEEP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VIVO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VIVO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QNTM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QNTM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BGL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BGL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VLN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VLN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BID', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BID 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BIRD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BIRD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'QREE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 QREE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DAVA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DAVA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BLGO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BLGO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BLMH', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BLMH 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VNTA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VNTA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BMR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BMR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BNR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BNR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BOOM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BOOM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BRAG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BRAG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BRN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BRN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BRVO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BRVO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BRYN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BRYN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RAIL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RAIL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RAIN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RAIN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BTCT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BTCT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BTCY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BTCY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BTRU', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BTRU 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'BYRG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 BYRG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RAN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RAN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RBNK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RBNK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CAMG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CAMG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CAN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CAN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CANL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CANL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CANN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CANN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CAPN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CAPN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CATX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CATX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CBDX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CBDX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RBOT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RBOT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CBTC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CBTC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CCG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CCG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RCD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RCD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CGLD', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CGLD 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CGLO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CGLO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SCOR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SCOR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CHR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CHR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CIA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CIA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RCT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RCT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CLAR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CLAR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CLHI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CLHI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CLUS', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CLUS 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CLW', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CLW 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CLZNF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CLZNF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'REA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 REA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CMT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CMT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CNVI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CNVI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'COPR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 COPR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VRNO', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VRNO 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'LINK', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 LINK 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CRDL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CRDL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CRE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CRE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CRTL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CRTL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CSBA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CSBA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CTBB', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CTBB 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CTM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CTM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VVVX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VVVX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CUE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CUE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CUPR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CUPR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CVM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CVM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CVR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CVR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'CVV', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 CVV 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'VYGR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 VYGR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'FUSE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 FUSE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RML', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RML 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DDT', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DDT 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DELX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DELX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DFSC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DFSC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DPM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DPM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DRUG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DRUG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DSY', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DSY 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DTG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DTG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DTI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DTI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ROC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ROC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DVLN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DVLN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DXF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DXF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DXG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DXG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'DXR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 DXR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EAI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EAI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ROYL', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ROYL 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WAST', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WAST 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'RYM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 RYM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EDGM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EDGM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WATR', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WATR 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EGG', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EGG 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ELBM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ELBM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EMAX', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EMAX 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WAVE', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WAVE 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EMI', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EMI 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SBBA', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SBBA 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EML', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EML 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EMP', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EMP 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'ENCC', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 ENCC 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'WHF', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 WHF 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'EPM', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 EPM 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
 ('P50', 'US', NULL, 'SCAN', NULL, 'FTMM', NULL, NULL, 100, 'ACTIVE', '*', '美股 SCAN 与其它市场同名，usc 用富途全码区分', 'USCMappingRule.md 附录 B', 'migration'),
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

    SELECT count(*) INTO v_item FROM futu.finv_futu_usc_rule_item;
    IF v_item <> 689 THEN
        RAISE EXCEPTION 'V300157 校验失败：条目应为 689 条，实际 %', v_item;
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
