-- =====================================================================
-- V300111：本地独有证券补发 finv_security + Prod 000985 错位纠正（需求 10945）
--
-- 背景：
--   · 899050 / AMAT / MRVL / NOW / PLTR / SMCI 只有 cdeu.finv_quote_cdeu_usc
--     登记行（V300102 与 DiscoverUsc），public.finv_security 里没有记录，
--     Prod 同样查不到 → 无法参与 RefreshSecurityMetadata / 主数据页面维护。
--     其中 899050 在 V300062 种子里有行，但被唯一约束
--     uq_finv_security_exchange_code (exchange_code, security_code) 挡掉：
--     旧别名行 BSE50I 已占用 (13, '899050')。
--   · Prod 的 000985 是错位记录（V300100 的 M5 挂账未处理）：内容写成
--     中证全指（StockIndex/SH），实际 000985 是深市个股「大庆华科」，
--     中证全指的 usc 是 SH0985。
--
-- 处理：
--   1) 备份旧别名行 BSE50I，并把它的 security_code 改为 'BSE50I'，释放
--      (13, '899050') 唯一键（不删除、不合并任何行）。
--   2) 插入 6 只证券的 finv_security 初始数据（flag_enable='1'，
--      ON CONFLICT DO NOTHING）。
--   3) M5：备份 Prod 的 000985 错位行后逐字段纠正为「大庆华科 / CN / SZ / Stock」。
--   4) 同步 cdeu.finv_quote_cdeu_usc：000985 纠正为个股，SH0985（中证全指）
--      补齐登记（缺则插入，只补不覆盖）。
--
-- 数据来源：LocalDocker 现网值（usi 表元数据来自 V300102 / V300096 / V300097）、
--   V300062 种子的 899050 行、公开上市日（需求 10925 已固化）。
--   中文名上游无来源，security_name_cn 暂用英文全称，不编造译名。
--
-- 幂等：重复执行影响 0 行。
-- =====================================================================

BEGIN;

DO $$
DECLARE
    c_reason  CONSTANT text := '需求 10945：本地独有证券补发 finv_security、Prod 000985 错位纠正';
    v_backup  bigint := 0;
    v_ins     bigint := 0;
    v_m5      bigint := 0;
    v_bad     text;
