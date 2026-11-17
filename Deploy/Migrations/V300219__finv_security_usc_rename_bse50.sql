-- =====================================================================
-- V300219：北证50（899050）的 usc 由 `899050` 更名为 `BSE50`（需求 11294）
--
-- 背景与决策（用户 2026-11-18 确认）：
--   · `public.finv_security` 里 899050 有两条相关行：
--       - 旧别名行：usc='BSE50I'，security_code='BSE50I'（V300111 改过代码以释放唯一键，行保留）
--       - 规范行  ：usc='899050'，security_code='899050'（V300111 插入，flag_enable='1'）
--   · 本次把规范行的 usc 改为 `BSE50`（security_code 保持交易所代码 `899050`），
--     旧别名行 `BSE50I` **保留不动**（usc 与 security_code 均为 `BSE50I`）；
--     两行不能同时用 `BSE50`（主键冲突），也不删任何行。
--
-- 改名范围（用户确认）：
--   1) public.finv_security（主数据）
--   2) futu.finv_futu_mapping_security.finv_usc
--   3) cdeu.finv_quote_cdeu_usc（采集登记）
--   4) cdeu.finv_quote_secu_realtime（跨 provider 快照）
--   其余含 usc 列的表（历史行情等）**只巡检不改**，输出 NOTICE 由后续需求决定。
--
-- 备份：改名前整行备份到 archive.finv_security_purge_20261118（含 BSE50I 与 899050 两行）。
-- 幂等：每一步都带「目标不存在 / 值不相等」条件，重复执行影响 0 行。
-- 回滚：从 archive.finv_security_purge_20261118 取回 usc/security_code 原值即可；
--       引用表按同一映射反向更新（本期只改 4 张表，人工回滚可控）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 0. 备份表（沿用 V300098 / V300111 的结构）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_security_purge_20261118 (
    LIKE public.finv_security INCLUDING DEFAULTS,
    src_schema text        NOT NULL DEFAULT 'public',
    reason     text        NOT NULL,
    backup_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (usc, src_schema)
);

DO $$
DECLARE
    c_reason CONSTANT text := '需求 11294：北证50（899050）usc 由 899050 更名为 BSE50';
    v_backup   bigint := 0;
    v_rename   bigint := 0;
    v_futu     bigint := 0;
    v_registry bigint := 0;
    v_snapshot bigint := 0;
    v_target   bigint := 0;
    v_left     bigint := 0;
    v_bad      bigint := 0;
    v_code     text;
    r          record;
    n          bigint;
