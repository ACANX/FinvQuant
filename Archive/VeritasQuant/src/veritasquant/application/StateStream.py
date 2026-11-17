"""P2-030 鉴权 SSE 状态流与有界 replay cursor（应用层模型与端口）。

SSE 使用各自版本化协议（不套用 JSON 信封，TechSpec 10.2）。
本模块定义流事件模型、有界 replay 语义与订阅状态机，不依赖
FastAPI/SSE 传输；HTTP 接线在 apps.server.StateStreamRoutes。
"""

from __future__ import annotations

import time
from dataclasses import dataclass, field
from enum import Enum
from typing import Protocol

# SSE 流协议版本（TechSpec：SSE 使用各自版本化协议）
SSE_PROTOCOL_VERSION = "v1"


class StreamEventType(Enum):
    """可订阅的领域状态事件类型。"""

    CommandStatus = "command.status"
    BacktestStatus = "backtest.status"
    FundNavPublished = "fund.nav_published"
    AccountSnapshot = "account.snapshot"
    Alert = "alert.updated"


class StreamCloseReason(Enum):
    """服务端主动断开原因（TechSpec 10.2.3）。"""

    BacklogExceeded = "backlog_exceeded"  # 积压超限明确关闭
    PermissionRevoked = "permission_revoked"  # 权限撤销立即断开
    TokenExpired = "token_expired"  # 令牌过期
    Shutdown = "shutdown"  # 服务端停机


@dataclass(frozen=True, slots=True)
class StreamEventV1:
    """一条 SSE 状态事件（wire 为 `event: <type>\\ndata: <json>\\n`）。"""

    sequence: int  # 单调递增序列号（replay cursor 的基础）
    eventType: StreamEventType
    accountId: str
    payload: dict[str, object]
    occurredAtIso: str


@dataclass(frozen=True, slots=True)
class ReplayCursor:
    """有界 replay cursor：客户端从指定序列号恢复。"""

    sequence: int

    @classmethod
    def parse(cls, value: str | None) -> "ReplayCursor":
        """解析客户端 cursor；缺失/非法回退到最新（不重放旧事件）。"""
        if not value:
            return cls(sequence=-1)
        try:
            return cls(sequence=int(value))
        except ValueError:
            return cls(sequence=-1)


@dataclass(frozen=True, slots=True)
class StreamOpenResult:
    """SSE 连接建立结果：事件流起点 + 断开关闭原因（若有）。"""

    subscriptionId: str
    events: tuple[StreamEventV1, ...]  # 从 cursor 之后可立即回放的事件
    nextSequence: int
    closeReason: StreamCloseReason | None = None
    message: str = ""


class StreamSubscriptionState(Enum):
    """订阅生命周期（服务端视角）。"""

    Active = "ACTIVE"
    Closed = "CLOSED"  # 服务端已断开（含原因）


@dataclass(slots=True)
class StreamSubscription:
    """一个活跃订阅：主体、账户过滤、cursor 与积压水位。"""

    subscriptionId: str
    principalId: str
    accountIds: frozenset[str]  # 空 = 全账户（管理员）
    cursor: int  # 已投递的最大 sequence
    createdAtTs: float
    state: StreamSubscriptionState = StreamSubscriptionState.Active
    closeReason: StreamCloseReason | None = None
    pending: list[StreamEventV1] = field(default_factory=list)


class StreamEventSource(Protocol):
    """流事件源：查询指定账户在 cursor 之后的事件。"""

    def eventsSince(
        self, accountIds: frozenset[str], cursor: int, limit: int
    ) -> tuple[StreamEventV1, ...]: ...

    def latestSequence(self) -> int: ...

    def nextSequence(self) -> int: ...

    def append(self, event: StreamEventV1) -> None: ...


class InMemoryStreamEventSource:
    """进程内事件源（模拟盘默认）；测试可注入历史事件。"""

    def __init__(self, retentionLimit: int = 1000) -> None:
        self._events: list[StreamEventV1] = []
        self._retentionLimit = retentionLimit
        self._lock = _threadingLock()

    def append(self, event: StreamEventV1) -> None:
        with self._lock:
            self._events.append(event)
            # 有界保留：超出窗口丢弃最旧事件（replay 边界）
            if len(self._events) > self._retentionLimit:
                del self._events[: len(self._events) - self._retentionLimit]

    def eventsSince(
        self, accountIds: frozenset[str], cursor: int, limit: int = 100
    ) -> tuple[StreamEventV1, ...]:
        with self._lock:
            matched = [
                event
                for event in self._events
                if event.sequence > cursor and (not accountIds or event.accountId in accountIds)
            ]
            return tuple(matched[:limit])

    def latestSequence(self) -> int:
        with self._lock:
            return self._events[-1].sequence if self._events else 0

    def nextSequence(self) -> int:
        with self._lock:
            return (self._events[-1].sequence + 1) if self._events else 1


