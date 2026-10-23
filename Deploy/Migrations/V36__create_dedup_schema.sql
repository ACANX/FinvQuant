-- =====================================================================
-- FinvQuant PostgreSQL V36：新增 dedup schema（去重域）并授权
--
-- 决策（ACANX 2026-10-24）：
--   - 需求：入库数据需要先去重，为此单独建 dedup schema，承载去重相关的
--     暂存/指纹/批内比对对象，与 quote（业务表）、export（导出转存）、
--     sche（调度）职责分离。
--   - 本迁移只建空 schema + 授权 + search_path，**不建任何表**：去重表结构
--     依赖具体入库流程（批内去重 / 跨源指纹 / 入库审计），由后续业务迁移创建。
--   - ⚠ 去重的三个层次（建表前先确认需要哪一层，避免重复造轮子）：
--       1) 跨批次幂等：quote.finv_quote_secu_kline_min 已有主键 (ts, usc)，
--          直接用 INSERT ... ON CONFLICT DO NOTHING 即可，无需额外对象；
--       2) 批内去重：同一批 COPY/INSERT 内的重复行，用临时表 + DISTINCT ON
--          或去重暂存表（可放 dedup）；
--       3) 跨源/指纹比对：需要记录来源批次、内容指纹（hash）或原始报文的，
--          这类对象才真正需要落到 dedup schema。
--   - 属主与授权对象取 current_user（= 应用角色，当前 finv），不硬编码角色名。
--   - search_path：dedup 追加在末尾（public, quote, quant, futu, export, sche, dedup），
--     不改变既有 schema 的解析优先级（历史种子迁移依赖该优先级）。
--   - 幂等：CREATE SCHEMA IF NOT EXISTS + GRANT（重复执行无副作用）。
-- 迁移策略：单事务、失败回滚；末尾做收敛校验。
-- 前置：V35（export 转存表）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建 schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS dedup;

-- ---------------------------------------------------------------------
-- 2. 属主与权限：属主 = 应用角色；授予 USAGE（可用）+ CREATE（可建表）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl TEXT := current_user;
BEGIN
    EXECUTE format('ALTER SCHEMA dedup OWNER TO %I', appl);
    EXECUTE format('GRANT USAGE, CREATE ON SCHEMA dedup TO %I', appl);
    RAISE NOTICE 'V36: schema dedup 属主=%，已授予 USAGE/CREATE', appl;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据库默认 search_path（新 schema 追加到末尾，不改变既有优先级）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format(
        'ALTER DATABASE %I SET search_path TO public, quote, quant, futu, export, sche, dedup',
        current_database()
    );
    RAISE NOTICE 'V36: 数据库 % 的 search_path 设为 public, quote, quant, futu, export, sche, dedup',
        current_database();
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：schema 存在、属主为应用角色、具备 USAGE+CREATE
-- ---------------------------------------------------------------------
DO $$
DECLARE
    appl     TEXT := current_user;
    missing  INT;
    badowner INT;
    nopriv   INT;
BEGIN
    SELECT count(*) INTO missing
      FROM pg_namespace WHERE nspname = 'dedup';
    IF missing = 0 THEN
        RAISE EXCEPTION 'V36: dedup schema 未创建成功';
    END IF;

    SELECT count(*) INTO badowner
      FROM pg_namespace
     WHERE nspname = 'dedup' AND pg_get_userbyid(nspowner) <> appl;
    IF badowner > 0 THEN
        RAISE EXCEPTION 'V36: dedup schema 属主不是 %', appl;
    END IF;

    SELECT count(*) INTO nopriv
      FROM pg_namespace
     WHERE nspname = 'dedup'
       AND NOT (has_schema_privilege(appl, nspname, 'USAGE')
            AND has_schema_privilege(appl, nspname, 'CREATE'));
    IF nopriv > 0 THEN
        RAISE EXCEPTION 'V36: dedup schema 缺少 USAGE/CREATE 权限';
    END IF;

    RAISE NOTICE 'V36: 校验通过（dedup 就绪，属主=%，USAGE+CREATE 齐备）', appl;
END $$;

COMMIT;
