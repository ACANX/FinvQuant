-- =====================================================================
-- VeritasQuant PostgreSQL 首版迁移（P2-001）
--
-- 范围：事实表、投影表、索引、租约与检查点基础设施的首版设计。
-- 事实源原则：投影表不是事实源，任何余额或持仓必须能由不可变事实序列重建。
-- 约束原则：事实表不可变（禁止 UPDATE/DELETE）；NUMERIC 精度不低于
--   NUMERIC(38,18)；账户作用域列 + 复合唯一键保证账户间数据隔离。
-- 迁移策略：整个 V1 在一个事务内执行；任一步失败自动回滚，应用保持 not-ready。
-- 禁止运行时自动改表：结构变化只进入新的版本化迁移文件。
-- =====================================================================

BEGIN;

-- ---------------------------------------------------------------------
-- 1. 迁移版本跟踪
-- ---------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS schema_version (
    version          TEXT        NOT NULL,
    description      TEXT        NOT NULL,
    installed_by     TEXT        NOT NULL DEFAULT current_user,
    installed_on     TIMESTAMPTZ NOT NULL DEFAULT now(),
    success          BOOLEAN     NOT NULL DEFAULT TRUE,
    PRIMARY KEY (version)
);

-- ---------------------------------------------------------------------
-- 2. 运行清单（RunManifestV1 投影；运行身份与版本快照）
-- ---------------------------------------------------------------------
CREATE TABLE run_manifests (
    run_id                        TEXT        PRIMARY KEY,
    code_version                  TEXT        NOT NULL,
    event_schema_registry_hash    TEXT        NOT NULL,
    strategy_version              TEXT        NOT NULL,
    strategy_source_hash          TEXT        NOT NULL,
    dependency_lock_hash          TEXT        NOT NULL,
    interpreter_version           TEXT        NOT NULL,
    sandbox_image_digest          TEXT        NOT NULL,
    strategy_sandbox_policy_version TEXT      NOT NULL,
    strategy_dsl_schema_version   TEXT        NOT NULL,
    investment_plan_schema_version TEXT       NOT NULL,
    config_hash                   TEXT        NOT NULL,
    config_schema_version         TEXT        NOT NULL,
    data_version_id               TEXT        NOT NULL,
    asset_capability_version      TEXT        NOT NULL,
    account_group_id              TEXT        NOT NULL,
    account_ranks                 JSONB       NOT NULL,
    random_seed                   BIGINT      NOT NULL CHECK (random_seed >= 0),
    ts_precision                  TEXT        NOT NULL,
    event_ordering_version        TEXT        NOT NULL,
    execution_model_version       TEXT        NOT NULL,
    fund_execution_model_version  TEXT        NOT NULL,
    nav_availability_policy_version TEXT      NOT NULL,
    bar_path_model_version        TEXT        NOT NULL,
    liquidity_allocation_version  TEXT        NOT NULL,
    risk_policy_version           TEXT        NOT NULL,
    reliability_policy_version    TEXT        NOT NULL,
    started_at                    TIMESTAMPTZ NOT NULL,
    completed_at                  TIMESTAMPTZ,
    event_count                   BIGINT      NOT NULL DEFAULT 0 CHECK (event_count >= 0),
    order_count                   BIGINT      NOT NULL DEFAULT 0 CHECK (order_count >= 0),
    fund_subscription_count       BIGINT      NOT NULL DEFAULT 0 CHECK (fund_subscription_count >= 0),
    fund_confirmation_count       BIGINT      NOT NULL DEFAULT 0 CHECK (fund_confirmation_count >= 0),
    execution_count               BIGINT      NOT NULL DEFAULT 0 CHECK (execution_count >= 0),
    report_path                   TEXT
);
CREATE INDEX idx_run_manifests_started_at ON run_manifests (started_at);

