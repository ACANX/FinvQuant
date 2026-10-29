-- =====================================================================
-- 一次性数据迁移（需求 10773）
--   public.finv_quote_secu_kline_min  →  cdeu.finv_quote_secu_kline_min
--
-- 背景：
--   目标库 finv 为多服务共用库，其中 public.finv_quote_secu_kline_min 是
--   FinvQuant 早期版本建在 public 下的历史分钟 K 线表；新版运行时只读写
--   cdeu.finv_quote_secu_kline_min。本脚本把历史数据一次性搬入 cdeu，
--   源表只读、保持原样（不改名、不删除、不 TRUNCATE）。
--
-- 特性：
--   · 只读源表：对 public.finv_quote_secu_kline_min 只执行 SELECT；
--   · 幂等：可重复执行，已存在的 (ts, usc) 直接跳过（ON CONFLICT DO NOTHING）；
--   · 列名兼容旧版：secu_code → usc、prev_close → last_close，
--     market_code 丢弃（新表以 usc 绑定市场）；新列 change_price /
--     change_ratio / provider 源表没有，不复制（保留目标表默认值）；
--   · 同 (ts, usc) 多 market_code 重复行去重（按 gmt_update 取最新一条）；
--   · 单事务：任何异常或校验不通过整体回滚，cdeu 表保持执行前状态。
--
-- 执行（建议先备份数据库、在低峰期执行）：
--   psql -h <host> -p <port> -U <user> -d finv -v ON_ERROR_STOP=1 \
--        -f MigratePublicKlineToCdeu.sql
--
-- 执行后验证：
--   select count(*) from cdeu.finv_quote_secu_kline_min;
--   select count(*) from public.finv_quote_secu_kline_min;   -- 源表行数不变
--
-- 回滚（仅当确认本次导入数据无需保留；按源表实际列名二选一）：
--   -- 源表为旧结构（secu_code）：
--   delete from cdeu.finv_quote_secu_kline_min d
--    where exists (select 1 from public.finv_quote_secu_kline_min s
--                   where s.ts = d.ts and s.secu_code = d.usc);
--   -- 源表已是新结构（usc）：
--   delete from cdeu.finv_quote_secu_kline_min d
--    where exists (select 1 from public.finv_quote_secu_kline_min s
--                   where s.ts = d.ts and s.usc = d.usc);
--
-- 说明：本脚本为一次性任务，上线验证通过后可直接注释或删除，
--       不进入 Deploy/Migrations 迁移链，不参与版本化升级。
-- =====================================================================

\set ON_ERROR_STOP on

BEGIN;

-- 避免等锁过久（有其它会话长时间持有源表锁时快速失败）
SET LOCAL lock_timeout = '15s';
-- 大表复制不设语句超时；如需限制请自行调整
SET LOCAL statement_timeout = 0;

DO $mig$
DECLARE
    c_src CONSTANT text := 'public.finv_quote_secu_kline_min';
    c_dst CONSTANT text := 'cdeu.finv_quote_secu_kline_min';

    v_src_has_usc       boolean := false;
    v_src_has_secucode  boolean := false;
    v_src_has_lastclose boolean := false;
    v_src_has_prevclose boolean := false;
    v_src_has_gmtupdate boolean := false;

    v_usc_col   text;
    v_dst_cols  text := '';
    v_sel_exprs text := '';
    v_order_by  text;
    v_col       text;
    v_src_col   text;

    v_src_rows bigint := 0;
    v_before   bigint := 0;
    v_after    bigint := 0;
    v_inserted bigint := 0;
    v_dup      bigint := 0;
    v_missing  bigint := 0;
