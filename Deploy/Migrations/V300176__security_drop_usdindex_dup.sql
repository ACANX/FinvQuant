-- =====================================================================
-- FinvQuant PostgreSQL V300176：删除 finv_security 的重复垃圾行 USDindex（需求 11092）
--
-- 用户 2026-11-10 明确要求：删除 public.finv_security 里的 `USDindex`（小写 i）这一行。
--
-- 依据（删前核查）：
--   · 权威写法是 `USDIndex`（大写 I，需求 11091 已确认在册，Forex / CIX / market 1253）；
--   · `USDindex` 行：name='USD'、type_secu_sub 为空、provider / remark 均为空，
--     且**全库零引用**——键表、采集表、指标表、finv_fxcer、K 线、实时行情、cdeu.usc 均 0 行；
--   · 因此删除不产生悬空引用，也不影响任何页面数据。
--
-- 归档：删前整行写入既有 archive.finv_security_purge_20261110（未新建表）。
-- 幂等：重跑删除 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾 RAISE EXCEPTION 收敛校验，失败整体回滚。
-- =====================================================================

BEGIN;

-- 先确认没有引用，再归档、再删（任一条件不满足即中止）
DO $$
DECLARE
    v_refs BIGINT;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM public.finv_security WHERE usc = 'USDindex') THEN
        RAISE NOTICE 'V300176：USDindex 已不存在，跳过';
        RETURN;
    END IF;

    SELECT (SELECT count(*) FROM futu.finv_futu_secu_key WHERE usc = 'USDindex' OR ftmm_symbol = 'USDindex')
         + (SELECT count(*) FROM futu.finv_futu_quote_collect WHERE usc = 'USDindex')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_indicator WHERE usc = 'USDindex')
         + (SELECT count(*) FROM public.finv_fxcer WHERE usc = 'USDindex')
         + (SELECT count(*) FROM cdeu.finv_quote_secu_kline_day WHERE usc = 'USDindex')
         + (SELECT count(*) FROM cdeu.finv_cdeu_secu_realtime_stock WHERE usc = 'USDindex')
         + (SELECT count(*) FROM cdeu.finv_quote_cdeu_usc WHERE usc = 'USDindex')
      INTO v_refs;
    IF v_refs <> 0 THEN
        RAISE EXCEPTION 'V300176 中止：USDindex 仍被 % 行引用，不能删除', v_refs;
    END IF;
END $$;

INSERT INTO archive.finv_security_purge_20261110
       (usc, exchange_code, security_type, security_code, security_name, security_name_cn,
        security_name_full, currency_type, init_date, timezone, tz, gmt_create, gmt_update,
        flag_enable, market_code, type_secu_sub, provider, remark, src_table, reason, backup_at)
SELECT s.usc, s.exchange_code, s.security_type, s.security_code, s.security_name, s.security_name_cn,
       s.security_name_full, s.currency_type, s.init_date, s.timezone, s.tz, s.gmt_create, s.gmt_update,
       s.flag_enable, s.market_code, s.type_secu_sub, s.provider, s.remark,
       'public.finv_security',
       '需求 11092 删除：与 USDIndex 重复的历史垃圾行（name=USD，无 provider/remark，零引用）',
       now()
  FROM public.finv_security s
 WHERE s.usc = 'USDindex';

DELETE FROM public.finv_security WHERE usc = 'USDindex';

DO $$
DECLARE
    v_left     BIGINT;
    v_arch     BIGINT;
    v_rows     BIGINT;
    v_canonical BIGINT;
BEGIN
    SELECT count(*) INTO v_left     FROM public.finv_security WHERE usc = 'USDindex';
    SELECT count(*) INTO v_arch     FROM archive.finv_security_purge_20261110
                                     WHERE usc = 'USDindex' AND reason LIKE '需求 11092%';
    SELECT count(*) INTO v_rows     FROM public.finv_security;
    SELECT count(*) INTO v_canonical FROM public.finv_security WHERE usc = 'USDIndex';

    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300176 未收敛：USDindex 仍在册';
    END IF;
    IF v_arch <> 1 THEN
        RAISE EXCEPTION 'V300176 未收敛：归档行数 %（应为 1）', v_arch;
    END IF;
    -- 权威写法 USDIndex 的存在性只通报：本迁移只负责删除小写 i 的重复行，
    -- 某些环境（如按需重建的库）可能本来就没有它，此时不该让服务启动失败。
    IF v_canonical = 0 THEN
        RAISE NOTICE 'V300176：权威写法 USDIndex 不在册（本环境无该证券），仅通报';
    END IF;

    RAISE NOTICE 'V300176 完成：删除 USDindex 1 行（已归档），finv_security 共 % 行，USDIndex 保留', v_rows;
END $$;

COMMIT;
