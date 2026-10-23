from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.core.Time import TsPrecision, TimestampPrecisionError, parseUtcTimestamp, serializeUtcTimestamp


def test_second_and_millisecond_timestamp_round_trip() -> None:
    second = parseUtcTimestamp("2026-07-31T08:15:30Z", TsPrecision.Second)
    millisecond = parseUtcTimestamp("2026-07-31T08:15:30.123Z", TsPrecision.Millisecond)
    assert serializeUtcTimestamp(second, TsPrecision.Second) == "2026-07-31T08:15:30Z"
    assert serializeUtcTimestamp(second, TsPrecision.Millisecond) == "2026-07-31T08:15:30.000Z"
    assert serializeUtcTimestamp(millisecond, TsPrecision.Millisecond) == "2026-07-31T08:15:30.123Z"


def test_second_run_rejects_millisecond_without_rounding() -> None:
    with pytest.raises(TimestampPrecisionError):
        parseUtcTimestamp("2026-07-31T08:15:30.123Z", TsPrecision.Second)
    with pytest.raises(TimestampPrecisionError):
        parseUtcTimestamp("2026-07-31T08:15:30.123456Z", TsPrecision.Millisecond)


def test_timestamp_must_be_utc() -> None:
    with pytest.raises(TimestampPrecisionError):
        parseUtcTimestamp(datetime(2026, 7, 31, 8, 15, 30), TsPrecision.Second)
    assert parseUtcTimestamp(datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc), TsPrecision.Second)
