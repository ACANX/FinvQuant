-- =====================================================================
-- V300150：BRS 全量备份登记──cdeu 补充 4 张（需求 11066）
--
-- 背景：V300149（第一批 17 张）之后，cdeu 下仍有 19 张历史遗漏表未登记。
--   经确认，本批只登记其中 4 张，其余 15 张**暂不纳入备份**（保持不登记即可，
--   不写入任何 flag_enable='0' 的占位行，避免登记表被「不备份的表」污染）。
--
-- 本批登记（4 张，均来自 V200078 建表或其后加前缀/独立迁移）：
--   cdeu.finv_quote_cdeu_export_batch            导出批次清单（739 行）
--   cdeu.finv_quote_cdeu_export_job              证券月导出任务（436 行）
--   cdeu.finv_quote_cdeu_usc_day_incr_row_tpl    日增量行数模板（20 行）
--   cdeu.finv_quote_cdeu_ingest_stats            每日入口流量统计（5 行）
--
-- 暂不登记（如需再补，另起迁移）：
--   cdeu.finv_quote_cdeu_job_run_log_line、cdeu.finv_quote_cdeu_job_run_log_export_file、
--   cdeu.finv_quote_cdeu_job_step_dynamic、cdeu.finv_quote_cdeu_quote_maturity_mon、
--   cdeu.finv_quote_cdeu_job_dynamic、cdeu.finv_quote_cdeu_export_file、
--   cdeu.finv_quote_cdeu_export_file_upload_log、cdeu.finv_quote_cdeu_job_run、
--   cdeu.finv_quote_cdeu_process_run、cdeu.finv_quote_cdeu_job_step_run、
--   cdeu.finv_quote_cdeu_phase_run、cdeu.finv_quote_cdeu_scan_watermark、
--   cdeu.finv_quote_cdeu_workflow_run、cdeu.finv_quote_cdeu_export_file_patch、
--   cdeu.finv_quote_secu_kline_min_export
--
-- 约定：
--   · priority 唯一：现有 24 条为 690～1000，V300149 的 17 条为 510～680，
--     本批从 490 起往下取 10 的整数（460～490），互不冲突；
--   · 备份范围 both（结构 + 数据），方式 pg_dump，压缩 zstd，上传开关 '1'；
--   · 登记库名 db='finv'（当前库；本字段仅登记归属，执行仍连当前库）；
--   · 每行带 to_regclass 守卫：某环境缺这张表就跳过登记，不制造僵尸登记。
--
-- 幂等：INSERT ... ON CONFLICT DO NOTHING（唯一键 rdb_type+db+schema_name+table_name）。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验（存在的表必须都登记上）。
-- =====================================================================

BEGIN;

-- 记录本批执行前的登记行数，供末尾汇报「本批新登记 N 张」用（事务级设置）
DO $$
DECLARE
    v_before bigint;
BEGIN
    SELECT count(*) INTO v_before FROM brs.brs_bak_rdb_table_full;
    PERFORM set_config('finv.v300150_before', v_before::text, true);
END $$;

INSERT INTO brs.brs_bak_rdb_table_full
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
SELECT 'postgresql', 'finv', v.schema_name, v.table_name, '1', v.priority, 'both',
       'pg_dump', 'zstd', '1', 'migration', v.remark
  FROM (VALUES
        ('cdeu', 'finv_quote_cdeu_export_batch',          490, '导出批次清单（当前批次需导出的证券、业务年月与 BASE/INCR 类型）'),
        ('cdeu', 'finv_quote_cdeu_export_job',            480, '证券月导出任务（状态、游标、锁与版本；按 usc 逻辑关联）'),
        ('cdeu', 'finv_quote_cdeu_usc_day_incr_row_tpl',  470, '日增量行数模板（region + market + category → day_expect_rows）'),
        ('cdeu', 'finv_quote_cdeu_ingest_stats',          460, '每日入口流量统计（用于计算日预算）')
       ) AS v(schema_name, table_name, priority, remark)
 WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：本批清单中「库里确实存在」的表必须都已登记
--
-- 不校验「cdeu 下所有表都已登记」：另有 15 张经确认暂不备份，全表校验会误报失败。
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing text;
    v_registered bigint;
BEGIN
    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_missing
      FROM (VALUES
              ('cdeu', 'finv_quote_cdeu_export_batch'),
              ('cdeu', 'finv_quote_cdeu_export_job'),
              ('cdeu', 'finv_quote_cdeu_usc_day_incr_row_tpl'),
              ('cdeu', 'finv_quote_cdeu_ingest_stats')
           ) AS v(schema_name, table_name)
     WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
       AND NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table_full b
              WHERE b.db = 'finv'
                AND b.schema_name = v.schema_name
                AND b.table_name = v.table_name
           );

    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300150 校验失败：以下表未登记备份：%', v_missing;
    END IF;

    SELECT count(*) INTO v_registered FROM brs.brs_bak_rdb_table_full;

    RAISE NOTICE 'V300150：全量备份登记表共 % 张（本批新登记 % 张，清单 4 张）',
        v_registered, v_registered - current_setting('finv.v300150_before')::bigint;
END $$;

COMMIT;