-- ---------------------------------------------------------------------
-- 3. 事件事实表（EventEnvelopeV1 全字段 + 分区投递元数据）
--    账户分区：account_group_id/account_id 为账户作用域列，结合
--    (account_group_id, delivery_sequence) 与 (account_id, ts) 索引
--    保证分区内顺序与账户隔离；物理分区在账户组拓扑冻结后演进。
-- ---------------------------------------------------------------------
CREATE TABLE fact_events (
    event_id                  TEXT        NOT NULL,
    event_type                TEXT        NOT NULL,
    schema_version            TEXT        NOT NULL,
    run_id                    TEXT        NOT NULL REFERENCES run_manifests (run_id),
    ts                        TIMESTAMPTZ NOT NULL,
    occurred_at               TIMESTAMPTZ,
    published_at              TIMESTAMPTZ,
    ingested_at               TIMESTAMPTZ NOT NULL,
    source                    TEXT        NOT NULL,
    producer                  TEXT        NOT NULL,
    producer_version          TEXT        NOT NULL,
    correlation_id            TEXT        NOT NULL,
    causation_id              TEXT,
    account_id                TEXT,
    subaccount_id             TEXT,
    event_ordering_version    TEXT        NOT NULL,
    phase                     INTEGER     NOT NULL CHECK (phase IN (10, 20, 30, 40, 50, 60)),
    priority                  INTEGER     NOT NULL CHECK (priority >= 0),
    source_rank               INTEGER     NOT NULL CHECK (source_rank >= 0),
    source_sequence           BIGINT      NOT NULL CHECK (source_sequence >= 0),
    payload                   JSONB       NOT NULL,
    content_hash              TEXT        NOT NULL,
    account_group_id          TEXT        NOT NULL,
    partition_rank            INTEGER     NOT NULL DEFAULT 0 CHECK (partition_rank >= 0),
    delivery_sequence         BIGINT      NOT NULL CHECK (delivery_sequence >= 1),
    -- 同一共享事件在每个账户组分区各持有一行（主键 = 事件 + 分区）
    PRIMARY KEY (event_id, account_group_id)
);
-- 分区内确定性顺序：同一共享事件在每个分区必须按相同 delivery_sequence 扇出
CREATE UNIQUE INDEX uq_fact_events_partition_delivery
    ON fact_events (run_id, account_group_id, delivery_sequence);
-- 全序排序键：ts + phase + priority + source_rank + source_sequence + event_id
CREATE INDEX idx_fact_events_total_order
    ON fact_events (ts, phase, priority, source_rank, source_sequence, event_id);
CREATE INDEX idx_fact_events_account_ts
    ON fact_events (account_id, ts);
CREATE INDEX idx_fact_events_group_ts
    ON fact_events (account_group_id, ts);
CREATE INDEX idx_fact_events_type
    ON fact_events (event_type, ts);
CREATE INDEX idx_fact_events_causation
    ON fact_events (causation_id);
CREATE INDEX idx_fact_events_correlation
    ON fact_events (correlation_id);

-- ---------------------------------------------------------------------
-- 4. inbox / outbox（可重试输入幂等 + 领域提交后至少一次投递）
-- ---------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS inbox_receipt_seq START 1;

CREATE TABLE inbox_records (
    idempotency_key      TEXT        PRIMARY KEY,
    content_hash         TEXT        NOT NULL,
    receipt_sequence     BIGINT      NOT NULL CHECK (receipt_sequence >= 1),
    disposition          TEXT        NOT NULL CHECK (disposition IN ('APPLIED', 'DUPLICATE', 'CONFLICT')),
    run_id               TEXT        NOT NULL REFERENCES run_manifests (run_id),
    partition_id         TEXT        NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_attempt_at      TIMESTAMPTZ,
    attempt_count        INTEGER     NOT NULL DEFAULT 0 CHECK (attempt_count >= 0)
);
CREATE UNIQUE INDEX uq_inbox_partition_receipt
    ON inbox_records (run_id, partition_id, receipt_sequence);
CREATE INDEX idx_inbox_partition
    ON inbox_records (run_id, partition_id, created_at);

-- 同键异载荷的协议冲突隔离审计（不可变）
CREATE TABLE inbox_conflicts (
    conflict_id               TEXT        PRIMARY KEY,
    idempotency_key           TEXT        NOT NULL,
    existing_content_hash     TEXT        NOT NULL,
    conflicting_content_hash  TEXT        NOT NULL,
    isolated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    run_id                    TEXT        NOT NULL REFERENCES run_manifests (run_id),
    partition_id              TEXT        NOT NULL
);
CREATE INDEX idx_inbox_conflicts_key
    ON inbox_conflicts (run_id, partition_id, idempotency_key);

