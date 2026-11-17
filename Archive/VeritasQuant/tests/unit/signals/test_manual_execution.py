"""P3-005 人工成交授权命令写入订单/账本测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.application.CommandResource import CommandService, CommandStatus
from veritasquant.signals.ManualExecution import (
    AuthorizedExecutionV1,
    InMemoryCommandStoreV1,
    ManualExecutionExecutorV1,
    RecordingOrderWriterV1,
)
from veritasquant.signals.ManualActionService import ManualActionError
from veritasquant.signals.SignalReference import (
    ManualExecutionV1,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _execution(**overrides: object) -> ManualExecutionV1:
    values: dict[str, object] = {
        "executionId": "exec-001",
        "signalReferenceId": "sig-ref-001",
        "actionId": "act-001",
        "operatorId": "op-alice",
        "executedAt": _T0 + timedelta(minutes=2),
        "direction": "BUY",
        "quantity": "100.0000",
        "price": "5.0100",
        "deviationReason": None,
        "note": "",
    }
    values.update(overrides)
    return ManualExecutionV1.create(**values)


class TestManualExecutionExecutor:
    def _setup(self) -> tuple[ManualExecutionExecutorV1, InMemoryCommandStoreV1, RecordingOrderWriterV1]:
        store = InMemoryCommandStoreV1()
        commandService = CommandService(store)
        writer = RecordingOrderWriterV1()
        executor = ManualExecutionExecutorV1(commandService, writer)
        return executor, store, writer

    def test_submit_authorized_command(self) -> None:
        executor, _, _ = self._setup()
        command, created = executor.submitAuthorizedCommand(
            execution=_execution(),
            runId="run-001",
            requestedBy="op-alice",
            accountId="acc-001",
            idempotencyKey="idem-001",
        )
        assert created is True
        assert command.commandType == "manual_execution"
        assert command.status is CommandStatus.Pending

    def test_submit_idempotent(self) -> None:
        executor, _, _ = self._setup()
        kwargs = {
            "execution": _execution(),
            "runId": "run-001",
            "requestedBy": "op-alice",
            "accountId": "acc-001",
            "idempotencyKey": "idem-001",
        }
        first, createdFirst = executor.submitAuthorizedCommand(**kwargs)
        second, createdSecond = executor.submitAuthorizedCommand(**kwargs)
        assert createdFirst is True
        assert createdSecond is False
        assert first.commandId == second.commandId

    def test_submit_conflict_different_payload(self) -> None:
        from veritasquant.application.CommandResource import IdempotencyConflict

        executor, _, _ = self._setup()
        kwargs = {
            "runId": "run-001",
            "requestedBy": "op-alice",
            "accountId": "acc-001",
            "idempotencyKey": "idem-001",
        }
        executor.submitAuthorizedCommand(execution=_execution(), **kwargs)
        with pytest.raises(IdempotencyConflict):
            executor.submitAuthorizedCommand(execution=_execution(quantity="200.0000"), **kwargs)

    def test_execute_after_authorize(self) -> None:
        executor, store, writer = self._setup()
        execution = _execution()
        command, _ = executor.submitAuthorizedCommand(
            execution=execution,
            runId="run-001",
            requestedBy="op-alice",
            accountId="acc-001",
            idempotencyKey="idem-001",
        )
        # 推进到 AUTHORIZING（人工审批后）——用 CommandStateMachineV1 转换
        from veritasquant.application.CommandResource import CommandStateMachineV1

        stateMachine = CommandStateMachineV1()
        authorized = store.update(stateMachine.transition(command, CommandStatus.Authorizing))
        result = executor.execute(authorized, execution)
        assert isinstance(result, AuthorizedExecutionV1)
        assert result.commandId == command.commandId
        assert len(writer.calls) == 1

    def test_execute_pending_rejected(self) -> None:
        """绕过命令资源：PENDING 状态不允许写入订单/账本。"""
        executor, _, writer = self._setup()
        execution = _execution()
        command, _ = executor.submitAuthorizedCommand(
            execution=execution,
            runId="run-001",
            requestedBy="op-alice",
            accountId="acc-001",
            idempotencyKey="idem-001",
        )
        with pytest.raises(ManualActionError, match="绕过命令资源"):
            executor.execute(command, execution)
        assert len(writer.calls) == 0

    def test_execute_wrong_command_type_rejected(self) -> None:
        executor, store, writer = self._setup()
        execution = _execution()
        command, _ = executor.submitAuthorizedCommand(
            execution=execution,
            runId="run-001",
            requestedBy="op-alice",
            accountId="acc-001",
            idempotencyKey="idem-001",
        )
        wrong = store.get(command.commandId)  # type: ignore[union-attr]
        from dataclasses import replace

        wrongType = replace(wrong, commandType="other")
        with pytest.raises(ManualActionError, match="manual_execution"):
            executor.execute(wrongType, execution)
        assert len(writer.calls) == 0

    def test_writer_rejects_direct_projection_write(self) -> None:
        """直接改投影的请求被拒绝：writer 只接受授权状态命令。"""
        executor, store, writer = self._setup()
        execution = _execution()
        command, _ = executor.submitAuthorizedCommand(
            execution=execution,
            runId="run-001",
            requestedBy="op-alice",
            accountId="acc-001",
            idempotencyKey="idem-001",
        )
        pending = store.get(command.commandId)  # type: ignore[union-attr]
        # 模拟直接以 PENDING 状态调用 writer（绕过 executor 校验）
        with pytest.raises(ManualActionError, match="绕过命令资源"):
            writer.writeFromAuthorizedCommand(pending, execution)
        assert len(writer.calls) == 0
