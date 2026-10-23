"""P2-008 传输背压与连接状态策略（纯逻辑）。

- 背压：分区队列/流积压达到 70% 告警、90% 硬上限停止消费，不得丢弃事件；
- 重连：连接状态机 CONNECTED -> DISCONNECTED -> RECONNECTING -> CONNECTED，
  采用版本化指数退避；重连期间不丢已发布事件（消费从上次确认位点继续）。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class BackpressureLevel(StrEnum):
    Normal = "NORMAL"
    Warning = "WARNING"  # 达到 70%：告警并禁止新增风险
    Critical = "CRITICAL"  # 达到 90%：硬上限，停止该分区消费


@dataclass(frozen=True, slots=True)
class BackpressureStateV1:
    level: BackpressureLevel
    pendingCount: int
    capacity: int
    utilization: float  # 0.0 ~ 1.0


class StreamBackpressurePolicyV1:
    """按容量利用率判定背压等级。"""

    WARNING_RATIO = 0.7
    CRITICAL_RATIO = 0.9

    def __init__(self, capacity: int) -> None:
        if capacity <= 0:
            raise ValueError("队列容量必须为正")
        self._capacity = capacity

    def evaluate(self, pendingCount: int) -> BackpressureStateV1:
        if pendingCount < 0:
            raise ValueError("积压数量不得为负")
        utilization = pendingCount / self._capacity
        if utilization >= self.CRITICAL_RATIO:
            level = BackpressureLevel.Critical
        elif utilization >= self.WARNING_RATIO:
            level = BackpressureLevel.Warning
        else:
            level = BackpressureLevel.Normal
        return BackpressureStateV1(level, pendingCount, self._capacity, utilization)

    def canConsume(self, pendingCount: int) -> bool:
        """硬阈值时禁止新增风险/停止消费；不丢关键事件。"""
        return self.evaluate(pendingCount).level is not BackpressureLevel.Critical


class ConnectionState(StrEnum):
    Connected = "CONNECTED"
    Disconnected = "DISCONNECTED"
    Reconnecting = "RECONNECTING"


class StreamConnectionStateMachineV1:
    """传输连接状态机：断线进入重连，退避后恢复，不丢事件。"""

    def __init__(self, maxRetries: int = 5, baseBackoffSeconds: float = 1.0) -> None:
        if maxRetries <= 0 or baseBackoffSeconds <= 0:
            raise ValueError("重试次数与基础退避必须为正")
        self._state = ConnectionState.Connected
        self._attempt = 0
        self._maxRetries = maxRetries
        self._baseBackoffSeconds = baseBackoffSeconds

    @property
    def state(self) -> ConnectionState:
        return self._state

    def onDisconnect(self) -> None:
        """检测到断连：进入重连状态机。"""
        self._state = ConnectionState.Disconnected
        self._attempt = 0

    def nextBackoffSeconds(self) -> float:
        """版本化指数退避：base * 2^attempt（确定性）。"""
        if self._state is not ConnectionState.Reconnecting:
            self._state = ConnectionState.Reconnecting
        if self._attempt >= self._maxRetries:
            raise RuntimeError("重连超过最大重试次数")
        backoff = self._baseBackoffSeconds * (2**self._attempt)
        self._attempt += 1
        return backoff

    def onConnected(self) -> None:
        """重连成功：恢复 CONNECTED，消费从上次确认位点继续。"""
        self._state = ConnectionState.Connected
        self._attempt = 0
