"""P4-007 高精度诊断时间和执行原因采集。

对齐 TechSpec 7.4/13 阶段 4：
- submitted/accepted/filled 等保留来源精度且不参与 `ts` 因果排序。

- `ExecutionPhase`：执行阶段（SUBMITTED/ACCEPTED/FILLED/REJECTED/...）；
- `DiagnosticTimestampV1`：某阶段的高精度诊断时间（来源精度，不参与排序）；
- `ExecutionReasonV1`：执行原因（拒单原因/撤单原因/更正原因等受控枚举）；
- `DiagnosticCollectorV1`：按 clientOrderId 采集阶段时间与原因；
- `DiagnosticReportV1`：一次执行的完整诊断时间线。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from enum import StrEnum
from typing import Protocol

from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class DiagnosticError(ValueError):
    """诊断采集不满足契约时抛出。"""


class ExecutionPhase(StrEnum):
    Submitted = "SUBMITTED"
    Accepted = "ACCEPTED"
    Working = "WORKING"
    PartialFill = "PARTIAL_FILL"
    Filled = "FILLED"
    Cancelled = "CANCELLED"
    Rejected = "REJECTED"
    Expired = "EXPIRED"
    Corrected = "CORRECTED"


class ExecutionReasonCode(StrEnum):
    """受控执行原因代码（拒单/撤单/更正）。"""

    InsufficientBalance = "INSUFFICIENT_BALANCE"
    InvalidSymbol = "INVALID_SYMBOL"
    UnsupportedOrderType = "UNSUPPORTED_ORDER_TYPE"
    PriceViolation = "PRICE_VIOLATION"
    ManualCancel = "MANUAL_CANCEL"
    Timeout = "TIMEOUT"
    Correction = "CORRECTION"
    Unknown = "UNKNOWN"


@dataclass(frozen=True, slots=True)
class DiagnosticTimestampV1:
    """某阶段的高精度诊断时间。

    保留来源精度（毫秒），只用于诊断分析；不参与事件 `ts` 因果排序。
    """

    phase: ExecutionPhase
    sourceTs: datetime
    sourcePrecision: str  # SECOND / MILLISECOND
    detail: str = ""

    def __post_init__(self) -> None:
        validateUtcTimestamp(self.sourceTs, TsPrecision.Millisecond)


@dataclass(frozen=True, slots=True)
class ExecutionReasonV1:
    """执行原因（受控代码 + 说明）。"""

    reasonCode: ExecutionReasonCode
    detail: str = ""

    def __post_init__(self) -> None:
        if not self.reasonCode:
            raise DiagnosticError("执行原因代码不能为空")


@dataclass(frozen=True, slots=True)
class DiagnosticReportV1:
    """一次执行的完整诊断时间线。"""

    clientOrderId: str
    brokerOrderId: str | None
    timestamps: tuple[DiagnosticTimestampV1, ...]
    reason: ExecutionReasonV1 | None = None

    def __post_init__(self) -> None:
        if not self.clientOrderId:
            raise DiagnosticError("诊断报告标识字段不能为空")

    def phaseTs(self, phase: ExecutionPhase) -> datetime | None:
        for timestamp in self.timestamps:
            if timestamp.phase is phase:
                return timestamp.sourceTs
        return None

    def latencyBetween(self, start: ExecutionPhase, end: ExecutionPhase) -> float | None:
        """两阶段间延迟秒数；缺任一阶段返回 None。"""
        startTs = self.phaseTs(start)
        endTs = self.phaseTs(end)
        if startTs is None or endTs is None:
            return None
        return (endTs - startTs).total_seconds()


class DiagnosticCollectorV1:
    """按 clientOrderId 采集执行阶段时间与原因。"""

    def __init__(self) -> None:
        self._reports: dict[str, DiagnosticReportV1] = {}

    def record(
        self,
        *,
        clientOrderId: str,
        brokerOrderId: str | None,
        phase: ExecutionPhase,
        sourceTs: datetime,
        precision: str = "MILLISECOND",
        detail: str = "",
    ) -> None:
        """记录一个阶段时间；重复阶段以最晚为准（不产生重复副作用）。"""
        existing = self._reports.get(clientOrderId)
        timestamps: list[DiagnosticTimestampV1] = list(existing.timestamps) if existing else []
        timestamps = [t for t in timestamps if t.phase is not phase]
        timestamps.append(
            DiagnosticTimestampV1(
                phase=phase, sourceTs=sourceTs, sourcePrecision=precision, detail=detail
            )
        )
        timestamps.sort(key=lambda t: (t.sourceTs, t.phase.value))
        self._reports[clientOrderId] = DiagnosticReportV1(
            clientOrderId=clientOrderId,
            brokerOrderId=brokerOrderId,
            timestamps=tuple(timestamps),
            reason=existing.reason if existing else None,
        )

    def recordReason(
        self, *, clientOrderId: str, reasonCode: ExecutionReasonCode, detail: str = ""
    ) -> None:
        """记录执行原因；原因与时间线独立（原因不参与排序）。"""
        existing = self._reports.get(clientOrderId)
        if existing is None:
            self._reports[clientOrderId] = DiagnosticReportV1(
                clientOrderId=clientOrderId,
                brokerOrderId=None,
                timestamps=(),
                reason=ExecutionReasonV1(reasonCode=reasonCode, detail=detail),
            )
            return
        self._reports[clientOrderId] = DiagnosticReportV1(
            clientOrderId=existing.clientOrderId,
            brokerOrderId=existing.brokerOrderId,
            timestamps=existing.timestamps,
            reason=ExecutionReasonV1(reasonCode=reasonCode, detail=detail),
        )

    def report(self, clientOrderId: str) -> DiagnosticReportV1 | None:
        return self._reports.get(clientOrderId)

    def reports(self) -> tuple[DiagnosticReportV1, ...]:
        return tuple(self._reports.values())


class DiagnosticTimeProvider(Protocol):
    """高精度时间源端口：由适配器注入（仿真券商时钟）。"""

    def now(self) -> datetime: ...
