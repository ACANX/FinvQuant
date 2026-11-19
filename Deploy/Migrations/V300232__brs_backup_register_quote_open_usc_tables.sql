-- =====================================================================
-- V300232：BRS 小表全量备份登记补登 11 张（需求 11342）
--
-- 背景：用户圈定清单里 16 张表，其中 5 张已登记（幂等跳过）：
--   cdeu.finv_quote_secu_realtime_crypto  (idn 117, priority 40)
--   cdeu.finv_quote_secu_realtime_future  (idn 114, priority 10)
--   quote.finv_quote_secu_fav             (idn 116, priority 30)
--   futu.finv_futu_secu_key               (idn  50, priority 670)
--   public.finv_security                  (idn  29, priority 700)
-- 本次新增 11 张（priority 取空闲值；优先级越大越先备份）：
--   960  futu.finv_futu_usc_rule           usc 映射规则层（10 条，人工维护）
--   950  futu.finv_futu_usc_rule_item      usc 规则条目（705 条，含人工调整）
--   940  futu.finv_futu_usc_map            usc 映射结果（含 RETIRED/复用链/首次分配时间，重算不可复原）
--   930  futu.finv_futu_usc_rule_log       usc 规则变更审计
--    90  cdeu.finv_quote_secu_realtime     跨 provider 行情快照（crypto/future 同族已登记）
--    85  open.open_api_client              开放 API 客户端
--    80  open.open_api_endpoint            开放 API 端点授权
--    75  open.open_api_credential          开放 API 凭据（密文）
--    70  open.open_api_credential_scope    开放 API 凭据授权范围
--    65  cdeu.finv_quote_cdeu_export_file  导出文件台账（避免重复上传/漏导）
--    55  cdeu.finv_quote_cdeu_scan_watermark 扫描水位（丢了只全量重扫，不丢数据）
--
-- 幂等与安全：
--   - 唯一键命中（rdb_type/db/schema_name/table_name）→ 跳过，不覆盖运维改过的
--     flag_enable / compression / flag_upload 等字段；
--   - priority 已被别的表占用 → 跳过并 RAISE NOTICE，让后续人工处理，
--     绝不为新表挤掉既有登记的优先级；
--   - 表不存在（部分环境还没跑到建表迁移）→ 跳过，不报错。
--   - 全部为 bak_scope=both / bak_method=pg_dump / compression=zstd / flag_upload=1，
--     与既有 51 行口径一致。
--
-- 回滚：
--   DELETE FROM brs.brs_bak_rdb_table_full
--    WHERE creator = 'migration'
--      AND priority IN (960, 950, 940, 930, 90, 85, 80, 75, 70, 65, 55);
-- =====================================================================

BEGIN;

DO $$
DECLARE
    v_item     record;
    v_added    int := 0;
    v_skipped  text := '';
BEGIN
    IF to_regclass('brs.brs_bak_rdb_table_full') IS NULL THEN
        RAISE NOTICE 'V300232：备份登记表不存在，跳过';
        RETURN;
    END IF;

    FOR v_item IN
        SELECT * FROM (VALUES
            ('futu', 'finv_futu_usc_rule',            960, 'usc 映射规则层（10 条，人工维护；需求 11342）'),
            ('futu', 'finv_futu_usc_rule_item',       950, 'usc 规则条目（705 条，含人工调整；需求 11342）'),
            ('futu', 'finv_futu_usc_map',             940, 'usc 映射结果（含退市/复用链/首次分配时间，重算不可复原；需求 11342）'),
            ('futu', 'finv_futu_usc_rule_log',        930, 'usc 规则变更审计（需求 11342）'),
            ('cdeu', 'finv_quote_secu_realtime',       90, '跨 provider 证券行情快照（crypto/future 同族已登记；需求 11342）'),
            ('open', 'open_api_client',                85, '开放 API 客户端（需求 11342）'),
            ('open', 'open_api_endpoint',              80, '开放 API 端点授权（需求 11342）'),
            ('open', 'open_api_credential',            75, '开放 API 凭据（密文；需求 11342）'),
            ('open', 'open_api_credential_scope',      70, '开放 API 凭据授权范围（需求 11342）'),
            ('cdeu', 'finv_quote_cdeu_export_file',    65, '导出文件台账（避免重复上传/漏导；需求 11342）'),
            ('cdeu', 'finv_quote_cdeu_scan_watermark', 55, '导出扫描水位（丢失只全量重扫，不丢数据；需求 11342）')
        ) AS t(schema_name, table_name, priority, remark)
    LOOP
        IF to_regclass(format('%I.%I', v_item.schema_name, v_item.table_name)) IS NULL THEN
            v_skipped := v_skipped || format('%s.%s（表不存在）；', v_item.schema_name, v_item.table_name);
            CONTINUE;
        END IF;

        IF EXISTS (
            SELECT 1 FROM brs.brs_bak_rdb_table_full
             WHERE rdb_type = 'postgresql' AND db = 'finv'
               AND schema_name = v_item.schema_name
               AND table_name = v_item.table_name
        ) THEN
            v_skipped := v_skipped || format('%s.%s（已登记）；', v_item.schema_name, v_item.table_name);
            CONTINUE;
        END IF;

        IF EXISTS (SELECT 1 FROM brs.brs_bak_rdb_table_full WHERE priority = v_item.priority) THEN
            v_skipped := v_skipped || format('%s.%s（priority %s 已被占用）；',
                v_item.schema_name, v_item.table_name, v_item.priority);
            CONTINUE;
        END IF;

        INSERT INTO brs.brs_bak_rdb_table_full
            (rdb_type, db, schema_name, table_name, flag_enable, priority,
             bak_scope, bak_method, compression, flag_upload, remark, creator)
        VALUES
            ('postgresql', 'finv', v_item.schema_name, v_item.table_name, '1', v_item.priority,
             'both', 'pg_dump', 'zstd', '1', v_item.remark, 'migration');
        v_added := v_added + 1;
    END LOOP;

    RAISE NOTICE 'V300232：新增备份登记 % 张；跳过：%', v_added, COALESCE(NULLIF(v_skipped, ''), '无');
END $$;

-- 收敛校验：11 张目标表必须已登记（本迁移新插的或此前已有的都算）
DO $$
DECLARE
    v_missing text;
BEGIN
    SELECT string_agg(x.t, ', ' ORDER BY x.t) INTO v_missing
      FROM (VALUES
            ('futu.finv_futu_usc_rule'), ('futu.finv_futu_usc_rule_item'),
            ('futu.finv_futu_usc_map'), ('futu.finv_futu_usc_rule_log'),
            ('cdeu.finv_quote_secu_realtime'),
            ('open.open_api_client'), ('open.open_api_endpoint'),
            ('open.open_api_credential'), ('open.open_api_credential_scope'),
            ('cdeu.finv_quote_cdeu_export_file'), ('cdeu.finv_quote_cdeu_scan_watermark')
           ) AS x(t)
     WHERE to_regclass(x.t) IS NOT NULL
       AND NOT EXISTS (
            SELECT 1 FROM brs.brs_bak_rdb_table_full r
             WHERE r.rdb_type = 'postgresql' AND r.db = 'finv'
               AND r.schema_name || '.' || r.table_name = x.t
       );
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V300232 校验失败：以下表仍在备份登记之外（%）；请检查 priority 是否被占用', v_missing;
    END IF;
    RAISE NOTICE 'V300232：目标表均已登记，备份登记总行数 = %',
        (SELECT count(*) FROM brs.brs_bak_rdb_table_full);
END $$;

COMMIT;
