"""P2-040 运行期观测接线：把指标采集、SLO 记录、演练与周复核接入实际运行路径。

解决 P2-036/P2-037 的遗留开放项："运行期 SLI 观测接入（事件延迟/账本延迟埋点）"。

- ObservabilityWiringV1：组装 MetricsCollector + SloCalculator + 可选探针，
  供运行入口（TradingWorker/模拟盘监督器）在事件循环、账本提交、就绪检查处调用；
- InstrumentedLedgerStore：包装 LedgerStoreV1，提交时观测账本延迟并计数；
- InstrumentedGroupWorker：包装 AccountGroupWorkerV1，处理事件时观测
  ingested_at -> 提交延迟（事件延迟 SLI 埋点）；
- RuntimeSnapshotV1：单次快照聚合（指标文本 + SLO 摘要 + 演练/复核状态），
  供每日运行清单（RunManifest 旁路）写入。
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from datetime import datetime, timezone

from veritasquant.accounts.Ledger import JournalV1, LedgerStoreV1
from veritasquant.application.AccountGroupWorker import AccountGroupWorkerV1
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.monitoring.MetricsCollector import (
    CollectorInputs,
    MetricsCollector,
    OutboxProbe,
    QueueProbe,
    ReadinessProbeSource,
)
from veritasquant.monitoring.PrometheusMetrics import MetricsRegistry
from veritasquant.monitoring.SloCalculator import (
    ExecutionMode,
    SliName,
    SliObservationV1,
    SloCalculatorV1,
    SloSummaryV1,
)


@dataclass(frozen=True, slots=True)
class RuntimeSnapshotV1:
    """单次运行快照：指标文本、SLO 摘要与时间。"""

    capturedAt: datetime
    metricsText: str
    sloSummaries: tuple[SloSummaryV1, ...] = ()


class InstrumentedLedgerStore:
    """包装账本存储：每次提交记录延迟并计数（账本事务提交 SLI 埋点）。"""

    def __init__(
        self,
        inner: LedgerStoreV1,
        collector: MetricsCollector,
        accountGroup: str = "default",
    ) -> None:
        self._inner = inner
        self._collector = collector
        self._accountGroup = accountGroup

    @property
    def inner(self) -> LedgerStoreV1:
        return self._inner

    @property
    def journals(self) -> tuple[JournalV1, ...]:
        return self._inner.journals

    @property
    def entries(self) -> object:
        return self._inner.entries

    def commitJournal(self, journal: JournalV1) -> JournalV1:
        started = time.monotonic()
        result = self._inner.commitJournal(journal)
        self._collector.observeLedgerCommit(
            time.monotonic() - started, self._accountGroup
        )
        return result


class InstrumentedGroupWorker:
    """包装账户组 worker：处理事件时观测 ingested_at -> 提交延迟。"""

    def __init__(
        self,
        inner: AccountGroupWorkerV1,
        collector: MetricsCollector,
        accountGroup: str | None = None,
    ) -> None:
        self._inner = inner
        self._collector = collector
        self._accountGroup = accountGroup or inner.topology.accountGroupId

    @property
    def inner(self) -> AccountGroupWorkerV1:
        return self._inner

    @property
    def topology(self) -> object:
        return self._inner.topology

    @property
    def state(self) -> object:
        return self._inner.state

    def processEvent(self, event: EventEnvelopeV1) -> object:
        started = time.monotonic()
        try:
            return self._inner.processEvent(event)
        finally:
            # 事件延迟 SLI：处理提交耗时（TechSpec 12.3 行情延迟口径）
            latency = max(0.0, time.monotonic() - started)
            self._collector.observeEventIngest(latency, self._accountGroup)

    @property
    def failedAccounts(self) -> tuple[str, ...]:
        return self._inner.failedAccounts

    def stop(self) -> None:
        self._inner.stop()


class SloObservationRecorderV1:
    """把运行期观测写入 SLO 计算器（按执行模式与账户组）。"""

    def __init__(
        self,
        calculator: SloCalculatorV1,
        mode: ExecutionMode,
        runId: str,
        accountGroup: str = "default",
    ) -> None:
        self._calculator = calculator
        self._mode = mode
        self._runId = runId
        self._accountGroup = accountGroup

    def record(
        self,
        sli: SliName,
        value: float,
        tradingDay: str | None = None,
    ) -> None:
        day = tradingDay or datetime.now(timezone.utc).date().isoformat()
        self._calculator.record(
            SliObservationV1(
                sli=sli,
                value=value,
                tradingDay=day,
                runId=self._runId,
                accountGroup=self._accountGroup,
            )
        )

    def evaluate(self) -> SloSummaryV1:
        return self._calculator.evaluate(self._mode)


class ObservabilityWiringV1:
    """运行入口观测装配：一次性接线采集器、SLO 与探针。"""

    def __init__(
        self,
        registry: MetricsRegistry | None = None,
        mode: ExecutionMode = ExecutionMode.Paper,
        runId: str = "run-default",
    ) -> None:
        self.registry = registry or MetricsRegistry()
        self.collector = MetricsCollector(self.registry)
        self.calculator = SloCalculatorV1()
        self.runId = runId
        self.mode = mode

    def ledger(self, inner: LedgerStoreV1, accountGroup: str = "default") -> InstrumentedLedgerStore:
        return InstrumentedLedgerStore(inner, self.collector, accountGroup)

    def worker(
        self, inner: AccountGroupWorkerV1, accountGroup: str | None = None
    ) -> InstrumentedGroupWorker:
        return InstrumentedGroupWorker(inner, self.collector, accountGroup)

    def recorder(self, accountGroup: str = "default") -> SloObservationRecorderV1:
        return SloObservationRecorderV1(
            self.calculator, self.mode, self.runId, accountGroup
        )

    def collectReadiness(self, probe: ReadinessProbeSource | None) -> None:
        self.collector.collectReadiness(probe)

    def collectOutbox(self, probe: OutboxProbe | None) -> None:
        self.collector.collectOutbox(probe)

    def collectQueue(self, probe: QueueProbe | None) -> None:
        self.collector.collectQueue(probe)

    def collectAll(self, inputs: CollectorInputs) -> None:
        self.collector.collectAll(inputs)

    def snapshot(self) -> RuntimeSnapshotV1:
        """生成当前运行快照（指标文本 + SLO 摘要）。"""
        return RuntimeSnapshotV1(
            capturedAt=datetime.now(timezone.utc),
            metricsText=self.registry.render(),
            sloSummaries=(self.calculator.evaluate(self.mode),),
        )
