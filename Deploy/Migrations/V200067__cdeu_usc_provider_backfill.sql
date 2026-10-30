-- =====================================================================
-- FinvQuant PostgreSQL V200067：补全 cdeu.finv_quote_cdeu_usc.provider（需求 10832）
--
-- 背景（用户实测「上个月有数据却导不出来」的直接原因）：
--   导出条件要求 cdeu.finv_quote_cdeu_usc.provider 非空，但实测 39 个证券的
--   provider 全部为空（finv_security.provider 对这 39 个证券同样为空），
--   于是所有 (usc, biz_month) 都被判为「元数据不全」，候选恒为 0，
--   流程跑完没有任何动态 JOB，日志也只说「候选 0 个」。
--
-- 取值依据（不编造）：
--   · 数据源就是富途：futu.finv_futu_quote_collect 覆盖这 39 个证券；
--   · 历史 202 条已导出文件的文件名里，provider 段全部为 FTMM
--     （形如 USA_US-NSDQ_NVDA_Min_FTMM_Mon_202607_Base.mvsv）。
--
-- 只补空值，且只补「有富途采集记录」的证券，不覆盖人工设置、也不给非富途来源的证券填值。
-- 代码侧同一兜底逻辑已加入 Store.BackfillUscDefaults（第 5.6 步），二者口径一致。
-- =====================================================================

BEGIN;

UPDATE cdeu.finv_quote_cdeu_usc u
   SET provider   = 'FTMM',
       gmt_update = now()
 WHERE u.provider IS NULL
   AND EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect f WHERE f.usc = u.usc);

DO $$
DECLARE
    v_missing int;
BEGIN
    SELECT count(*) INTO v_missing
      FROM cdeu.finv_quote_cdeu_usc u
     WHERE u.provider IS NULL
       AND EXISTS (SELECT 1 FROM futu.finv_futu_quote_collect f WHERE f.usc = u.usc);
    IF v_missing <> 0 THEN
        RAISE EXCEPTION 'V200067 后置校验失败：仍有 % 个有富途采集记录的证券缺 provider', v_missing;
    END IF;
    RAISE NOTICE 'V200067 完成：有富途采集记录的证券 provider 已补全为 FTMM';
END $$;

COMMIT;