BEGIN
    -- -----------------------------------------------------------------
    -- 1. 旧别名行 BSE50I 释放 (exchange_code, security_code) 唯一键
    -- -----------------------------------------------------------------
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
     WHERE s.usc IN ('BSE50I', '000985')
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_backup = ROW_COUNT;

    UPDATE public.finv_security
       SET security_code = 'BSE50I', gmt_update = now()
     WHERE usc = 'BSE50I'
       AND security_code IS DISTINCT FROM 'BSE50I';

    -- -----------------------------------------------------------------
    -- 2. 6 只证券的初始数据
    --    列顺序同 V300062：
    --    usc, exchange_code, market_code, security_type, type_secu_sub,
    --    security_code, security_name, security_name_cn, security_name_full,
    --    currency_type, provider, remark, init_date, tz, timezone, flag_enable
    -- -----------------------------------------------------------------
    INSERT INTO public.finv_security
        (usc, exchange_code, market_code, security_type, type_secu_sub,
         security_code, security_name, security_name_cn, security_name_full,
         currency_type, provider, remark, init_date, tz, timezone, flag_enable)
    VALUES
        ('899050', 13, 1130, 'StockIndex', 'StockIndex.BJ', '899050',
         '北证50成份指数', '北证50成份指数', '北证50成份指数', 'CNY', 'FTMM', c_reason,
         20000000, '+08:00', 'Asia/Shanghai', '1'),
        ('AMAT', 30, 1315, 'Stock', 'Stock.US', 'AMAT',
         'Applied Materials, Inc.', 'Applied Materials, Inc.', 'Applied Materials, Inc.',
         'USD', 'FTMM', c_reason, 19721012, '-04:00', 'America/New_York', '1'),
        ('MRVL', 30, 1315, 'Stock', 'Stock.US', 'MRVL',
         'Marvell Technology, Inc.', 'Marvell Technology, Inc.', 'Marvell Technology, Inc.',
         'USD', 'FTMM', c_reason, 20000627, '-04:00', 'America/New_York', '1'),
        ('NOW', 30, 1316, 'Stock', 'Stock.US', 'NOW',
         'ServiceNow, Inc.', 'ServiceNow, Inc.', 'ServiceNow, Inc.',
         'USD', 'FTMM', c_reason, 20120629, '-04:00', 'America/New_York', '1'),
        ('PLTR', 30, 1315, 'Stock', 'Stock.US', 'PLTR',
         'Palantir Technologies Inc.', 'Palantir Technologies Inc.', 'Palantir Technologies Inc.',
         'USD', 'FTMM', c_reason, 20200930, '-04:00', 'America/New_York', '1'),
        ('SMCI', 30, 1315, 'Stock', 'Stock.US', 'SMCI',
         'Super Micro Computer, Inc.', 'Super Micro Computer, Inc.', 'Super Micro Computer, Inc.',
         'USD', 'FTMM', c_reason, 20070329, '-04:00', 'America/New_York', '1')
    ON CONFLICT DO NOTHING;
    GET DIAGNOSTICS v_ins = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 3. M5：Prod 000985 错位记录纠正为深市个股「大庆华科」
    --    只动错位那条（类型/市场码/交易所不符的记录），正确记录不碰。
    -- -----------------------------------------------------------------
    UPDATE public.finv_security
       SET exchange_code      = 12,
           security_type      = 'Stock',
           security_name      = '大庆华科',
           security_name_cn   = '大庆华科',
           security_name_full = '大庆华科',
           market_code        = 1120,
           type_secu_sub      = 'Stock.SZ',
           provider           = COALESCE(NULLIF(provider, ''), 'FTMM'),
           flag_enable        = '1',
           gmt_update         = now()
     WHERE usc = '000985'
       AND (security_type IS DISTINCT FROM 'Stock'
            OR market_code   IS DISTINCT FROM 1120
            OR exchange_code IS DISTINCT FROM 12);
    GET DIAGNOSTICS v_m5 = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 4. 同步登记表：000985 纠正、SH0985 补登记
    -- -----------------------------------------------------------------
    UPDATE cdeu.finv_quote_cdeu_usc u
       SET category   = s.security_type,
           name       = COALESCE(NULLIF(s.security_name_cn, ''), s.security_name),
           region     = COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region),
           market     = NULLIF(m.market_symbol, ''),
           gmt_update = now()
      FROM public.finv_security s
      JOIN public.finv_market m ON m.market_code = s.market_code
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_region rg ON rg.region = e.region
     WHERE u.usc = s.usc
       AND u.usc = '000985'
       AND (u.category IS DISTINCT FROM s.security_type
            OR u.name IS DISTINCT FROM COALESCE(NULLIF(s.security_name_cn, ''), s.security_name)
            OR u.region IS DISTINCT FROM COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, ''), u.region)
            OR u.market IS DISTINCT FROM NULLIF(m.market_symbol, ''));

    INSERT INTO cdeu.finv_quote_cdeu_usc
        (usc, category, day_expect_rows, date_list, flag_auto_discover,
         region, market, provider, name, timezone, currency)
    SELECT s.usc, s.security_type, NULL, NULL, false,
           COALESCE(NULLIF(rg.region, ''), NULLIF(e.region, '')),
           NULLIF(m.market_symbol, ''), NULLIF(s.provider, ''),
           COALESCE(NULLIF(s.security_name_cn, ''), s.security_name),
           COALESCE(NULLIF(s.tz, ''), NULLIF(s.timezone, '')), NULLIF(s.currency_type, '')
      FROM public.finv_security s
      JOIN public.finv_market m ON m.market_code = s.market_code
      LEFT JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
      LEFT JOIN public.finv_region rg ON rg.region = e.region
     WHERE s.usc = 'SH0985'
    ON CONFLICT (usc) DO NOTHING;

    -- -----------------------------------------------------------------
    -- 5. 校验
    -- -----------------------------------------------------------------
    -- 5.1 6 只证券：存在、启用、市场码正确
    SELECT string_agg(v.usc || '=' || COALESCE(s.market_code::text, '缺失'),
                      ', ' ORDER BY v.usc)
      INTO v_bad
      FROM (VALUES ('899050', 1130), ('AMAT', 1315), ('MRVL', 1315),
                   ('NOW', 1316), ('PLTR', 1315), ('SMCI', 1315)) AS v(usc, market_code)
      LEFT JOIN public.finv_security s
             ON s.usc = v.usc AND s.flag_enable = '1' AND s.market_code = v.market_code
     WHERE s.usc IS NULL;
    IF v_bad IS NOT NULL THEN
        RAISE EXCEPTION '证券初始数据未按预期落库：%', v_bad;
    END IF;

    -- 5.2 000985 必须是深市个股，SH0985 必须是中证全指且在登记表里
    --     【全新库容忍（需求 11022）】这两行来自生产库既有数据（C 步骤只做 UPDATE），
    --     全新空库里本就没有 000985/SH0985 记录，无从纠正也无从校验，故跳过；
    --     只要库里出现过这两行（存量库），校验照旧严格。
    IF EXISTS (SELECT 1 FROM public.finv_security WHERE usc IN ('000985', 'SH0985'))
       OR EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc WHERE usc IN ('000985', 'SH0985')) THEN
        IF NOT EXISTS (SELECT 1 FROM public.finv_security
                        WHERE usc = '000985' AND security_type = 'Stock' AND market_code = 1120) THEN
            RAISE EXCEPTION '000985 未纠正为深市个股，回滚';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM public.finv_security
                        WHERE usc = 'SH0985' AND security_type = 'StockIndex') THEN
            RAISE EXCEPTION 'SH0985 主数据缺失，回滚';
        END IF;
        IF NOT EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc
                        WHERE usc = 'SH0985' AND region = 'CN' AND market = 'SH'
                          AND category = 'StockIndex') THEN
            RAISE EXCEPTION 'SH0985 未按 CN/SH/StockIndex 登记，回滚';
        END IF;
        -- 登记行允许「不存在」（全新库的 cdeu 种子没有 000985，本脚本只做纠正不做补登记），
        -- 但一旦存在就必须是纠正后的口径（需求 11022）
        IF EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc
                    WHERE usc = '000985'
                      AND NOT (region = 'CN' AND market = 'SZ'
                               AND category = 'Stock' AND name = '大庆华科')) THEN
            RAISE EXCEPTION '000985 登记行未纠正为 大庆华科/CN/SZ/Stock，回滚';
        END IF;
    ELSE
        RAISE NOTICE '全新库：无 000985 / SH0985 历史行，跳过 M5 纠正校验（需求 11022）';
    END IF;

    RAISE NOTICE '完成：备份 % 行；补发证券 % 行；000985 纠正 % 行；SH0985 登记已就绪',
        v_backup, v_ins, v_m5;
END $$;

COMMIT;

-- 执行后核对
-- SELECT usc, security_type, market_code, exchange_code, security_name_cn, flag_enable, init_date
--   FROM public.finv_security
--  WHERE usc IN ('899050','AMAT','MRVL','NOW','PLTR','SMCI','000985','SH0985','BSE50I')
--  ORDER BY usc;
-- SELECT usc, category, region, market, name FROM cdeu.finv_quote_cdeu_usc
--  WHERE usc IN ('000985','SH0985');
