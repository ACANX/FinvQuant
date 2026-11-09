-- =====================================================================
-- V300151：BRS 全量备份登记──撤销 public.finv_cer（需求 11067）
--
-- 背景：public.finv_cer 是缺省汇率表 v1，已被 public.finv_fxcer（V300124）取代：
--   · 建表脚本 V300123 已按需求 11021 从版本库删除，新装环境根本不会建这张表；
--   · Go / 前端 0 引用，表格数据停在 2026-11-08 的一次性批量更新，无写入路径。
--   V300149（第一批）按「最近新增 8 张」把老环境残留的这张表也登记了，
--   现确认其已过时，撤销该条备份登记。
--
-- 范围（严格限定）：
--   · 只删 brs.brs_bak_rdb_table_full 中 public.finv_cer 这一行登记；
--   · **不动 public.finv_cer 表本身，也不动它的数据**（删表需单独授权，本迁移不做）；
--   · 其余 44 条登记（含 futu.finv_futu_mapping_* 5 张）保持不动。
--
-- 影响：priority 600 随该行释放，后续批次可复用。
--
-- 幂等：DELETE 命中 0 行即为已撤销，不报错。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验（该行必须已不存在）。
-- =====================================================================

BEGIN;

DELETE FROM brs.brs_bak_rdb_table_full
 WHERE db = 'finv'
   AND schema_name = 'public'
   AND table_name = 'finv_cer';

-- ---------------------------------------------------------------------
-- 收敛校验：public.finv_cer 不得再出现在登记表中；总量应为 44
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_left     bigint;
    v_registered bigint;
BEGIN
    SELECT count(*) INTO v_left
      FROM brs.brs_bak_rdb_table_full
     WHERE db = 'finv' AND schema_name = 'public' AND table_name = 'finv_cer';

    IF v_left <> 0 THEN
        RAISE EXCEPTION 'V300151 校验失败：public.finv_cer 仍留在备份登记中（% 行）', v_left;
    END IF;

    SELECT count(*) INTO v_registered FROM brs.brs_bak_rdb_table_full;

    RAISE NOTICE 'V300151：已撤销 public.finv_cer 的备份登记，全量备份登记表共 % 张', v_registered;
END $$;

COMMIT;
