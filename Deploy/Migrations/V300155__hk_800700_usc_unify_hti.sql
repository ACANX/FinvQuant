-- =====================================================================
-- V300155：HK.800700（恒生科技指数）usc 统一为 HTI（需求 11071）
--
-- 背景：该指数在本地库里存在两种 usc 写法，且有两处「同一证券两行」的重复数据：
--   · public.finv_security         usc='HSTI'（security_code=800700，早期手写种子）
--                                  usc='HTI' （security_code='HTI'，20261030 批量导入，code 值无效）
--   · cdeu.finv_quote_secu_indicator  usc='800700' 与 usc='HTI' 两行，ftmm_symbol 同为 'HK.800700'
--   · futu.finv_futu_secu_key       ftmm_symbol='HK.800700' → usc='HTI'（本迁移执行前即已正确）
--   注意：键表里 usc='HSTI' 的行属于 **US.HSTI（美股，code=HSTI）**，按「US 市场默认 usc=code」
--   它就是 'HSTI'，与本指数无关，**不得改动**（本迁移只处理 market='HK'）。
--
-- 上游现状（2026-11-10 实测）：
--   public.finv_quote_secu_meta: usc='HTI'、ftmm_symbol='HK.800700'、stock_code='800700'
--   public.finv_quote_secu_key : 只剩 ftmm_symbol='HK.800700'（usc='HTI'），'HK.HSTI' 已不存在
--   → 规范值就是 **HTI**，与用户确认结论一致。
--
-- 本迁移动作（严格限定，单事务「备份 → 删除 → 改 usc → 校验」）：
--   ① 备份受影响整行到 archive.*（含 reason / backup_at / src_table，符合 DbSchemaSpec §0.7）
--   ② 删除 public.finv_security 中 security_code='HTI' 的空壳行（usc='HTI'，code 值无效）
--   ③ 删除 cdeu.finv_quote_secu_indicator 中 usc='800700' 的孤儿行（上游 usc 已是 HTI）
--   ④ public.finv_security：usc 'HSTI' → 'HTI'（保留 security_code=800700 那一行）
--   ⑤ futu.finv_futu_secu_key：只把 **market='HK'** 且 usc='HSTI' 的行改成 'HTI'
--     （当前数据无此类行，属规则兜底；US.HSTI 一行不动）
--
-- 不做的事：不动 SH 指数 149 行裸 code（用户明确：生产数据双份，下月自然收敛）；
--   不动其它市场；不删 futu.finv_futu_secu_key 的任何行。
--
-- 幂等：备份 INSERT ... WHERE NOT EXISTS；DELETE/UPDATE 命中 0 行即已处理完；
--   末尾校验用 RAISE EXCEPTION，任一不符整事务回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- ① 备份表（仅建表；列结构跟随源表，额外加来源/原因/时间三列）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS archive.finv_security_purge_20261110
    (LIKE public.finv_security);
ALTER TABLE archive.finv_security_purge_20261110
    ADD COLUMN IF NOT EXISTS src_table text,
    ADD COLUMN IF NOT EXISTS reason    text,
    ADD COLUMN IF NOT EXISTS backup_at timestamptz NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS archive.finv_quote_secu_indicator_purge_20261110
    (LIKE cdeu.finv_quote_secu_indicator);
ALTER TABLE archive.finv_quote_secu_indicator_purge_20261110
    ADD COLUMN IF NOT EXISTS src_table text,
    ADD COLUMN IF NOT EXISTS reason    text,
    ADD COLUMN IF NOT EXISTS backup_at timestamptz NOT NULL DEFAULT now();

-- 备份：将被删除/改写的行整行留档（已存在则跳过，保证幂等）
INSERT INTO archive.finv_security_purge_20261110
SELECT s.*, 'public.finv_security',
       'HK.800700 usc 统一为 HTI：删除 security_code=HTI 的空壳行（需求 11071）', now()
  FROM public.finv_security s
 WHERE s.usc = 'HTI' AND s.security_code = 'HTI'
   AND NOT EXISTS (SELECT 1 FROM archive.finv_security_purge_20261110 a
                    WHERE a.src_table = 'public.finv_security' AND a.usc = 'HTI');

