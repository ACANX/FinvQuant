from __future__ import annotations

from datetime import datetime, timedelta, timezone
from decimal import Decimal

import pytest

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.strategy.IndicatorWindow import (
    BarQueryWindowV1,
    IncrementalWindowV1,
    IndicatorError,
)

UTC = timezone.utc


def _bar(ts: datetime, close: Decimal, volume: Decimal = Decimal("1000")) -> MinuteBarSchemaV1:
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": ts,
            "BarStart": ts - timedelta(minutes=1),
            "BarEnd": ts - timedelta(seconds=1),
            "Symbol": "518880",
            "Market": "SSE",
            "Open": close,
            "High": close,
            "Low": close,
            "Close": close,
            "Volume": volume,
            "Currency": "CNY",
            "SessionId": "cn-morning",
            "Source": "fixture",
            "SourceRecordId": f"bar-{ts.isoformat()}",
            "SourceSequence": 1,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "meta-v1",
            "QualityFlags": 0,
        }
    )


def _t(minute: int) -> datetime:
    return datetime(2026, 8, 2, 10, minute, tzinfo=UTC)


def _push(window: IncrementalWindowV1, minutes: range, closes: list[str]) -> None:
    for index, minute in enumerate(minutes):
        window.push(_bar(_t(minute), Decimal(closes[index])))


def test_window_metrics_use_only_consumed_data() -> None:
    window = IncrementalWindowV1()
    _push(window, range(3), ["1.000", "1.100", "1.200"])
    metrics = window.metrics()
    assert metrics.barCount == 3
    assert metrics.closeMean == Decimal("1.100")
    assert metrics.closeMax == Decimal("1.200")
    assert metrics.closeMin == Decimal("1.000")
    assert metrics.volumeSum == Decimal("3000")


def test_window_rejects_future_or_out_of_order_bars() -> None:
    window = IncrementalWindowV1()
    window.push(_bar(_t(1), Decimal("1.000")))
    with pytest.raises(IndicatorError, match="严格递增"):
        window.push(_bar(_t(1), Decimal("1.100")))
    with pytest.raises(IndicatorError, match="严格递增"):
        window.push(_bar(_t(0), Decimal("0.900")))


def test_moving_average_uses_lookback_only() -> None:
    window = IncrementalWindowV1()
    _push(window, range(5), ["1.000", "1.100", "1.200", "1.300", "1.400"])
    assert window.movingAverage(3) == Decimal("1.300")
    assert window.movingAverage(5) == Decimal("1.200")
    with pytest.raises(IndicatorError, match="周期"):
        window.movingAverage(6)


def test_window_bounds_and_capacity() -> None:
    window = IncrementalWindowV1(capacity=3)
    for index in range(5):
        window.push(_bar(_t(index), Decimal(f"1.0{index}")))
    assert window.barCount == 3  # 有界窗口
    with pytest.raises(IndicatorError, match="容量"):
        IncrementalWindowV1(capacity=1)


def test_metrics_hash_is_deterministic() -> None:
    first = IncrementalWindowV1()
    second = IncrementalWindowV1()
    _push(first, range(3), ["1.000", "1.100", "1.200"])
    _push(second, range(3), ["1.000", "1.100", "1.200"])
    assert first.metrics().metricsHash == second.metrics().metricsHash
    # 未来数据修改不影响当前输出：追加新 Bar 后历史指标不变
    third = IncrementalWindowV1()
    _push(third, range(3), ["1.000", "1.100", "1.200"])
    history = third.metrics(lookback=3).metricsHash
    third.push(_bar(_t(3), Decimal("9.999")))
    assert third.metrics(lookback=3).metricsHash != history  # 窗口滑动
    assert third.metrics(lookback=3).barCount == 3


def test_empty_window_rejects_metrics() -> None:
    with pytest.raises(IndicatorError, match="为空"):
        IncrementalWindowV1().metrics()


def test_bar_query_returns_only_completed_bars() -> None:
    query = BarQueryWindowV1()
    query.registerBar(_bar(_t(1), Decimal("1.000")))
    query.registerBar(_bar(_t(2), Decimal("1.100")))
    bars = query.queryCompleted()
    assert len(bars) == 2
    assert query.latestCompleted() is not None
    assert query.latestCompleted().close == Decimal("1.100")  # type: ignore[union-attr]
    # 未完成 Bar 不存在于查询结果：queryAsOf 只返回 barEnd <= asOf 的 Bar
    asOf1 = query.queryAsOf(_t(1))
    assert len(asOf1) == 1
    assert asOf1[0].barEnd <= _t(1)
    assert len(query.queryAsOf(_t(2))) == 2


def test_bar_query_rejects_non_monotonic_clock() -> None:
    query = BarQueryWindowV1()
    query.registerBar(_bar(_t(2), Decimal("1.100")))
    with pytest.raises(IndicatorError, match="推进"):
        query.registerBar(_bar(_t(1), Decimal("1.000")))
