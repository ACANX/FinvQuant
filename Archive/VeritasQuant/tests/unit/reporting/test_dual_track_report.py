from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.reporting.DualTrackReport import (
    DualTrackReportBuilderV1,
    ReportError,
)
from veritasquant.reporting.Performance import EquityPointV1, TradeRecordV1

UTC = timezone.utc


def _t(day: int) -> datetime:
    return datetime(2026, 8, day, tzinfo=UTC)


def _equity(values: list[tuple[int, str]]) -> tuple[EquityPointV1, ...]:
    return tuple(EquityPointV1(_t(day), Decimal(value)) for day, value in values)


def _trades() -> tuple[TradeRecordV1, ...]:
    return (
        TradeRecordV1(_t(1), Decimal("100"), Decimal("10.0"), Decimal("2"), isIdeal=True),
        TradeRecordV1(_t(2), Decimal("100"), Decimal("10.0"), Decimal("5"), isIdeal=False),
    )


def test_report_records_modes_and_versions() -> None:
    builder = DualTrackReportBuilderV1()
    report = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    assert report.ideal.mode == "IDEAL"
    assert report.real.mode == "REAL"
    assert report.executionModelVersion == "DELAYED_SLIPPAGE_PARTIAL_V1"
    assert report.barPathModelVersion == "DIRECTIONAL_OHLC_V1"
    assert len(report.reportHash) == 64


def test_friction_gap_reflects_real_minus_ideal() -> None:
    builder = DualTrackReportBuilderV1()
    report = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    # 理想 20%，真实 10%，摩擦 = -10%
    assert report.friction.returnGap == Decimal("-0.10")
    assert report.friction.fillRate == Decimal("1.0")


def test_ideal_not_marked_as_live() -> None:
    builder = DualTrackReportBuilderV1()
    report = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    report.assertIdealNotMarkedAsLive()  # 不抛错


def test_data_gaps_recorded_in_report() -> None:
    builder = DualTrackReportBuilderV1()
    report = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
        dataGaps=("2026-08-03", "2026-08-04"),
    )
    assert report.dataGaps == ("2026-08-03", "2026-08-04")


def test_report_hash_deterministic() -> None:
    builder = DualTrackReportBuilderV1()
    first = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    second = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1200")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    assert first.reportHash == second.reportHash
    changed = builder.build(
        reportId="backtest-run-1",
        idealEquity=_equity([(1, "1000"), (2, "1300")]),
        realEquity=_equity([(1, "1000"), (2, "1100")]),
        trades=_trades(),
    )
    assert first.reportHash != changed.reportHash


def test_rejects_missing_report_id_and_versions() -> None:
    builder = DualTrackReportBuilderV1()
    with pytest.raises(ReportError, match="报告 ID"):
        builder.build(
            reportId="",
            idealEquity=_equity([(1, "1000"), (2, "1200")]),
            realEquity=_equity([(1, "1000"), (2, "1100")]),
            trades=_trades(),
        )
    with pytest.raises(ReportError, match="版本"):
        DualTrackReportBuilderV1(executionModelVersion="")
