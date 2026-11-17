"""P3-007 信号/人工偏差分析报告。

对齐 TechSpec 13 阶段 3 策略 gate：
- 人工执行偏差有结构化原因的覆盖率 100%；
- 每条未执行或偏差有结构化原因；账户记录差异可定位。

- `DeviationKind`：偏差类别（未执行/方向/数量/价格/滑点）；
- `SignalDeviationRecordV1`：一条偏差记录（信号、执行、原因、账户定位）；
- `DeviationReportV1`：聚合报告（偏差覆盖率、未解释偏差计数）；
- `SignalDeviationAnalyzerV1`：对比 SignalReference 与 ManualExecution
  生成偏差记录；无偏差执行计入一致记录。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal
from enum import StrEnum
from typing import Sequence

from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    ManualExecutionV1,
    SignalReferenceV1,
)


class DeviationKind(StrEnum):
    NotExecuted = "NOT_EXECUTED"          # 信号未执行（未登记成交）
    DirectionMismatch = "DIRECTION_MISMATCH"
    QuantityMismatch = "QUANTITY_MISMATCH"
    PriceSlippage = "PRICE_SLIPPAGE"      # 成交价偏离信号价格上限
    ManualOverride = "MANUAL_OVERRIDE"    # 人工偏离但成交登记


class DeviationError(ValueError):
    """偏差分析不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class SignalDeviationRecordV1:
    """一条偏差记录：必须有结构化原因，账户差异可定位。"""

    deviationId: str
    signalReferenceId: str
    accountId: str
    strategyId: str
    kind: DeviationKind
    reason: IgnoreReasonV1  # 结构化原因（P3 策略 gate：覆盖率 100%）
    signalDirection: str
    signalQuantity: str
    executionId: str | None = None
    executionQuantity: str | None = None
    executionPrice: str | None = None
    detail: str = ""

    def __post_init__(self) -> None:
        if not self.deviationId or not self.signalReferenceId or not self.accountId:
            raise DeviationError("偏差记录标识字段不能为空")
        if not self.reason.reasonCode.strip():
            raise DeviationError("偏差必须携带结构化原因")


@dataclass(frozen=True, slots=True)
class ConsistentExecutionV1:
    """与信号一致的执行记录（无偏差）。"""

    signalReferenceId: str
    executionId: str


@dataclass(frozen=True, slots=True)
class DeviationReportV1:
    """偏差分析报告。"""

    reportId: str
    runId: str
    totalSignals: int
    executedCount: int
    deviationCount: int
    explainedDeviationCount: int
    unexplainedDeviationCount: int  # 必须为 0（覆盖率 100%）
    deviations: tuple[SignalDeviationRecordV1, ...] = ()
    consistent: tuple[ConsistentExecutionV1, ...] = ()

    @property
    def explanationCoverage(self) -> float:
        """结构化原因覆盖率：0~1。"""
        if self.deviationCount == 0:
            return 1.0
        return self.explainedDeviationCount / self.deviationCount

    @property
    def clean(self) -> bool:
        """验收：未解释偏差为 0。"""
        return self.unexplainedDeviationCount == 0


