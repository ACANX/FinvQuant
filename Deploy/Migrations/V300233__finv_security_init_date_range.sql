-- =====================================================================
-- V300233：放宽 finv_security.init_date 的取值下限（需求 11348）
--
-- 背景：原约束 finv_security_init_date_check 为
--         init_date BETWEEN 19500101 AND 21001231
--       下限 1950-01-01 过窄：美股存在上市时间超过 200 年的公司
--       （如 Bank of New York，1784 年成立；纽约证券交易所 1792 年成立），
--       这类证券的上市日期无法录入，会被 23514 CHECK 违反拒绝。
--
-- 调整：下限放宽到 10000101（即 1000-01-01），上限保持 21001231。
--       下限取 1000 年仅为挡住明显误输入（如少写一位变成 6 位数字），
--       实际业务日期不会早于 1600 年；占位值 20000000 仍落在区间内。
--
-- 影响面：只放宽下限，原有数据（含 23736 行占位值 20000000）全部仍合法，
--         无需回填、无需改数据。
--
-- 幂等：先 DROP IF EXISTS 再 ADD，可重复执行。
-- 回滚：把下限改回 19500101（注意：若已录入 1950 年以前的上市日期，回滚会失败，
--       需先修正这些行）：
--         ALTER TABLE public.finv_security
--           DROP CONSTRAINT IF EXISTS finv_security_init_date_check;
--         ALTER TABLE public.finv_security
--           ADD CONSTRAINT finv_security_init_date_check
--           CHECK (init_date BETWEEN 19500101 AND 21001231);
-- =====================================================================

BEGIN;

ALTER TABLE public.finv_security
    DROP CONSTRAINT IF EXISTS finv_security_init_date_check;

ALTER TABLE public.finv_security
    ADD CONSTRAINT finv_security_init_date_check
    CHECK (init_date BETWEEN 10000101 AND 21001231);

COMMENT ON COLUMN public.finv_security.init_date IS
    '首次上市交易日期 yyyymmdd；20000000 表示未知/未上市；取值范围 10000101–21001231（V300233 放宽下限，支持 1950 年前上市的老公司）';

COMMIT;
