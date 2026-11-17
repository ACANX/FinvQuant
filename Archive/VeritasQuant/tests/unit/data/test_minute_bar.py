from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.core.Time import TsPrecision
from veritasquant.data.MinuteBar import MinuteBarContractError, MinuteBarSchemaV1
from veritasquant.instruments.Registry import InstrumentContractError, InstrumentV1


def makeEtf() -> InstrumentV1:
    return InstrumentV1.model_validate({
        "InstrumentId": "sse-518880",
        "Symbol": "518880",
        "Market": "SSE",
        "AssetClass": "EQUITY_ETF",
        "Currency": "CNY",
        "MetadataVersion": "2026.1",
        "CalendarId": "calendar-sse",
        "FeeScheduleId": "fee-cny-etf",
        "TickSize": Decimal("0.001"),
        "LotSize": Decimal("100"),
        "SettlementRule": "SECURITY_T_PLUS_1",
    })


def makeBar(**overrides: object) -> MinuteBarSchemaV1:
    values: dict[str, object] = {
        "Ts": datetime(2026, 7, 31, 2, 31, tzinfo=timezone.utc),
        "BarStart": datetime(2026, 7, 31, 2, 30, tzinfo=timezone.utc),
        "BarEnd": datetime(2026, 7, 31, 2, 31, tzinfo=timezone.utc),
        "Symbol": "518880", "Market": "SSE", "Open": Decimal("1.001"), "High": Decimal("1.005"),
        "Low": Decimal("1.000"), "Close": Decimal("1.004"), "Volume": Decimal("1200"), "Amount": Decimal("1204.8"),
        "TradeCount": 5, "Currency": "CNY", "SessionId": "day", "Source": "fixture",
        "SourceRecordId": "row-1", "SourceSequence": 1, "IsAdjusted": False,
        "AdjustmentVersion": None, "InstrumentMetadataVersion": "2026.1", "QualityFlags": 0,
    }
    values.update(overrides)
    return MinuteBarSchemaV1.model_validate(values)


def test_minute_bar_accepts_complete_decimal_etf_bar() -> None:
    bar = makeBar()
    assert bar.validateAgainstInstrument(makeEtf(), TsPrecision.Second) is bar
    assert bar.model_dump(by_alias=True)["Ts"] == datetime(2026, 7, 31, 2, 31, tzinfo=timezone.utc)


def test_minute_bar_rejects_invalid_ohlc_time_and_adjustment_contracts() -> None:
    with pytest.raises(ValidationError):
        makeBar(Open=Decimal("0.999"))
    with pytest.raises(ValidationError):
        makeBar(BarEnd=datetime(2026, 7, 31, 2, 29, tzinfo=timezone.utc))
    with pytest.raises(ValidationError):
        makeBar(IsAdjusted=True)
    with pytest.raises(ValidationError):
        makeBar(Open=1.001)


def test_minute_bar_rejects_wrong_tick_lot_or_metadata_version() -> None:
    instrument = makeEtf()
    with pytest.raises(MinuteBarContractError, match="tickSize"):
        makeBar(Open=Decimal("1.0015")).validateAgainstInstrument(instrument, TsPrecision.Second)
    with pytest.raises(MinuteBarContractError, match="lotSize"):
        makeBar(Volume=Decimal("1050")).validateAgainstInstrument(instrument, TsPrecision.Second)
    with pytest.raises(InstrumentContractError, match="元数据版本"):
        makeBar(InstrumentMetadataVersion="old").validateAgainstInstrument(instrument, TsPrecision.Second)
