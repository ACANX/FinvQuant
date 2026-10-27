-- =====================================================================
-- FinvQuant PostgreSQL V100081：sys.sys_user 增加 uid 唯一索引
--
-- 决策（ACANX 2026-10-28）：
--   - 需求 10761 引入基于 sys_user 的登录校验，登录以 uid 为账号标识，
--     因此 uid 必须唯一，否则同一账号可能对应多条记录导致校验歧义。
--   - V74 建表时 uid 只有 NOT NULL，未加唯一约束（原 DDL 未定义）；
--     此处以唯一索引补齐，不改表结构、不动既有列。
--   - 索引名沿用 uk_ 前缀与 V74 命名风格（uk_sys_user_uid），
--     与 uk_sys_user_email / uk_sys_user_mobile 保持一致。
--   - 幂等：CREATE UNIQUE INDEX IF NOT EXISTS；重复执行无副作用。
-- 迁移策略：单事务、失败整体回滚；末尾做收敛校验。
-- 前置：V74（sys.sys_user）已应用。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. uid 唯一索引
-- ---------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_user_uid ON sys.sys_user (uid);

-- ---------------------------------------------------------------------
-- 2. 注释
-- ---------------------------------------------------------------------
COMMENT ON INDEX sys.uk_sys_user_uid IS '用户ID唯一索引（登录账号标识，需求 10761）';

-- ---------------------------------------------------------------------
-- 3. 收敛校验
-- ---------------------------------------------------------------------
DO $$
DECLARE
    idx_cnt  INT;
    dup_cnt  INT;
    uniq_ok  INT;
BEGIN
    SELECT count(*) INTO idx_cnt
      FROM pg_indexes
     WHERE schemaname = 'sys'
       AND tablename  = 'sys_user'
       AND indexname  = 'uk_sys_user_uid';
    IF idx_cnt <> 1 THEN
        RAISE EXCEPTION 'V100081: 唯一索引 uk_sys_user_uid 未创建成功';
    END IF;

    SELECT count(*) INTO uniq_ok
      FROM pg_index i
      JOIN pg_class c ON c.oid = i.indexrelid
      JOIN pg_namespace n ON n.oid = c.relnamespace
     WHERE n.nspname = 'sys' AND c.relname = 'uk_sys_user_uid' AND i.indisunique;
    IF uniq_ok <> 1 THEN
        RAISE EXCEPTION 'V100081: uk_sys_user_uid 不是唯一索引';
    END IF;

    -- 现存数据不得有重复 uid（唯一索引已保证，此处做显式断言便于排障）
    SELECT count(*) INTO dup_cnt
      FROM (SELECT uid FROM sys.sys_user GROUP BY uid HAVING count(*) > 1) d;
    IF dup_cnt > 0 THEN
        RAISE EXCEPTION 'V100081: sys_user 存在 % 组重复 uid', dup_cnt;
    END IF;

    RAISE NOTICE 'V100081: 校验通过（sys_user.uid 唯一索引就绪）';
END $$;

COMMIT;
