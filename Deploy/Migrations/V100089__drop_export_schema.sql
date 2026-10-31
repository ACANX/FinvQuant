-- =====================================================================
-- FinvQuant PostgreSQL V100089：删除已空的 export schema
--
-- 决策（ACANX 2026-10-31，用户在会话中明确要求「可以删掉 export schema 了」）：
--   - `export` 域唯一一张表 `finv_quote_secu_kline_min`（导出中转快照）已由
--     V100088 迁入 `cdeu.finv_quote_secu_kline_min_export` 并 DROP 源表；
--   - 该 schema 现无任何对象（迁移内部先校验），属纯空壳，予以删除；
--   - 前置保护：若 schema 内仍有任何表/视图/序列/函数，直接 RAISE EXCEPTION
--     中止并回滚，避免误删未来新增的暂存对象（本迁移只处理「确认为空」的情形）；
--   - 代码侧同步：`internal/database/postgres.go` 的 search_path 已移除 `export`
--     （以及 `V100086` 删除的 `dedup`），注释同步更新；
--   - `DbSchemaSpec` §1 / §3.1 同步登记删除事实。
--
-- 幂等：DROP SCHEMA IF EXISTS；重复执行无副作用。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾收敛校验，不符整体回滚。
-- 前置：V100088（中转表迁入 cdeu、export 表已删）、V100086（dedup 已删）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 前置校验：export schema 必须为空壳（否则拒绝删除）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_objects int;
BEGIN
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'export') THEN
        SELECT count(*) INTO v_objects
          FROM pg_class c
          JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'export';
        IF v_objects <> 0 THEN
            RAISE EXCEPTION 'V100089 中止：export schema 内仍有 % 个对象，先确认归属再删除', v_objects;
        END IF;

        SELECT count(*) INTO v_objects
          FROM pg_proc p
          JOIN pg_namespace n ON n.oid = p.pronamespace
         WHERE n.nspname = 'export';
        IF v_objects <> 0 THEN
            RAISE EXCEPTION 'V100089 中止：export schema 内仍有 % 个函数，先确认归属再删除', v_objects;
        END IF;
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. 删除空的 export schema
-- ---------------------------------------------------------------------
DROP SCHEMA IF EXISTS export;

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'export') THEN
        RAISE EXCEPTION 'V100089 收敛失败：export schema 仍存在';
    END IF;
    RAISE NOTICE 'V100089 完成：export schema 已删除（表与数据已于 V100088 迁入 cdeu.finv_quote_secu_kline_min_export）';
END $$;

COMMIT;
