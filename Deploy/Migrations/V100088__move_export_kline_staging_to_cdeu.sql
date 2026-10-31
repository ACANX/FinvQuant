-- =====================================================================
-- FinvQuant PostgreSQL V100088：导出中转表由 export 域迁入 cdeu 域
--   旧表：export.finv_quote_secu_kline_min（V100035 建，导出中转快照）
--   新表：cdeu.finv_quote_secu_kline_min_export（同名同构，改域 + 加 _export 后缀）
--
-- 决策（ACANX 2026-10-31，用户在会话中明确要求）：
--   - 导出中转表「准备迁移到 cdeu.finv_quote_secu_kline_min_export 中来实现」，
--     即导出/补丁链路的暂存表从 export 域并入 cdeu 域，与
--     cdeu.finv_quote_secu_kline_min（去重/导出主表）同域，避免跨域事务与
--     search_path 位置的隐性依赖；
--   - 表名保留原名并追加 `_export` 后缀，与 cdeu 域内
--     finv_quote_cdeu_* 前缀约定不同（该表沿用业务表名，属刻意例外，
--     已在 Docs/DevSpec/DbSchemaSpec.md §0.2 / §1 登记）；
--   - 表结构沿用「与业务主表同构」的既有做法（V100035 的 LIKE 快照机制），
--     LIKE 不复制触发器，单独补齐（触发器名带 _export，避免与同 schema 内
--     cdeu.finv_quote_secu_kline_min 的同名触发器冲突）；
--   - 源表 export.finv_quote_secu_kline_min 在本迁移中**删除**：这是用户
--     「迁移该表」的明确授权（DbSchemaSpec §0.3 情形 1），迁移前先把存量行
--     全量搬入新表（本库实测 0 行；其他环境若有数据一并搬走，不丢数据）；
--   - export schema 本身**保留不动**：本次只迁移这一张表，schema 的删除属
--     另一次需要单独授权的变更（DbSchemaSpec §0.1）。
--
-- 幂等：建表 / 建触发器均先判存在；数据搬迁用 ON CONFLICT DO NOTHING；
--       源表已删除时跳过搬迁与 DROP；重跑影响 0 行。
-- 迁移策略：单事务（BEGIN/COMMIT），末尾收敛校验，不符整体回滚。
-- 前置：V100035（export 中转表）、V100038（按新结构重建）、V100080（cdeu 主表）。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 建表：与 cdeu.finv_quote_secu_kline_min 同构（幂等）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF to_regclass('cdeu.finv_quote_secu_kline_min_export') IS NOT NULL THEN
        RAISE NOTICE 'V100088：cdeu.finv_quote_secu_kline_min_export 已存在，跳过建表';
    ELSE
        IF to_regclass('cdeu.finv_quote_secu_kline_min') IS NULL THEN
            RAISE EXCEPTION 'V100088 前置失败：cdeu.finv_quote_secu_kline_min 不存在，无法同构建表';
        END IF;
        EXECUTE 'CREATE TABLE cdeu.finv_quote_secu_kline_min_export
                   (LIKE cdeu.finv_quote_secu_kline_min
                    INCLUDING DEFAULTS
                    INCLUDING CONSTRAINTS
                    INCLUDING INDEXES
                    INCLUDING STORAGE
                    INCLUDING COMMENTS)';
        RAISE NOTICE 'V100088：已创建 cdeu.finv_quote_secu_kline_min_export（结构取自 cdeu 主表）';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 2. gmt_update 触发器（LIKE 不复制触发器；名字带 _export 避免同 schema 冲突）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger
         WHERE tgname = 'trg_finv_quote_secu_kline_min_export_gmt_update'
           AND tgrelid = 'cdeu.finv_quote_secu_kline_min_export'::regclass
    ) THEN
        EXECUTE 'CREATE TRIGGER trg_finv_quote_secu_kline_min_export_gmt_update
                   BEFORE UPDATE ON cdeu.finv_quote_secu_kline_min_export
                   FOR EACH ROW EXECUTE FUNCTION public.vq_set_gmt_update()';
        RAISE NOTICE 'V100088：已补建 gmt_update 触发器 trg_finv_quote_secu_kline_min_export_gmt_update';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 3. 搬迁存量行（源表存在时；显式列名，避免列序差异；主键冲突跳过）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_rows bigint := 0;
