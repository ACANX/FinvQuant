"""P4-005 异步回报、序列缺口、迟到成交和更正映射测试。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.broker.BrokerPort import BrokerPortError
from veritasquant.broker.ReportHandling import (
    BrokerReportV1,
    ReportCorrectionV1,
    ReportDeduplicatorV1,
    ReportGuardStatus,
    ReportSequenceGuardV1,
    UnknownOrderIsolationV1,
)
from veritasquant.execution.Orders import ExecutionType

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _report(sequence: int, **overrides: object) -> BrokerReportV1:
    values: dict[str, object] = {
        "brokerReportId": f"rep-{sequence}",
        "clientOrderId": "co-001",
        "brokerOrderId": "broker-001",
        "reportSequence": sequence,
        "executionType": ExecutionType.New,
        "executionId": None,
        "lastQuantity": "0",
        "lastPrice": None,
        "cumulativeQuantity": "0",
        "receivedAt": _T0,
    }
    values.update(overrides)
    return BrokerReportV1(**values)


class TestReportDeduplicator:
    def test_duplicate_report_not_booked_twice(self) -> None:
        """重复回报不重复记账。"""
        dedup = ReportDeduplicatorV1()
        first = dedup.check(_report(1))
        second = dedup.check(_report(1))  # 同 brokerReportId
        assert first.status is ReportGuardStatus.InOrder
        assert second.status is ReportGuardStatus.Duplicate


class TestReportSequenceGuard:
    def test_in_order(self) -> None:
        guard = ReportSequenceGuardV1()
        assert guard.check(_report(1)).status is ReportGuardStatus.InOrder
        assert guard.check(_report(2)).status is ReportGuardStatus.InOrder
        assert guard.expectedFor("broker-001") == 3

    def test_gap_detected(self) -> None:
        guard = ReportSequenceGuardV1()
        assert guard.check(_report(1)).status is ReportGuardStatus.InOrder
        gap = guard.check(_report(3))
        assert gap.status is ReportGuardStatus.GapDetected
        assert "缺口" in gap.message

    def test_old_sequence_duplicate(self) -> None:
        guard = ReportSequenceGuardV1()
        guard.check(_report(1))
        guard.check(_report(2))
        duplicate = guard.check(_report(1))
        assert duplicate.status is ReportGuardStatus.Duplicate

    def test_late_arrival(self) -> None:
        guard = ReportSequenceGuardV1()
        guard.check(_report(2))  # 先收到 seq=2
        late = guard.check(_report(1))  # seq=1 迟到
        assert late.status is ReportGuardStatus.LateArrival

    def test_requires_positive_sequence(self) -> None:
        with pytest.raises(BrokerPortError):
            _report(0)


class TestReportCorrection:
    def test_corrected_fill_does_not_add_bookkeeping(self) -> None:
        """更正映射：CORRECTED 替换原成交，不新增记账。"""
        correction = ReportCorrectionV1()
        original = _report(
            1,
            brokerReportId="rep-original",
            executionType=ExecutionType.Fill,
            executionId="exec-001",
            lastQuantity="100",
            lastPrice="5.0000",
            cumulativeQuantity="100",
        )
        correction.register(original)
        corrected = _report(
            2,
            brokerReportId="rep-corrected",
            executionType=ExecutionType.Corrected,
            executionId=None,
            lastQuantity="100",
            lastPrice="5.0100",
            cumulativeQuantity="100",
        )
        handling = correction.applyCorrection(corrected, "exec-001")
        assert handling.status is ReportGuardStatus.Corrected
        assert handling.correctedReportId == "rep-original"
        assert correction.isCorrected("rep-original") is True

    def test_correct_unknown_execution_rejected(self) -> None:
        correction = ReportCorrectionV1()
        corrected = _report(2, executionType=ExecutionType.Corrected)
        with pytest.raises(BrokerPortError, match="未知成交"):
            correction.applyCorrection(corrected, "exec-unknown")

    def test_fill_requires_execution_id(self) -> None:
        correction = ReportCorrectionV1()
        fill = _report(1, executionType=ExecutionType.Fill, executionId=None)
        with pytest.raises(BrokerPortError, match="execution_id"):
            correction.register(fill)


class TestUnknownOrderIsolation:
    def test_unknown_order_isolated_and_queried(self) -> None:
        """未知订单隔离并触发查询对账。"""
        isolation = UnknownOrderIsolationV1()
        report = _report(1, brokerOrderId="broker-unknown")
        handling = isolation.isolate(report)
        assert handling.status is ReportGuardStatus.GapDetected
        assert "隔离" in handling.message
        assert report.brokerReportId in isolation
        assert len(isolation.records()) == 1
