"""执行回报去重、乱序、序列缺口、未知订单与更正处理。

技术方案 4.6 节规则：broker_report_id/execution_id 相同且哈希相同返回
已提交结果；相同 ID 不同哈希进入协议冲突；report_sequence 不大于已应用
序号时只保留审计不回退状态；序号缺口时缓冲并暂停推进，只有连续序列或
权威快照核验后才能恢复；无法关联 client_order_id 的回报进入未知订单
隔离区；累计量不得下降或超过订单量。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.OrderStateMachine import (
    OrderStateMachineError,
    OrderStateMachineV1,
    TransitionKind,
)
from veritasquant.execution.Orders import ExecutionReportEventV1, ExecutionType


def _reportHash(report: ExecutionReportEventV1) -> str:
    """载荷规范哈希，用于重复/冲突检测。"""
    return canonicalHash(report.model_dump(mode="python", by_alias=False))


class ReportProcessingError(ValueError):
    """回报处理违反去重、缺口或累计量契约时抛出。"""


class ReportDisposition(StrEnum):
    Applied = "APPLIED"
    Duplicate = "DUPLICATE"
    Conflict = "CONFLICT"
    GapPaused = "GAP_PAUSED"
    UnknownOrder = "UNKNOWN_ORDER"
    StaleSequence = "STALE_SEQUENCE"
    Quarantined = "QUARANTINED"


@dataclass(frozen=True, slots=True)
class ReportResultV1:
    """单条回报的处理结果。"""

    disposition: ReportDisposition
    reportId: str
    clientOrderId: str | None
    reportSequence: int
    message: str = ""


@dataclass(frozen=True, slots=True)
class AppliedReportV1:
    """已应用的回报及其增量成交信息。"""

    reportId: str
    reportSequence: int
    executionId: str | None
    lastQuantity: Decimal
    cumulativeQuantity: Decimal


@dataclass(slots=True)
class UnknownOrderRecordV1:
    """无法关联本地订单的回报隔离记录。"""

    reportId: str
    rawHash: str
    clientOrderId: str | None
    reason: str


class ReportProcessorV1:
    """按账户处理执行回报，维持累计量不变量并处理缺口与未知订单。"""

    def __init__(self, stateMachine: OrderStateMachineV1) -> None:
        self._stateMachine = stateMachine
        self._applied: dict[str, AppliedReportV1] = {}
        self._reportHashes: dict[str, str] = {}
        self._executionIds: dict[str, tuple[Decimal, Decimal]] = {}
        self._lastSequences: dict[str, int] = {}
        self._pendingGaps: dict[str, dict[int, ExecutionReportEventV1]] = {}
        self._gapPaused: set[str] = set()
        self._unknownOrders: dict[str, UnknownOrderRecordV1] = {}
        self._cumulativeByOrder: dict[str, Decimal] = {}
        self._audit: list[ReportResultV1] = []

    @property
    def audit(self) -> tuple[ReportResultV1, ...]:
        """全部回报的处置审计轨迹。"""
        return tuple(self._audit)

    def process(self, report: ExecutionReportEventV1) -> ReportResultV1:
        """处理一条回报；重复、冲突、缺口、未知订单各归其位。"""
        if report.brokerReportId in self._reportHashes:
            if self._reportHashes[report.brokerReportId] != _reportHash(report):
                result = self._result(ReportDisposition.Conflict, report, "相同 reportId 不同内容哈希")
            else:
                result = self._result(ReportDisposition.Duplicate, report, "重复回报返回已提交结果")
            self._audit.append(result)
            return result
        if report.executionId is not None and report.executionId in self._executionIds:
            prior = self._executionIds[report.executionId]
            current = (report.lastQuantity, report.cumulativeQuantity)
            if prior != current:
                result = self._result(ReportDisposition.Conflict, report, "相同 executionId 不同成交数量")
                self._audit.append(result)
                return result
            result = self._result(ReportDisposition.Duplicate, report, "重复 executionId 返回已提交结果")
            self._audit.append(result)
            return result

        accountKey = report.accountId
        lastSequence = self._lastSequences.get(accountKey, 0)
        if report.reportSequence <= lastSequence:
            result = self._result(ReportDisposition.StaleSequence, report, "旧序号只保留审计，不回退状态")
            self._audit.append(result)
            return result
        if report.reportSequence > lastSequence + 1:
            self._pendingGaps.setdefault(accountKey, {})[report.reportSequence] = report
            self._gapPaused.add(accountKey)
            result = self._result(
                ReportDisposition.GapPaused,
                report,
                f"序列缺口: 期望 {lastSequence + 1}，收到 {report.reportSequence}",
            )
            self._audit.append(result)
            return result

        return self._apply(report)

    def applyVerifiedSnapshot(self, accountId: str, verifiedSequence: int) -> ReportResultV1 | None:
        """券商权威快照核验后恢复推进：补齐缺口缓冲。"""
        pending = self._pendingGaps.get(accountId, {})
        if verifiedSequence < self._lastSequences.get(accountId, 0):
            raise ReportProcessingError("权威快照序号不得低于已应用序号")
        for sequence in sorted(pending):
            if sequence <= verifiedSequence:
                report = pending.pop(sequence)
                self._apply(report)
        if not pending:
            self._gapPaused.discard(accountId)
            return ReportResultV1(
                ReportDisposition.Applied,
                f"snapshot:{verifiedSequence}",
                None,
                verifiedSequence,
                "缺口已由权威快照补齐",
            )
        return None

    def _apply(self, report: ExecutionReportEventV1) -> ReportResultV1:
        """应用一条序号连续的回报并更新订单状态与累计量。"""
        try:
            snapshot = self._stateMachine.snapshot(report.clientOrderId)
        except OrderStateMachineError:
            record = UnknownOrderRecordV1(
                reportId=report.brokerReportId,
                rawHash=_reportHash(report),
                clientOrderId=report.clientOrderId,
                reason="无法关联本地订单",
            )
            self._unknownOrders[report.brokerReportId] = record
            result = self._result(ReportDisposition.UnknownOrder, report, "未知订单进入隔离区")
            self._audit.append(result)
            return result

        priorCumulative = self._cumulativeByOrder.get(report.clientOrderId, Decimal("0"))
        if report.cumulativeQuantity < priorCumulative:
            result = self._result(ReportDisposition.Quarantined, report, "累计量下降，禁止回退")
            self._audit.append(result)
            return result
        if report.cumulativeQuantity > snapshot.quantity:
            result = self._result(ReportDisposition.Quarantined, report, "累计量超过订单量")
            self._audit.append(result)
            return result
        if report.cumulativeQuantity < priorCumulative + report.lastQuantity:
            result = self._result(ReportDisposition.Quarantined, report, "累计量小于已应用量加增量")
            self._audit.append(result)
            return result

        if report.executionType in (ExecutionType.PartialFill, ExecutionType.Fill):
            self._stateMachine.transition(
                report.clientOrderId,
                report.accountId,
                TransitionKind.IncrementalFill,
                snapshot.orderVersion,
                fillQuantity=report.lastQuantity,
            )
        elif report.executionType is ExecutionType.Cancelled:
            self._stateMachine.transition(
                report.clientOrderId,
                report.accountId,
                TransitionKind.CancelConfirmed,
                snapshot.orderVersion,
                cancelQuantity=report.remainingQuantity,
            )
        elif report.executionType is ExecutionType.Rejected:
            self._stateMachine.transition(
                report.clientOrderId,
                report.accountId,
                TransitionKind.BrokerReject,
                snapshot.orderVersion,
            )

        self._reportHashes[report.brokerReportId] = _reportHash(report)
        self._lastSequences[report.accountId] = report.reportSequence
        self._cumulativeByOrder[report.clientOrderId] = report.cumulativeQuantity
        applied = AppliedReportV1(
            reportId=report.brokerReportId,
            reportSequence=report.reportSequence,
            executionId=report.executionId,
            lastQuantity=report.lastQuantity,
            cumulativeQuantity=report.cumulativeQuantity,
        )
        self._applied[report.brokerReportId] = applied
        if report.executionId is not None:
            self._executionIds[report.executionId] = (report.lastQuantity, report.cumulativeQuantity)
        # 序号缺口缓冲中若有连续下一条，立即消费以恢复推进。
        pending = self._pendingGaps.get(report.accountId, {})
        nextSequence = report.reportSequence + 1
        if nextSequence in pending:
            buffered = pending.pop(nextSequence)
            self._apply(buffered)
        if not pending and report.accountId in self._gapPaused:
            self._gapPaused.discard(report.accountId)
        result = self._result(ReportDisposition.Applied, report, "已应用")
        self._audit.append(result)
        return result

    def pendingGapCount(self, accountId: str) -> int:
        """返回账户当前缓冲的缺口回报数量。"""
        return len(self._pendingGaps.get(accountId, {}))

    def isGapPaused(self, accountId: str) -> bool:
        """账户是否因序列缺口暂停推进。"""
        return accountId in self._gapPaused

    def unknownOrders(self) -> tuple[UnknownOrderRecordV1, ...]:
        """返回未知订单隔离区记录。"""
        return tuple(self._unknownOrders.values())

    def cumulativeFor(self, clientOrderId: str) -> Decimal:
        """返回订单已应用累计量。"""
        return self._cumulativeByOrder.get(clientOrderId, Decimal("0"))

    def _result(
        self, disposition: ReportDisposition, report: ExecutionReportEventV1, message: str
    ) -> ReportResultV1:
        return ReportResultV1(
            disposition=disposition,
            reportId=report.brokerReportId,
            clientOrderId=report.clientOrderId,
            reportSequence=report.reportSequence,
            message=message,
        )