BEGIN
    IF to_regclass('export.finv_quote_secu_kline_min') IS NOT NULL THEN
        EXECUTE 'INSERT INTO cdeu.finv_quote_secu_kline_min_export
                    (usc, ts, date, time, open, high, low, close, paocd, volume,
                     turnover, change_price, change_ratio, ext_field, last_close,
                     remark, provider, gmt_create, gmt_update)
                 SELECT usc, ts, date, time, open, high, low, close, paocd, volume,
                        turnover, change_price, change_ratio, ext_field, last_close,
                        remark, provider, gmt_create, gmt_update
                   FROM export.finv_quote_secu_kline_min
                 ON CONFLICT DO NOTHING';
        GET DIAGNOSTICS v_rows = ROW_COUNT;
        RAISE NOTICE 'V100088：export.finv_quote_secu_kline_min 搬迁 % 行到 cdeu.finv_quote_secu_kline_min_export', v_rows;
    ELSE
        RAISE NOTICE 'V100088：源表 export.finv_quote_secu_kline_min 不存在（已迁移过），跳过搬迁';
    END IF;
END $$;

-- ---------------------------------------------------------------------
-- 4. 删除源表（用户明确授权「迁移该表」，DbSchemaSpec §0.3 情形 1）
--    注：存量行已在第 3 步全量搬入新表；本库实测源表 0 行。
-- ---------------------------------------------------------------------
DROP TABLE IF EXISTS export.finv_quote_secu_kline_min;

-- ---------------------------------------------------------------------
-- 5. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    v_cols_new int;
    v_cols_ref int;
    v_trg      int;
    v_pk       int;
    v_src      bool;
BEGIN
    IF to_regclass('cdeu.finv_quote_secu_kline_min_export') IS NULL THEN
        RAISE EXCEPTION 'V100088 收敛失败：cdeu.finv_quote_secu_kline_min_export 未创建';
    END IF;

    SELECT count(*) INTO v_cols_new
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_kline_min_export';
    SELECT count(*) INTO v_cols_ref
      FROM information_schema.columns
     WHERE table_schema = 'cdeu' AND table_name = 'finv_quote_secu_kline_min';
    IF v_cols_new <> v_cols_ref THEN
        RAISE EXCEPTION 'V100088 收敛失败：新表列数 % 与主表列数 % 不一致', v_cols_new, v_cols_ref;
    END IF;

    SELECT count(*) INTO v_pk
      FROM pg_constraint
     WHERE conrelid = 'cdeu.finv_quote_secu_kline_min_export'::regclass AND contype = 'p';
    IF v_pk <> 1 THEN
        RAISE EXCEPTION 'V100088 收敛失败：新表主键数量为 %，预期 1', v_pk;
    END IF;

    SELECT count(*) INTO v_trg
      FROM pg_trigger
     WHERE tgname = 'trg_finv_quote_secu_kline_min_export_gmt_update'
       AND tgrelid = 'cdeu.finv_quote_secu_kline_min_export'::regclass;
    IF v_trg <> 1 THEN
        RAISE EXCEPTION 'V100088 收敛失败：新表缺少 gmt_update 触发器';
    END IF;

    v_src := to_regclass('export.finv_quote_secu_kline_min') IS NOT NULL;
    IF v_src THEN
        RAISE EXCEPTION 'V100088 收敛失败：源表 export.finv_quote_secu_kline_min 仍存在';
    END IF;

    RAISE NOTICE 'V100088 完成：导出中转表已迁至 cdeu.finv_quote_secu_kline_min_export（% 列），export.finv_quote_secu_kline_min 已删除', v_cols_new;
END $$;

COMMIT;
