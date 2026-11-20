-- =====================================================================
-- 字段改名（需求 10902）：paocd → vwap
--   quote.finv_quote_secu_kline_min
--   cdeu.finv_quote_secu_kline_min
--   cdeu.finv_quote_secu_kline_min_export
--
-- 背景：
--   paocd 是早期数据源 MVSV 文件里的列名（供应商缩写），语义为「当日实时
--   累计均价」。本次统一为业界通用的 vwap（成交量加权平均价）。
--   改名为纯 catalog 操作：该列无索引 / 视图 / 物化视图 / 触发器 / 函数依赖，
--   且 RENAME COLUMN 不改列顺序，历史迁移里的 paocd 字样一律不动
--   （全新库按旧结构建表后由本迁移改名）。
--
-- 幂等：仅当表存在且列仍叫 paocd 时执行 RENAME；已改名或无该列则跳过。
--
-- 同批对外契约调整：
--   · API 出参 JSON pacd → vwap；入参 vwap 优先，pacd 兼容期保留（待办 10903）；
--   · /PageUni 第 13 项短名 pa → vwa（全名 vwap）；
--   · MVSV-2 导出列名 paocd → vwap，导入兼容 paocd / pacd / vwap。
--
-- 回滚（人工）：ALTER TABLE <表> RENAME COLUMN vwap TO paocd;
-- =====================================================================

DO $$
DECLARE
    c_comment CONSTANT text := '成交量加权平均价（VWAP；原名 paocd / 当日实时累计均价）';
    v_table    text;
    v_has_old  boolean;
    v_has_new  boolean;
    v_left     integer;
BEGIN
    FOREACH v_table IN ARRAY ARRAY[
        'quote.finv_quote_secu_kline_min',
        'cdeu.finv_quote_secu_kline_min',
        'cdeu.finv_quote_secu_kline_min_export'
    ] LOOP
        IF to_regclass(v_table) IS NULL THEN
            RAISE NOTICE 'V300087：% 不存在，跳过', v_table;
            CONTINUE;
        END IF;

        SELECT count(*) FILTER (WHERE column_name = 'paocd') > 0,
               count(*) FILTER (WHERE column_name = 'vwap') > 0
          INTO v_has_old, v_has_new
          FROM information_schema.columns
         WHERE table_schema = split_part(v_table, '.', 1)
           AND table_name   = split_part(v_table, '.', 2);

        IF v_has_old THEN
            EXECUTE format('ALTER TABLE %s RENAME COLUMN paocd TO vwap', v_table);
            RAISE NOTICE 'V300087：% 列 paocd → vwap 已改名', v_table;
        ELSIF v_has_new THEN
            RAISE NOTICE 'V300087：% 列已是 vwap，跳过改名', v_table;
        ELSE
            RAISE NOTICE 'V300087：% 无 paocd / vwap 列，跳过', v_table;
        END IF;

        EXECUTE format('COMMENT ON COLUMN %s.vwap IS %L', v_table, c_comment);
    END LOOP;

    -- 收敛校验：现存三张表都不应再有 paocd 列。
    SELECT count(*) INTO v_left
      FROM information_schema.columns
     WHERE column_name = 'paocd'
       AND table_schema IN ('quote', 'cdeu')
       AND table_name IN ('finv_quote_secu_kline_min', 'finv_quote_secu_kline_min_export');

    IF v_left > 0 THEN
        RAISE EXCEPTION 'V300087 校验失败：仍有 % 处 paocd 列未改名', v_left;
    END IF;

    RAISE NOTICE 'V300087：三张 K 线表的 paocd 列已统一为 vwap';
END $$;
