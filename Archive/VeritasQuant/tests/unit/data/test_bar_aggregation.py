"""P1-023 小时/日 Bar 完成后聚合与受限窗口验证。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.core.Time import TsPrecision
from veritasquant.data.BarAggregation import (
    AggregatedBarV1,
    AggregationError,
    BarAggregatorV1,
    BarPeriod,
    ClosedWindowViewV1,
)
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.instruments.Registry import TradingCalendarV1


def _calendar() -> TradingCalendarV1:
    return TradingCalendarV1.model_validate({
        "CalendarId": "calendar-sse",
        "Version": "2026.1",
        "Market": "SSE",
        "TimeZone": "Asia/Shanghai",
        "Sessions": (
            {
                "SessionId": "day",
                "OpenLocalTime": "09:30",
                "CloseLocalTime": "15:00",
                "SpansMidnight": False,
                "TradingWeekdays": (0, 1, 2, 3, 4),
            },
        ),
        "Holidays": (),
    })


def _bar(
    minute: int,
    *,
    hour: int = 9,
    day: int = 3,
    close: str = "10.200",
    volume: str = "1000",
) -> MinuteBarSchemaV1:
    start = datetime(2026, 8, day, hour, minute, tzinfo=timezone.utc)
    end = start + timedelta(minutes=1)
    return MinuteBarSchemaV1.model_validate({
        "Ts": end,
        "BarStart": start,
        "BarEnd": end,
        "Symbol": "518880",
        "Market": "SSE",
        "Open": Decimal("10.000"),
        "High": Decimal("10.500"),
        "Low": Decimal("9.900"),
        "Close": Decimal(close),
        "Volume": Decimal(volume),
        "Amount": Decimal("100.00000000"),
        "TradeCount": 5,
        "Currency": "CNY",
        "SessionId": "day",
        "Source": "fixture",
        "SourceRecordId": f"fixture:{day}-{hour}-{minute}",
        "SourceSequence": minute,
        "IsAdjusted": False,
        "AdjustmentVersion": None,
        "InstrumentMetadataVersion": "2026.1",
        "QualityFlags": 0,
    })


def test_hourly_aggregation_completes_only_after_window_end() -> None:
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    aggregator.append(_bar(1, hour=9))
    aggregator.append(_bar(2, hour=9))
    # 09:02 尚未过 09:00-10:00 窗口：不可查询
    with pytest.raises(AggregationError, match="未完成"):
        aggregator.query(BarPeriod.Hour, "518880", "fixture", datetime(2026, 8, 3, 9, 2, tzinfo=timezone.utc))
    # 10:00 之后窗口完成：可查询到聚合结果
    bars = aggregator.query(BarPeriod.Hour, "518880", "fixture", datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc))
    assert len(bars) == 1
    assert bars[0].period is BarPeriod.Hour
    assert bars[0].volume == Decimal("2000")
    assert bars[0].high == Decimal("10.500")
    assert bars[0].low == Decimal("9.900")
    assert bars[0].close == Decimal("10.200")


def test_hourly_rolls_to_next_window() -> None:
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    aggregator.append(_bar(59, hour=9))
    aggregator.append(_bar(0, hour=10))
    # 推进到 10 点窗口后，09 点窗口归档
    bars = aggregator.query(BarPeriod.Hour, "518880", "fixture", datetime(2026, 8, 3, 11, 0, tzinfo=timezone.utc), limit=10)
    assert len(bars) == 2
    assert bars[0].periodStart.hour == 9
    assert bars[1].periodStart.hour == 10


def test_daily_bar_only_available_after_session_close_next_open() -> None:
    calendar = _calendar()
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, calendar)
    aggregator.append(_bar(1, hour=9))
    aggregator.append(_bar(2, hour=9))
    # 当日 09:02 UTC（=17:02 上海，已收盘但未到次日开盘）：不可查询
    with pytest.raises(AggregationError, match="下一有效时点"):
        aggregator.query(BarPeriod.Day, "518880", "fixture", datetime(2026, 8, 3, 9, 2, tzinfo=timezone.utc))
    # 次一交易日开盘后（次日 01:30 UTC = 09:30 上海）：可查询
    bars = aggregator.query(BarPeriod.Day, "518880", "fixture", datetime(2026, 8, 4, 1, 30, tzinfo=timezone.utc))
    assert len(bars) == 1
    assert bars[0].period is BarPeriod.Day
    assert bars[0].volume == Decimal("2000")


def test_append_rejects_out_of_order_minute_bars() -> None:
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    aggregator.append(_bar(5))
    with pytest.raises(AggregationError, match="升序"):
        aggregator.append(_bar(3))


def test_query_requires_positive_limit() -> None:
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    with pytest.raises(AggregationError, match="limit"):
        aggregator.query(BarPeriod.Hour, "518880", "fixture", datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc), limit=0)


def test_aggregated_bar_contract_validates_window() -> None:
    with pytest.raises(ValidationError, match="periodStart"):
        AggregatedBarV1.model_validate({
            "Period": "Hour",
            "Symbol": "518880",
            "Market": "SSE",
            "PeriodStart": datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc),
            "PeriodEnd": datetime(2026, 8, 3, 9, 0, tzinfo=timezone.utc),
            "Open": Decimal("10"),
            "High": Decimal("10"),
            "Low": Decimal("10"),
            "Close": Decimal("10"),
            "Volume": Decimal("100"),
            "Source": "fixture",
            "CompletedAt": datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc),
        })


def test_closed_window_view_is_read_only() -> None:
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    aggregator.append(_bar(1, hour=9))
    view = ClosedWindowViewV1(aggregator)
    # 小时线在窗口结束后可用
    bars = view.getHourlyBars("518880", "fixture", datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc), n=3)
    assert len(bars) == 1
    # 日线在次日开盘前不可用
    with pytest.raises(AggregationError, match="下一有效时点"):
        view.getDailyBars("518880", "fixture", datetime(2026, 8, 3, 9, 2, tzinfo=timezone.utc), n=3)


def test_half_day_and_boundary_minute_bars() -> None:
    """覆盖会话边界：09:30 开盘首分钟与 15:00 收盘前最后一分钟。"""
    aggregator = BarAggregatorV1(TsPrecision.Millisecond, _calendar())
    aggregator.append(_bar(30, hour=9))  # 09:30 首分钟（上海 09:30 开盘）
    aggregator.append(_bar(59, hour=14))  # 14:59 收盘前最后一分钟
    # 两分钟 Bar 属于同一日线窗口
    bars = aggregator.query(BarPeriod.Day, "518880", "fixture", datetime(2026, 8, 4, 1, 30, tzinfo=timezone.utc))
    assert len(bars) == 1
    assert bars[0].volume == Decimal("2000")
    assert bars[0].periodStart.hour == 0  # 两分钟 Bar 同属 8 月 3 日日线窗口
