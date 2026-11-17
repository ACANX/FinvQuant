"""P3-005 人工成交通过授权命令写入订单/账本。

契约（P3-005 验收标准）：
1. 对账一致 —— 人工成交只通过命令资源（CommandResource）创建订单意图，
   由订单/账本域消费命令执行写入；不允许绕过命令资源直接修改投影；
2. 绕过命令资源或直接改投影的请求被拒绝 —— `ManualExecutionExecutorV1`
   只接受已提交且状态合法的命令记录，任何直接调用写入投影的入口抛错。

流程：人工成交登记（P3-004） -> 创建授权命令 -> 命令执行器生成订单意图
      -> 订单/账本域消费（由现有 OMS 链路处理）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Mapping, Protocol

from veritasquant.application.CommandResource import (
    CommandRecordV1,
    CommandService,
    CommandStatus,
)
from veritasquant.signals.ManualActionService import ManualActionError
from veritasquant.signals.SignalReference import ManualExecutionV1

_MANUAL_EXECUTION_ROUTE = "/api/v1/signals/{signal_id}/executions/authorize"


def _utcNowMs() -> datetime:
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


@dataclass(frozen=True, slots=True)
class AuthorizedExecutionV1:
    """授权命令执行结果。"""

    commandId: str
    executionId: str
    signalReferenceId: str
    accountId: str
    status: CommandStatus


class OrderWriter(Protocol):
    """订单写入端口：由订单/账本域实现；仅接受授权命令上下文的调用。"""

    def writeFromAuthorizedCommand(self, command: CommandRecordV1, execution: ManualExecutionV1) -> str: ...


class ManualExecutionExecutorV1:
    """人工成交授权命令执行器。

    只接受已通过命令资源提交且状态为 AUTHORIZING 的命令；直接传入投影
    写入意图会被拒绝（绕过命令资源 = 契约违规）。
    """

    def __init__(self, commandService: CommandService, orderWriter: OrderWriter) -> None:
        if commandService is None or orderWriter is None:
            raise ManualActionError("命令服务与订单写入器不能为空")
        self._commandService = commandService
        self._orderWriter = orderWriter
        self._counter = 0
        self._written: dict[str, str] = {}

    def submitAuthorizedCommand(
        self,
        *,
        execution: ManualExecutionV1,
        runId: str,
        requestedBy: str,
        accountId: str,
        idempotencyKey: str,
        expectedVersion: str | None = None,
    ) -> tuple[CommandRecordV1, bool]:
        """创建授权命令资源（幂等）；返回 (命令记录, 是否新建)。"""
        self._counter += 1
        commandId = f"cmd-manual-{self._counter:06d}"
        payload: dict[str, Any] = {
            "execution_id": execution.executionId,
            "signal_reference_id": execution.signalReferenceId,
            "action_id": execution.actionId,
            "operator_id": execution.operatorId,
            "direction": execution.direction,
            "quantity": execution.quantity,
            "price": execution.price,
            "deviation_reason": (
                {
                    "reason_code": execution.deviationReason.reasonCode,
                    "detail": execution.deviationReason.detail,
                }
                if execution.deviationReason is not None
                else None
            ),
        }
        route = _MANUAL_EXECUTION_ROUTE.format(signal_id=execution.signalReferenceId)
        return self._commandService.submit(
            commandId=commandId,
            commandType="manual_execution",
            accountId=accountId,
            runId=runId,
            requestedBy=requestedBy,
            idempotencyKey=idempotencyKey,
            route=route,
            payload=payload,
            expectedVersion=expectedVersion,
        )

    def execute(self, command: CommandRecordV1, execution: ManualExecutionV1) -> AuthorizedExecutionV1:
        """执行授权命令：必须携带合法命令资源，禁止绕过命令资源直接写投影。

        命令状态必须是 AUTHORIZING/ACCEPTED 之后才能写入订单；
        直接以任意状态调用写入会被拒绝（绕过命令资源 = 契约违规）。
        """
        if command.commandType != "manual_execution":
            raise ManualActionError("只有 manual_execution 命令可以执行人工成交")
        if command.status not in (CommandStatus.Authorizing, CommandStatus.Accepted, CommandStatus.Running):
            raise ManualActionError(
                f"绕过命令资源：命令状态 {command.status.value} 不允许写入订单/账本"
            )
        resultReference = self._orderWriter.writeFromAuthorizedCommand(command, execution)
        self._written[command.commandId] = resultReference
        return AuthorizedExecutionV1(
            commandId=command.commandId,
            executionId=execution.executionId,
            signalReferenceId=execution.signalReferenceId,
            accountId=command.accountId,
            status=command.status,
        )

    def writtenReferences(self) -> Mapping[str, str]:
        return dict(self._written)


class RecordingOrderWriterV1:
    """测试/演示用订单写入器：只接受授权命令上下文，记录调用。"""

    def __init__(self) -> None:
        self._calls: list[tuple[str, str]] = []

    def writeFromAuthorizedCommand(self, command: CommandRecordV1, execution: ManualExecutionV1) -> str:
        if command.status not in (CommandStatus.Authorizing, CommandStatus.Accepted, CommandStatus.Running):
            raise ManualActionError("绕过命令资源：写入被拒绝")
        reference = f"order-{command.commandId}"
        self._calls.append((command.commandId, execution.executionId))
        return reference

    @property
    def calls(self) -> tuple[tuple[str, str], ...]:
        return tuple(self._calls)


class InMemoryCommandStoreV1:
    """内存命令资源存储（测试用，与 CommandService 配套）。"""

    def __init__(self) -> None:
        self._records: dict[str, CommandRecordV1] = {}
        self._byScope: dict[str, str] = {}

    def create(self, record: CommandRecordV1) -> CommandRecordV1:
        if record.commandId in self._records:
            raise ManualActionError(f"命令已存在: {record.commandId}")
        self._records[record.commandId] = record
        self._byScope[record.idempotencyScope] = record.commandId
        return record

    def get(self, commandId: str) -> CommandRecordV1 | None:
        return self._records.get(commandId)

    def update(self, record: CommandRecordV1, expectedUpdatedTs: datetime | None = None) -> CommandRecordV1:
        self._records[record.commandId] = record
        return record

    def findByIdempotencyScope(self, scope: str) -> CommandRecordV1 | None:
        commandId = self._byScope.get(scope)
        if commandId is None:
            return None
        return self._records.get(commandId)
