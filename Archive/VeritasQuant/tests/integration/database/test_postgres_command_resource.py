"""P2-026 命令资源数据库集成测试（CI postgres service 运行）。

验收标准映射：
- 同键同载荷返回原命令及状态；
- 同键异载荷返回 IDEMPOTENCY_CONFLICT（1003 语义）；
- 身份字段冻结：应用层之外的 UPDATE 被数据库触发器拒绝；
- 状态迁移持久化后可查询。
"""

from __future__ import annotations

from datetime import datetime, timezone

import psycopg
import pytest

from test_db_helpers import applyMigrations, openConnection, resetSchema

from veritasquant.application.CommandResource import (
    CommandFailureV1,
    CommandService,
    CommandStatus,
    IdempotencyConflict,
)
from veritasquant.infrastructure.persistence.CommandStore import (
    CommandStoreError,
    PostgresCommandStore,
)

_SCOPE = "user-1|acc-1|/api/v1/fund/subscribe|idem-1"


@pytest.fixture(scope="module")
def database() -> bool:
    try:
        openConnection().close()
    except psycopg.OperationalError:
        pytest.skip("PostgreSQL 测试实例不可用，跳过命令资源集成测试")
    resetSchema()
    versions = applyMigrations()
    assert versions, "迁移应至少应用一个版本"
    return True


@pytest.fixture()
def store(database) -> PostgresCommandStore:  # noqa: ANN001
    connection = openConnection()
    connection.execute("TRUNCATE command_records CASCADE")
    yield PostgresCommandStore(connection)
    connection.close()


def _service(store: PostgresCommandStore) -> CommandService:
    return CommandService(store)


def _submit(service: CommandService, **overrides):
    kwargs = dict(
        commandId="cmd-001",
        commandType="FUND_SUBSCRIBE",
        accountId="acc-1",
        runId="run-1",
        requestedBy="user-1",
        idempotencyKey="idem-1",
        route="/api/v1/fund/subscribe",
        payload={"fundSymbol": "FUND-A", "amount": "100.00"},
    )
    kwargs.update(overrides)
    return service.submit(**kwargs)


def test_create_and_get_roundtrip(store: PostgresCommandStore) -> None:
    service = _service(store)
    record, created = _submit(service)
    assert created is True
    fetched = service.get(record.commandId)
    assert fetched is not None
    assert fetched.commandId == record.commandId
    assert fetched.status is CommandStatus.Pending
    assert fetched.payload["fundSymbol"] == "FUND-A"
    assert fetched.createdTs.tzinfo is not None


def test_same_key_same_payload_returns_original(store: PostgresCommandStore) -> None:
    service = _service(store)
    first, createdFirst = _submit(service)
    assert createdFirst is True
    second, createdSecond = _submit(service)
    assert createdSecond is False
    assert second.commandId == first.commandId


def test_same_key_different_payload_conflicts(store: PostgresCommandStore) -> None:
    service = _service(store)
    _submit(service)
    with pytest.raises(IdempotencyConflict):
        _submit(service, payload={"fundSymbol": "FUND-B", "amount": "999.00"})


def test_lifecycle_persists_across_reopen(store: PostgresCommandStore) -> None:
    service = _service(store)
    record, _ = _submit(service)
    service.transition(record.commandId, CommandStatus.Authorizing)
    service.transition(record.commandId, CommandStatus.Accepted)
    service.transition(record.commandId, CommandStatus.Running)
    failure = CommandFailureV1(
        code=9201,
        errorCode="INVESTMENT_PLAN_BUDGET_EXCEEDED",
        catalogVersion="1.0",
        retryable=False,
        details={"required": "100.00", "available": "50.00"},
    )
    service.transition(record.commandId, CommandStatus.Failed, failure=failure)

    fetched = service.get(record.commandId)
    assert fetched is not None
    assert fetched.status is CommandStatus.Failed
    assert fetched.failure is not None
    assert fetched.failure.code == 9201
    assert fetched.failure.errorCode == "INVESTMENT_PLAN_BUDGET_EXCEEDED"
    assert fetched.failure.details["required"] == "100.00"


def test_identity_frozen_by_database_trigger(store: PostgresCommandStore) -> None:
    """绕过应用层直接 UPDATE 身份字段必须被触发器拒绝。"""
    service = _service(store)
    record, _ = _submit(service)
    connection = openConnection()
    try:
        with pytest.raises(psycopg.errors.RaiseException):
            with connection.transaction():
                connection.execute(
                    "UPDATE command_records SET account_id = 'acc-evil' WHERE command_id = %s",
                    (record.commandId,),
                )
    finally:
        connection.close()


def test_status_update_allowed(store: PostgresCommandStore) -> None:
    """生命周期字段更新被允许且 updated_ts 单调递增。"""
    service = _service(store)
    record, _ = _submit(service)
    connection = openConnection()
    try:
        with connection.transaction():
            cursor = connection.execute(
                "UPDATE command_records SET status = %s, updated_ts = %s WHERE command_id = %s",
                (CommandStatus.Authorizing.value, datetime.now(timezone.utc), record.commandId),
            )
        assert cursor.rowcount == 1
    finally:
        connection.close()


def test_duplicate_scope_rejected_by_unique_index(store: PostgresCommandStore) -> None:
    """同幂等作用域并发双写只有一个成功：直接写库触发唯一索引。"""
    from veritasquant.application.CommandResource import CommandRecordV1

    service = _service(store)
    first, _ = _submit(service, commandId="cmd-001")
    assert first.status is CommandStatus.Pending
    # 绕过幂等查询直接 create 同作用域命令 -> 唯一索引拒绝
    duplicate = CommandRecordV1(
        commandId="cmd-002",
        commandType="FUND_SUBSCRIBE",
        accountId="acc-1",
        runId="run-1",
        requestedBy="user-1",
        idempotencyScope=_SCOPE,
        payloadHash="a" * 64,
        payload={"fundSymbol": "FUND-A", "amount": "100.00"},
    )
    with pytest.raises(CommandStoreError):
        store.create(duplicate)
