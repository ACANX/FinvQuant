"""P2-009 背压、磁盘/队列阈值与 trading-readiness。

TechSpec 12.3：
- 磁盘可用空间低于 20% 或队列容量达到 70% 告警（Warning）；
- 低于 10% 或队列达到 90% 禁止新增风险并停止非关键写入（Critical）；
- 关键 inbox、账本、控制和审计写入不得丢弃；
- trading-readiness 只有在行情新鲜、对账完成、账本不变量成立、
  活动控制恢复、outbox/队列/磁盘低于硬阈值、时钟同步时才能授权发单。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum


class ThresholdLevel(StrEnum):
    Normal = "NORMAL"
    Warning = "WARNING"
    Critical = "CRITICAL"


@dataclass(frozen=True, slots=True)
class ThresholdStateV1:
    level: ThresholdLevel
    utilization: float  # 0.0 ~ 1.0（磁盘为已用比例，队列/outbox 为容量利用率）


class DiskSpacePolicyV1:
    """磁盘可用空间阈值：<20% 告警，<10% 禁止新增风险。"""

    WARNING_FREE_RATIO = 0.20
    CRITICAL_FREE_RATIO = 0.10

    def evaluate(self, freeRatio: float) -> ThresholdStateV1:
        """freeRatio 为磁盘可用比例（0.0 ~ 1.0）。"""
        if not 0.0 <= freeRatio <= 1.0:
            raise ValueError("磁盘可用比例必须在 0~1 之间")
        if freeRatio < self.CRITICAL_FREE_RATIO:
            level = ThresholdLevel.Critical
        elif freeRatio < self.WARNING_FREE_RATIO:
            level = ThresholdLevel.Warning
        else:
            level = ThresholdLevel.Normal
        return ThresholdStateV1(level, 1.0 - freeRatio)


class QueueThresholdPolicyV1:
    """队列容量阈值：>=70% 告警，>=90% 硬上限停止消费。"""

    WARNING_RATIO = 0.70
    CRITICAL_RATIO = 0.90

    def evaluate(self, utilization: float) -> ThresholdStateV1:
        if not 0.0 <= utilization <= 1.0:
            raise ValueError("队列利用率必须在 0~1 之间")
        if utilization >= self.CRITICAL_RATIO:
            level = ThresholdLevel.Critical
        elif utilization >= self.WARNING_RATIO:
            level = ThresholdLevel.Warning
        else:
            level = ThresholdLevel.Normal
        return ThresholdStateV1(level, utilization)

    def mayOpenNewRisk(self, utilization: float) -> bool:
        """硬阈值时禁止新增风险。"""
        return self.evaluate(utilization).level is not ThresholdLevel.Critical

    def allowCriticalWrites(self, utilization: float) -> bool:
        """关键 inbox/账本/控制/审计写入即使在硬阈值也不得丢弃。"""
        return True  # 关键写入永不因队列背压丢弃


class TradingReadinessState(StrEnum):
    Ready = "READY"
    NotReady = "NOT_READY"


@dataclass(frozen=True, slots=True)
class ReadinessCheckV1:
    """单项 readiness 检查结果。"""

    name: str
    passed: bool
    detail: str = ""


@dataclass(frozen=True, slots=True)
class TradingReadinessReportV1:
    """trading-readiness 评估结果；只有全部通过才授权发单。"""

    checks: tuple[ReadinessCheckV1, ...]

    @property
    def ready(self) -> bool:
        return all(check.passed for check in self.checks)

    @property
    def state(self) -> TradingReadinessState:
        return TradingReadinessState.Ready if self.ready else TradingReadinessState.NotReady

    @property
    def failedChecks(self) -> tuple[ReadinessCheckV1, ...]:
        return tuple(check for check in self.checks if not check.passed)


class TradingReadinessGateV1:
    """trading-readiness 门禁：硬阈值违规或任一检查失败即禁止发单。"""

    def __init__(self, maxClockSkewSeconds: float = 0.5) -> None:
        """时钟偏差目标 <=100ms；超过 500ms 时 trading-readiness 失败。"""
        self._maxClockSkewSeconds = maxClockSkewSeconds

    def evaluate(
        self,
        *,
        marketFresh: bool,
        reconciliationComplete: bool,
        ledgerInvariantsHeld: bool,
        controlsRecovered: bool,
        outboxUtilization: float,
        queueUtilization: float,
        diskFreeRatio: float,
        clockSkewSeconds: float,
    ) -> TradingReadinessReportV1:
        """评估全部 readiness 检查；任一失败 -> NOT_READY。"""
        queueLevel = QueueThresholdPolicyV1().evaluate(queueUtilization)
        diskLevel = DiskSpacePolicyV1().evaluate(diskFreeRatio)
        outboxLevel = QueueThresholdPolicyV1().evaluate(outboxUtilization)
        checks = (
            ReadinessCheckV1("market_fresh", marketFresh, "行情新鲜度"),
            ReadinessCheckV1("reconciliation", reconciliationComplete, "对账完成"),
            ReadinessCheckV1("ledger_invariants", ledgerInvariantsHeld, "账本不变量"),
            ReadinessCheckV1("controls_recovered", controlsRecovered, "活动控制恢复"),
            ReadinessCheckV1(
                "queue_below_hard_limit",
                queueLevel.level is not ThresholdLevel.Critical,
                f"队列利用率 {queueUtilization:.1%}",
            ),
            ReadinessCheckV1(
                "outbox_below_hard_limit",
                outboxLevel.level is not ThresholdLevel.Critical,
                f"outbox 利用率 {outboxUtilization:.1%}",
            ),
            ReadinessCheckV1(
                "disk_above_hard_limit",
                diskLevel.level is not ThresholdLevel.Critical,
                f"磁盘可用 {diskFreeRatio:.1%}",
            ),
            ReadinessCheckV1(
                "clock_sync",
                clockSkewSeconds <= self._maxClockSkewSeconds,
                f"时钟偏差 {clockSkewSeconds:.3f}s",
            ),
        )
        return TradingReadinessReportV1(checks)
