-- =====================================================================
-- FinvQuant PostgreSQL V33：finv_futu_* 迁入 futu schema
--
-- 决策（ACANX 2026-10-24，改造项 D）：
--   - 需求：finv_futu 开头的表迁入 futu schema，与 quote / quant 一起形成
--     「业务域 = schema」的划分；富途映射表是「富途 OpenAPI 编码 ↔ Finv 内部
--     编码」的对照字典，属外部源对接映射子域；
--   - 只改 schema、不改表名、不改列：ALTER TABLE ... SET SCHEMA 为 catalog
--     级操作，10 个索引、5 个 gmt_update 触发器自动跟随，不扫描数据；
--   - 无物理外键、无视图引用，Go 代码对本组表零引用（纯字典查询走 finv_security）；
--   - search_path：更新数据库默认值为 public, quote, quant, futu（同时对应用
--     连接串生效，见 internal/database/postgres.go）。原因是本迁移（V33）早于
--     种子段 V100003+ 执行，而历史种子 SQL 不带 schema 前缀，必须依赖
--     search_path；其中 V100009 会读取 finv_futu_mapping_security 回填
--     finv_security，两侧表名需同时可解析；
--   - 与 V100023 的差异：本组表的 CREATE TABLE 都在 V7～V17（早于本迁移），
--     种子段只做 INSERT/UPDATE/DELETE，因此**不需要**额外的收尾迁移；
--   - 幂等：逐表用 pg_tables.schemaname 判断，已迁出则跳过；末尾断言收敛，
--     任何一张表未迁出即整事务回滚。
-- 迁移策略：与既有迁移一致，单事务、失败回滚。
-- 前置：V32（quote/quant schema）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 创建 futu schema（幂等）
-- ---------------------------------------------------------------------
CREATE SCHEMA IF NOT EXISTS futu;

-- ---------------------------------------------------------------------
-- 2. finv_futu_* → futu schema（5 张，幂等）
-- ---------------------------------------------------------------------
DO $$
DECLARE
    t TEXT;
    moved INT := 0;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'finv_futu_mapping_cs_market',
        'finv_futu_mapping_exchange',
        'finv_futu_mapping_market_code',
        'finv_futu_mapping_region',
        'finv_futu_mapping_security'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = t) THEN
            EXECUTE format('ALTER TABLE public.%I SET SCHEMA futu', t);
            RAISE NOTICE 'V33: public.% -> futu.%', t, t;
            moved := moved + 1;
        ELSE
            RAISE NOTICE 'V33: public.% 不存在或已迁出，跳过', t;
        END IF;
    END LOOP;
    RAISE NOTICE 'V33: 本次移动 % 张表', moved;
END $$;

-- ---------------------------------------------------------------------
-- 3. 数据库默认 search_path（纳入 futu）
-- ---------------------------------------------------------------------
DO $$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path TO public, quote, quant, futu', current_database());
    RAISE NOTICE 'V33: 数据库 % 的 search_path 设为 public, quote, quant, futu', current_database();
END $$;

-- ---------------------------------------------------------------------
-- 4. 收敛校验：public 无残留 + futu 中 5 张齐全，否则整体回滚
-- ---------------------------------------------------------------------
DO $$
DECLARE
    left_public INT;
    futu_cnt    INT;
BEGIN
    SELECT count(*) INTO left_public
      FROM pg_tables
     WHERE schemaname = 'public' AND tablename LIKE 'finv\_futu\_%';

    SELECT count(*) INTO futu_cnt
      FROM pg_tables
     WHERE schemaname = 'futu' AND tablename LIKE 'finv\_futu\_%';

    IF left_public > 0 THEN
        RAISE EXCEPTION 'V33: public 中仍有 % 张 finv_futu_* 表未迁出', left_public;
    END IF;
    IF futu_cnt <> 5 THEN
        RAISE EXCEPTION 'V33: futu schema 中 finv_futu_* 表数为 %（期望 5）', futu_cnt;
    END IF;

    RAISE NOTICE 'V33: 校验通过（futu=%, public 残留=0）', futu_cnt;
END $$;

COMMIT;