BEGIN
    -- -----------------------------------------------------------------
    -- 1. 前置检查
    -- -----------------------------------------------------------------
    IF to_regclass(c_src) IS NULL THEN
        RAISE EXCEPTION '源表 % 不存在，无需迁移（请确认 -d 指定的数据库）', c_src;
    END IF;
    IF to_regclass(c_dst) IS NULL THEN
        RAISE EXCEPTION '目标表 % 不存在：请先完成数据库迁移（V100001–V100080）',
                        c_dst;
    END IF;

    SELECT
        bool_or(column_name = 'usc'),
        bool_or(column_name = 'secu_code'),
        bool_or(column_name = 'last_close'),
        bool_or(column_name = 'prev_close'),
        bool_or(column_name = 'gmt_update')
      INTO v_src_has_usc, v_src_has_secucode,
           v_src_has_lastclose, v_src_has_prevclose, v_src_has_gmtupdate
      FROM information_schema.columns
     WHERE table_schema = 'public'
       AND table_name   = 'finv_quote_secu_kline_min';

    IF NOT v_src_has_usc AND NOT v_src_has_secucode THEN
        RAISE EXCEPTION '源表既没有 usc 也没有 secu_code 列，无法确定证券代码，终止';
    END IF;

    v_usc_col := CASE WHEN v_src_has_usc THEN 'usc' ELSE 'secu_code' END;

    -- -----------------------------------------------------------------
    -- 2. 构造「目标列 ← 源列」清单（按目标列顺序，仅复制两边都有的列）
    -- -----------------------------------------------------------------
    FOR v_col IN
        SELECT column_name
          FROM information_schema.columns
         WHERE table_schema = 'cdeu'
           AND table_name   = 'finv_quote_secu_kline_min'
           AND is_generated = 'NEVER'
         ORDER BY ordinal_position
    LOOP
        v_src_col := v_col;

        -- 目标列在源表不存在时，尝试旧列名映射
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
             WHERE table_schema = 'public'
               AND table_name   = 'finv_quote_secu_kline_min'
               AND column_name  = v_src_col
        ) THEN
            IF v_col = 'usc' AND v_src_has_secucode THEN
                v_src_col := 'secu_code';
            ELSIF v_col = 'last_close' AND v_src_has_prevclose THEN
                v_src_col := 'prev_close';
            ELSE
                -- market_code / change_price / change_ratio / provider 等
                CONTINUE;
            END IF;
        END IF;

        v_dst_cols  := v_dst_cols  || format('%I,', v_col);
        v_sel_exprs := v_sel_exprs || format('s.%I,', v_src_col);
    END LOOP;

    v_dst_cols  := rtrim(v_dst_cols, ',');
    v_sel_exprs := rtrim(v_sel_exprs, ',');

    IF v_dst_cols = '' THEN
        RAISE EXCEPTION '未匹配到任何可复制列，终止';
    END IF;

    RAISE NOTICE '列映射：%', v_dst_cols;
    RAISE NOTICE '源列取值：%', v_sel_exprs;

    -- -----------------------------------------------------------------
    -- 3. 迁移前计数与重复键检查
    -- -----------------------------------------------------------------
    EXECUTE format('SELECT count(*) FROM %s', c_src) INTO v_src_rows;
    EXECUTE format('SELECT count(*) FROM %s', c_dst) INTO v_before;

    EXECUTE format(
        'SELECT count(*) FROM (SELECT ts, %s AS u FROM %s GROUP BY 1, 2 HAVING count(*) > 1) x',
        v_usc_col, c_src
    ) INTO v_dup;

    RAISE NOTICE '迁移前：源表 % 行，目标表 % 行；源表重复 (ts, usc) 组数 %',
                 v_src_rows, v_before, v_dup;
    IF v_dup > 0 THEN
        RAISE WARNING '源表存在 % 组重复 (ts, usc)，将保留 gmt_update 最新的一条', v_dup;
    END IF;

    IF v_src_rows = 0 THEN
        RAISE NOTICE '源表为空，无需迁移';
        RETURN;
    END IF;

    -- -----------------------------------------------------------------
    -- 4. 复制（去重 + 幂等）
    -- -----------------------------------------------------------------
    v_order_by := format('ts, %s', v_usc_col);
    IF v_src_has_gmtupdate THEN
        v_order_by := v_order_by || ', gmt_update DESC NULLS LAST';
    END IF;

    EXECUTE format($sql$
        INSERT INTO %1$s (%2$s)
        SELECT %3$s
          FROM (
                SELECT DISTINCT ON (ts, %4$s) *
                  FROM %5$s
                 ORDER BY %6$s
               ) s
        ON CONFLICT (ts, usc) DO NOTHING
    $sql$, c_dst, v_dst_cols, v_sel_exprs, v_usc_col, c_src, v_order_by);

    GET DIAGNOSTICS v_inserted = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 5. 校验
    -- -----------------------------------------------------------------
    EXECUTE format('SELECT count(*) FROM %s', c_dst) INTO v_after;

    EXECUTE format($sql$
        SELECT count(*)
          FROM %1$s s
         WHERE NOT EXISTS (
               SELECT 1 FROM %2$s d
                WHERE d.ts  = s.ts
                  AND d.usc = s.%3$I
         )
    $sql$, c_src, c_dst, v_usc_col) INTO v_missing;

    RAISE NOTICE '迁移后：目标表 % 行（新增 % 行，跳过已存在 % 行）',
                 v_after, v_inserted, v_src_rows - v_inserted;
    RAISE NOTICE '校验：源表仍未在目标表落地的行数 = %', v_missing;

    IF v_missing > 0 THEN
        RAISE EXCEPTION '校验失败：仍有 % 行源数据未写入目标表，已回滚', v_missing;
    END IF;

    IF v_after <> v_before + v_inserted THEN
        RAISE EXCEPTION '计数校验失败：目标表 % → %，新增 %，已回滚',
                        v_before, v_after, v_inserted;
    END IF;

    RAISE NOTICE '迁移完成（幂等：重复执行不会重复插入）';
END
$mig$;

COMMIT;
