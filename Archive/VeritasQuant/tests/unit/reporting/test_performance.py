from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.reporting.Performance import (
    CashFlowPointV1,
    EquityPointV1,
    MetricsError,
    PerformanceCalculatorV1,
    TradeRecordV1,
)

UTC = timezone.utc


def _t(day: int) -> datetime:
    return datetime(2026, 8, day, tzinfo=UTC)


def _equity(points: list[tuple[int, str]]) -> tuple[EquityPointV1, ...]:
    return tuple(EquityPointV1(_t(day), Decimal(value)) for day, value in points)


def test_hand_calculated_return_sample() -> None:
    """固定手算样本：1000 -> 1100，无外部现金流，总收益 10%。"""
    calculator = PerformanceCalculatorV1()
    metrics = calculator.calculate(
        equityCurve=_equity([(1, "1000"), (2, "1100")]),
        cashFlows=(),
        trades=(),
    )
    assert metrics.totalReturn == Decimal("0.10")


def test_external_cash_flow_not_counted_as_strategy_return() -> None:
    """外部入金 500 不计为策略收益：1000 -> 1600（含 500 入金）= 策略收益 10%。"""
    calculator = PerformanceCalculatorV1()
    metrics = calculator.calculate(
        equityCurve=_equity([(1, "1000"), (2, "1600")]),
        cashFlows=(CashFlowPointV1(_t(2), Decimal("500"), isExternal=True),),
        trades=(),
    )
    assert metrics.totalReturn == Decimal("0.10")


def test_max_drawdown_hand_calculated() -> None:
    """1000 -> 1200 -> 900 -> 1100：峰值 1200，回撤 300/1200 = 25%。"""
    calculator = PerformanceCalculatorV1()
    metrics = calculator.calculate(
        equityCurve=_equity([(1, "1000"), (2, "1200"), (3, "900"), (4, "1100")]),
        cashFlows=(),
        trades=(),
    )
    assert metrics.maxDrawdown == Decimal("0.25")


def test_sharpe_ratio_positive_for_steady_growth() -> None:
    calculator = PerformanceCalculatorV1()
    metrics = calculator.calculate(
        equityCurve=_equity([(1, "1000"), (2, "1010"), (3, "1020"), (4, "1030")]),
        cashFlows=(),
        trades=(),
    )
    assert metrics.sharpeRatio > 0


def test_fees_turnover_and_fill_rate() -> None:
    calculator = PerformanceCalculatorV1()
    trades = (
        TradeRecordV1(_t(1), Decimal("100"), Decimal("10.0"), Decimal("5"), isIdeal=True),
        TradeRecordV1(_t(2), Decimal("100"), Decimal("10.0"), Decimal("8"), isIdeal=False),
    )
    metrics = calculator.calculate(
        equityCurve=_equity([(1, "1000"), (2, "2000")]),
        cashFlows=(),
        trades=trades,
    )
    assert metrics.totalFees == Decimal("13")
    assert metrics.fillRate == Decimal("1.0")  # 真实 1 笔 / 理想 1 笔
    assert metrics.turnover == Decimal("1.0")  # 2000/2000
    assert metrics.averageSlippage == Decimal("8")


def test_rejects_empty_or_single_point_curve() -> None:
    calculator = PerformanceCalculatorV1()
    with pytest.raises(MetricsError, match="不能为空"):
        calculator.calculate(equityCurve=(), cashFlows=(), trades=())
    with pytest.raises(MetricsError, match="至少"):
        calculator.calculate(equityCurve=_equity([(1, "1000")]), cashFlows=(), trades=())


def test_rejects_non_positive_equity() -> None:
    calculator = PerformanceCalculatorV1()
    with pytest.raises(MetricsError, match="净值"):
        calculator.calculate(equityCurve=_equity([(1, "1000"), (2, "0")]), cashFlows=(), trades=())


def test_metrics_hash_deterministic() -> None:
    calculator = PerformanceCalculatorV1()
    first = calculator.calculate(equityCurve=_equity([(1, "1000"), (2, "1100")]), cashFlows=(), trades=())
    second = calculator.calculate(equityCurve=_equity([(1, "1000"), (2, "1100")]), cashFlows=(), trades=())
    assert first.metricsHash == second.metricsHash
    assert len(first.metricsHash) == 64
