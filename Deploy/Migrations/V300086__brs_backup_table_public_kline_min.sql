-- =====================================================================
-- V300086：把历史遗留表 public.finv_quote_secu_kline_min 加入 BRS 备份登记
-- （需求 10900）
--
-- 背景：该表即将按需求 10899 清理，但数据必须先备份到对象存储（R2）留档。
--   删除动作已在 V300085 暂缓；在此之前让它进入每日/手动备份清单，
--   备份产物路径：<39002.prefix>/<1002.str_value>/<yyyyMMdd>/finv.public.finv_quote_secu_kline_min.dump。
--
-- 约定：
--   · priority 660（现有最低 680），排在全部登记之后执行，避免大表拖慢常规备份；
--   · bak_scope=both / pg_dump / zstd / flag_upload='1'，与其它登记一致；
--   · 仅当该表存在时才登记（全新库/已清理环境不会产生指向不存在表的登记）；
--     该表由 Deploy/Upgrade/2610-003-DropPublicKlineMin.sql 删除时会同步删除本登记。
--
-- 幂等：ON CONFLICT (rdb_type, db, schema_name, table_name) DO NOTHING。
-- 迁移策略：单事务；末尾校验登记已存在。
-- =====================================================================

BEGIN;

-- 仅当表存在时登记：全新库 / 已清理过该表的环境没有这张表，
-- 登记一条指向不存在表的记录会让每轮备份都报错。
INSERT INTO brs.brs_bak_rdb_table
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
SELECT 'postgresql', 'finv', 'public', 'finv_quote_secu_kline_min', '1', 660, 'both', 'pg_dump', 'zstd', '1', 'migration',
       '历史遗留分钟K线表：清理前先备份到对象存储留档（需求 10900）；删除后须同步删除本登记'
 WHERE to_regclass('public.finv_quote_secu_kline_min') IS NOT NULL
ON CONFLICT DO NOTHING;

DO $$
DECLARE
    v_idn    bigint;
    v_exists boolean := to_regclass('public.finv_quote_secu_kline_min') IS NOT NULL;
BEGIN
    SELECT idn INTO v_idn
      FROM brs.brs_bak_rdb_table
     WHERE db = 'finv' AND schema_name = 'public' AND table_name = 'finv_quote_secu_kline_min';

    IF NOT v_exists THEN
        RAISE NOTICE 'V300086：public.finv_quote_secu_kline_min 不存在，跳过登记';
        RETURN;
    END IF;

    IF v_idn IS NULL THEN
        RAISE EXCEPTION 'V300086 校验失败：public.finv_quote_secu_kline_min 存在但未登记到备份清单';
    END IF;

    RAISE NOTICE 'V300086：public.finv_quote_secu_kline_min 已登记备份（idn=%）', v_idn;
END $$;

COMMIT;
