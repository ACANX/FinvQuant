-- =====================================================================
-- FinvQuant PostgreSQL V100002：基础准备（二）——创建 schema（需求 10881）
--
-- 号段规划见 V100001 头部说明：本脚本属「V100000～V199999 基础准备」段，
-- 在任何表结构 DDL（V2xxxxx）与数据脚本（V3xxxxx）之前执行。
--
-- schema 清单与顺序（用户 2026-11-01 确认）：
--   public（PostgreSQL 自带，不创建、只校验）
--   → quote（行情主表）→ quant（量化回测）→ futu（外部源映射）
--   → sys（系统配置/字典/用户）→ sche（调度）→ cdeu（CDEU 工作流域）→ archive（归档）
--
-- 说明：
--   - 归属角色用 `CURRENT_USER`（默认即 `finv`），避免硬编码角色名后换角色失效；
--   - `brs`（备份恢复域）**不在本清单**，由 `V300060__brs_backup_table.sql`
--     （原 `V200060`）自建 `CREATE SCHEMA IF NOT EXISTS brs`；
--   - 历史 DDL 脚本（`V200032`/`V200033`/`V200034`/`V200037`/`V200071`/`V200077` 等）
--     里残留的 `CREATE SCHEMA IF NOT EXISTS` 与本脚本重复执行无副作用（幂等）。
--
-- 幂等：`CREATE SCHEMA IF NOT EXISTS`，重复执行 0 变更。
-- 迁移策略：单事务、失败整体回滚；末尾收敛校验。
-- =====================================================================

BEGIN;

CREATE SCHEMA IF NOT EXISTS quote   AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS quant   AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS futu    AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS sys     AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS sche    AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS cdeu    AUTHORIZATION CURRENT_USER;
CREATE SCHEMA IF NOT EXISTS archive AUTHORIZATION CURRENT_USER;

-- 收敛校验：8 个 schema 全部就位（public 为 PG 自带）
DO $$
DECLARE
    v_missing text;
BEGIN
    SELECT string_agg(name, ', ' ORDER BY name) INTO v_missing
      FROM (VALUES ('public'), ('quote'), ('quant'), ('futu'),
                   ('sys'), ('sche'), ('cdeu'), ('archive')) AS v(name)
     WHERE NOT EXISTS (SELECT 1 FROM pg_namespace n WHERE n.nspname = v.name);
    IF v_missing IS NOT NULL THEN
        RAISE EXCEPTION 'V100002 校验失败：以下 schema 缺失：%', v_missing;
    END IF;

    RAISE NOTICE 'V100002 完成：8 个 schema 就位（public 自带 + 7 个显式创建）';
END $$;

COMMIT;
