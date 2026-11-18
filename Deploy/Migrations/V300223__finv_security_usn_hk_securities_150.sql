-- =====================================================================
-- V300223：港股 usn 段位调整——证券 150+5 位、指数 15+6 位（需求 11311）
--
-- 用户 2026-11-18 定稿（覆盖 V300222 的港股证券段）：
--   港股证券（Stock / ETF / Fund） 150 + 5 位数字代码   （00700 → 15000700）
--   港股指数（StockIndex）        15  + 6 位数字指数代码（800000 → 15800000）
--
-- 现状（V300222 实现的段位）：证券 151+5 位（15100000–15199999）。
-- 本迁移把港股证券行整体左移 10 万（151xxxxx → 150xxxxx），指数行不变。
--
-- 真实数据（2026-11-18）：港股证券 3,102 只（Stock 2,855 / ETF 246 / Fund 1），
-- 指数 255 只（代码 800000–800935）；调整后证券段 15000001–15089988、
-- 指数段 15800000–15800935，交集 0。
--
-- 幂等：函数 CREATE OR REPLACE；UPDATE 带 `usn <> 新值` 条件，重复执行影响 0 行。
-- 回滚：把函数里的 150 改回 151，并对港股证券行重算（usn + 100000）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 更新编制函数（港股证券 151 → 150）
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
            THEN 15000000 + p_security_code::integer
        WHEN p_exchange_flag = 'HK'
             AND p_security_type = 'StockIndex'
             AND p_security_code ~ '^[0-9]{6}$'
            THEN 15000000 + p_security_code::integer
    END;
$$;

COMMENT ON FUNCTION public.f_finv_security_usn(text, text, text) IS
    'usn 编制规则（需求 11307/11311）：沪 11+6 / 深 12+6 / 京 13+6 / 港股证券 150+5 / 港股指数 15+6；无法编制返回 NULL';

-- ---------------------------------------------------------------------
-- 2. 重算港股证券行（151xxxxx → 150xxxxx；指数行不变）
-- ---------------------------------------------------------------------
UPDATE public.finv_security s
   SET usn = public.f_finv_security_usn(e.exchange_flag, s.security_type, s.security_code)
  FROM public.finv_exchange e
 WHERE e.exchange_code = s.exchange_code
   AND e.exchange_flag = 'HK'
   AND s.security_type IN ('Stock', 'ETF', 'Fund')
   AND s.security_code ~ '^[0-9]{5}$'
   AND s.usn IS DISTINCT FROM public.f_finv_security_usn(e.exchange_flag, s.security_type, s.security_code);

-- ---------------------------------------------------------------------
-- 3. 冲突检测（同一 usn 对应两个 usc → 整事务回滚）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_usn integer;
    v_usc text;
BEGIN
    SELECT usn, string_agg(usc, ', ') INTO v_usn, v_usc
      FROM public.finv_security
     WHERE usn IS NOT NULL
     GROUP BY usn
    HAVING count(*) > 1
     LIMIT 1;
    IF v_usn IS NOT NULL THEN
        RAISE EXCEPTION 'V300223 校验失败：usn=% 对应多个 usc（%）', v_usn, v_usc;
    END IF;
END $$;

COMMIT;

-- ---------------------------------------------------------------------
-- 4. 执行结果概览
-- ---------------------------------------------------------------------
SELECT e.exchange_flag AS flag, s.security_type AS type, count(*) AS rows,
       count(*) FILTER (WHERE s.usn IS NOT NULL) AS usn_filled,
       min(s.usn) AS usn_min, max(s.usn) AS usn_max
  FROM public.finv_security s
  JOIN public.finv_exchange e ON e.exchange_code = s.exchange_code
 WHERE e.exchange_flag = 'HK'
 GROUP BY 1, 2
 ORDER BY 1, 2;
