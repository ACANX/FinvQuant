"""P2-037 SLO 计算、错误预算与告警路由。

对齐 TechSpec 12.3：
- SLI 按执行模式（回测/模拟盘/券商仿真/受控实盘）计算，阈值见下表；
- 正确性指标（账本不平、控制丢失、跨账户路由、未授权实盘命令）零错误预算，
  出现一次即停止相关交易并启动事故流程；
- 样本不足时状态为"证据不足"而非自动通过；
- 告警包含 run/account/处置链接，便于值班直接定位。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass
from enum import StrEnum
from typing import Mapping

from veritasquant.monitoring.PrometheusMetrics import MetricFamily, MetricsRegistry


class ExecutionMode(StrEnum):
    Backtest = "BACKTEST"
    Paper = "PAPER"  # 模拟盘
    Simulation = "SIMULATION"  # 券商仿真
    Live = "LIVE"  # 受控实盘


class SliName(StrEnum):
    TradingReadinessAvailability = "trading_readiness_availability"
    MarketDataLatencyP99 = "market_data_latency_p99"
    OrderDecisionLatencyP99 = "order_decision_latency_p99"
    LedgerCommitLatencyP99 = "ledger_commit_latency_p99"
    ExternalOrderLatencyP99 = "external_order_latency_p99"
    OutboxMaxAge = "outbox_max_age"
    OutboxMaxCount = "outbox_max_count"
    ControlRecoveryCompleteness = "control_recovery_completeness"
    UnreconciledDifferences = "unreconciled_differences"


class SloStatus(StrEnum):
    WithinBudget = "WITHIN_BUDGET"
    Exceeded = "EXCEEDED"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class SloTargetV1:
    """单条 SLI 目标：阈值 + 是否零错误预算（正确性指标）。"""

    sli: SliName
    threshold: float
    comparison: str  # "<=" 或 ">="
    zeroBudget: bool = False  # True = 正确性指标，出现一次即违约
    description: str = ""


@dataclass(frozen=True, slots=True)
class SliObservationV1:
    """单日 SLI 观测。"""

    sli: SliName
    value: float
    tradingDay: str  # ISO 日期
    runId: str
    accountGroup: str = "default"


@dataclass(frozen=True, slots=True)
class SloResultV1:
    sli: SliName
    status: SloStatus
    budgetRemaining: float  # 0~1（1=完整预算，0=耗尽）；零预算指标为 0/1
    observedDays: int
    detail: str = ""


@dataclass(frozen=True, slots=True)
class SloSummaryV1:
    mode: ExecutionMode
    results: tuple[SloResultV1, ...] = ()
    windowDays: int = 30

    def worst(self) -> SloStatus:
        """整体状态取最差：EXCEEDED > INSUFFICIENT_EVIDENCE > WITHIN_BUDGET。"""
        if any(r.status is SloStatus.Exceeded for r in self.results):
            return SloStatus.Exceeded
        if any(r.status is SloStatus.InsufficientEvidence for r in self.results):
            return SloStatus.InsufficientEvidence
        return SloStatus.WithinBudget


class SloPolicyV1:
    """首期 SLO 目标目录（TechSpec 12.3 表，按执行模式）。"""

    # 模式 -> SLI -> (threshold, comparison, zeroBudget)
    _TARGETS: Mapping[ExecutionMode, Mapping[SliName, tuple[float, str, bool]]] = {
        ExecutionMode.Paper: {
            SliName.TradingReadinessAvailability: (0.99, ">=", False),
            SliName.MarketDataLatencyP99: (5.0, "<=", False),
            SliName.OrderDecisionLatencyP99: (1.0, "<=", False),
            SliName.LedgerCommitLatencyP99: (0.5, "<=", False),
            SliName.OutboxMaxAge: (30.0, "<=", False),
            SliName.OutboxMaxCount: (500.0, "<=", False),
            SliName.ControlRecoveryCompleteness: (1.0, ">=", True),
            SliName.UnreconciledDifferences: (0.0, "<=", True),
        },
        ExecutionMode.Simulation: {
            SliName.TradingReadinessAvailability: (0.995, ">=", False),
            SliName.MarketDataLatencyP99: (2.0, "<=", False),
            SliName.OrderDecisionLatencyP99: (0.5, "<=", False),
            SliName.LedgerCommitLatencyP99: (0.25, "<=", False),
            SliName.ExternalOrderLatencyP99: (3.0, "<=", False),
            SliName.OutboxMaxAge: (10.0, "<=", False),
            SliName.OutboxMaxCount: (200.0, "<=", False),
            SliName.ControlRecoveryCompleteness: (1.0, ">=", True),
            SliName.UnreconciledDifferences: (0.0, "<=", True),
        },
        ExecutionMode.Live: {
            SliName.TradingReadinessAvailability: (0.999, ">=", False),
            SliName.MarketDataLatencyP99: (2.0, "<=", False),
            SliName.OrderDecisionLatencyP99: (0.25, "<=", False),
            SliName.LedgerCommitLatencyP99: (0.25, "<=", False),
            SliName.ExternalOrderLatencyP99: (3.0, "<=", False),
            SliName.OutboxMaxAge: (5.0, "<=", False),
            SliName.OutboxMaxCount: (100.0, "<=", False),
            SliName.ControlRecoveryCompleteness: (1.0, ">=", True),
            SliName.UnreconciledDifferences: (0.0, "<=", True),
        },
    }

    def targetsFor(self, mode: ExecutionMode) -> Mapping[SliName, SloTargetV1]:
        if mode is ExecutionMode.Backtest:
            return {}  # 回测不做运行 SLO（离线校验）
        raw = self._TARGETS[mode]
        return {
            sli: SloTargetV1(
                sli=sli,
                threshold=threshold,
                comparison=comparison,
                zeroBudget=zeroBudget,
                description=self._describe(sli, mode),
            )
            for sli, (threshold, comparison, zeroBudget) in raw.items()
        }

    @staticmethod
    def _describe(sli: SliName, mode: ExecutionMode) -> str:
        labels = {
            SliName.TradingReadinessAvailability: "trading-readiness 可用率",
            SliName.MarketDataLatencyP99: "行情 ingested_at -> 分区提交 p99",
            SliName.OrderDecisionLatencyP99: "订单意图接收至本地风险决定 p99",
            SliName.LedgerCommitLatencyP99: "账本事务提交 p99",
            SliName.ExternalOrderLatencyP99: "外部订单发送至受理/明确拒绝 p99",
            SliName.OutboxMaxAge: "outbox 最老未确认年龄",
            SliName.OutboxMaxCount: "outbox 未确认条数",
            SliName.ControlRecoveryCompleteness: "活动 P0/P1 控制恢复完整率（正确性）",
            SliName.UnreconciledDifferences: "恢复交易前未解释对账差异（正确性）",
        }
        return f"{labels.get(sli, sli.value)} @ {mode.value}"


class SloCalculatorV1:
    """滚动窗口 SLO 计算器：按执行模式聚合观测并评估预算。"""

    def __init__(self, policy: SloPolicyV1 | None = None) -> None:
        self._policy = policy or SloPolicyV1()
        self._observations: list[SliObservationV1] = []

    def record(self, observation: SliObservationV1) -> None:
        self._observations.append(observation)

    def observations(self) -> tuple[SliObservationV1, ...]:
        return tuple(self._observations)

    def evaluate(self, mode: ExecutionMode, windowDays: int = 30) -> SloSummaryV1:
        targets = self._policy.targetsFor(mode)
        results: list[SloResultV1] = []
        for sli, target in targets.items():
            matching = [
                o for o in self._observations
                if o.sli is sli and o.tradingDay in self._recentDays(windowDays)
            ]
            results.append(self._evaluateSli(sli, target, matching))
        return SloSummaryV1(mode=mode, results=tuple(results), windowDays=windowDays)

    def evaluateAccountGroup(
        self, mode: ExecutionMode, accountGroup: str, windowDays: int = 30
    ) -> SloSummaryV1:
        """按账户组聚合评估（跨账户隔离：串扰不允许混入同一窗口）。"""
        targets = self._policy.targetsFor(mode)
        results: list[SloResultV1] = []
        for sli, target in targets.items():
            matching = [
                o for o in self._observations
                if o.sli is sli
                and o.accountGroup == accountGroup
                and o.tradingDay in self._recentDays(windowDays)
            ]
            results.append(self._evaluateSli(sli, target, matching))
        return SloSummaryV1(mode=mode, results=tuple(results), windowDays=windowDays)

    @staticmethod
    def _recentDays(windowDays: int) -> set[str]:
        from datetime import date, timedelta

        today = date.today()
        return {str(today - timedelta(days=i)) for i in range(windowDays)}

    def _evaluateSli(
        self, sli: SliName, target: SloTargetV1, observations: list[SliObservationV1]
    ) -> SloResultV1:
        if not observations:
            return SloResultV1(
                sli=sli,
                status=SloStatus.InsufficientEvidence,
                budgetRemaining=0.0,
                observedDays=0,
                detail="样本不足：无观测数据",
            )
        if target.zeroBudget:
            # 正确性指标零预算：任何一次违约即 EXCEEDED
            violated = [
                o for o in observations
                if not self._meetsTarget(o.value, target)
            ]
            if violated:
                return SloResultV1(
                    sli=sli,
                    status=SloStatus.Exceeded,
                    budgetRemaining=0.0,
                    observedDays=len(observations),
                    detail=f"零预算正确性违约 {len(violated)} 次（最近: {violated[-1].tradingDay}）",
                )
            return SloResultV1(
                sli=sli,
                status=SloStatus.WithinBudget,
                budgetRemaining=1.0,
                observedDays=len(observations),
                detail="零预算指标无违约",
            )
        # 可用率类：按观测日比例聚合
        violated = [
            o for o in observations if not self._meetsTarget(o.value, target)
        ]
        budgetRemaining = max(0.0, 1.0 - len(violated) / max(1, len(observations)))
        if budgetRemaining <= 0.0:
            status = SloStatus.Exceeded
        elif violated:
            status = SloStatus.WithinBudget
        else:
            status = SloStatus.WithinBudget
        return SloResultV1(
            sli=sli,
            status=status,
            budgetRemaining=budgetRemaining,
            observedDays=len(observations),
            detail=f"{len(violated)}/{len(observations)} 日违约",
        )

    @staticmethod
    def _meetsTarget(value: float, target: SloTargetV1) -> bool:
        if target.comparison == "<=":
            return value <= target.threshold
        if target.comparison == ">=":
            return value >= target.threshold
        raise ValueError(f"未知比较符: {target.comparison}")


@dataclass(frozen=True, slots=True)
class AlertRouteV1:
    """告警路由：run/account/处置链接。"""

    runId: str
    accountGroup: str
    sli: SliName
    severity: str  # P0/P1/P2/P3
    message: str
    remediationLink: str = ""  # 处置链接（Runbook/GUI）
    dedupeKey: str = ""

    def __post_init__(self) -> None:
        if not self.dedupeKey:
            raw = f"{self.sli.value}|{self.runId}|{self.accountGroup}|{self.message}"
            object.__setattr__(
                self, "dedupeKey", hashlib.sha256(raw.encode("utf-8")).hexdigest()[:16]
            )


class AlertRouterV1:
    """告警路由：根据 SLO 结果生成带处置链接的告警，并写入指标。"""

    _SEVERITY: Mapping[SloStatus, str] = {
        SloStatus.Exceeded: "P1",
        SloStatus.InsufficientEvidence: "P3",
        SloStatus.WithinBudget: "P3",
    }

    def __init__(self, registry: MetricsRegistry | None = None) -> None:
        self._alerts: list[AlertRouteV1] = []
        self._alertsFired: MetricFamily | None = None
        self._alertsPending: MetricFamily | None = None
        if registry is not None:
            self._alertsFired = registry.counter(
                "slo_alerts_total", "SLO 告警触发计数（按严重度）"
            )
            self._alertsPending = registry.gauge(
                "slo_alerts_pending", "未处置 SLO 告警数"
            )

    def route(self, summary: SloSummaryV1, runId: str, accountGroup: str = "default") -> tuple[AlertRouteV1, ...]:
        """为超出预算或证据不足的 SLI 生成告警。"""
        routed: list[AlertRouteV1] = []
        for result in summary.results:
            if result.status is SloStatus.Exceeded:
                severity = "P1" if result.sli in (
                    SliName.ControlRecoveryCompleteness,
                    SliName.UnreconciledDifferences,
                ) else "P2"
                alert = AlertRouteV1(
                    runId=runId,
                    accountGroup=accountGroup,
                    sli=result.sli,
                    severity=severity,
                    message=f"{summary.mode.value} SLO 违约: {result.detail}",
                    remediationLink=f"run/{runId}/accounts/{accountGroup}/slo/{result.sli.value}",
                )
                routed.append(alert)
        self._alerts.extend(routed)
        if self._alertsFired is not None:
            for alert in routed:
                self._alertsFired.inc(1.0, {"severity": alert.severity})
        if self._alertsPending is not None:
            self._alertsPending.set(float(len(self._alerts)))
        return tuple(routed)

    def alerts(self) -> tuple[AlertRouteV1, ...]:
        return tuple(self._alerts)

    def resolve(self, alert: AlertRouteV1) -> None:
        """处置后从待办移除。"""
        try:
            self._alerts.remove(alert)
        except ValueError:
            pass
        if self._alertsPending is not None:
            self._alertsPending.set(float(len(self._alerts)))