CREATE SEQUENCE IF NOT EXISTS outbox_sequence_seq START 1;

CREATE TABLE outbox_records (
    outbox_id        TEXT        PRIMARY KEY,
    message_id       TEXT        NOT NULL,
    sequence         BIGINT      NOT NULL CHECK (sequence >= 1),
    topic            TEXT        NOT NULL,
    payload_hash     TEXT        NOT NULL,
    status           TEXT        NOT NULL CHECK (status IN ('PENDING', 'PUBLISHED')),
    run_id           TEXT        NOT NULL REFERENCES run_manifests (run_id),
    partition_id     TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at     TIMESTAMPTZ,
    attempt_count    INTEGER     NOT NULL DEFAULT 0 CHECK (attempt_count >= 0)
);
CREATE UNIQUE INDEX uq_outbox_partition_message
    ON outbox_records (run_id, partition_id, message_id);
-- 发布器按提交序号顺序扫描：先按 status 过滤，再按 sequence 升序
CREATE INDEX idx_outbox_pending_sequence
    ON outbox_records (run_id, partition_id, status, sequence);

-- ---------------------------------------------------------------------
-- 5. 单活租约与 fencing token（每个账户组同一时刻只有一个写入者）
-- ---------------------------------------------------------------------
CREATE TABLE partition_leases (
    account_group_id   TEXT        PRIMARY KEY,
    lease_holder       TEXT        NOT NULL,
    fencing_token      BIGINT      NOT NULL CHECK (fencing_token >= 0),
    lease_acquired_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    lease_expires_at   TIMESTAMPTZ NOT NULL,
    lease_ttl_seconds  INTEGER     NOT NULL CHECK (lease_ttl_seconds > 0),
    renewed_at         TIMESTAMPTZ
);
CREATE INDEX idx_partition_leases_expiry
    ON partition_leases (lease_expires_at);

-- ---------------------------------------------------------------------
-- 6. 分区检查点（EventProcessingCheckpointV1）
-- ---------------------------------------------------------------------
CREATE TABLE partition_checkpoints (
    run_id                  TEXT        NOT NULL REFERENCES run_manifests (run_id),
    partition_id            TEXT        NOT NULL,
    last_committed_sequence BIGINT      NOT NULL CHECK (last_committed_sequence >= 0),
    transaction_id          TEXT        NOT NULL,
    checkpoint_ts           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, partition_id)
);

-- ---------------------------------------------------------------------
-- 7. 账本事实表（不可变复式分录；JournalV1 + LedgerEntryV1）
-- ---------------------------------------------------------------------
CREATE TABLE ledger_journals (
    journal_id                  TEXT        PRIMARY KEY,
    journal_type                TEXT        NOT NULL CHECK (journal_type IN (
        'OPENING_BALANCE', 'ORDER_RESERVATION', 'ORDER_RELEASE', 'TRADE',
        'TRADE_SETTLEMENT', 'FUND_SUBSCRIPTION', 'FUND_REDEMPTION',
        'FUND_DISTRIBUTION', 'FEE', 'TAX', 'DEPOSIT', 'WITHDRAWAL',
        'INTEREST', 'DIVIDEND', 'CORPORATE_ACTION', 'MARK_TO_MARKET',
        'MARGIN', 'SETTLEMENT', 'DELIVERY', 'FX_CONVERSION',
        'BROKER_CORRECTION', 'REVERSAL', 'MANUAL_ADJUSTMENT')),
    account_id              TEXT        NOT NULL,
    subaccount_id           TEXT,
    ts                      TIMESTAMPTZ NOT NULL,
    commit_sequence         BIGINT      NOT NULL CHECK (commit_sequence >= 1),
    source_event_id         TEXT        NOT NULL,
    reversal_of_journal_id  TEXT,
    instrument_metadata_version TEXT    NOT NULL,
    fee_schedule_version    TEXT        NOT NULL,
    accounting_policy_version TEXT      NOT NULL,
    run_id                  TEXT        NOT NULL REFERENCES run_manifests (run_id),
    journal_hash            TEXT        NOT NULL
);
-- 账户内提交序号唯一，保证重放顺序确定
CREATE UNIQUE INDEX uq_ledger_journals_account_sequence
    ON ledger_journals (account_id, commit_sequence);
