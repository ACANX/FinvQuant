-- =====================================================================
-- FinvQuant PostgreSQL V100051：合并 finv_dedup_usc_priority 到 finv_dedup_usc（需求 10745）
--
-- 背景：优先级/SLA 原本是独立的稀疏覆盖表（39 个证券里只有 3 个配了），
-- 与 finv_dedup_usc 是一对一的逻辑关联。实际维护时需要两个界面、两个 API、
-- 两次写入，用户决定合并为一张表统一维护。
--
-- 语义保持：
--   priority_rank IS NULL  → 未配置，按 priority_rank=100 兜底
--   sla_days      IS NULL  → 未配置，按 sla_days=5 兜底
--   max_wait_days IS NULL  → 未配置，按 7 兜底
--   三列必须同时为 NULL 或同时非 NULL（组一致性约束）。
--
-- 授权：用户 2026-10-27 明确授权 DROP TABLE dedup.finv_dedup_usc_priority。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：源表存在且数据可迁移
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_src_rows  bigint;
    v_orphan    bigint;
BEGIN
    SELECT count(*) INTO v_src_rows FROM dedup.finv_dedup_usc_priority;

    SELECT count(*) INTO v_orphan
      FROM dedup.finv_dedup_usc_priority p
     WHERE NOT EXISTS (SELECT 1 FROM dedup.finv_dedup_usc u WHERE u.usc = p.usc);
    IF v_orphan > 0 THEN
        RAISE EXCEPTION 'V100051 前置校验失败：% 条优先级找不到对应证券（合并后会丢失）', v_orphan;
    END IF;

    RAISE NOTICE 'V100051 前置校验：待迁移优先级 % 行，孤儿 0 行', v_src_rows;
END $$;

-- ---------------------------------------------------------------------
-- 2. 主表加列（全 NULL 表示未配置）
-- ---------------------------------------------------------------------
ALTER TABLE dedup.finv_dedup_usc
    ADD COLUMN IF NOT EXISTS priority_rank       int,
    ADD COLUMN IF NOT EXISTS sla_days            int,
    ADD COLUMN IF NOT EXISTS max_wait_days       int,
    ADD COLUMN IF NOT EXISTS priority_gmt_update timestamptz;

COMMENT ON COLUMN dedup.finv_dedup_usc.priority_rank IS '归档优先级，数值越小越优先；NULL 表示未配置，按 100 兜底';
COMMENT ON COLUMN dedup.finv_dedup_usc.sla_days IS '收口（seal_date）后多少天内必须完成导出，决定 freeze_date；NULL 表示未配置，按 5 兜底';
COMMENT ON COLUMN dedup.finv_dedup_usc.max_wait_days IS '低优先级最长等待天数，超过后提升排序防饿死；NULL 表示未配置，按 7 兜底';
COMMENT ON COLUMN dedup.finv_dedup_usc.priority_gmt_update IS '优先级配置最后修改时间（与主数据 gmt_update 分离，便于审计）';

-- ---------------------------------------------------------------------
-- 3. 搬数据
-- ---------------------------------------------------------------------
UPDATE dedup.finv_dedup_usc u
   SET priority_rank       = p.priority_rank,
       sla_days            = p.sla_days,
       max_wait_days       = p.max_wait_days,
       priority_gmt_update = p.gmt_update
  FROM dedup.finv_dedup_usc_priority p
 WHERE p.usc = u.usc;

-- ---------------------------------------------------------------------
-- 4. 组一致性约束：三列同时为 NULL 或同时非 NULL
-- ---------------------------------------------------------------------
ALTER TABLE dedup.finv_dedup_usc
    DROP CONSTRAINT IF EXISTS finv_dedup_usc_priority_group_check;
ALTER TABLE dedup.finv_dedup_usc
    ADD CONSTRAINT finv_dedup_usc_priority_group_check CHECK (
        (priority_rank IS NULL AND sla_days IS NULL AND max_wait_days IS NULL)
        OR
        (priority_rank IS NOT NULL AND sla_days IS NOT NULL AND max_wait_days IS NOT NULL)
    );

-- ---------------------------------------------------------------------
-- 5. 收敛校验（删旧表前必须全部通过）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_src_rows   bigint;
    v_migrated   bigint;
    v_partial    bigint;
BEGIN
    SELECT count(*) INTO v_src_rows FROM dedup.finv_dedup_usc_priority;
    SELECT count(*) INTO v_migrated FROM dedup.finv_dedup_usc WHERE priority_rank IS NOT NULL;
    SELECT count(*) INTO v_partial
      FROM dedup.finv_dedup_usc
     WHERE num_nonnulls(priority_rank, sla_days, max_wait_days) NOT IN (0, 3);

    IF v_migrated <> v_src_rows THEN
        RAISE EXCEPTION 'V100051 收敛失败：源表 % 行，主表已配置 % 行，数量不一致', v_src_rows, v_migrated;
    END IF;
    IF v_partial > 0 THEN
        RAISE EXCEPTION 'V100051 收敛失败：% 条优先级配置不完整（三列必须同时有值）', v_partial;
    END IF;

    RAISE NOTICE 'V100051 收敛校验通过：迁移 % 行优先级到 finv_dedup_usc', v_migrated;
END $$;

-- ---------------------------------------------------------------------
-- 6. 删除旧表（用户已授权）
-- ---------------------------------------------------------------------
DROP TABLE dedup.finv_dedup_usc_priority;

-- 最终确认旧表已不存在
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                WHERE n.nspname = 'dedup' AND c.relname = 'finv_dedup_usc_priority'
                  AND c.relkind = 'r') THEN
        RAISE EXCEPTION 'V100051 失败：finv_dedup_usc_priority 仍存在';
    END IF;
    RAISE NOTICE 'V100051 完成：finv_dedup_usc_priority 已合并并删除';
END $$;

COMMIT;
