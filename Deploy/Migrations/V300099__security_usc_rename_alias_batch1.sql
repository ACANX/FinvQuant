-- =====================================================================
-- V300099：证券主数据 usc 助记名改名（第一批）（需求 10927）
--
-- 背景：
--   2026-11-02 定稿的助记名规范统一了上证指数的助记名写法。第一批处理
--   三条，与已完成的 KC50 → STAR50（需求 10926、V300098）同一批：
--
--     旧 usc    新 usc     证券代码   market_code  名称
--     SHI       SHCI       000001     1110         上证综指数
--     A500      CSIA500    000510     1110         中证A500指数
--     HS300I    CSI300     000300     1110         沪深300指数
--
-- 影响面（执行前已核对）：
--   · cdeu.finv_quote_secu_kline_min          三个旧 usc 均 0 行
--   · cdeu.finv_quote_cdeu_usc                三个旧 usc 均 0 行
--   · cdeu.finv_quote_cdeu_quote_maturity_mon 无引用
--   · 三个新 usc 在 public.finv_security 中均未占用
--   · 无外键引用 public.finv_security
--
-- 规范合规（Docs/DevSpec/DbSchemaSpec.md §0.7）：
--   · 改名前整行备份到 archive.finv_security_purge_20261102；
--   · 单事务内「备份 → 改名 → 校验」，校验不符 RAISE EXCEPTION 整体回滚；
--   · 幂等：旧 usc 不存在或新 usc 已被占用时跳过该条，不影响其它条。
--
-- 回滚：见脚本末尾注释。
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
    c_reason CONSTANT text := '需求 10927：上证指数助记名统一，第一批改名';
    m          record;
    v_before   bigint;
    v_backup   bigint;
    v_renamed  bigint;
BEGIN
    FOR m IN
        SELECT * FROM (VALUES
            ('SHI',    'SHCI'),
            ('A500',   'CSIA500'),
            ('HS300I', 'CSI300')
        ) AS t(old_usc, new_usc)
    LOOP
        SELECT count(*) INTO v_before FROM public.finv_security WHERE usc = m.old_usc;

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
         WHERE s.usc = m.old_usc
        ON CONFLICT (usc, src_schema) DO NOTHING;
        GET DIAGNOSTICS v_backup = ROW_COUNT;

        UPDATE public.finv_security s
           SET usc        = m.new_usc,
               gmt_update = now()
         WHERE s.usc = m.old_usc
           AND NOT EXISTS (SELECT 1 FROM public.finv_security t WHERE t.usc = m.new_usc);
        GET DIAGNOSTICS v_renamed = ROW_COUNT;

        IF v_renamed NOT IN (0, v_backup) THEN
            RAISE EXCEPTION '% → %：备份 % 行但改名 % 行，不一致，回滚',
                m.old_usc, m.new_usc, v_backup, v_renamed;
        END IF;

        RAISE NOTICE '% → %：改名前 % 行 / 备份 % 行 / 改名 % 行',
            m.old_usc, m.new_usc, v_before, v_backup, v_renamed;
    END LOOP;

    -- 收敛校验：三个旧 usc 必须已不存在，三个新 usc 必须各存在 1 行
    IF EXISTS (SELECT 1 FROM public.finv_security WHERE usc IN ('SHI', 'A500', 'HS300I')) THEN
        RAISE EXCEPTION '仍有旧 usc 存在，回滚';
    END IF;
    IF (SELECT count(*) FROM public.finv_security WHERE usc IN ('SHCI', 'CSIA500', 'CSI300')) <> 3 THEN
        RAISE EXCEPTION '新 usc 未全部就位，回滚';
    END IF;

    RAISE NOTICE '完成：三条助记名改名全部就位';
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 执行后核对
-- ---------------------------------------------------------------------
-- SELECT usc, security_code, market_code, security_type, security_name_cn
--   FROM public.finv_security
--  WHERE usc IN ('SHCI','CSIA500','CSI300','SHI','A500','HS300I')
--  ORDER BY usc;
--
-- 回滚（先确认备份存在，再改回）：
-- UPDATE public.finv_security SET usc = 'SHI'
--  WHERE usc = 'SHCI'
--    AND EXISTS (SELECT 1 FROM archive.finv_security_purge_20261102
--                 WHERE usc = 'SHI' AND reason LIKE '需求 10927%');
-- UPDATE public.finv_security SET usc = 'A500'
--  WHERE usc = 'CSIA500'
--    AND EXISTS (SELECT 1 FROM archive.finv_security_purge_20261102
--                 WHERE usc = 'A500' AND reason LIKE '需求 10927%');
-- UPDATE public.finv_security SET usc = 'HS300I'
--  WHERE usc = 'CSI300'
--    AND EXISTS (SELECT 1 FROM archive.finv_security_purge_20261102
--                 WHERE usc = 'HS300I' AND reason LIKE '需求 10927%');
