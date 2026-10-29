-- =====================================================================
-- FinvQuant PostgreSQL V75：finv_market 增加 market_symbol 市场符号字段
--
-- 决策（ACANX 2026-10-26）：
--   - 需求：交易市场字典需要一个「市场对外标识」字段，用于与行情链路的
--     market 取值（如 dedup.finv_dedup_usc.market = US-NSDQ / FX-CFD / SZ）
--     对齐，并作为前端选择市场时的可读符号。
--   - 表归属：finv_market 属基础主数据，位于 public schema（见
--     Docs/DevSpec/DbSchemaSpec.md 第 3 节）；V32 只迁移了 finv_quote_* 前缀
--     的表，故此处显式限定 public.finv_market。
--   - 字段名 market_symbol、类型 TEXT（与表内 market_flag / market_abbr /
--     market_name 等既有字符串列保持一致），NOT NULL DEFAULT ''，
--     空字符串表示「未维护」，避免存量行出现 NULL 语义分裂。
--   - 物理位置：ALTER TABLE ADD COLUMN 只能追加到表尾；语义位置在 market_flag
--     之后（market_code → market_flag → market_symbol → market_abbr → ...），
--     结构体、文档与查询按语义顺序展示（与 V20 的 exchange_code 同一约定）。
--   - 暂不建索引：当前无按 market_symbol 的点查或关联查询，避免无谓写放大；
--     后续若出现此类查询，另起迁移补 B-tree 索引。
--   - 暂不加唯一约束：存量行该列全为 ''，跨市场语义唯一性尚未定稿；待数据
--     补齐后按需加部分唯一索引（WHERE market_symbol <> ''），避免误伤空值行。
--   - 幂等：ADD COLUMN IF NOT EXISTS + COMMENT 覆盖写（COMMENT 本身幂等）。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验（列类型/可空性/默认值/注释）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 增加市场符号列
-- ---------------------------------------------------------------------
ALTER TABLE public.finv_market
    ADD COLUMN IF NOT EXISTS market_symbol TEXT NOT NULL DEFAULT '';

-- ---------------------------------------------------------------------
-- 2. 列注释（语义与取值约定）
-- ---------------------------------------------------------------------
COMMENT ON COLUMN public.finv_market.market_symbol IS
    '市场符号：市场的对外标识（如 US-NSDQ / FX-CFD / SZ），与行情链路 market 取值对齐；空字符串表示未维护';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_type    TEXT;
    v_nullable TEXT;
    v_default TEXT;
    v_comment TEXT;
BEGIN
    SELECT c.data_type, c.is_nullable, c.column_default
      INTO v_type, v_nullable, v_default
      FROM information_schema.columns AS c
     WHERE c.table_schema = 'public'
       AND c.table_name = 'finv_market'
       AND c.column_name = 'market_symbol';

    IF v_type IS NULL THEN
        RAISE EXCEPTION 'V75: public.finv_market.market_symbol 未创建';
    END IF;
    IF v_type <> 'text' THEN
        RAISE EXCEPTION 'V75: market_symbol 类型为 %，预期 text', v_type;
    END IF;
    IF v_nullable <> 'NO' THEN
        RAISE EXCEPTION 'V75: market_symbol 允许为空，预期 NOT NULL';
    END IF;
    IF v_default IS DISTINCT FROM '''''::text' THEN
        RAISE EXCEPTION 'V75: market_symbol 默认值为 %，预期空字符串', COALESCE(v_default, '(NULL)');
    END IF;

    SELECT col_description('public.finv_market'::regclass, a.attnum)
      INTO v_comment
      FROM pg_attribute AS a
     WHERE a.attrelid = 'public.finv_market'::regclass
       AND a.attname = 'market_symbol';
    IF v_comment IS NULL OR position('市场符号' in v_comment) = 0 THEN
        RAISE EXCEPTION 'V75: market_symbol 缺少列注释';
    END IF;

    RAISE NOTICE 'V75: public.finv_market.market_symbol 就绪（TEXT NOT NULL DEFAULT ''''）';
END $$;

COMMIT;
