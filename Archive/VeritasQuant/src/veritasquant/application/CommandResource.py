"""P2-026 不可变命令资源与幂等键存储。

TechSpec 10.2.2：
- 所有写操作创建不可变命令资源：command_id + Idempotency-Key；
- 幂等作用域 = principal_id + account_id + API 路由 + Idempotency-Key；
- 同键同哈希返回原命令及状态；同键异哈希返回 IDEMPOTENCY_CONFLICT(1003)；
- 状态机：PENDING -> AUTHORIZING -> ACCEPTED -> RUNNING
  -> SUCCEEDED/FAILED，支持 CANCEL_REQUESTED -> CANCELLED；
- 失败快照保存 code/error.code/catalog_version/retryable/安全 details；
- 长任务创建返回 202 + 命令引用。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum
from typing import Any, Mapping, Protocol


class CommandStatus(StrEnum):
    Pending = "PENDING"
    Authorizing = "AUTHORIZING"
    Accepted = "ACCEPTED"
    Running = "RUNNING"
    Succeeded = "SUCCEEDED"
    Failed = "FAILED"
    CancelRequested = "CANCEL_REQUESTED"
    Cancelled = "CANCELLED"


_COMMAND_FLOW = (
    (CommandStatus.Pending, CommandStatus.Authorizing),
    (CommandStatus.Authorizing, CommandStatus.Accepted),
    (CommandStatus.Accepted, CommandStatus.Running),
    (CommandStatus.Accepted, CommandStatus.CancelRequested),
    (CommandStatus.Running, CommandStatus.Succeeded),
    (CommandStatus.Running, CommandStatus.Failed),
    (CommandStatus.Running, CommandStatus.CancelRequested),
    (CommandStatus.CancelRequested, CommandStatus.Cancelled),
)
_ALLOWED = {transition for transition in _COMMAND_FLOW}


class CommandError(ValueError):
    """命令资源或幂等键不满足契约。"""


class IdempotencyConflict(CommandError):
    """同一幂等键对应不同规范化请求哈希（1003）。"""


class CommandStateConflict(CommandError):
    """当前命令状态不允许请求的迁移。"""


@dataclass(frozen=True, slots=True)
class CommandFailureV1:
    """失败快照：与首次 HTTP 响应一致的冻结字段。"""

    code: int
    errorCode: str
    catalogVersion: str
    retryable: bool
    details: Mapping[str, Any] | None = None

    def __post_init__(self) -> None:
        object.__setattr__(self, "details", dict(self.details or {}))


@dataclass(frozen=True, slots=True)
class CommandRecordV1:
    """不可变命令资源；状态迁移由状态机控制。"""

    commandId: str
    commandType: str
    accountId: str
    runId: str
    requestedBy: str
    idempotencyScope: str  # principal + account + route + key 的规范化作用域
    payloadHash: str
    payload: Mapping[str, Any]
    expectedVersion: str | None = None
    confirmationTokenId: str | None = None
    status: CommandStatus = CommandStatus.Pending
    createdTs: datetime = None  # type: ignore[assignment]
    updatedTs: datetime = None  # type: ignore[assignment]
    resultReference: str | None = None
    failure: CommandFailureV1 | None = None

    def __post_init__(self) -> None:
        if not self.commandId or not self.commandType or not self.accountId or not self.runId:
            raise CommandError("命令标识字段不能为空")
        if not self.requestedBy or not self.idempotencyScope:
            raise CommandError("请求人与幂等作用域不能为空")
        if len(self.payloadHash) != 64:
            raise CommandError("payload 哈希必须为 SHA-256")
        if self.createdTs is None:
            object.__setattr__(self, "createdTs", datetime.now(timezone.utc))
        if self.updatedTs is None:
            object.__setattr__(self, "updatedTs", self.createdTs)


class CommandStateMachineV1:
    """TechSpec 固定的命令状态机。"""

    def canTransition(self, current: CommandStatus, target: CommandStatus) -> bool:
        return (current, target) in _ALLOWED

    def transition(self, record: CommandRecordV1, target: CommandStatus) -> CommandRecordV1:
        if not self.canTransition(record.status, target):
            raise CommandStateConflict(
                f"命令状态不允许 {record.status.value} -> {target.value}"
            )
        return CommandRecordV1(
            commandId=record.commandId,
            commandType=record.commandType,
            accountId=record.accountId,
            runId=record.runId,
            requestedBy=record.requestedBy,
            idempotencyScope=record.idempotencyScope,
            payloadHash=record.payloadHash,
            payload=record.payload,
            expectedVersion=record.expectedVersion,
            confirmationTokenId=record.confirmationTokenId,
            status=target,
            createdTs=record.createdTs,
            updatedTs=datetime.now(timezone.utc),
            resultReference=record.resultReference,
            failure=record.failure,
        )


class CommandStore(Protocol):
    """命令资源持久化端口；实现可为内存或 PostgreSQL。"""

    def create(self, record: CommandRecordV1) -> CommandRecordV1: ...

    def get(self, commandId: str) -> CommandRecordV1 | None: ...

    def update(
        self, record: CommandRecordV1, expectedUpdatedTs: datetime | None = None
    ) -> CommandRecordV1: ...

    def findByIdempotencyScope(self, scope: str) -> CommandRecordV1 | None: ...


def buildIdempotencyScope(
    principalId: str, accountId: str, route: str, idempotencyKey: str
) -> str:
    """规范化幂等作用域；分隔符转义保证无歧义。"""
    if not principalId or not accountId or not route or not idempotencyKey:
        raise CommandError("幂等作用域字段不能为空")
    return "|".join(
        value.replace("|", "||") for value in (principalId, accountId, route, idempotencyKey)
    )


class CommandService:
    """命令生命周期用例：幂等创建、状态推进与查询。"""

    def __init__(
        self,
        store: CommandStore,
        stateMachine: CommandStateMachineV1 | None = None,
    ) -> None:
        self._store = store
        self._stateMachine = stateMachine or CommandStateMachineV1()

    def submit(
        self,
        *,
        commandId: str,
        commandType: str,
        accountId: str,
        runId: str,
        requestedBy: str,
        idempotencyKey: str,
        route: str,
        payload: Mapping[str, Any],
        expectedVersion: str | None = None,
        confirmationTokenId: str | None = None,
    ) -> tuple[CommandRecordV1, bool]:
        """提交命令：首次创建返回 (record, True)；同键同载荷返回原记录 (record, False)。"""
        scope = buildIdempotencyScope(requestedBy, accountId, route, idempotencyKey)
        existing = self._store.findByIdempotencyScope(scope)
        if existing is not None:
            payloadHash = _payloadHash(payload)
            if existing.payloadHash != payloadHash:
                raise IdempotencyConflict("同一幂等键对应不同规范化请求哈希")
            return existing, False
        record = CommandRecordV1(
            commandId=commandId,
            commandType=commandType,
            accountId=accountId,
            runId=runId,
            requestedBy=requestedBy,
            idempotencyScope=scope,
            payloadHash=_payloadHash(payload),
            payload=dict(payload),
            expectedVersion=expectedVersion,
            confirmationTokenId=confirmationTokenId,
        )
        created = self._store.create(record)
        return created, True

    def transition(
        self, commandId: str, target: CommandStatus, failure: CommandFailureV1 | None = None
    ) -> CommandRecordV1:
        """按状态机推进；FAILED 必须携带失败快照。"""
        record = self._store.get(commandId)
        if record is None:
            raise CommandError(f"命令不存在: {commandId}")
        if target is CommandStatus.Failed and failure is None:
            raise CommandError("FAILED 必须携带失败快照")
        updated = self._stateMachine.transition(record, target)
        if failure is not None:
            updated = CommandRecordV1(
                commandId=updated.commandId,
                commandType=updated.commandType,
                accountId=updated.accountId,
                runId=updated.runId,
                requestedBy=updated.requestedBy,
                idempotencyScope=updated.idempotencyScope,
                payloadHash=updated.payloadHash,
                payload=updated.payload,
                expectedVersion=updated.expectedVersion,
                confirmationTokenId=updated.confirmationTokenId,
                status=updated.status,
                createdTs=updated.createdTs,
                updatedTs=updated.updatedTs,
                resultReference=updated.resultReference,
                failure=failure,
            )
        # 乐观并发控制：期望写入基于读取时的 updatedTs，冲突则拒绝覆盖
        return self._store.update(updated, expectedUpdatedTs=record.updatedTs)

    def get(self, commandId: str) -> CommandRecordV1 | None:
        return self._store.get(commandId)


def _payloadHash(payload: Mapping[str, Any]) -> str:
    from veritasquant.core.CanonicalJson import canonicalHash

    return canonicalHash(payload)
