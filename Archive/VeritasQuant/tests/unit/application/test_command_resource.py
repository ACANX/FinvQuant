"""P2-026 命令资源状态机、幂等键与不可变契约测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.application.CommandResource import (
    CommandError,
    CommandFailureV1,
    CommandRecordV1,
    CommandService,
    CommandStateConflict,
    CommandStateMachineV1,
    CommandStatus,
    CommandStore,
    IdempotencyConflict,
    buildIdempotencyScope,
)


class InMemoryCommandStore(CommandStore):
    """测试用内存实现；作用域唯一，身份冻结由模型层保证。"""

    def __init__(self) -> None:
        self._records: dict[str, CommandRecordV1] = {}
        self._byScope: dict[str, str] = {}

    def create(self, record: CommandRecordV1) -> CommandRecordV1:
        if record.commandId in self._records:
            raise CommandError(f"命令已存在: {record.commandId}")
        if record.idempotencyScope in self._byScope:
            raise CommandError("幂等作用域重复")
        self._records[record.commandId] = record
        self._byScope[record.idempotencyScope] = record.commandId
        return record

    def get(self, commandId: str) -> CommandRecordV1 | None:
        return self._records.get(commandId)

    def update(self, record: CommandRecordV1, expectedUpdatedTs=None) -> CommandRecordV1:
        existing = self._records.get(record.commandId)
        if existing is None:
            raise CommandError(f"命令不存在: {record.commandId}")
        if existing.idempotencyScope != record.idempotencyScope:
            raise CommandError("身份字段不可变")
        if expectedUpdatedTs is not None and existing.updatedTs != expectedUpdatedTs:
            raise CommandError(f"命令并发版本冲突: {record.commandId}")
        self._records[record.commandId] = record
        return record

    def findByIdempotencyScope(self, scope: str) -> CommandRecordV1 | None:
        commandId = self._byScope.get(scope)
        return self._records.get(commandId) if commandId else None


def _store() -> InMemoryCommandStore:
    return InMemoryCommandStore()


def _service(store: CommandStore | None = None) -> CommandService:
    return CommandService(store or _store())


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


class TestCommandStateMachine:
    def test_valid_flow(self) -> None:
        machine = CommandStateMachineV1()
        assert machine.canTransition(CommandStatus.Pending, CommandStatus.Authorizing)
        assert machine.canTransition(CommandStatus.Authorizing, CommandStatus.Accepted)
        assert machine.canTransition(CommandStatus.Accepted, CommandStatus.Running)
        assert machine.canTransition(CommandStatus.Running, CommandStatus.Succeeded)
        assert machine.canTransition(CommandStatus.Running, CommandStatus.Failed)

    def test_cancel_flow(self) -> None:
        machine = CommandStateMachineV1()
        assert machine.canTransition(CommandStatus.Accepted, CommandStatus.CancelRequested)
        assert machine.canTransition(CommandStatus.CancelRequested, CommandStatus.Cancelled)

    def test_invalid_transition_rejected(self) -> None:
        machine = CommandStateMachineV1()
        assert not machine.canTransition(CommandStatus.Pending, CommandStatus.Succeeded)
        assert not machine.canTransition(CommandStatus.Succeeded, CommandStatus.Running)
        assert not machine.canTransition(CommandStatus.Cancelled, CommandStatus.Running)


class TestCommandServiceSubmit:
    def test_first_submit_creates(self) -> None:
        service = _service()
        record, created = _submit(service)
        assert created is True
        assert record.status is CommandStatus.Pending
        assert len(record.payloadHash) == 64

    def test_same_key_same_payload_returns_original(self) -> None:
        service = _service()
        first, createdFirst = _submit(service)
        assert createdFirst is True
        second, createdSecond = _submit(service)
        assert createdSecond is False
        assert second.commandId == first.commandId
        assert second.status is CommandStatus.Pending

    def test_same_key_different_payload_conflicts(self) -> None:
        service = _service()
        _submit(service)
        with pytest.raises(IdempotencyConflict):
            _submit(service, payload={"fundSymbol": "FUND-B", "amount": "999.00"})

    def test_different_key_different_command(self) -> None:
        service = _service()
        first, _ = _submit(service)
        second, createdSecond = _submit(service, commandId="cmd-002", idempotencyKey="idem-2")
        assert createdSecond is True
        assert second.commandId != first.commandId

    def test_scope_encoding_escapes_separator(self) -> None:
        scope = buildIdempotencyScope("a|b", "acc", "/route", "key")
        assert "||" in scope
        # 不同字段组合不会碰撞
        other = buildIdempotencyScope("a", "b|acc", "/route", "key")
        assert scope != other


class TestCommandLifecycle:
    def test_full_flow_to_succeeded(self) -> None:
        service = _service()
        record, _ = _submit(service)
        record = service.transition(record.commandId, CommandStatus.Authorizing)
        record = service.transition(record.commandId, CommandStatus.Accepted)
        record = service.transition(record.commandId, CommandStatus.Running)
        record = service.transition(record.commandId, CommandStatus.Succeeded)
        assert record.status is CommandStatus.Succeeded
        assert record.updatedTs >= record.createdTs

    def test_failure_requires_snapshot(self) -> None:
        service = _service()
        record, _ = _submit(service)
        record = service.transition(record.commandId, CommandStatus.Authorizing)
        record = service.transition(record.commandId, CommandStatus.Accepted)
        record = service.transition(record.commandId, CommandStatus.Running)
        with pytest.raises(CommandError):
            service.transition(record.commandId, CommandStatus.Failed)  # 缺快照

    def test_failure_snapshot_preserved(self) -> None:
        service = _service()
        record, _ = _submit(service)
        record = service.transition(record.commandId, CommandStatus.Authorizing)
        record = service.transition(record.commandId, CommandStatus.Accepted)
        record = service.transition(record.commandId, CommandStatus.Running)
        failure = CommandFailureV1(
            code=9201,
            errorCode="INVESTMENT_PLAN_BUDGET_EXCEEDED",
            catalogVersion="1.0",
            retryable=False,
            details={"required": "100.00", "available": "50.00"},
        )
        record = service.transition(record.commandId, CommandStatus.Failed, failure=failure)
        assert record.status is CommandStatus.Failed
        assert record.failure is not None
        assert record.failure.code == 9201
        assert record.failure.retryable is False
        assert record.failure.details["required"] == "100.00"

    def test_cancel_flow(self) -> None:
        service = _service()
        record, _ = _submit(service)
        record = service.transition(record.commandId, CommandStatus.Authorizing)
        record = service.transition(record.commandId, CommandStatus.Accepted)
        record = service.transition(record.commandId, CommandStatus.CancelRequested)
        record = service.transition(record.commandId, CommandStatus.Cancelled)
        assert record.status is CommandStatus.Cancelled

    def test_invalid_transition_raises(self) -> None:
        service = _service()
        record, _ = _submit(service)
        with pytest.raises(CommandStateConflict):
            service.transition(record.commandId, CommandStatus.Succeeded)

    def test_concurrent_update_conflict_does_not_overwrite(self) -> None:
        """并发冲突不覆盖：基于读取时 updatedTs 的乐观锁拒绝陈旧写入。"""
        store = _store()
        service = _service(store)
        record, _ = _submit(service)
        # 两个并发读都拿到 PENDING 基线
        firstRead = service.get(record.commandId)
        assert firstRead is not None
        # 第一个写入推进到 AUTHORIZING
        service.transition(record.commandId, CommandStatus.Authorizing)
        # 第二个基于陈旧基线（PENDING 的 updatedTs）推进 -> 拒绝
        stale = CommandRecordV1(
            commandId=firstRead.commandId,
            commandType=firstRead.commandType,
            accountId=firstRead.accountId,
            runId=firstRead.runId,
            requestedBy=firstRead.requestedBy,
            idempotencyScope=firstRead.idempotencyScope,
            payloadHash=firstRead.payloadHash,
            payload=firstRead.payload,
            expectedVersion=firstRead.expectedVersion,
            confirmationTokenId=firstRead.confirmationTokenId,
            status=CommandStatus.Authorizing,
            createdTs=firstRead.createdTs,
            updatedTs=firstRead.updatedTs,
        )
        with pytest.raises(CommandError, match="并发版本冲突"):
            store.update(stale, expectedUpdatedTs=firstRead.updatedTs)
        # 当前状态未被覆盖
        current = service.get(record.commandId)
        assert current is not None
        assert current.status is CommandStatus.Authorizing
        assert current.updatedTs > firstRead.updatedTs

    def test_get_unknown_command_returns_none(self) -> None:
        service = _service()
        assert service.get("missing") is None


class TestCommandRecordValidation:
    def test_empty_fields_rejected(self) -> None:
        with pytest.raises(CommandError):
            CommandRecordV1(
                commandId="",
                commandType="FUND_SUBSCRIBE",
                accountId="acc-1",
                runId="run-1",
                requestedBy="user-1",
                idempotencyScope="scope",
                payloadHash="a" * 64,
                payload={},
            )

    def test_bad_payload_hash_rejected(self) -> None:
        with pytest.raises(CommandError):
            CommandRecordV1(
                commandId="cmd-1",
                commandType="FUND_SUBSCRIBE",
                accountId="acc-1",
                runId="run-1",
                requestedBy="user-1",
                idempotencyScope="scope",
                payloadHash="short",
                payload={},
            )

    def test_decimal_payload_canonicalized(self) -> None:
        """Decimal 负载规范化后哈希稳定（100.00 与 100.0 同值同哈希）。"""
        service = _service()
        first, _ = _submit(service, payload={"amount": Decimal("100.00")})
        second, created = _submit(
            service,
            commandId="cmd-2",
            idempotencyKey="idem-2",
            payload={"amount": Decimal("100.0")},
        )
        assert created is True
        assert first.payloadHash == second.payloadHash  # Decimal 同值同规范哈希


def test_created_ts_defaults_to_utc() -> None:
    record = CommandRecordV1(
        commandId="cmd-1",
        commandType="FUND_SUBSCRIBE",
        accountId="acc-1",
        runId="run-1",
        requestedBy="user-1",
        idempotencyScope="scope",
        payloadHash="a" * 64,
        payload={},
    )
    assert record.createdTs.tzinfo is not None
    assert record.updatedTs == record.createdTs