CREATE INDEX idx_ledger_journals_account_ts
    ON ledger_journals (account_id, ts);
CREATE INDEX idx_ledger_journals_source_event
    ON ledger_journals (source_event_id);
CREATE INDEX idx_ledger_journals_reversal
    ON ledger_journals (reversal_of_journal_id);
ALTER TABLE ledger_journals
    ADD CONSTRAINT fk_ledger_journals_reversal
    FOREIGN KEY (reversal_of_journal_id) REFERENCES ledger_journals (journal_id);

CREATE TABLE ledger_entries (
    entry_id        TEXT        PRIMARY KEY,
    journal_id      TEXT        NOT NULL REFERENCES ledger_journals (journal_id),
    ledger_account  TEXT        NOT NULL CHECK (ledger_account IN (
        'CASH_AVAILABLE', 'CASH_FROZEN', 'CASH_RECEIVABLE', 'CASH_PAYABLE',
        'SECURITIES_AVAILABLE', 'SECURITIES_FROZEN', 'SECURITIES_RECEIVABLE',
        'MARGIN_AVAILABLE', 'MARGIN_FROZEN', 'ROUNDING_RESIDUAL')),
    direction       TEXT        NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    unit_id         TEXT        NOT NULL,
    asset_id        TEXT        NOT NULL,
    currency        TEXT,
    quantity        NUMERIC(38, 18) NOT NULL CHECK (quantity > 0),
    book_currency   TEXT        NOT NULL,
    book_amount     NUMERIC(38, 18) NOT NULL CHECK (book_amount >= 0),
    cost_amount     NUMERIC(38, 18) NOT NULL CHECK (cost_amount >= 0),
    account_id      TEXT        NOT NULL,
    run_id          TEXT        NOT NULL REFERENCES run_manifests (run_id)
);
CREATE UNIQUE INDEX uq_ledger_entries_journal_entry
    ON ledger_entries (journal_id, entry_id);
CREATE INDEX idx_ledger_entries_account
    ON ledger_entries (account_id, journal_id);

-- ---------------------------------------------------------------------
-- 8. 订单与执行事实表（OrderIntentV1 / OrderEventV1 / 撤改单 / 成交回报）
-- ---------------------------------------------------------------------
CREATE TABLE order_intents (
    intent_id                TEXT        PRIMARY KEY,
    run_id                   TEXT        NOT NULL REFERENCES run_manifests (run_id),
    account_id               TEXT        NOT NULL,
    subaccount_id            TEXT,
    strategy_id              TEXT        NOT NULL,
    strategy_version         TEXT        NOT NULL,
    symbol                   TEXT        NOT NULL,
    instrument_metadata_version TEXT     NOT NULL,
    side                     TEXT        NOT NULL CHECK (side IN ('BUY', 'SELL')),
    position_effect          TEXT        NOT NULL CHECK (position_effect IN ('OPEN', 'CLOSE', 'OPEN_CLOSE')),
    order_type               TEXT        NOT NULL CHECK (order_type IN ('MARKET', 'LIMIT', 'STOP', 'STOP_LIMIT')),
    quantity                 NUMERIC(38, 18) NOT NULL CHECK (quantity > 0),
    time_in_force            TEXT        NOT NULL CHECK (time_in_force IN ('DAY', 'GTC', 'IOC', 'FOK')),
    ts                       TIMESTAMPTZ NOT NULL,
    created_from_event_id    TEXT        NOT NULL,
    expected_account_version BIGINT      NOT NULL CHECK (expected_account_version >= 0),
    limit_price              NUMERIC(38, 12) CHECK (limit_price > 0),
    stop_price               NUMERIC(38, 12) CHECK (stop_price > 0),
    intent_hash              TEXT        NOT NULL
);
CREATE INDEX idx_order_intents_account_ts
    ON order_intents (account_id, ts);
