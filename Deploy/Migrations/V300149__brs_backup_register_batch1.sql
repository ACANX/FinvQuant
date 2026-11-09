-- =====================================================================
-- V300149：BRS 全量备份登记──第一批补录（需求 11065）
--
-- 背景：V300060（sys + cdeu 配置表）与 V300083（public 字典 + futu 映射）之后，
--   又新增了一批业务表，但一直没有登记到 brs.brs_bak_rdb_table_full，
--   因此不在每日全量备份范围内。本脚本按「第一批」清单显式补录 17 张表：
--
--   A. 最近迁移新建/改名的 8 张
--      cdeu.finv_quote_secu_indicator            V300144 建 / V300145 改名（需求 11049）
--      futu.finv_futu_secu_key                   V300137 建 / V300141 改名（需求 11037–11041）
--      cdeu.finv_cdeu_secu_realtime_stock        V300115
--      cdeu.finv_cdeu_secu_realtime_sync_config  V300121 建 / V300122 改名
--      cdeu.finv_quote_secu_kline_day            V300114
--      cdeu.finv_quote_cdeu_usc_patch_rule       V300103
--      public.finv_fxcer                         V300124
--      public.finv_cer                           V300123（该脚本已删除，仅老环境有表）
--
--   B. quant 回测业务表中「体积可控」的 9 张（4 张大表不在本批，见下）
--
-- 本批**不含**（第二批处理，需先定大表口径）：
--   quant.finv_quant_backtest_event_trace（约 60 万行）
--   quant.finv_quant_backtest_position_log / _equity / _cashflow（各 7 万～14 万行）
--   以及 cdeu 的 export_* / job_* / *_run 历史遗漏表、quote.* 三张表。
--
-- 约定：
--   · 已登记 24 条的 priority 为 690～1000（步长 10），本批从 680 起往下取 10 的整数；
--   · 跳过 660：V300086 曾用该值条件登记 public.finv_quote_secu_kline_min，
--     若某环境仍有该表，占用 660 会让本批某一行因 priority 唯一键冲突被 DO NOTHING 静默跳过；
--   · 备份范围 both（结构 + 数据），方式 pg_dump，压缩 zstd，上传开关 '1'；
--   · 登记库名 db='finv'（当前库；本字段仅登记归属，执行仍连当前库）；
--   · 每行都用 to_regclass 守卫：某环境缺这张表就跳过登记，不制造「指向不存在表」的僵尸登记
--     （典型是 public.finv_cer：建表脚本 V300123 已删除，新装环境没有这张表）。
--
-- 幂等：INSERT ... ON CONFLICT DO NOTHING（唯一键 rdb_type+db+schema_name+table_name）；
--   priority 冲突同样 DO NOTHING。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验（存在的表必须都登记上）。
-- =====================================================================

BEGIN;

-- 记录本批执行前的登记行数，供末尾汇报「本批新登记 N 张」用（事务级设置）
DO $$
DECLARE
    v_before bigint;
BEGIN
    SELECT count(*) INTO v_before FROM brs.brs_bak_rdb_table_full;
    PERFORM set_config('finv.v300149_before', v_before::text, true);
END $$;

INSERT INTO brs.brs_bak_rdb_table_full
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
SELECT 'postgresql', 'finv', v.schema_name, v.table_name, '1', v.priority, 'both',
       'pg_dump', 'zstd', '1', 'migration', v.remark
  FROM (VALUES
        -- A. 最近新增/改名的 8 张
        ('cdeu',   'finv_quote_secu_indicator',           680, '证券指标（源上游 public.finv_quote_secu_meta，需求 11049）'),
        ('futu',   'finv_futu_secu_key',                  670, '富途证券键表（需求 11037–11041）'),
        ('cdeu',   'finv_cdeu_secu_realtime_stock',       650, '证券实时快照（34 列，需求 11020 前后）'),
        ('cdeu',   'finv_cdeu_secu_realtime_sync_config', 640, '证券实时快照同步配置'),
        ('cdeu',   'finv_quote_secu_kline_day',           630, '证券日 K 线'),
        ('cdeu',   'finv_quote_cdeu_usc_patch_rule',      620, 'USC 补丁规则'),
        ('public', 'finv_fxcer',                          610, '汇率（含变动列，由 finv_cer 演进）'),
        ('public', 'finv_cer',                            600, '汇率（v1，历史保留；建表脚本已删除，新环境无此表）'),
        -- B. quant 体积可控的 9 张
        ('quant',  'finv_quant_backtest_account',         590, '回测账户'),
        ('quant',  'finv_quant_backtest_del_log',         580, '回测删除日志'),
        ('quant',  'finv_quant_backtest_del_task',        570, '回测删除任务'),
        ('quant',  'finv_quant_backtest_run',             560, '回测运行'),
        ('quant',  'finv_quant_backtest_run_archive',     550, '回测运行归档'),
        ('quant',  'finv_quant_backtest_strategy',        540, '回测策略'),
        ('quant',  'finv_quant_backtest_trade',           530, '回测成交（约 7 万行，仍在全量可接受范围）'),
        ('quant',  'finv_quant_environment',              520, '量化环境'),
        ('quant',  'finv_quant_template',                 510, '量化模板')
       ) AS v(schema_name, table_name, priority, remark)
 WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：本批清单中「库里确实存在」的表必须都已登记
--
-- 不校验「quant/cdeu 下所有表都已登记」：第二批（大表 + 历史遗漏）尚未处理，
-- 全表校验会让迁移直接失败、服务无法启动。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing text;
    v_skipped text;
    v_registered bigint;
BEGIN
    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_missing
      FROM (VALUES
              ('cdeu',   'finv_quote_secu_indicator'),
              ('futu',   'finv_futu_secu_key'),
              ('cdeu',   'finv_cdeu_secu_realtime_stock'),
              ('cdeu',   'finv_cdeu_secu_realtime_sync_config'),
              ('cdeu',   'finv_quote_secu_kline_day'),
              ('cdeu',   'finv_quote_cdeu_usc_patch_rule'),
              ('public', 'finv_fxcer'),
              ('public', 'finv_cer'),
              ('quant',  'finv_quant_backtest_account'),
              ('quant',  'finv_quant_backtest_del_log'),
              ('quant',  'finv_quant_backtest_del_task'),
              ('quant',  'finv_quant_backtest_run'),
              ('quant',  'finv_quant_backtest_run_archive'),
              ('quant',  'finv_quant_backtest_strategy'),
              ('quant',  'finv_quant_backtest_trade'),
              ('quant',  'finv_quant_environment'),
              ('quant',  'finv_quant_template')
           ) AS v(schema_name, table_name)
     WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
       AND NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table_full b
              WHERE b.db = 'finv'
                AND b.schema_name = v.schema_name
                AND b.table_name = v.table_name
           );

    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300149 校验失败：以下表未登记备份：%', v_missing;
    END IF;

    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_skipped
      FROM (VALUES
              ('public', 'finv_cer')
           ) AS v(schema_name, table_name)
     WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NULL;

    IF v_skipped IS NOT NULL THEN
        RAISE NOTICE 'V300149：本环境不存在以下表，已跳过登记：%', v_skipped;
    END IF;

    SELECT count(*) INTO v_registered FROM brs.brs_bak_rdb_table_full;

    RAISE NOTICE 'V300149：全量备份登记表共 % 张（本批新登记 % 张，清单 17 张）',
        v_registered, v_registered - current_setting('finv.v300149_before')::bigint;
END $$;

COMMIT;
