-- =====================================================================
-- VeritasQuant PostgreSQL 第二版迁移（P2-026）
--
-- 范围：不可变命令资源与幂等键存储。
-- 契约（TechSpec 10.2.2）：
--   - 所有写操作创建不可变命令资源：command_id + Idempotency-Key；
--   - 幂等作用域 = principal_id + account_id + API 路由 + Idempotency-Key；
--   - 同键同哈希返回原命令及状态；同键异哈希返回 1003；
--   - 命令身份字段（command_id/type/account/run/scope/payload_hash/payload/
--     expected_version/confirmation_token/requested_by/created_ts）一经写入
--     冻结，触发器禁止修改；status/updated_ts/result/failure 为生命周期字段；
--   - 失败快照保存 code/error.code/catalog_version/retryable/details。
-- 迁移策略：整个 V2 在一个事务内执行；任一步失败自动回滚。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 命令资源（身份不可变 + 生命周期状态）
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS command_records (
    command_id           TEXT        PRIMARY KEY,
    command_type         TEXT        NOT NULL,
    account_id           TEXT        NOT NULL,
    run_id               TEXT        NOT NULL,
    requested_by         TEXT        NOT NULL,
    idempotency_scope    TEXT        NOT NULL,
    payload_hash         TEXT        NOT NULL CHECK (payload_hash ~ '^[0-9a-f]{64}$'),
    payload              JSONB       NOT NULL,
    expected_version     TEXT,
    confirmation_token_id TEXT,
    status               TEXT        NOT NULL,
    created_ts           TIMESTAMPTZ NOT NULL,
    updated_ts           TIMESTAMPTZ NOT NULL,
    result_reference     TEXT,
    failure_code         INTEGER,
    failure_error_code   TEXT,
    failure_catalog_version TEXT,
    failure_retryable    BOOLEAN,
    failure_details      JSONB
);

-- 幂等作用域唯一约束：同作用域只能有一个命令
CREATE UNIQUE INDEX IF NOT EXISTS uq_command_records_idempotency_scope
    ON command_records (idempotency_scope);

-- 状态查询与按账户/运行过滤索引
CREATE INDEX IF NOT EXISTS idx_command_records_account
    ON command_records (account_id, created_ts DESC);
CREATE INDEX IF NOT EXISTS idx_command_records_status
    ON command_records (status, updated_ts DESC);

-- 身份冻结触发器：禁止修改不可变身份字段
CREATE OR REPLACE FUNCTION assert_command_identity_frozen() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'command_records 禁止 DELETE（审计保留）';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        -- 身份字段逐列比较（记录类型不支持变量下标，必须显式列出）
        IF NEW.command_id IS DISTINCT FROM OLD.command_id
           OR NEW.command_type IS DISTINCT FROM OLD.command_type
           OR NEW.account_id IS DISTINCT FROM OLD.account_id
           OR NEW.run_id IS DISTINCT FROM OLD.run_id
           OR NEW.requested_by IS DISTINCT FROM OLD.requested_by
           OR NEW.idempotency_scope IS DISTINCT FROM OLD.idempotency_scope
           OR NEW.payload_hash IS DISTINCT FROM OLD.payload_hash
           OR NEW.payload IS DISTINCT FROM OLD.payload
           OR NEW.expected_version IS DISTINCT FROM OLD.expected_version
           OR NEW.confirmation_token_id IS DISTINCT FROM OLD.confirmation_token_id
           OR NEW.created_ts IS DISTINCT FROM OLD.created_ts THEN
            RAISE EXCEPTION '命令身份字段不可变';
        END IF;
        IF NEW.updated_ts <= OLD.updated_ts THEN
            RAISE EXCEPTION 'updated_ts 必须单调递增';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_command_records_frozen ON command_records;
CREATE TRIGGER trg_command_records_frozen
    BEFORE UPDATE OR DELETE ON command_records
    FOR EACH ROW EXECUTE FUNCTION assert_command_identity_frozen();

COMMIT;