BEGIN
    -- -----------------------------------------------------------------
    -- 1. 备份（BSE50I 旧别名行 + 899050 规范行）
    -- -----------------------------------------------------------------
    -- 列清单与 V300111 一致（LIKE 会复制 NOT NULL，security_type 必须显式带上）
    INSERT INTO archive.finv_security_purge_20261118
        (usc, exchange_code, security_type, security_code, security_name,
         security_name_cn, security_name_full, currency_type, init_date,
         timezone, tz, gmt_create, gmt_update, flag_enable, market_code,
         type_secu_sub, provider, remark, src_schema, reason)
    SELECT s.usc, s.exchange_code, s.security_type, s.security_code, s.security_name,
           s.security_name_cn, s.security_name_full, s.currency_type, s.init_date,
           s.timezone, s.tz, s.gmt_create, s.gmt_update, s.flag_enable, s.market_code,
           s.type_secu_sub, s.provider, s.remark, 'public', c_reason
      FROM public.finv_security s
     WHERE s.usc IN ('899050', 'BSE50I')
    ON CONFLICT (usc, src_schema) DO NOTHING;
    GET DIAGNOSTICS v_backup = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 2. 主数据改名：899050 → BSE50（目标不存在才改，避免主键冲突）
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_target FROM public.finv_security WHERE usc = 'BSE50';
    IF v_target > 0 THEN
        RAISE NOTICE 'V300219：usc=BSE50 已存在（%）行，跳过主数据改名', v_target;
    ELSE
        UPDATE public.finv_security
           SET usc = 'BSE50', gmt_update = now()
         WHERE usc = '899050';
        GET DIAGNOSTICS v_rename = ROW_COUNT;
    END IF;

    -- -----------------------------------------------------------------
    -- 3. futu 映射表（finv_usc 无唯一约束，直接改）
    -- -----------------------------------------------------------------
    UPDATE futu.finv_futu_mapping_security
       SET finv_usc = 'BSE50', gmt_update = now()
     WHERE finv_usc = '899050';
    GET DIAGNOSTICS v_futu = ROW_COUNT;

    -- -----------------------------------------------------------------
    -- 4. cdeu 采集登记表（主键 usc；目标已存在则跳过并提示人工合并）
    -- -----------------------------------------------------------------
    IF EXISTS (SELECT 1 FROM cdeu.finv_quote_cdeu_usc WHERE usc = 'BSE50') THEN
        RAISE NOTICE 'V300219：cdeu.finv_quote_cdeu_usc 已有 usc=BSE50，跳过（请人工核对 899050 行）';
    ELSE
        UPDATE cdeu.finv_quote_cdeu_usc
           SET usc = 'BSE50', gmt_update = now()
         WHERE usc = '899050';
        GET DIAGNOSTICS v_registry = ROW_COUNT;
    END IF;

    -- -----------------------------------------------------------------
    -- 5. 跨 provider 快照表（主键 usc；目标已存在则跳过并提示）
    -- -----------------------------------------------------------------
    IF EXISTS (SELECT 1 FROM cdeu.finv_quote_secu_realtime WHERE usc = 'BSE50') THEN
        RAISE NOTICE 'V300219：cdeu.finv_quote_secu_realtime 已有 usc=BSE50，跳过（请人工核对 899050 行）';
    ELSE
        UPDATE cdeu.finv_quote_secu_realtime
           SET usc = 'BSE50'
         WHERE usc = '899050';
        GET DIAGNOSTICS v_snapshot = ROW_COUNT;
    END IF;

    -- -----------------------------------------------------------------
    -- 6. 巡检：其余含 usc 列的表只统计不改（历史行情改主键风险高）
    -- -----------------------------------------------------------------
    FOR r IN
        SELECT c.table_schema, c.table_name
          FROM information_schema.columns c
          JOIN information_schema.tables t
            ON t.table_schema = c.table_schema AND t.table_name = c.table_name
         WHERE c.column_name = 'usc'
           AND t.table_type = 'BASE TABLE'
           AND NOT (c.table_schema = 'public'
                    AND c.table_name = 'finv_security')
           AND NOT (c.table_schema = 'cdeu'
                    AND c.table_name IN ('finv_quote_cdeu_usc', 'finv_quote_secu_realtime'))
         ORDER BY 1, 2
    LOOP
        EXECUTE format('SELECT count(*) FROM %I.%I WHERE usc IN (%L, %L)',
                       r.table_schema, r.table_name, '899050', 'BSE50') INTO n;
        IF n > 0 THEN
            RAISE NOTICE 'V300219 巡检：%.% 有 % 行 usc 为 899050/BSE50（本期不改，需人工评估）',
                         r.table_schema, r.table_name, n;
        END IF;
    END LOOP;

    -- -----------------------------------------------------------------
    -- 7. 收敛校验（不满足整事务回滚）
    -- -----------------------------------------------------------------
    SELECT count(*) INTO v_left FROM public.finv_security WHERE usc = '899050';
    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300219 校验失败：public.finv_security 仍有 % 行 usc=899050', v_left;
    END IF;

    SELECT count(*) INTO v_target FROM public.finv_security WHERE usc = 'BSE50';
    IF v_target <> 1 THEN
        RAISE EXCEPTION 'V300219 校验失败：usc=BSE50 期望 1 行，实际 %', v_target;
    END IF;

    SELECT security_code INTO v_code FROM public.finv_security WHERE usc = 'BSE50';
    IF v_code IS DISTINCT FROM '899050' THEN
        RAISE EXCEPTION 'V300219 校验失败：BSE50 的 security_code 期望 899050，实际 %',
            COALESCE(v_code, '<NULL>');
    END IF;

    SELECT count(*) INTO v_bad FROM public.finv_security WHERE usc = 'BSE50I';
    IF v_bad <> 1 THEN
        RAISE EXCEPTION 'V300219 校验失败：旧别名行 usc=BSE50I 期望保留 1 行，实际 %', v_bad;
    END IF;

    SELECT count(*) INTO v_bad FROM futu.finv_futu_mapping_security WHERE finv_usc = '899050';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300219 校验失败：futu 映射仍有 % 行 finv_usc=899050', v_bad;
    END IF;

    SELECT count(*) INTO v_bad FROM cdeu.finv_quote_cdeu_usc WHERE usc = '899050';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300219 校验失败：cdeu.finv_quote_cdeu_usc 仍有 % 行 usc=899050', v_bad;
    END IF;

    SELECT count(*) INTO v_bad FROM cdeu.finv_quote_secu_realtime WHERE usc = '899050';
    IF v_bad <> 0 THEN
        RAISE EXCEPTION 'V300219 校验失败：cdeu.finv_quote_secu_realtime 仍有 % 行 usc=899050', v_bad;
    END IF;

    RAISE NOTICE 'V300219 完成：备份 % 行；主数据改名 % 行；futu % 行；登记表 % 行；快照表 % 行；BSE50I 旧行保留',
        v_backup, v_rename, v_futu, v_registry, v_snapshot;
END $$;

COMMIT;
