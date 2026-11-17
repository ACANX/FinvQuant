"""P4-007 高精度诊断时间和执行原因采集测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.broker.Diagnostics import (
    DiagnosticCollectorV1,
    DiagnosticError,
    DiagnosticReportV1,
    DiagnosticTimestampV1,
    ExecutionPhase,
    ExecutionReasonCode,
    ExecutionReasonV1,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, 0, tzinfo=timezone.utc)


class TestDiagnosticTimestamp:
    def test_valid(self) -> None:
        timestamp = DiagnosticTimestampV1(
            phase=ExecutionPhase.Accepted, sourceTs=_T0, sourcePrecision="MILLISECOND"
        )
        assert timestamp.phase is ExecutionPhase.Accepted
        assert timestamp.sourcePrecision == "MILLISECOND"

    def test_requires_utc(self) -> None:
        from veritasquant.core.Time import TimestampPrecisionError

        with pytest.raises(TimestampPrecisionError):
            DiagnosticTimestampV1(
                phase=ExecutionPhase.Submitted,
                sourceTs=datetime(2026, 8, 3, 2, 0, 0),  # naive
                sourcePrecision="MILLISECOND",
            )


class TestExecutionReason:
    def test_valid(self) -> None:
        reason = ExecutionReasonV1(reasonCode=ExecutionReasonCode.InsufficientBalance, detail="余额不足")
        assert reason.reasonCode is ExecutionReasonCode.InsufficientBalance

    def test_requires_code(self) -> None:
        with pytest.raises(DiagnosticError):
            ExecutionReasonV1(reasonCode="")  # type: ignore[arg-type]


class TestDiagnosticCollector:
    def test_record_phases(self) -> None:
        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Submitted,
            sourceTs=_T0,
        )
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Accepted,
            sourceTs=_T0 + timedelta(milliseconds=250),
        )
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Filled,
            sourceTs=_T0 + timedelta(milliseconds=900),
        )
        report = collector.report("co-001")
        assert report is not None
        assert len(report.timestamps) == 3
        assert report.phaseTs(ExecutionPhase.Accepted) == _T0 + timedelta(milliseconds=250)
        latency = report.latencyBetween(ExecutionPhase.Submitted, ExecutionPhase.Filled)
        assert latency == pytest.approx(0.9)

    def test_duplicate_phase_keeps_latest(self) -> None:
        """重复阶段以最晚为准（不产生重复时间线条目）。"""
        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Working,
            sourceTs=_T0,
        )
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Working,
            sourceTs=_T0 + timedelta(milliseconds=100),
        )
        report = collector.report("co-001")
        assert report is not None
        assert len(report.timestamps) == 1
        assert report.phaseTs(ExecutionPhase.Working) == _T0 + timedelta(milliseconds=100)

    def test_record_reason(self) -> None:
        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Rejected,
            sourceTs=_T0,
        )
        collector.recordReason(
            clientOrderId="co-001",
            reasonCode=ExecutionReasonCode.InsufficientBalance,
            detail="余额不足",
        )
        report = collector.report("co-001")
        assert report is not None
        assert report.reason is not None
        assert report.reason.reasonCode is ExecutionReasonCode.InsufficientBalance

    def test_reason_without_timeline(self) -> None:
        collector = DiagnosticCollectorV1()
        collector.recordReason(
            clientOrderId="co-002",
            reasonCode=ExecutionReasonCode.Timeout,
            detail="超时未知",
        )
        report = collector.report("co-002")
        assert report is not None
        assert report.reason is not None
        assert len(report.timestamps) == 0

    def test_missing_phase_latency_none(self) -> None:
        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Submitted,
            sourceTs=_T0,
        )
        report = collector.report("co-001")
        assert report is not None
        assert report.latencyBetween(ExecutionPhase.Submitted, ExecutionPhase.Filled) is None

    def test_report_requires_client_order_id(self) -> None:
        with pytest.raises(DiagnosticError):
            DiagnosticReportV1(clientOrderId="", brokerOrderId=None, timestamps=())

    def test_timestamps_sorted(self) -> None:
        collector = DiagnosticCollectorV1()
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Filled,
            sourceTs=_T0 + timedelta(milliseconds=900),
        )
        collector.record(
            clientOrderId="co-001",
            brokerOrderId="broker-001",
            phase=ExecutionPhase.Submitted,
            sourceTs=_T0,
        )
        report = collector.report("co-001")
        assert report is not None
        phases = [t.phase for t in report.timestamps]
        assert phases == [ExecutionPhase.Submitted, ExecutionPhase.Filled]
