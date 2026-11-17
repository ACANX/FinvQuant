"""P2-036 指标采集器：把交易内核、API、队列与日志状态映射为 Prometheus 指标。

采集范围（对齐 TechSpec 12.3 与 P2-036 验收标准）：
- readiness：三层健康门禁（liveness/readiness/trading-readiness）状态；
- 事件延迟：行情 ingested_at -> 分区提交延迟直方图；
- 订单：订单状态机计数、拒绝/撤单/未知结果计数；
- 账本：事务提交延迟直方图与提交计数；
- outbox：最老未确认年龄与未确认条数（gauge）；
- 队列：背压利用率、连接状态；
- 错误码：API 业务错误码计数；
- 日志降级：结构化日志丢弃/降级计数。

采集器只读复制状态，绝不修改交易状态或作出交易决定（TechSpec 3.1 虚线旁路）。
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Any, Protocol

from veritasquant.monitoring.PrometheusMetrics import MetricsRegistry


class ReadinessProbeSource(Protocol):
    """trading-readiness 门禁只读视图。"""

    def ready(self) -> bool: ...
    def state(self) -> Any: ...
    def failedChecks(self) -> tuple[Any, ...]: ...


class OutboxProbe(Protocol):
    """outbox 只读视图：最老未确认年龄（秒）与未确认条数。"""

    def oldestUnconfirmedAgeSeconds(self) -> float | None: ...
    def unconfirmedCount(self) -> int: ...


class QueueProbe(Protocol):
    """队列/背压只读视图。"""

    def utilization(self) -> float: ...
    def connected(self) -> bool: ...
    def pendingCount(self) -> int: ...


@dataclass(frozen=True, slots=True)
class CollectorInputs:
    """采集器依赖的只读探针；均为可选，缺省时输出零值或跳过。"""

    readiness: ReadinessProbeSource | None = None
    outbox: OutboxProbe | None = None
    queue: QueueProbe | None = None
    logDegraded: bool = False
    logDroppedCount: int = 0
    errorCodeCounts: dict[int, int] | None = None


class MetricsCollector:
    """把探针状态写入注册表；导出由外部调用 registry.render()。"""

    def __init__(self, registry: MetricsRegistry) -> None:
        self.registry = registry
        # readiness 状态机：TRADING_READY / DEGRADED / PROTECTED / NOT_READY
        self._readinessState = registry.gauge(
            "trading_readiness_state", "trading-readiness 门禁状态（1=就绪，0=未就绪）"
        )
        self._readinessChecksFailed = registry.gauge(
            "readiness_checks_failed", "未通过的 readiness 检查数"
        )
        self._eventIngestLatency = registry.histogram(
            "event_ingest_latency_seconds", "行情 ingested_at -> 分区提交延迟（秒）"
        )
        self._orderStateTransitions = registry.counter(
            "order_state_transitions_total", "订单状态机转移计数（按目标状态）"
        )
        self._orderRejections = registry.counter(
            "order_rejections_total", "订单拒绝计数（按原因码）"
        )
        self._ledgerCommitLatency = registry.histogram(
            "ledger_commit_latency_seconds", "账本事务提交延迟（秒）"
        )
        self._ledgerCommits = registry.counter(
            "ledger_commits_total", "账本事务提交计数（按账户组）"
        )
        self._outboxOldestAge = registry.gauge(
            "outbox_oldest_unconfirmed_age_seconds", "outbox 最老未确认年龄（秒）"
        )
        self._outboxUnconfirmed = registry.gauge(
            "outbox_unconfirmed_count", "outbox 未确认条数"
        )
        self._queueUtilization = registry.gauge(
            "queue_utilization", "队列/outbox 容量利用率（0~1）"
        )
        self._queuePending = registry.gauge("queue_pending_count", "队列待处理条数")
        self._queueConnected = registry.gauge(
            "queue_connected", "消息传输连接状态（1=已连接，0=断开）"
        )
        self._apiErrorCodes = registry.counter(
            "api_error_codes_total", "API 业务错误码计数（按 code）"
        )
        self._logDropped = registry.counter(
            "structured_log_dropped_total", "结构化日志丢弃计数（有界队列溢出）"
        )
        self._logDegraded = registry.gauge(
            "structured_log_degraded", "日志系统降级状态（1=降级）"
        )

    # ---- readiness ----
    def collectReadiness(self, probe: ReadinessProbeSource | None) -> None:
        if probe is None:
            self._readinessState.set(0.0)
            self._readinessChecksFailed.set(0.0)
            return
        failed = len(probe.failedChecks()) if hasattr(probe, "failedChecks") else 0
        self._readinessChecksFailed.set(float(failed))
        try:
            state = probe.state()
            name = str(state.name if hasattr(state, "name") else state)
        except Exception:
            name = "UNKNOWN"
        if probe.ready():
            self._readinessState.set(1.0, {"state": name})
        else:
            self._readinessState.set(0.0, {"state": name})

    # ---- 事件延迟 ----
    def observeEventIngest(self, latencySeconds: float, accountGroup: str = "default") -> None:
        self._eventIngestLatency.observe(latencySeconds, {"account_group": accountGroup})

    # ---- 订单 ----
    def observeOrderTransition(self, targetState: str, accountGroup: str = "default") -> None:
        self._orderStateTransitions.inc(1.0, {"state": targetState, "account_group": accountGroup})

    def observeOrderRejection(self, reasonCode: str) -> None:
        self._orderRejections.inc(1.0, {"reason": reasonCode})

    # ---- 账本 ----
    def observeLedgerCommit(
        self, latencySeconds: float, accountGroup: str = "default"
    ) -> None:
        self._ledgerCommitLatency.observe(latencySeconds, {"account_group": accountGroup})
        self._ledgerCommits.inc(1.0, {"account_group": accountGroup})

    # ---- outbox / 队列 ----
    def collectOutbox(self, probe: OutboxProbe | None) -> None:
        if probe is None:
            self._outboxOldestAge.set(0.0)
            self._outboxUnconfirmed.set(0.0)
            return
        age = probe.oldestUnconfirmedAgeSeconds()
        self._outboxOldestAge.set(age if age is not None else 0.0)
        self._outboxUnconfirmed.set(float(probe.unconfirmedCount()))

    def collectQueue(self, probe: QueueProbe | None) -> None:
        if probe is None:
            self._queueUtilization.set(0.0)
            self._queuePending.set(0.0)
            self._queueConnected.set(1.0)
            return
        self._queueUtilization.set(probe.utilization())
        self._queuePending.set(float(probe.pendingCount()))
        self._queueConnected.set(1.0 if probe.connected() else 0.0)

    # ---- 错误码 ----
    def collectErrorCodes(self, counts: dict[int, int] | None) -> None:
        if not counts:
            return
        for code, count in counts.items():
            self._apiErrorCodes.inc(float(count), {"code": str(code)})

    # ---- 日志降级 ----
    def collectLogState(self, degraded: bool, dropped: int) -> None:
        self._logDegraded.set(1.0 if degraded else 0.0)
        if dropped > 0:
            self._logDropped.inc(float(dropped))

    def collectAll(self, inputs: CollectorInputs) -> None:
        """一键采集所有探针。"""
        self.collectReadiness(inputs.readiness)
        self.collectOutbox(inputs.outbox)
        self.collectQueue(inputs.queue)
        self.collectErrorCodes(inputs.errorCodeCounts)
        self.collectLogState(inputs.logDegraded, inputs.logDroppedCount)


class WallClockLatencyRecorder:
    """秒级延迟记录的便捷封装：记录开始时间，结束时可观测延迟。"""

    def __init__(self, collector: MetricsCollector, accountGroup: str = "default") -> None:
        self._collector = collector
        self._accountGroup = accountGroup
        self._started = time.monotonic()

    def observeEventIngest(self) -> None:
        self._collector.observeEventIngest(
            time.monotonic() - self._started, self._accountGroup
        )

    def observeLedgerCommit(self) -> None:
        self._collector.observeLedgerCommit(
            time.monotonic() - self._started, self._accountGroup
        )
