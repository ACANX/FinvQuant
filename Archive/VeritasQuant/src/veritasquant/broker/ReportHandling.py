"""P4-005 异步回报、序列缺口、迟到成交和更正映射。

对齐 TechSpec 7.1/7.3 与 13 阶段 4：
- 重复/乱序/断线重放不重复记账；
- 未知订单隔离并查询。

- `BrokerReportV1`：券商侧原始回报（适配边界，字段保留来源协议）；
- `ReportDeduplicatorV1`：按 brokerReportId 去重（重复回报不重复记账）；
- `ReportSequenceGuardV1`：按 brokerOrderId 序列缺口检测；
- `ReportCorrectionV1`：更正映射（CORRECTED 回报替换原成交，不新增记账）；
- `UnknownOrderIsolationV1`：未知订单隔离区并触发查询。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import StrEnum

from veritasquant.broker.BrokerPort import BrokerPortError
from veritasquant.execution.Orders import ExecutionType


class ReportGuardStatus(StrEnum):
    InOrder = "IN_ORDER"
    Duplicate = "DUPLICATE"       # 重复回报：不重复记账
    GapDetected = "GAP_DETECTED"  # 序列缺口：等待补齐
    LateArrival = "LATE_ARRIVAL"  # 迟到回报：需更正/重放语义
    Corrected = "CORRECTED"       # 更正：替换原成交


@dataclass(frozen=True, slots=True)
class BrokerReportV1:
    """券商侧原始回报（适配边界模型；第三方字段不扩散到领域）。"""

    brokerReportId: str
    clientOrderId: str
    brokerOrderId: str
    reportSequence: int
    executionType: ExecutionType
    executionId: str | None
    lastQuantity: str  # Decimal 字符串
    lastPrice: str | None
    cumulativeQuantity: str
    receivedAt: datetime

    def __post_init__(self) -> None:
        if not self.brokerReportId or not self.clientOrderId or not self.brokerOrderId:
            raise BrokerPortError("回报标识字段不能为空")
        if self.reportSequence < 1:
            raise BrokerPortError("回报序列必须为正")


@dataclass(frozen=True, slots=True)
class ReportHandlingV1:
    """回报处理结论。"""

    status: ReportGuardStatus
    report: BrokerReportV1
    correctedReportId: str | None = None
    message: str = ""


@dataclass(slots=True)
class ReportDeduplicatorV1:
    """按 brokerReportId 去重：重复回报不重复记账。"""

    _seen: set[str] = field(default_factory=set)

    def check(self, report: BrokerReportV1) -> ReportHandlingV1:
        if report.brokerReportId in self._seen:
            return ReportHandlingV1(
                status=ReportGuardStatus.Duplicate,
                report=report,
                message=f"重复回报 {report.brokerReportId} 不重复记账",
            )
        self._seen.add(report.brokerReportId)
        return ReportHandlingV1(status=ReportGuardStatus.InOrder, report=report)


@dataclass(slots=True)
class ReportSequenceGuardV1:
    """按 brokerOrderId 的回报序列缺口检测与乱序/迟到判定。"""

    _nextExpected: dict[str, int] = field(default_factory=dict)
    _latest: dict[str, int] = field(default_factory=dict)
    _seen: dict[str, set[int]] = field(default_factory=dict)

    def check(self, report: BrokerReportV1) -> ReportHandlingV1:
        brokerOrderId = report.brokerOrderId
        expected = self._nextExpected.get(brokerOrderId, 1)
        seen = self._seen.setdefault(brokerOrderId, set())
        if report.reportSequence in seen:
            return ReportHandlingV1(
                status=ReportGuardStatus.Duplicate,
                report=report,
                message=f"重复回报 seq={report.reportSequence} 不重复记账",
            )
        seen.add(report.reportSequence)
        if report.reportSequence == expected:
            self._nextExpected[brokerOrderId] = expected + 1
            self._latest[brokerOrderId] = report.reportSequence
            return ReportHandlingV1(status=ReportGuardStatus.InOrder, report=report)
        if report.reportSequence < expected:
            # 迟到但此前未处理过：允许补齐（重放/更正场景），需上游确认
            self._latest[brokerOrderId] = max(self._latest.get(brokerOrderId, 0), report.reportSequence)
            return ReportHandlingV1(
                status=ReportGuardStatus.LateArrival,
                report=report,
                message=f"迟到回报 seq={report.reportSequence}（期望 {expected}）",
            )
        # 序列缺口：等待补齐
        self._nextExpected[brokerOrderId] = report.reportSequence + 1
        self._latest[brokerOrderId] = report.reportSequence
        return ReportHandlingV1(
            status=ReportGuardStatus.GapDetected,
            report=report,
            message=f"序列缺口：收到 seq={report.reportSequence}，期望 {expected}",
        )

    def expectedFor(self, brokerOrderId: str) -> int:
        return self._nextExpected.get(brokerOrderId, 1)


@dataclass(slots=True)
class ReportCorrectionV1:
    """更正映射：CORRECTED 回报替换原成交，不新增记账。"""

    _executionToReport: dict[str, str] = field(default_factory=dict)  # executionId -> original report
    _corrected: dict[str, str] = field(default_factory=dict)  # original report -> corrected report

    def register(self, report: BrokerReportV1) -> None:
        if report.executionType in (ExecutionType.Fill, ExecutionType.PartialFill):
            if report.executionId is None:
                raise BrokerPortError("成交类回报必须携带 execution_id")
            self._executionToReport[report.executionId] = report.brokerReportId

    def applyCorrection(self, correction: BrokerReportV1, originalExecutionId: str) -> ReportHandlingV1:
        """更正：替换原成交的记账引用，不新增记账条目。"""
        originalReportId = self._executionToReport.get(originalExecutionId)
        if originalReportId is None:
            raise BrokerPortError(f"未知成交 {originalExecutionId}，无法更正")
        self._corrected[originalReportId] = correction.brokerReportId
        return ReportHandlingV1(
            status=ReportGuardStatus.Corrected,
            report=correction,
            correctedReportId=originalReportId,
            message=f"成交 {originalExecutionId} 被回报 {correction.brokerReportId} 更正",
        )

    def isCorrected(self, reportId: str) -> bool:
        return reportId in self._corrected


@dataclass(slots=True)
class UnknownOrderIsolationV1:
    """未知订单隔离区：回报引用未登记订单 -> 隔离并触发查询。"""

    _isolated: dict[str, BrokerReportV1] = field(default_factory=dict)

    def isolate(self, report: BrokerReportV1) -> ReportHandlingV1:
        self._isolated[report.brokerReportId] = report
        return ReportHandlingV1(
            status=ReportGuardStatus.GapDetected,
            report=report,
            message=f"未知订单 {report.brokerOrderId} 已隔离，触发查询对账",
        )

    def records(self) -> tuple[BrokerReportV1, ...]:
        return tuple(self._isolated.values())

    def __contains__(self, brokerReportId: str) -> bool:
        return brokerReportId in self._isolated
