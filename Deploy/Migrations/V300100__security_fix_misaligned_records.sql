-- =====================================================================
-- V300100：证券主数据错位记录修正与字段互校（需求 10928，清单 M4、M6、M7）
--
-- 背景：
--   2026-10-30 一次批量导入（remark 均为「20261030-SupabasePG-finv_quote_collect_futu_view
--   批量导入」）把上海指数记录写进了深圳个股的 usc，造成 usc 与内容错位。
--   LocalDocker 上这几条是正确的，Prod 上错误，本迁移把两侧收敛到同一份正确值。
--
-- 处理范围：
--   M4  usc='000680' 且 security_type='StockIndex'  → 改正为山推股份（深市个股）
--   M6  usc='SH0680'                                → 逐字段互校（科创综指）
--   M7  usc='SH0985'                                → 补齐 type_secu_sub / provider / remark
--
-- 幂等与安全：
--   · M4 带 security_type='StockIndex' 条件，LocalDocker 上该行为 Stock，不会被改；
--   · M6 / M7 用 COALESCE 与显式定值，重复执行影响 0 行；
--   · 改前整行备份到 archive.finv_security_purge_20261102（DbSchemaSpec §0.7）。
--
-- 未包含：M5（Prod 000985 错位）待确认后再单独处理。
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
    c_reason CONSTANT text := '需求 10928：错位记录修正与字段互校（M4、M6、M7）';
    v_before  bigint;
    v_backup  bigint;
    v_updated bigint;
BEGIN
    -- 备份本次会触碰的三行（不论最终是否更新）
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
     WHERE s.usc IN ('000680', 'SH0680', 'SH0985')
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_backup = ROW_COUNT;

    -- M4：000680 错位记录改正为山推股份（只动错位那条，正确记录不碰）
    UPDATE public.finv_security
       SET exchange_code    = '12',
           security_type    = 'Stock',
           security_name    = '山推股份',
           security_name_cn = '山推股份',
           security_name_full = '山推股份',
           market_code      = '1120',
           type_secu_sub    = 'Stock.SZ',
           provider         = 'FTMM',
           flag_enable      = '1',
           gmt_update       = now()
     WHERE usc = '000680'
       AND security_type = 'StockIndex';
    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RAISE NOTICE 'M4  000680 错位记录改正：% 行', v_updated;

    -- M6：SH0680 科创综指逐字段互校
    UPDATE public.finv_security
       SET security_code = '000680',
           market_code   = '1112',
           security_name = '科创综指',
           type_secu_sub = COALESCE(NULLIF(type_secu_sub, ''), 'StockIndex.SH'),
           provider      = COALESCE(NULLIF(provider, ''), 'FTMM'),
           remark        = COALESCE(NULLIF(remark, ''),
                                    '20261030-SupabasePG-finv_quote_collect_futu_view批量导入'),
           gmt_update    = now()
     WHERE usc = 'SH0680'
       AND (security_code <> '000680'
            OR market_code <> '1112'
            OR security_name <> '科创综指'
            OR COALESCE(type_secu_sub, '') = ''
            OR COALESCE(provider, '') = ''
            OR COALESCE(remark, '') = '');
    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RAISE NOTICE 'M6  SH0680 字段互校：% 行', v_updated;

    -- M7：SH0985 中证全指补齐空字段
    UPDATE public.finv_security
       SET type_secu_sub = COALESCE(NULLIF(type_secu_sub, ''), 'StockIndex.SH'),
           provider      = COALESCE(NULLIF(provider, ''), 'FTMM'),
           remark        = COALESCE(NULLIF(remark, ''),
                                    '20261030-SupabasePG-finv_quote_collect_futu_view批量导入'),
           gmt_update    = now()
     WHERE usc = 'SH0985'
       AND (COALESCE(type_secu_sub, '') = ''
            OR COALESCE(provider, '') = ''
            OR COALESCE(remark, '') = '');
    GET DIAGNOSTICS v_updated = ROW_COUNT;
    RAISE NOTICE 'M7  SH0985 补齐字段：% 行', v_updated;

    -- 收敛校验
    IF EXISTS (SELECT 1 FROM public.finv_security
                WHERE usc = '000680' AND security_type = 'StockIndex') THEN
        RAISE EXCEPTION '000680 仍为错位记录，回滚';
    END IF;
    IF (SELECT count(*) FROM public.finv_security
         WHERE usc = 'SH0680' AND security_code = '000680'
           AND market_code = '1112' AND COALESCE(type_secu_sub, '') <> ''
           AND COALESCE(provider, '') <> '') <> 1 THEN
        RAISE EXCEPTION 'SH0680 互校未完成，回滚';
    END IF;

    RAISE NOTICE '完成：备份 % 行，三条记录修正完毕', v_backup;
END $$;

COMMIT;

-- 执行后核对
-- SELECT usc, security_code, market_code, exchange_code, security_type,
--        security_name, security_name_cn, type_secu_sub, provider, remark
--   FROM public.finv_security WHERE usc IN ('000680','SH0680','SH0985') ORDER BY usc;
