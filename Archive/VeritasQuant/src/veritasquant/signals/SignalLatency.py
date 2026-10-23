"""P3-006 信号端到端延迟 SLI 与告警。

对齐 TechSpec 13 阶段 3 平台 gate：
- 99.5% 信号在事件可用后 10 秒内送达；
- 可计算事件可用至送达 p50/p95/p99；缺样本不判通过。

- `SignalDeliverySampleV1`：一次信号送达的端到端延迟样本
  （事件可用时间 -> 送达时间）；
- `SignalLatencySliV1`：按窗口聚合 p50/p95/p99 与达标率；
- `SignalLatencyEvaluatorV1`：对照目标评估，缺样本返回
  INSUFFICIENT_EVIDENCE（不自动通过）；
- `SignalLatencyAlertV1`：SLO 违约告警（含 run/account/处置链接）。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Sequence

from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class SignalLatencyError(ValueError):
    """信号延迟 SLI 不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class SignalDeliverySampleV1:
    """一次信号端到端送达样本。"""

    sampleId: str
    signalReferenceId: str
    accountId: str
    runId: str
    eventAvailableTs: datetime  # 事件可用时间（ts）
    deliveredTs: datetime       # 通知/信号送达时间
    channel: str

    def __post_init__(self) -> None:
        if not self.sampleId or not self.signalReferenceId or not self.accountId or not self.runId:
            raise SignalLatencyError("样本标识字段不能为空")
        validateUtcTimestamp(self.eventAvailableTs, TsPrecision.Millisecond)
        validateUtcTimestamp(self.deliveredTs, TsPrecision.Millisecond)
        if self.deliveredTs < self.eventAvailableTs:
            raise SignalLatencyError("送达时间不得早于事件可用时间")

    @property
    def latencySeconds(self) -> float:
        return (self.deliveredTs - self.eventAvailableTs).total_seconds()


@dataclass(frozen=True, slots=True)
class SignalLatencySliV1:
    """信号延迟 SLI 聚合结果。"""

    sampleCount: int
    p50Seconds: float | None
    p95Seconds: float | None
    p99Seconds: float | None
    withinTargetRatio: float | None  # 10 秒内送达比例（None = 无样本）
    targetSeconds: float = 10.0

    @property
    def hasEnoughSamples(self) -> bool:
        return self.sampleCount >= 1


@dataclass(frozen=True, slots=True)
class SignalLatencyEvaluationV1:
    """对照目标的评估结论。"""

    sli: SignalLatencySliV1
    passed: bool
    evidenceStatus: str  # SUFFICIENT / INSUFFICIENT_EVIDENCE
    detail: str = ""


@dataclass(frozen=True, slots=True)
class SignalLatencyAlertV1:
    """SLO 违约告警：含 run/account/处置链接。"""

    alertId: str
    accountId: str
    runId: str
    p99Seconds: float | None
    targetSeconds: float
    violatedAt: datetime
    dispositionLink: str = ""

    def __post_init__(self) -> None:
        if not self.alertId or not self.accountId or not self.runId:
            raise SignalLatencyError("告警标识字段不能为空")


class SignalLatencyEvaluatorV1:
    """信号端到端延迟评估器。

    验收标准：99.5% 在 10 秒内送达；缺样本不判通过。
    默认达标阈值 99.5%（0.995）；最小样本数为 1（单样本即可计算，
    但真实 gate 由 P3-009 的 50 条信号证据窗口保证）。
    """

    def __init__(self, *, targetSeconds: float = 10.0, passRatio: float = 0.995) -> None:
        if targetSeconds <= 0:
            raise SignalLatencyError("目标延迟必须为正")
        if not 0 < passRatio <= 1:
            raise SignalLatencyError("达标比例必须在 (0,1] 区间")
        self._targetSeconds = targetSeconds
        self._passRatio = passRatio

    def evaluate(self, samples: Sequence[SignalDeliverySampleV1]) -> SignalLatencyEvaluationV1:
        """评估延迟 SLI；无样本返回 INSUFFICIENT_EVIDENCE。"""
        if not samples:
            sli = SignalLatencySliV1(
                sampleCount=0,
                p50Seconds=None,
                p95Seconds=None,
                p99Seconds=None,
                withinTargetRatio=None,
                targetSeconds=self._targetSeconds,
            )
            return SignalLatencyEvaluationV1(
                sli=sli,
                passed=False,
                evidenceStatus="INSUFFICIENT_EVIDENCE",
                detail="无信号送达样本，无法评估延迟 SLO",
            )
        latencies = sorted(s.latencySeconds for s in samples)
        p50 = _percentile(latencies, 0.50)
        p95 = _percentile(latencies, 0.95)
        p99 = _percentile(latencies, 0.99)
        within = sum(1 for s in samples if s.latencySeconds <= self._targetSeconds) / len(samples)
        sli = SignalLatencySliV1(
            sampleCount=len(samples),
            p50Seconds=p50,
            p95Seconds=p95,
            p99Seconds=p99,
            withinTargetRatio=within,
            targetSeconds=self._targetSeconds,
        )
        passed = within >= self._passRatio
        return SignalLatencyEvaluationV1(
            sli=sli,
            passed=passed,
            evidenceStatus="SUFFICIENT",
            detail=(
                f"p50={p50:.3f}s p95={p95:.3f}s p99={p99:.3f}s "
                f"10s 内送达 {within:.1%}（目标 {self._passRatio:.1%}）"
            ),
        )

    def buildAlert(
        self,
        *,
        accountId: str,
        runId: str,
        evaluation: SignalLatencyEvaluationV1,
        dispositionLink: str = "",
    ) -> SignalLatencyAlertV1 | None:
        """SLO 违约时生成告警；证据不足或达标时不告警。"""
        if evaluation.evidenceStatus != "SUFFICIENT" or evaluation.passed:
            return None
        return SignalLatencyAlertV1(
            alertId=f"sig-latency-{runId}-{accountId}",
            accountId=accountId,
            runId=runId,
            p99Seconds=evaluation.sli.p99Seconds,
            targetSeconds=evaluation.sli.targetSeconds,
            violatedAt=datetime.now(timezone.utc),
            dispositionLink=dispositionLink,
        )


def _percentile(sortedValues: Sequence[float], quantile: float) -> float:
    """线性插值百分位。"""
    if not sortedValues:
        raise SignalLatencyError("空序列无法计算百分位")
    if len(sortedValues) == 1:
        return sortedValues[0]
    position = (len(sortedValues) - 1) * quantile
    lower = int(position)
    upper = min(lower + 1, len(sortedValues) - 1)
    weight = position - lower
    return sortedValues[lower] * (1 - weight) + sortedValues[upper] * weight


@dataclass(slots=True)
class InMemoryLatencyStoreV1:
    """内存延迟样本存储（模拟盘/测试）。"""

    _samples: list[SignalDeliverySampleV1] = field(default_factory=list)

    def record(self, sample: SignalDeliverySampleV1) -> None:
        self._samples.append(sample)

    def all(self) -> tuple[SignalDeliverySampleV1, ...]:
        return tuple(self._samples)

    def clear(self) -> None:
        self._samples.clear()