CREATE INDEX idx_order_intents_strategy
    ON order_intents (strategy_id, strategy_version);

-- 订单状态迁移事实：每次迁移严格增加 order_version
CREATE TABLE order_events (
    event_id               TEXT        PRIMARY KEY,
    client_order_id        TEXT        NOT NULL,
    intent_id              TEXT        NOT NULL REFERENCES order_intents (intent_id),
    command_id             TEXT        NOT NULL,
    order_version          INTEGER     NOT NULL CHECK (order_version >= 1),
    state                  TEXT        NOT NULL CHECK (state IN (
        'NEW', 'PARTIALLY_FILLED', 'FILLED', 'CANCELLED', 'REJECTED',
        'EXPIRED', 'UNKNOWN')),
    approved_quantity      NUMERIC(38, 18) NOT NULL CHECK (approved_quantity > 0),
    order_type             TEXT        NOT NULL CHECK (order_type IN ('MARKET', 'LIMIT', 'STOP', 'STOP_LIMIT')),
    side                   TEXT        NOT NULL CHECK (side IN ('BUY', 'SELL')),
    quantity               NUMERIC(38, 18) NOT NULL CHECK (quantity > 0),
    limit_price            NUMERIC(38, 12) CHECK (limit_price > 0),
    stop_price             NUMERIC(38, 12) CHECK (stop_price > 0),
    effective_after_event_id TEXT       NOT NULL,
    risk_decision_id       TEXT        NOT NULL,
    account_id             TEXT        NOT NULL,
    subaccount_id          TEXT,
    ts                     TIMESTAMPTZ NOT NULL,
    run_id                 TEXT        NOT NULL REFERENCES run_manifests (run_id),
    event_hash             TEXT        NOT NULL
);
-- 同一订单的版本单调递增且唯一
CREATE UNIQUE INDEX uq_order_events_client_version
    ON order_events (client_order_id, order_version);
CREATE INDEX idx_order_events_account_ts
    ON order_events (account_id, ts);
CREATE INDEX idx_order_events_command
    ON order_events (command_id);
CREATE INDEX idx_order_events_intent
    ON order_events (intent_id);

CREATE TABLE cancel_order_requests (
    cancel_request_id   TEXT        PRIMARY KEY,
    client_order_id     TEXT        NOT NULL,
    broker_order_id     TEXT,
    expected_order_version INTEGER   NOT NULL CHECK (expected_order_version >= 1),
    reason              TEXT        NOT NULL,
    requested_by        TEXT        NOT NULL,
    account_id          TEXT        NOT NULL,
    ts                  TIMESTAMPTZ NOT NULL,
    run_id              TEXT        NOT NULL REFERENCES run_manifests (run_id),
    request_hash        TEXT        NOT NULL
);
CREATE INDEX idx_cancel_requests_account
    ON cancel_order_requests (account_id, client_order_id);

CREATE TABLE replace_order_requests (
    replace_request_id  TEXT        PRIMARY KEY,
    client_order_id     TEXT        NOT NULL,
    expected_order_version INTEGER   NOT NULL CHECK (expected_order_version >= 1),
    new_quantity        NUMERIC(38, 18) CHECK (new_quantity > 0),
    new_limit_price     NUMERIC(38, 12) CHECK (new_limit_price > 0),
    new_stop_price      NUMERIC(38, 12) CHECK (new_stop_price > 0),
    new_time_in_force   TEXT        CHECK (new_time_in_force IN ('DAY', 'GTC', 'IOC', 'FOK')),
    reason              TEXT        NOT NULL,
    account_id          TEXT        NOT NULL,
    ts                  TIMESTAMPTZ NOT NULL,
    run_id              TEXT        NOT NULL REFERENCES run_manifests (run_id),
    request_hash        TEXT        NOT NULL
);
CREATE INDEX idx_replace_requests_account
    ON replace_order_requests (account_id, client_order_id);

