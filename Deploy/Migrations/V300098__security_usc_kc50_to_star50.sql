-- =====================================================================
-- V300098：证券主数据 usc 改名 KC50 → STAR50（需求 10926）
--
-- 背景：
--   科创50 指数在 public.finv_security 里有两个 usc：
--     · KC50     —— 助记名（旧写法）
--     · SH0688   —— 4 位形态（新写法）
--   按 2026-11-02 定稿的助记名规范，科创50 的助记名统一为 STAR50，
--   与 SHCI（上证综指）、CSIA500（中证A500）、CSI300（沪深300）同一批。
--
-- 影响面（执行前已核对）：
--   · public.finv_security      —— 1 行，usc='KC50'（security_code='000688'，market_code='1110'）
--   · cdeu.finv_quote_cdeu_usc  —— 0 行
--   · cdeu.finv_quote_secu_kline_min —— 0 行
--   · cdeu.finv_quote_cdeu_quote_maturity_mon —— 0 行
--   无外键引用 public.finv_security，改名不牵连其它表。
--
-- 规范合规（Docs/DevSpec/DbSchemaSpec.md §0.7）：
--   · 改名前整行备份到 archive.finv_security_purge_20261102，带 src_schema / reason / backup_at；
--   · 单事务内「备份 → 改名 → 校验」，校验不符 RAISE EXCEPTION 整体回滚；
--   · 脚本幂等：KC50 不存在或 STAR50 已存在时影响 0 行。
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
    c_reason CONSTANT text :=
        '需求 10926：科创50 助记名由 KC50 统一为 STAR50（旧写法保留为别名，不再作为 usc）';
    v_before   bigint;
    v_backup   bigint;
    v_renamed  bigint;
BEGIN
    SELECT count(*) INTO v_before FROM public.finv_security WHERE usc = 'KC50';

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
     WHERE s.usc = 'KC50'
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_backup = ROW_COUNT;

    -- 目标 usc 未占用时才改名，避免覆盖已有记录
    UPDATE public.finv_security s
       SET usc        = 'STAR50',
           gmt_update = now()
     WHERE s.usc = 'KC50'
       AND NOT EXISTS (SELECT 1 FROM public.finv_security t WHERE t.usc = 'STAR50');
    GET DIAGNOSTICS v_renamed = ROW_COUNT;

    -- 收敛校验
    IF v_renamed NOT IN (0, v_backup) THEN
        RAISE EXCEPTION '备份 % 行但改名 % 行，不一致，回滚', v_backup, v_renamed;
    END IF;
    IF EXISTS (SELECT 1 FROM public.finv_security WHERE usc = 'STAR50')
       AND EXISTS (SELECT 1 FROM public.finv_security WHERE security_code = '000688' AND usc = 'KC50') THEN
        RAISE EXCEPTION 'KC50 与 STAR50 同时存在，回滚';
    END IF;

    RAISE NOTICE '完成：改名前 % 行，备份 % 行，改名 % 行；STAR50 现存在 % 行',
        v_before, v_backup, v_renamed,
        (SELECT count(*) FROM public.finv_security WHERE usc = 'STAR50');
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 执行后核对
-- ---------------------------------------------------------------------
-- SELECT usc, security_code, market_code, security_type,
--        COALESCE(security_name_cn, security_name) AS name
--   FROM public.finv_security WHERE usc IN ('KC50','STAR50','SH0688');
--
-- 回滚（先确认备份存在，再改回）：
-- UPDATE public.finv_security SET usc = 'KC50', gmt_update = now()
--  WHERE usc = 'STAR50'
--    AND EXISTS (SELECT 1 FROM archive.finv_security_purge_20261102
--                 WHERE usc = 'KC50' AND reason LIKE '需求 10926%');