def _threadingLock():
    import threading

    return threading.Lock()


class StreamService:
    """订阅用例：鉴权、replay、积压控制与主动断开。"""

    def __init__(
        self,
        eventSource: StreamEventSource,
        maxBacklog: int = 500,
        replayWindow: int = 1000,
    ) -> None:
        self._source = eventSource
        self._maxBacklog = maxBacklog
        self._replayWindow = replayWindow
        self._subscriptions: dict[str, StreamSubscription] = {}
        self._lock = _threadingLock()
        self._nextSubId = 1

    def open(
        self,
        principalId: str,
        accountIds: frozenset[str],
        cursorValue: str | None,
    ) -> StreamOpenResult:
        """建立订阅：校验 cursor 边界，回放已保留事件。"""
        cursor = ReplayCursor.parse(cursorValue)
        with self._lock:
            latest = self._source.latestSequence()
            # 有界 replay：cursor 超出保留窗口 -> 明确告知客户端（close with reason）
            if cursor.sequence >= 0 and cursor.sequence < latest - self._replayWindow:
                return StreamOpenResult(
                    subscriptionId="",
                    events=(),
                    nextSequence=latest + 1,
                    closeReason=StreamCloseReason.BacklogExceeded,
                    message=f"cursor {cursor.sequence} 超出 replay 窗口 {self._replayWindow}",
                )
            events = self._source.eventsSince(accountIds, cursor.sequence, limit=self._maxBacklog) if cursor.sequence >= 0 else ()
            subId = f"sub_{self._nextSubId}"
            self._nextSubId += 1
            self._subscriptions[subId] = StreamSubscription(
                subscriptionId=subId,
                principalId=principalId,
                accountIds=accountIds,
                cursor=cursor.sequence,
                createdAtTs=time.time(),
            )
            return StreamOpenResult(
                subscriptionId=subId,
                events=events,
                nextSequence=self._source.nextSequence(),
            )

    def deliver(self, event: StreamEventV1) -> None:
        """向匹配订阅投递事件；积压超限的订阅标记关闭。"""
        with self._lock:
            self._source.append(event)
            closed: list[str] = []
            for subId, sub in self._subscriptions.items():
                if sub.state is not StreamSubscriptionState.Active:
                    continue
                if sub.accountIds and event.accountId not in sub.accountIds:
                    continue
                sub.pending.append(event)
                sub.cursor = event.sequence
                if len(sub.pending) > self._maxBacklog:
                    sub.state = StreamSubscriptionState.Closed
                    sub.closeReason = StreamCloseReason.BacklogExceeded
                    closed.append(subId)
            return None

    def revokePrincipal(self, principalId: str, reason: StreamCloseReason = StreamCloseReason.PermissionRevoked) -> int:
        """权限撤销：立即断开该主体全部订阅（验收标准 2）。"""
        with self._lock:
            count = 0
            for sub in self._subscriptions.values():
                if sub.principalId == principalId and sub.state is StreamSubscriptionState.Active:
                    sub.state = StreamSubscriptionState.Closed
                    sub.closeReason = reason
                    count += 1
            return count

    def takePending(self, subscriptionId: str) -> tuple[StreamEventV1, ...] | None:
        """拉取订阅待投递事件；返回 None 表示订阅已关闭。"""
        with self._lock:
            sub = self._subscriptions.get(subscriptionId)
            if sub is None:
                return None
            if sub.state is StreamSubscriptionState.Closed:
                return None
            events = tuple(sub.pending)
            sub.pending.clear()
            return events

    def close(self, subscriptionId: str) -> None:
        with self._lock:
            self._subscriptions.pop(subscriptionId, None)

    def subscriptionCloseReason(self, subscriptionId: str) -> StreamCloseReason | None:
        with self._lock:
            sub = self._subscriptions.get(subscriptionId)
            return sub.closeReason if sub else None
