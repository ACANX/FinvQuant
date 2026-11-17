-- =====================================================================
-- FinvQuant PostgreSQL V35：export schema 新增行情转存表
--   表名：export.finv_quote_secu_kline_min
--   结构与业务表 quote.finv_quote_secu_kline_min 完全一致（同名同结构）
--
-- 决策（ACANX 2026-10-24）：
--   - 需求：某证券的月度/日历史行情在「数据导出」前先转存到 export schema，
--     导出流程只读写转存表，避免导出期间的长事务/大结果集压住业务表 quote。
--   - 同构方式：CREATE TABLE ... (LIKE quote.finv_quote_secu_kline_min
--     INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES
--     INCLUDING STORAGE INCLUDING COMMENTS)，一次快照列、默认值、CHECK、
--     主键与索引；**触发器不在 LIKE 覆盖范围**，单独补齐（与源表同名，便于比对）。
--   - 索引命名：INCLUDING INDEXES 生成的索引**按新表名+列名自动命名**
--     （实测为 finv_quote_secu_kline_min_pkey 与
--     finv_quote_secu_kline_min_secu_code_ts_idx），与源表索引名
--     idx_finv_quote_usc_ts 不同；因此校验按「索引列组合」判定而非按名字。
--   - 快照语义：LIKE 是建表时刻的结构快照。源表（quote）后续增删列**不会**自动
--     同步过来，需另建迁移；末尾的收敛校验只保证「建表当时」一致。
--   - 数据隔离：转存表初始为空，由导出流程写入；导出结束可 TRUNCATE 复用。
--     ⚠ 同名风险：quote 在 search_path 第 2 位、export 在第 5 位，**裸写表名
--     （finv_quote_secu_kline_min）永远命中 quote 表**。任何读写本表的 SQL 必须
--     写 export. 前缀（见 Docs/DevSpec/DbSchemaSpec.md §3.1）。
--   - 幂等：pg_tables 判断后建表；触发器用 pg_trigger 判断；重复执行无副作用。
-- 迁移策略：单事务、失败回滚；末尾做收敛校验。
-- 前置：V34（export / sche schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表：与源表同构（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'export' AND tablename = 'finv_quote_secu_kline_min') THEN
        RAISE NOTICE 'V35: export.finv_quote_secu_kline_min 已存在，跳过建表';
    ELSE
        EXECUTE 'CREATE TABLE export.finv_quote_secu_kline_min
                   (LIKE quote.finv_quote_secu_kline_min
                    INCLUDING DEFAULTS
                    INCLUDING CONSTRAINTS
                    INCLUDING INDEXES
                    INCLUDING STORAGE
                    INCLUDING COMMENTS)';
        RAISE NOTICE 'V35: 已创建 export.finv_quote_secu_kline_min（结构取自 quote 同名表）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. gmt_update 触发器（LIKE 不复制触发器，单独补齐；幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
           AND tgrelid = 'export.finv_quote_secu_kline_min'::regclass
    ) THEN
        EXECUTE 'CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update
                   BEFORE UPDATE ON export.finv_quote_secu_kline_min
                   FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update()';
        RAISE NOTICE 'V35: 已创建 gmt_update 触发器';
    ELSE
        RAISE NOTICE 'V35: gmt_update 触发器已存在，跳过';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 表与列注释（说明用途，避免被误当作业务表）
-- ---------------------------------------------------------------------
COMMENT ON TABLE export.finv_quote_secu_kline_min IS
    '行情导出转存表：结构与 quote.finv_quote_secu_kline_min 一致，用于某证券月度/日行情在导出前暂存；导出完成后可 TRUNCATE 复用。读写必须写 export. 前缀（与 quote 同名）。';

-- ---------------------------------------------------------------------
-- 4. 收敛校验：列集合、列数、主键、索引、触发器
-- ---------------------------------------------------------------------
DO $$
DECLARE
    src_cols   INT;
    dst_cols   INT;
    col_diff   INT;
    pk_cnt     INT;
    idx_cnt    INT;
    trg_cnt    INT;
BEGIN
    SELECT count(*) INTO src_cols FROM information_schema.columns
     WHERE table_schema = 'quote'  AND table_name = 'finv_quote_secu_kline_min';
    SELECT count(*) INTO dst_cols FROM information_schema.columns
     WHERE table_schema = 'export' AND table_name = 'finv_quote_secu_kline_min';

    IF dst_cols = 0 THEN
        RAISE EXCEPTION 'V35: export.finv_quote_secu_kline_min 未创建成功';
    END IF;
    IF src_cols <> dst_cols THEN
        RAISE EXCEPTION 'V35: 列数不一致（quote=% / export=%）', src_cols, dst_cols;
    END IF;

    -- 双向差集：列名 + 类型 + 可空 + 默认值
    SELECT count(*) INTO col_diff FROM (
        (SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
           FROM information_schema.columns
          WHERE table_schema = 'quote'  AND table_name = 'finv_quote_secu_kline_min'
         EXCEPT
         SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
           FROM information_schema.columns
          WHERE table_schema = 'export' AND table_name = 'finv_quote_secu_kline_min')
        UNION ALL
        (SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
           FROM information_schema.columns
          WHERE table_schema = 'export' AND table_name = 'finv_quote_secu_kline_min'
         EXCEPT
         SELECT column_name, data_type, is_nullable, coalesce(column_default, '')
           FROM information_schema.columns
          WHERE table_schema = 'quote'  AND table_name = 'finv_quote_secu_kline_min')
    ) diff;

    IF col_diff <> 0 THEN
        RAISE EXCEPTION 'V35: 列定义与 quote 同名表有 % 处差异', col_diff;
    END IF;

    SELECT count(*) INTO pk_cnt FROM pg_index i
      JOIN pg_class c ON c.oid = i.indrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'export' AND c.relname = 'finv_quote_secu_kline_min' AND i.indisprimary;
    IF pk_cnt <> 1 THEN
        RAISE EXCEPTION 'V35: 主键数量为 %（期望 1）', pk_cnt;
    END IF;

    -- 普通索引：(usc, ts)，名字由 PG 自动生成，按列组合判定
    SELECT count(*) INTO idx_cnt FROM pg_index i
      JOIN pg_class c ON c.oid = i.indrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'export' AND c.relname = 'finv_quote_secu_kline_min'
       AND NOT i.indisprimary
       AND pg_get_indexdef(i.indexrelid) LIKE '%(usc, ts)%';
    IF idx_cnt < 1 THEN
        RAISE EXCEPTION 'V35: 缺少 (usc, ts) 普通索引';
    END IF;

    SELECT count(*) INTO trg_cnt FROM pg_trigger
     WHERE tgname = 'trg_finv_quote_secu_kline_min_gmt_update'
       AND tgrelid = 'export.finv_quote_secu_kline_min'::regclass;
    IF trg_cnt <> 1 THEN
        RAISE EXCEPTION 'V35: 缺少 gmt_update 触发器';
    END IF;

    RAISE NOTICE 'V35: 校验通过（列 % 个与 quote 完全一致，主键/索引/触发器齐备）', dst_cols;
END $$;

COMMIT;
