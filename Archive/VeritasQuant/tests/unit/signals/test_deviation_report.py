"""P3-007 信号/人工偏差分析报告测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.signals.DeviationReport import (
    DeviationError,
    DeviationKind,
    DeviationReportV1,
    InMemoryDeviationStoreV1,
    SignalDeviationAnalyzerV1,
    SignalDeviationRecordV1,
)
from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    ManualExecutionV1,
    SignalReferenceV1,
    SignalStatus,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _signal(signalId: str = "sig-ref-001", **overrides: object) -> SignalReferenceV1:
    values: dict[str, object] = {
        "signalReferenceId": signalId,
        "version": 1,
        "status": SignalStatus.Pending,
        "accountId": "acc-001",
        "strategyId": "strat-dual-ma",
        "strategyChecksum": "a" * 64,
        "sourceEventId": "evt-bar-001",
        "sourceEventType": "MarketBarEvent",
        "direction": "BUY",
        "quantity": "100.0000",
        "priceLimit": "5.0000",
        "operatorId": None,
        "generatedTs": _T0,
        "expiresAt": _T0 + timedelta(minutes=15),
        "previousSignalReferenceId": None,
    }
    values.update(overrides)
    return SignalReferenceV1.create(**values)


def _execution(signalId: str = "sig-ref-001", **overrides: object) -> ManualExecutionV1:
    values: dict[str, object] = {
        "executionId": "exec-001",
        "signalReferenceId": signalId,
        "actionId": "act-001",
        "operatorId": "op-alice",
        "executedAt": _T0 + timedelta(minutes=2),
        "direction": "BUY",
        "quantity": "100.0000",
        "price": "5.0000",
        "deviationReason": None,
        "note": "",
    }
    values.update(overrides)
    return ManualExecutionV1.create(**values)


class TestSignalDeviationAnalyzer:
    def test_all_consistent(self) -> None:
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution()],
        )
        assert report.clean is True
        assert report.deviationCount == 0
        assert report.executedCount == 1
        assert report.totalSignals == 1
        assert report.explanationCoverage == 1.0

    def test_not_executed_deviation(self) -> None:
        """每条未执行必须有（人工提供的）结构化原因；analyzer 不能伪造。"""
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[],
        )
        assert report.clean is False
        assert report.deviationCount == 1
        assert report.unexplainedDeviationCount == 1
        assert report.explainedDeviationCount == 0
        assert report.deviations[0].kind is DeviationKind.NotExecuted
        assert report.deviations[0].reason.reasonCode == "NOT_EXECUTED"
        assert report.deviations[0].accountId == "acc-001"

    def test_ignored_signal_not_deviation(self) -> None:
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[],
            ignoredSignalIds=["sig-ref-001"],
        )
        assert report.clean is True
        assert report.deviationCount == 0

    def test_direction_mismatch(self) -> None:
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution(direction="SELL")],
        )
        assert report.deviationCount == 1
        assert report.deviations[0].kind is DeviationKind.DirectionMismatch
        assert report.deviations[0].reason.reasonCode == "DIRECTION_MISMATCH"

    def test_quantity_mismatch(self) -> None:
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution(quantity="50.0000")],
        )
        assert report.deviationCount == 1
        assert report.deviations[0].kind is DeviationKind.QuantityMismatch

    def test_price_slippage(self) -> None:
        """成交价 5.10 超过限价 5.00 + 0.5% 容差。"""
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution(price="5.1000")],
        )
        assert report.deviationCount == 1
        assert report.deviations[0].kind is DeviationKind.PriceSlippage

    def test_price_within_tolerance_ok(self) -> None:
        """成交价 5.02 在限价 5.00 + 0.5% 容差内 -> 一致。"""
        analyzer = SignalDeviationAnalyzerV1()
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution(price="5.0200")],
        )
        assert report.clean is True
        assert report.deviationCount == 0

    def test_user_reason_respected(self) -> None:
        """人工提供的偏差原因被保留（结构化原因覆盖率）。"""
        analyzer = SignalDeviationAnalyzerV1()
        reason = IgnoreReasonV1.create(reasonCode="MANUAL_OVERRIDE", detail="人工判断")
        report = analyzer.analyze(
            reportId="rep-001",
            runId="run-001",
            signals=[_signal()],
            executions=[_execution(direction="SELL", deviationReason=reason)],
        )
        assert report.deviations[0].reason.reasonCode == "MANUAL_OVERRIDE"
        assert report.explanationCoverage == 1.0

    def test_deviation_requires_reason(self) -> None:
        """偏差记录必须有结构化原因（空原因被模型拒绝）。"""
        from pydantic import ValidationError

        with pytest.raises(ValidationError):
            SignalDeviationRecordV1(
                deviationId="dev-001",
                signalReferenceId="sig-ref-001",
                accountId="acc-001",
                strategyId="s",
                kind=DeviationKind.NotExecuted,
                reason=IgnoreReasonV1.create(reasonCode="  ", detail=""),
                signalDirection="BUY",
                signalQuantity="100",
            )

    def test_invalid_slippage_tolerance(self) -> None:
        with pytest.raises(DeviationError):
            SignalDeviationAnalyzerV1(slippageToleranceRatio="-0.01")

    def test_report_properties(self) -> None:
        report = DeviationReportV1(
            reportId="rep-001",
            runId="run-001",
            totalSignals=2,
            executedCount=0,
            deviationCount=2,
            explainedDeviationCount=2,
            unexplainedDeviationCount=0,
        )
        assert report.clean is True
        assert report.explanationCoverage == 1.0
        report2 = DeviationReportV1(
            reportId="rep-002",
            runId="run-001",
            totalSignals=1,
            executedCount=0,
            deviationCount=1,
            explainedDeviationCount=0,
            unexplainedDeviationCount=1,
        )
        assert report2.clean is False
        assert report2.explanationCoverage == 0.0


class TestInMemoryDeviationStore:
    def test_save_get(self) -> None:
        store = InMemoryDeviationStoreV1()
        report = DeviationReportV1(
            reportId="rep-001",
            runId="run-001",
            totalSignals=0,
            executedCount=0,
            deviationCount=0,
            explainedDeviationCount=0,
            unexplainedDeviationCount=0,
        )
        store.save(report)
        assert store.get("rep-001") is report
        assert len(store.all()) == 1