CREATE TABLE execution_reports (
    broker_report_id    TEXT        PRIMARY KEY,
    client_order_id     TEXT        NOT NULL,
    broker_order_id     TEXT,
    report_sequence     BIGINT      NOT NULL CHECK (report_sequence >= 1),
    execution_type      TEXT        NOT NULL CHECK (execution_type IN (
        'NEW', 'PARTIAL_FILL', 'FILL', 'CANCEL', 'REJECT', 'REPLACE', 'EXPIRED')),
    execution_id        TEXT,
    last_quantity       NUMERIC(38, 18) NOT NULL CHECK (last_quantity >= 0),
    last_price          NUMERIC(38, 12) CHECK (last_price > 0),
    cumulative_quantity NUMERIC(38, 18) NOT NULL CHECK (cumulative_quantity >= 0),
    remaining_quantity  NUMERIC(38, 18) NOT NULL CHECK (remaining_quantity >= 0),
    broker_state        TEXT        NOT NULL CHECK (broker_state IN (
        'NEW', 'PARTIALLY_FILLED', 'FILLED', 'CANCELLED', 'REJECTED',
        'EXPIRED', 'UNKNOWN')),
    reason_code         TEXT,
    diagnostic_ts       TIMESTAMPTZ NOT NULL,
    account_id          TEXT        NOT NULL,
    ts                  TIMESTAMPTZ NOT NULL,
    run_id              TEXT        NOT NULL REFERENCES run_manifests (run_id),
    report_hash         TEXT        NOT NULL
);
-- 成交在账户内唯一；部分唯一索引允许非成交回报 execution_id 为空
CREATE UNIQUE INDEX uq_execution_reports_account_execution
    ON execution_reports (account_id, execution_id) WHERE execution_id IS NOT NULL;
CREATE UNIQUE INDEX uq_execution_reports_client_sequence
    ON execution_reports (client_order_id, report_sequence);
CREATE INDEX idx_execution_reports_account_ts
    ON execution_reports (account_id, ts);

-- ---------------------------------------------------------------------
-- 9. 风控事实表（RiskDecisionEventV1 / TradingControlEventV1）
-- ---------------------------------------------------------------------
CREATE TABLE risk_decisions (
    decision_id             TEXT        PRIMARY KEY,
    request_event_id        TEXT        NOT NULL,
    account_id              TEXT        NOT NULL,
    decision                TEXT        NOT NULL CHECK (decision IN ('APPROVED', 'REJECTED', 'REDUCED')),
    approved_quantity       NUMERIC(38, 18) NOT NULL CHECK (approved_quantity >= 0),
    rule_ids                JSONB       NOT NULL,
    risk_policy_version     TEXT        NOT NULL,
    account_snapshot_version BIGINT     NOT NULL CHECK (account_snapshot_version >= 0),
    order_snapshot_version  BIGINT      NOT NULL CHECK (order_snapshot_version >= 0),
    position_snapshot_version BIGINT    NOT NULL CHECK (position_snapshot_version >= 0),
    reason_codes            JSONB       NOT NULL,
    decision_hash           TEXT        NOT NULL,
    ts                      TIMESTAMPTZ NOT NULL,
    run_id                  TEXT        NOT NULL REFERENCES run_manifests (run_id)
);
CREATE INDEX idx_risk_decisions_account_ts
    ON risk_decisions (account_id, ts);
CREATE INDEX idx_risk_decisions_request
    ON risk_decisions (request_event_id);

CREATE TABLE trading_controls (
    control_id          TEXT        NOT NULL,
    control_version     INTEGER     NOT NULL CHECK (control_version >= 1),
    control_request_id  TEXT        NOT NULL,
    idempotency_key     TEXT        NOT NULL,
    scope               TEXT        NOT NULL,
    action              TEXT        NOT NULL CHECK (action IN (
        'REJECT_NEW_ORDERS', 'REDUCE_ONLY', 'PAUSE_SCOPE', 'STOP_TRADING')),
    strength            INTEGER     NOT NULL CHECK (strength >= 0),
    parameters          JSONB       NOT NULL,
    effective_from      TEXT        NOT NULL,
    expires_at          TEXT,
    source_decision_id  TEXT        NOT NULL,
    risk_policy_version TEXT        NOT NULL,
    status              TEXT        NOT NULL,
    control_hash        TEXT        NOT NULL,
    ts                  TIMESTAMPTZ NOT NULL,
    run_id              TEXT        NOT NULL REFERENCES run_manifests (run_id),
    PRIMARY KEY (control_id, control_version)
);
CREATE UNIQUE INDEX uq_trading_controls_idempotency
    ON trading_controls (idempotency_key);
