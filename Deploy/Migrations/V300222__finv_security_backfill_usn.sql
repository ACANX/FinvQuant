-- =====================================================================
-- V300222：public.finv_security 按内部规则生成并回填 usn（需求 11307）
--
-- 编制规则（用户 2026-11-18 定稿）：
--   沪市 11 + 6 位数字证券代码         （600000 → 11600000）
--   深市 12 + 6 位数字证券代码         （000001 → 12000001）
--   京市 13 + 6 位数字证券代码         （920000 → 13920000）
--   港股证券 151 + 5 位数字证券代码    （Stock / ETF / Fund；00700 → 15100700）
--   港股指数 15 + 6 位数字指数代码     （800000 → 15800000）
--   其余（美股 / 加密 / 外汇 / 期货 / 字母代码 / 非纯数字代码）不生成，保持 NULL。
--
-- 港股分段说明（真实数据验证，2026-11-18）：
--   证券段 15100000–15199999、指数段 15000000–15999999；
--   现有 3,102 只港股证券 usn 15100001–15189988、255 只港股指数 usn 15800000–15800935，
--   交集 0 行（指数代码均为 800xxx，首位 8，不会落进 151 段）。
--
-- 沪市旧形态：security_code 形如 SH000300（带市场前缀）的行**不归一、不生成 usn**。
--   原因：SH000300 与 CSI300（000300）是同一指数的两行存量重复，归一后都会得到 11000300；
--   留空等主数据合并后再补，避免同一 usn 对应两个 usc。
--
-- 幂等：只更新 usn IS NULL 的行；函数 IMMUTABLE，重复执行影响 0 行。
-- 回滚：UPDATE public.finv_security SET usn = NULL;（usn 原本全部为 NULL）
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 编制函数（供回填与后续写入复用）
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION public.f_finv_security_usn(
    p_exchange_flag text,
    p_security_type text,
    p_security_code text
) RETURNS integer
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE
        WHEN p_exchange_flag = 'SH' AND p_security_code ~ '^[0-9]{6}$'
            THEN ('11' || p_security_code)::integer
        WHEN p_exchange_flag = 'SZ' AND p_security_code ~ '^[0-9]{6}$'
            THEN ('12' || p_security_code)::integer
        WHEN p_exchange_flag = 'BJ' AND p_security_code ~ '^[0-9]{6}$'
            THEN ('13' || p_security_code)::integer
        WHEN p_exchange_flag = 'HK'
             AND p_security_type IN ('Stock', 'ETF', 'Fund')
             AND p_security_code ~ '^[0-9]{5}$'
            THEN 15100000 + p_security_code::integer
        WHEN p_exchange_flag = 'HK'
             AND p_security_type = 'StockIndex'
             AND p_security_code ~ '^[0-9]{6}$'
            THEN 15000000 + p_security_code::integer
    END;
$$;

COMMENT ON FUNCTION public.f_finv_security_usn(text, text, text) IS
    'usn 编制规则（需求 11307）：沪 11+6 / 深 12+6 / 京 13+6 / 港股证券 151+5 / 港股指数 15+6；无法编制返回 NULL';

-- ---------------------------------------------------------------------
-- 2. 回填（只填 NULL，不覆盖已有值）
-- ---------------------------------------------------------------------
UPDATE public.finv_security s
   SET usn = public.f_finv_security_usn(e.exchange_flag, s.security_type, s.security_code)
  FROM public.finv_exchange e
 WHERE e.exchange_code = s.exchange_code
   AND s.usn IS NULL
   AND public.f_finv_security_usn(e.exchange_flag, s.security_type, s.security_code) IS NOT NULL;

-- ---------------------------------------------------------------------
-- 3. 冲突检测（同一 usn 对应两个 usc → 整事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_usn   integer;
    v_usc   text;
BEGIN
    SELECT usn, string_agg(usc, ', ') INTO v_usn, v_usc
      FROM public.finv_security
     WHERE usn IS NOT NULL
     GROUP BY usn
    HAVING count(*) > 1
     LIMIT 1;
    IF v_usn IS NOT NULL THEN
        RAISE EXCEPTION 'V300222 校验失败：usn=% 对应多个 usc（%）', v_usn, v_usc;
    END IF;
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 4. 执行结果概览（便于迁移日志核对）
-- ---------------------------------------------------------------------
SELECT e.exchange_flag                                   AS flag,
       s.security_type                                    AS type,
       count(*)                                           AS rows,
       count(*) FILTER (WHERE s.usn IS NOT NULL)          AS usn_filled,
       min(s.usn)                                         AS usn_min,
       max(s.usn)                                         AS usn_max
  FROM public.finv_security s
  JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
 GROUP BY 1, 2
 ORDER BY 1, 2;
