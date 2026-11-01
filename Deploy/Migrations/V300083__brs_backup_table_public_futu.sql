-- =====================================================================
-- V300083：把 public / futu schema 中的表加入 BRS 备份登记（需求 10894）
--
-- 背景：V300060 只预置了 sys.* 与 cdeu 配置表的种子，public（证券主数据字典）
--   与 futu（富途映射 / 行情采集）两张 schema 的表未登记，不在每日备份范围内。
--
-- 约定：
--   · 已登记的 12 条 priority 为 1000～830，本次新增表使用 800 往下递减，互不冲突；
--   · 备份范围 both（结构 + 数据），方式 pg_dump，压缩 zstd，上传开关 '1'（跟随表级配置）；
--   · 登记库名 db='finv'（当前库；本字段仅登记归属，执行仍连当前库）。
--
-- 包含迁移元数据表 public.schema_version / public.schema_version_plan_shift：
--   这两张表记录已应用的迁移版本与号段平移标记，按「public schema 中的表全部登记」
--   的要求一并纳入；恢复时请用 pg_restore 选择表，勿整库覆盖（见 Docs/FinvQuant/Brs.md）。
--
-- 幂等：INSERT ... ON CONFLICT (rdb_type, db, schema_name, table_name) DO NOTHING；
--   priority 冲突同样 DO NOTHING（若有人手工占用 680～800 之间的值，需自行调整）。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- =====================================================================

BEGIN;

INSERT INTO brs.brs_bak_rdb_table
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
VALUES
    -- futu：富途映射与行情采集
    ('postgresql', 'finv', 'futu', 'finv_futu_mapping_cs_market',    '1', 800, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途映射：CS 市场'),
    ('postgresql', 'finv', 'futu', 'finv_futu_mapping_exchange',     '1', 790, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途映射：交易所'),
    ('postgresql', 'finv', 'futu', 'finv_futu_mapping_market_code',  '1', 780, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途映射：市场代码'),
    ('postgresql', 'finv', 'futu', 'finv_futu_mapping_region',       '1', 770, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途映射：地区'),
    ('postgresql', 'finv', 'futu', 'finv_futu_mapping_security',     '1', 760, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途映射：证券'),
    ('postgresql', 'finv', 'futu', 'finv_futu_quote_collect',        '1', 750, 'both', 'pg_dump', 'zstd', '1', 'migration', '富途行情采集'),
    -- public：证券主数据字典与迁移元数据
    ('postgresql', 'finv', 'public', 'finv_currency',                '1', 740, 'both', 'pg_dump', 'zstd', '1', 'migration', '货币字典'),
    ('postgresql', 'finv', 'public', 'finv_exchange',                '1', 730, 'both', 'pg_dump', 'zstd', '1', 'migration', '交易所字典'),
    ('postgresql', 'finv', 'public', 'finv_market',                  '1', 720, 'both', 'pg_dump', 'zstd', '1', 'migration', '交易市场字典'),
    ('postgresql', 'finv', 'public', 'finv_region',                  '1', 710, 'both', 'pg_dump', 'zstd', '1', 'migration', '地区字典'),
    ('postgresql', 'finv', 'public', 'finv_security',                '1', 700, 'both', 'pg_dump', 'zstd', '1', 'migration', '证券主数据'),
    ('postgresql', 'finv', 'public', 'schema_version',               '1', 690, 'both', 'pg_dump', 'zstd', '1', 'migration', '迁移版本记录（恢复时勿整库覆盖）'),
    ('postgresql', 'finv', 'public', 'schema_version_plan_shift',    '1', 680, 'both', 'pg_dump', 'zstd', '1', 'migration', '号段平移标记表')
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：public / futu 下的每张表都必须已登记
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing text;
    v_count   bigint;
BEGIN
    SELECT string_agg(t.schemaname || '.' || t.tablename, ', ' ORDER BY 1)
      INTO v_missing
      FROM pg_tables t
     WHERE t.schemaname IN ('public', 'futu')
       AND NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table b
              WHERE b.db = 'finv'
                AND b.schema_name = t.schemaname
                AND b.table_name = t.tablename
           );

    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300083 校验失败：以下 public/futu 表未登记备份：%', v_missing;
    END IF;

    SELECT count(*) INTO v_count
      FROM brs.brs_bak_rdb_table
     WHERE schema_name IN ('public', 'futu');
    RAISE NOTICE 'V300083：public/futu 已登记 % 张表', v_count;
END $$;

COMMIT;