CREATE INDEX idx_trading_controls_scope
    ON trading_controls (scope, effective_from);

-- ---------------------------------------------------------------------
-- 10. 投影表（可删除并由已提交事实序列确定性重建；不是事实源）
-- ---------------------------------------------------------------------
CREATE TABLE account_snapshots (
    account_id           TEXT        NOT NULL,
    snapshot_version     BIGINT      NOT NULL CHECK (snapshot_version >= 1),
    last_ledger_sequence BIGINT      NOT NULL CHECK (last_ledger_sequence >= 0),
    snapshot_ts          TIMESTAMPTZ NOT NULL,
    balances             JSONB       NOT NULL,
    content_hash         TEXT        NOT NULL,
    run_id               TEXT        NOT NULL REFERENCES run_manifests (run_id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, snapshot_version)
);
CREATE INDEX idx_account_snapshots_latest
    ON account_snapshots (account_id, snapshot_version DESC);

CREATE TABLE ledger_balance_projection (
    account_id    TEXT        NOT NULL,
    ledger_account TEXT       NOT NULL,
    unit_id       TEXT        NOT NULL,
    book_currency TEXT        NOT NULL,
    quantity      NUMERIC(38, 18) NOT NULL,
    cost_amount   NUMERIC(38, 18) NOT NULL,
    last_ledger_sequence BIGINT NOT NULL CHECK (last_ledger_sequence >= 0),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, ledger_account, unit_id, book_currency)
);

CREATE TABLE account_position_projection (
    account_id    TEXT        NOT NULL,
    asset_id      TEXT        NOT NULL,
    currency      TEXT,
    quantity      NUMERIC(38, 18) NOT NULL CHECK (quantity >= 0),
    cost_amount   NUMERIC(38, 18) NOT NULL,
    last_ledger_sequence BIGINT NOT NULL CHECK (last_ledger_sequence >= 0),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, asset_id, currency)
);

CREATE TABLE activity_control_projection (
    control_id          TEXT        PRIMARY KEY,
    control_version     INTEGER     NOT NULL CHECK (control_version >= 1),
    scope               TEXT        NOT NULL,
    action              TEXT        NOT NULL,
    strength            INTEGER     NOT NULL CHECK (strength >= 0),
    effective_from      TEXT        NOT NULL,
    expires_at          TEXT,
    source_decision_id  TEXT        NOT NULL,
    risk_policy_version TEXT        NOT NULL,
    status              TEXT        NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_activity_control_projection_scope
    ON activity_control_projection (scope);

-- ---------------------------------------------------------------------
-- 11. 不可变事实触发器：事实表禁止 UPDATE / DELETE
-- ---------------------------------------------------------------------
CREATE OR REPLACE FUNCTION prevent_fact_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION '事实表 % 不可变：禁止 UPDATE/DELETE', TG_TABLE_NAME
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DO $$
DECLARE
    immutable_table TEXT;
BEGIN
    FOREACH immutable_table IN ARRAY ARRAY[
        'fact_events',
        'inbox_records',
        'inbox_conflicts',
        'ledger_journals',
        'ledger_entries',
        'order_intents',
        'order_events',
        'cancel_order_requests',
        'replace_order_requests',
        'execution_reports',
        'risk_decisions',
        'trading_controls'
    ]
    LOOP
        EXECUTE format(
            'CREATE TRIGGER trg_%1$s_immutable BEFORE UPDATE OR DELETE ON %1$s '
            || 'FOR EACH ROW EXECUTE FUNCTION prevent_fact_mutation()',
            immutable_table
        );
    END LOOP;
END;
$$;

COMMIT;
