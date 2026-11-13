-- =====================================================================
-- V300152：BRS 全量备份登记──补录 brs schema 自身的 3 张表（需求 11068）
--
-- 背景：V300060 / V300083 / V300149 / V300150 / V300151 登记的都是业务表，
--   备份系统自身的登记表一直没进清单。后果是「恢复时不知道该备份哪些表」——
--   全量备份产物里没有任何一张表说明备份范围、切片规则与数据源，
--   只能靠人工回忆，属于备份链条的自指缺口。
--
-- 本批登记（3 张，均为几十行的元数据表，full 备份成本可忽略）：
--   brs.brs_bak_rdb_table_full     小表全量备份登记（本清单自身）
--   brs.brs_bak_rdb_table_incr     大表增量（分片）备份登记
--   brs.brs_bak_rdb_datasource     BRS RDB 备份数据源登记
--
-- 相关配置不在 brs schema：每日备份 cron 与对象存储配置存于 sys.sys_config
--   （config_no 39001 / 39002），该表已于 V300060 登记（priority 990），恢复时随它回来，
--   故本批不再重复登记。
--
-- 约定：
--   · priority 唯一：现占用 460～1000 中除 600/660 外的步长 10 空档，
--     本批用 600（V300151 撤销 public.finv_cer 后释放）与 450/440（低于现有最小值 460）；
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
    PERFORM set_config('finv.v300152_before', v_before::text, true);
END $$;

INSERT INTO brs.brs_bak_rdb_table_full
    (rdb_type, db, schema_name, table_name, flag_enable, priority, bak_scope,
     bak_method, compression, flag_upload, creator, remark)
SELECT 'postgresql', 'finv', v.schema_name, v.table_name, '1', v.priority, 'both',
       'pg_dump', 'zstd', '1', 'migration', v.remark
  FROM (VALUES
        ('brs', 'brs_bak_rdb_table_full',  600, 'BRS 小表全量备份登记（本清单自身，恢复时据此还原备份范围）'),
        ('brs', 'brs_bak_rdb_table_incr',  450, 'BRS 大表增量（分片）备份登记（分片键、模板与行数阈值）'),
        ('brs', 'brs_bak_rdb_datasource',  440, 'BRS RDB 备份数据源登记（连接信息与优先级）')
       ) AS v(schema_name, table_name, priority, remark)
 WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------------
-- 收敛校验：本批清单中「库里确实存在」的表必须都已登记
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_missing text;
    v_registered bigint;
BEGIN
    SELECT string_agg(v.schema_name || '.' || v.table_name, ', ' ORDER BY 1)
      INTO v_missing
      FROM (VALUES
              ('brs', 'brs_bak_rdb_table_full'),
              ('brs', 'brs_bak_rdb_table_incr'),
              ('brs', 'brs_bak_rdb_datasource')
           ) AS v(schema_name, table_name)
     WHERE to_regclass(v.schema_name || '.' || v.table_name) IS NOT NULL
       AND NOT EXISTS (
             SELECT 1 FROM brs.brs_bak_rdb_table_full b
              WHERE b.db = 'finv'
                AND b.schema_name = v.schema_name
                AND b.table_name = v.table_name
           );

    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300152 校验失败：以下表未登记备份：%', v_missing;
    END IF;

    SELECT count(*) INTO v_registered FROM brs.brs_bak_rdb_table_full;

    RAISE NOTICE 'V300152：全量备份登记表共 % 张（本批新登记 % 张，清单 3 张）',
        v_registered, v_registered - current_setting('finv.v300152_before')::bigint;
END $$;

COMMIT;
