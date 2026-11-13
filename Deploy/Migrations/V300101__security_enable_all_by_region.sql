-- =====================================================================
-- V300101：证券主数据 flag_enable 全量启用（需求 10929，清单 M8）
--
-- 背景：
--   Prod 的 public.finv_security 只有 552 行启用（flag_enable='1'），
--   其余 16647 行为 '0'；LocalDocker 这边全量启用是有意为之。
--   业务方确认以 LocalDocker 为准，向 Prod 发放。
--
-- 影响面：
--   RefreshSecurityMetadata 用 WHERE COALESCE(s.flag_enable, '1') = '1' 过滤，
--   发放后可参与元数据刷新的证券由 552 行扩到 17199 行，
--   这是补全 category / name / timezone / currency 的前提。
--
-- 执行方式（控制步子）：
--   按 finv_exchange.region 分组，每组每批最多 1000 行循环更新，
--   直到该区域没有待启用行为止；每组打印批次行数，便于观察。
--
-- 规范合规（DbSchemaSpec §0.7）：
--   更新前把待更新行整行备份到 archive.finv_security_purge_20261102，
--   备份与更新在同一事务内；主键 (usc, src_schema) 保证重复执行不重复备份。
--
-- 幂等：全部行都已启用时，备份 0 行、更新 0 行。
-- =====================================================================

BEGIN;

CREATE TABLE IF NOT EXISTS archive.finv_security_purge_20261102 (
    LIKE public.finv_security INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'public',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, src_schema)
);

DO $$
DECLARE
    c_reason   CONSTANT text := '需求 10929：flag_enable 全量启用（M8）';
    c_batch    CONSTANT int  := 1000;
    r            record;
    v_backup     bigint;
    v_updated    bigint;
    v_total      bigint := 0;
    v_backup_all bigint := 0;
BEGIN
    FOR r IN
        SELECT COALESCE(e.region, '') AS region
          FROM public.finv_security s
          LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
         GROUP BY 1
         ORDER BY 1
    LOOP
        LOOP
            -- 备份本批待更新行
            INSERT INTO archive.finv_security_purge_20261102
                (usc, exchange_code, security_type, security_code, security_name,
                 security_name_cn, security_name_full, currency_type, init_date,
                 timezone, tz, gmt_create, gmt_update, flag_enable, market_code,
                 type_secu_sub, provider, remark, src_schema, reason)
            SELECT s.usc, s.exchange_code, s.security_type, s.security_code, s.security_name,
                   s.security_name_cn, s.security_name_full, s.currency_type, s.init_date,
                   s.timezone, s.tz, s.gmt_create, s.gmt_update, s.flag_enable, s.market_code,
                   s.type_secu_sub, s.provider, s.remark, 'public', c_reason
              FROM public.finv_security s
              LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
             WHERE COALESCE(e.region, '') = r.region
               AND s.flag_enable <> '1'
             LIMIT c_batch
            ON CONFLICT (usc, src_schema) DO NOTHING;
            GET DIAGNOSTICS v_backup = ROW_COUNT;

            UPDATE public.finv_security s
               SET flag_enable = '1',
                   gmt_update  = now()
             WHERE s.usc IN (
                     SELECT s2.usc
                       FROM public.finv_security s2
                       LEFT JOIN public.finv_exchange e2 ON e2.exchange_code = s2.exchange_code
                      WHERE COALESCE(e2.region, '') = r.region
                        AND s2.flag_enable <> '1'
                      LIMIT c_batch
                   );
            GET DIAGNOSTICS v_updated = ROW_COUNT;

            v_backup_all := v_backup_all + v_backup;
            v_total      := v_total + v_updated;

            RAISE NOTICE 'region=% 本批：备份 % 行 / 启用 % 行', r.region, v_backup, v_updated;
            EXIT WHEN v_updated = 0;
        END LOOP;
    END LOOP;

    RAISE NOTICE '完成：共备份 % 行，共启用 % 行，剩余未启用 % 行',
        v_backup_all, v_total,
        (SELECT count(*) FROM public.finv_security WHERE flag_enable <> '1');
END $$;

COMMIT;

-- 执行后核对
-- SELECT flag_enable, count(*) FROM public.finv_security GROUP BY 1;
-- SELECT count(*) FROM public.finv_security WHERE flag_enable <> '1';  -- 应为 0
