-- =====================================================================
-- FinvQuant PostgreSQL V100080：把 dedup 的表数据迁入 cdeu 对应表（需求 10756）
--
-- 映射（V100078 建表 / V100079 统一前缀后的结果）：
--   dedup.<原表>                    → cdeu.finv_quote_cdeu_<后缀>
--   dedup.finv_quote_secu_kline_min → cdeu.finv_quote_secu_kline_min（表名不变）
--   dedup.prep_bak_*                → 不迁移（5 张）
--
-- 迁移语义：
--   · 这是一次**数据复制**，dedup 侧数据保留不动（后续由运维逐步清理淘汰）；
--   · 幂等：全部使用 `ON CONFLICT DO NOTHING`，重复执行不会产生重复行；
--   · 列清单从 information_schema 动态获取，不依赖 `SELECT *` 的列顺序假设；
--   · 迁移后对 17 个序列执行 setval，使 cdeu 侧 nextval 与已迁入的最大值衔接。
--
-- ⚠️ 切换状态说明：
--   应用代码当前仍读写 `dedup.*`。本迁移只完成「一次性数据复制」，
--   cdeu 侧数据会随 dedup 继续变化而**逐渐过期**。真正切换需要另行改造代码
--   （读写指向 cdeu）并处理过渡期的双写/补差，不在本次范围内。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 前置校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_src bigint;
    v_dst bigint;
BEGIN
    SELECT count(*) INTO v_src FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'dedup' AND c.relkind = 'r' AND c.relname NOT LIKE 'prep_bak%';
    SELECT count(*) INTO v_dst FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'cdeu' AND c.relkind = 'r';
    IF v_src <> 26 OR v_dst <> 26 THEN
        RAISE EXCEPTION 'V100080 前置校验失败：dedup 应 26 张源表（实际 %）、cdeu 应 26 张目标表（实际 %）', v_src, v_dst;
    END IF;
    RAISE NOTICE 'V100080 前置校验：源 26 张 / 目标 26 张，开始复制';
END $$;

-- ---------------------------------------------------------------------
-- 1. 逐表复制（动态列清单 + ON CONFLICT DO NOTHING）
--
--    映射函数：
--      dedup.cdeu_x           → cdeu.finv_quote_cdeu_x
--      dedup.finv_dedup_x     → cdeu.finv_quote_cdeu_x
--      dedup.finv_quote_secu_kline_min → cdeu.finv_quote_secu_kline_min
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r      record;
    cols   text;
    dst    text;
    cnt    bigint;
    total  bigint := 0;
BEGIN
    FOR r IN
        SELECT c.relname AS src
          FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'dedup' AND c.relkind = 'r' AND c.relname NOT LIKE 'prep_bak%'
         ORDER BY c.relname
    LOOP
        dst := CASE
                 WHEN r.src LIKE 'cdeu\_%'        THEN 'finv_quote_' || r.src
                 WHEN r.src LIKE 'finv\_dedup\_%' THEN 'finv_quote_cdeu_' || substr(r.src, length('finv_dedup_') + 1)
                 ELSE r.src
               END;

        -- 目标表必须存在
        IF to_regclass('cdeu.' || dst) IS NULL THEN
            RAISE EXCEPTION 'V100080 失败：目标表 cdeu.% 不存在', dst;
        END IF;

        -- 动态列清单（按目标表列顺序，保证与源表一一对应）
        SELECT string_agg(quote_ident(column_name), ',' ORDER BY ordinal_position)
          INTO cols
          FROM information_schema.columns
         WHERE table_schema = 'cdeu' AND table_name = dst;

        IF cols IS NULL THEN
            RAISE EXCEPTION 'V100080 失败：取不到 cdeu.% 的列清单', dst;
        END IF;

        EXECUTE format('INSERT INTO cdeu.%I (%s) SELECT %s FROM dedup.%I ON CONFLICT DO NOTHING',
                       dst, cols, cols, r.src);
        GET DIAGNOSTICS cnt = ROW_COUNT;
        total := total + cnt;
        RAISE NOTICE 'V100080 复制 dedup.% → cdeu.%：% 行', r.src, dst, cnt;
    END LOOP;
    RAISE NOTICE 'V100080 复制完成：合计 % 行', total;
END $$;

-- ---------------------------------------------------------------------
-- 2. 序列 setval：让 cdeu 侧 nextval 与已迁入的最大值衔接
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r     record;
    maxv  bigint;
BEGIN
    FOR r IN
        SELECT s.relname AS seq_name,
               t.relname AS tbl_name,
               a.attname AS col_name
          FROM pg_class s
          JOIN pg_depend d ON d.objid = s.oid AND d.deptype IN ('a','i')
          JOIN pg_class t ON t.oid = d.refobjid
          JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = d.refobjsubid
          JOIN pg_namespace n ON n.oid = t.relnamespace
         WHERE s.relkind = 'S' AND n.nspname = 'cdeu'
         ORDER BY s.relname
    LOOP
        EXECUTE format('SELECT coalesce(max(%I), 0) FROM cdeu.%I', r.col_name, r.tbl_name) INTO maxv;
        IF maxv > 0 THEN
            PERFORM setval('cdeu.' || quote_ident(r.seq_name), maxv, true);
        ELSE
            -- 空表：序列回初始态（下次 nextval 返回 1）
            PERFORM setval('cdeu.' || quote_ident(r.seq_name), 1, false);
        END IF;
        RAISE NOTICE 'V100080 序列 cdeu.% → setval(%s)', r.seq_name, maxv;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------
-- 3. 收敛校验：逐表行数必须一致
-- ---------------------------------------------------------------------
DO $$
DECLARE
    r       record;
    dst     text;
    src_cnt bigint;
    dst_cnt bigint;
    bad     text := '';
BEGIN
    FOR r IN
        SELECT c.relname AS src
          FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'dedup' AND c.relkind = 'r' AND c.relname NOT LIKE 'prep_bak%'
         ORDER BY c.relname
    LOOP
        dst := CASE
                 WHEN r.src LIKE 'cdeu\_%'        THEN 'finv_quote_' || r.src
                 WHEN r.src LIKE 'finv\_dedup\_%' THEN 'finv_quote_cdeu_' || substr(r.src, length('finv_dedup_') + 1)
                 ELSE r.src
               END;
        EXECUTE format('SELECT count(*) FROM dedup.%I', r.src) INTO src_cnt;
        EXECUTE format('SELECT count(*) FROM cdeu.%I', dst) INTO dst_cnt;
        IF src_cnt <> dst_cnt THEN
            bad := bad || format('[%s→%s: %s vs %s] ', r.src, dst, src_cnt, dst_cnt);
        END IF;
    END LOOP;

    IF bad <> '' THEN
        RAISE EXCEPTION 'V100080 收敛失败：行数不一致 %', bad;
    END IF;
    RAISE NOTICE 'V100080 收敛校验通过：26 张表行数与 dedup 完全一致';
END $$;

-- ---------------------------------------------------------------------
-- 4. 汇总输出
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rows bigint;
BEGIN
    SELECT sum(n_live_tup) INTO v_rows FROM pg_stat_user_tables WHERE schemaname = 'cdeu';
    RAISE NOTICE 'V100080 完成：cdeu 侧 26 张表合计约 % 行（统计值为估算，精确值以第 3 步逐表校验为准）', v_rows;
END $$;

COMMIT;