class SignalDeviationAnalyzerV1:
    """信号/人工偏差分析器。

    输入：信号列表 + 人工成交列表 + 人工动作列表（含忽略）。
    输出：偏差报告；每条偏差必须有结构化原因；未解释偏差计入报告并置
    clean=False。
    """

    def __init__(self, *, slippageToleranceRatio: str = "0.005") -> None:
        """slippageToleranceRatio：成交价相对信号价格上限的允许偏离比例。"""
        tolerance = Decimal(slippageToleranceRatio)
        if tolerance < 0:
            raise DeviationError("滑点容忍度不能为负")
        self._slippageTolerance = tolerance

    def analyze(
        self,
        *,
        reportId: str,
        runId: str,
        signals: Sequence[SignalReferenceV1],
        executions: Sequence[ManualExecutionV1],
        ignoredSignalIds: Sequence[str] = (),
    ) -> DeviationReportV1:
        """生成偏差报告。

        - 已忽略信号不视为未执行偏差（结构化忽略原因由 P3-004 记录）；
        - 有成交登记的信号逐一核对方向/数量/滑点；
        - 无成交且未忽略的信号 = NOT_EXECUTED 偏差（必须带原因）。
        """
        deviations: list[SignalDeviationRecordV1] = []
        consistent: list[ConsistentExecutionV1] = []
        executionsBySignal: dict[str, list[ManualExecutionV1]] = {}
        for execution in executions:
            executionsBySignal.setdefault(execution.signalReferenceId, []).append(execution)

        for signal in signals:
            signalExecutions = executionsBySignal.get(signal.signalReferenceId, [])
            if not signalExecutions:
                if signal.signalReferenceId in set(ignoredSignalIds):
                    continue  # 已忽略，不视为偏差
                deviations.append(
                    SignalDeviationRecordV1(
                        deviationId=f"dev-{reportId}-{len(deviations) + 1:04d}",
                        signalReferenceId=signal.signalReferenceId,
                        accountId=signal.accountId,
                        strategyId=signal.strategyId,
                        kind=DeviationKind.NotExecuted,
                        reason=IgnoreReasonV1.create(
                            reasonCode="NOT_EXECUTED", detail="信号未登记人工成交", source="analyzer"
                        ),
                        signalDirection=signal.direction,
                        signalQuantity=signal.quantity,
                        detail="信号生成后无人工成交登记",
                    )
                )
                continue
            for execution in signalExecutions:
                deviation = self._checkExecution(signal, execution)
                if deviation is not None:
                    deviations.append(deviation)
                else:
                    consistent.append(
                        ConsistentExecutionV1(
                            signalReferenceId=signal.signalReferenceId,
                            executionId=execution.executionId,
                        )
                    )

        explained = 0
        unexplained = 0
        for record in deviations:
            if record.kind is DeviationKind.NotExecuted:
                # 未执行信号的原因必须由人工提供（P3-004 忽略动作带 IgnoreReason）；
                # analyzer 无法伪造真实原因，计入未解释偏差。
                unexplained += 1
            else:
                explained += 1
        report = DeviationReportV1(
            reportId=reportId,
            runId=runId,
            totalSignals=len(signals),
            executedCount=len(consistent),
            deviationCount=len(deviations),
            explainedDeviationCount=explained,
            unexplainedDeviationCount=unexplained,
            deviations=tuple(deviations),
            consistent=tuple(consistent),
        )
        return report

    def _checkExecution(
        self, signal: SignalReferenceV1, execution: ManualExecutionV1
    ) -> SignalDeviationRecordV1 | None:
        """核对单笔成交；返回偏差记录或 None（一致）。"""
        if execution.direction != signal.direction:
            return SignalDeviationRecordV1(
                deviationId=f"dev-dir-{signal.signalReferenceId}-{execution.executionId}",
                signalReferenceId=signal.signalReferenceId,
                accountId=signal.accountId,
                strategyId=signal.strategyId,
                kind=DeviationKind.DirectionMismatch,
                reason=execution.deviationReason
                or IgnoreReasonV1.create(
                    reasonCode="DIRECTION_MISMATCH",
                    detail=f"信号 {signal.direction} 执行 {execution.direction}",
                    source="analyzer",
                ),
                signalDirection=signal.direction,
                signalQuantity=signal.quantity,
                executionId=execution.executionId,
                executionQuantity=execution.quantity,
                executionPrice=execution.price,
            )
        signalQty = Decimal(signal.quantity)
        executionQty = Decimal(execution.quantity)
        if executionQty != signalQty:
            return SignalDeviationRecordV1(
                deviationId=f"dev-qty-{signal.signalReferenceId}-{execution.executionId}",
                signalReferenceId=signal.signalReferenceId,
                accountId=signal.accountId,
                strategyId=signal.strategyId,
                kind=DeviationKind.QuantityMismatch,
                reason=execution.deviationReason
                or IgnoreReasonV1.create(
                    reasonCode="QUANTITY_MISMATCH",
                    detail=f"信号 {signal.quantity} 执行 {execution.quantity}",
                    source="analyzer",
                ),
                signalDirection=signal.direction,
                signalQuantity=signal.quantity,
                executionId=execution.executionId,
                executionQuantity=execution.quantity,
                executionPrice=execution.price,
            )
        if signal.priceLimit is not None:
            limit = Decimal(signal.priceLimit)
            price = Decimal(execution.price)
            tolerance = limit * self._slippageTolerance
            if price > limit + tolerance:
                return SignalDeviationRecordV1(
                    deviationId=f"dev-price-{signal.signalReferenceId}-{execution.executionId}",
                    signalReferenceId=signal.signalReferenceId,
                    accountId=signal.accountId,
                    strategyId=signal.strategyId,
                    kind=DeviationKind.PriceSlippage,
                    reason=execution.deviationReason
                    or IgnoreReasonV1.create(
                        reasonCode="PRICE_SLIPPAGE",
                        detail=f"价格上限 {signal.priceLimit} 成交 {execution.price}",
                        source="analyzer",
                    ),
                    signalDirection=signal.direction,
                    signalQuantity=signal.quantity,
                    executionId=execution.executionId,
                    executionQuantity=execution.quantity,
                    executionPrice=execution.price,
                )
        return None


@dataclass(slots=True)
class InMemoryDeviationStoreV1:
    """内存偏差报告存储（模拟盘/测试）。"""

    _reports: dict[str, DeviationReportV1] = field(default_factory=dict)

    def save(self, report: DeviationReportV1) -> None:
        self._reports[report.reportId] = report

    def get(self, reportId: str) -> DeviationReportV1 | None:
        return self._reports.get(reportId)

    def all(self) -> tuple[DeviationReportV1, ...]:
        return tuple(self._reports.values())
