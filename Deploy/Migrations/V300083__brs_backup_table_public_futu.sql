-- =====================================================================
-- V300083：把 public / futu schema 中的表加入 BRS 备份登记（需求 10894）
--
-- 背景：V300060 只预置了 sys.* 与 cdeu 配置表的种子，public（证券主数据字典）
--   与 futu（富途映射 / 行情采集）两张 schema 的表未登记，不在每日备份范围内。
--
-- 约定：
--   · 已登记的 12 条 priority 为 1000～830，本次新增表使用 800 往下递减，互不冲突；
--   · 仅登记下列 13 张表；各环境若有额外历史遗留表（如生产库 public 下早期分钟 K 线表），
--     不在此脚本强制登记，由运维按需在「备份登记维护」页面追加。
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
-- 收敛校验：本次显式登记的表必须都已存在登记行
--
-- 为什么**不**遍历 pg_tables 要求「public/futu 下所有表都已登记」：
--   各环境的表集合不同（生产库 public 下可能仍保留早期落在 public 的
--   分钟 K 线表 public.finv_quote_secu_kline_min），全表校验会让迁移在
--   这些环境直接失败、服务无法启动（2026-11-01 线上即因此反复重启）。
--   单表登记策略改为「按清单显式登记」，未列入的表由运维在
--   「备份登记维护」页面自行追加。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing text;
    v_count   bigint;
BEGIN
    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_missing
      FROM (VALUES
              ('futu',   'finv_futu_mapping_cs_market'),
              ('futu',   'finv_futu_mapping_exchange'),
              ('futu',   'finv_futu_mapping_market_code'),
              ('futu',   'finv_futu_mapping_region'),
              ('futu',   'finv_futu_mapping_security'),
              ('futu',   'finv_futu_quote_collect'),
              ('public', 'finv_currency'),
              ('public', 'finv_exchange'),
              ('public', 'finv_market'),
              ('public', 'finv_region'),
              ('public', 'finv_security'),
              ('public', 'schema_version'),
              ('public', 'schema_version_plan_shift')
           ) AS v(schema_name, table_name)
     WHERE NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table b
              WHERE b.db = 'finv'
                AND b.schema_name = v.schema_name
                AND b.table_name = v.table_name
           );

    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300083 校验失败：以下表未登记备份：%', v_missing;
    END IF;

    SELECT count(*) INTO v_count
      FROM brs.brs_bak_rdb_table
     WHERE schema_name IN ('public', 'futu');
    RAISE NOTICE 'V300083：public/futu 已登记 % 张表', v_count;
END $$;

COMMIT;