INSERT INTO archive.finv_security_purge_20261110
SELECT s.*, 'public.finv_security',
       'HK.800700 usc 统一为 HTI：改写 usc HSTI→HTI 前留档（需求 11071）', now()
  FROM public.finv_security s
 WHERE s.usc = 'HSTI'
   AND NOT EXISTS (SELECT 1 FROM archive.finv_security_purge_20261110 a
                    WHERE a.src_table = 'public.finv_security' AND a.usc = 'HSTI');

INSERT INTO archive.finv_quote_secu_indicator_purge_20261110
SELECT i.*, 'cdeu.finv_quote_secu_indicator',
       'HK.800700 usc 统一为 HTI：删除 usc=800700 的孤儿行（需求 11071）', now()
  FROM cdeu.finv_quote_secu_indicator i
 WHERE i.usc = '800700'
   AND NOT EXISTS (SELECT 1 FROM archive.finv_quote_secu_indicator_purge_20261110 a
                    WHERE a.src_table = 'cdeu.finv_quote_secu_indicator' AND a.usc = '800700');

-- ---------------------------------------------------------------------
-- ②③ 删除重复行（先删，避免与 ④ 的主键/唯一键冲突）
-- ---------------------------------------------------------------------
DELETE FROM public.finv_security
 WHERE usc = 'HTI' AND security_code = 'HTI';

DELETE FROM cdeu.finv_quote_secu_indicator
 WHERE usc = '800700';

-- ---------------------------------------------------------------------
-- ④⑤ 统一 usc 写法为 HTI
-- ---------------------------------------------------------------------
UPDATE public.finv_security
   SET usc = 'HTI'
 WHERE usc = 'HSTI';

UPDATE futu.finv_futu_secu_key
   SET usc = 'HTI'
 WHERE market = 'HK' AND usc = 'HSTI';

-- ---------------------------------------------------------------------
-- 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_sec      bigint;
    v_sec_code text;
    v_old_sec  bigint;
    v_old_key  bigint;
    v_ind      bigint;
    v_ind_old  bigint;
BEGIN
    -- finv_security：usc='HTI' 必须恰好 1 行，且 security_code 为 800700
    SELECT count(*), max(security_code) INTO v_sec, v_sec_code
      FROM public.finv_security WHERE usc = 'HTI';
    IF v_sec <> 1 OR v_sec_code <> '800700' THEN
        RAISE EXCEPTION 'V300155 校验失败：finv_security 中 usc=HTI 应恰有 1 行且 security_code=800700，实际 % 行 / code=%',
            v_sec, v_sec_code;
    END IF;

    -- 不得再有 usc='HSTI'
    SELECT count(*) INTO v_old_sec FROM public.finv_security WHERE usc = 'HSTI';
    SELECT count(*) INTO v_old_key FROM futu.finv_futu_secu_key WHERE market = 'HK' AND usc = 'HSTI';
    IF v_old_sec <> 0 OR v_old_key <> 0 THEN
        RAISE EXCEPTION 'V300155 校验失败：仍有 HK 口径的 usc=HSTI 残留（finv_security=% / secu_key=%）',
            v_old_sec, v_old_key;
    END IF;

    -- 指标表：HK.800700 只保留 usc='HTI' 一行
    SELECT count(*) FILTER (WHERE usc = 'HTI'), count(*) FILTER (WHERE usc = '800700')
      INTO v_ind, v_ind_old
      FROM cdeu.finv_quote_secu_indicator WHERE ftmm_symbol = 'HK.800700';
    IF v_ind <> 1 OR v_ind_old <> 0 THEN
        RAISE EXCEPTION 'V300155 校验失败：指标表 HK.800700 应为 usc=HTI 1 行、usc=800700 0 行，实际 % / %',
            v_ind, v_ind_old;
    END IF;

    RAISE NOTICE 'V300155：HK.800700 usc 已统一为 HTI（finv_security 1 行 / 指标表 1 行；US.HSTI 未受影响）';
END $$;

COMMIT;
