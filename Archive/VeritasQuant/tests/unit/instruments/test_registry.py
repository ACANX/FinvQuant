from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.instruments.Registry import (
    AssetCapabilityManifestV1,
    AssetClass,
    ExecutionMode,
    InstrumentContractError,
    InstrumentRegistryV1,
    Market,
)


def _capability(instrumentId: str, assetClass: AssetClass, market: Market, metadataVersion: str, calendarVersion: str, feeVersion: str) -> dict[str, object]:
    return {
        "CapabilityVersion": "A-1",
        "InstrumentId": instrumentId,
        "AssetClass": assetClass,
        "Market": market,
        "AllowedExecutionModes": (ExecutionMode.Backtest,),
        "MinuteBarSchemaId": "MinuteBarSchemaV1",
        "CalendarVersion": calendarVersion,
        "InstrumentMetadataVersion": metadataVersion,
        "FeeScheduleVersion": feeVersion,
        "ExecutionAdapterId": "ideal-v1",
        "ContractTestHashes": ("a" * 64,),
    }


def makeRegistry() -> InstrumentRegistryV1:
    sseCalendar = {
        "CalendarId": "calendar-sse",
        "Version": "2026.1",
        "Market": Market.Sse,
        "TimeZone": "Asia/Shanghai",
        "Sessions": (
            {"SessionId": "day", "OpenLocalTime": "09:30", "CloseLocalTime": "15:00", "SpansMidnight": False, "TradingWeekdays": (0, 1, 2, 3, 4)},
        ),
        "Holidays": (date(2026, 10, 1),),
    }
    shfeCalendar = {
        "CalendarId": "calendar-shfe",
        "Version": "2026.1",
        "Market": Market.Shfe,
        "TimeZone": "Asia/Shanghai",
        "Sessions": (
            {"SessionId": "night", "OpenLocalTime": "21:00", "CloseLocalTime": "02:30", "SpansMidnight": True, "TradingWeekdays": (0, 1, 2, 3, 4)},
            {"SessionId": "day", "OpenLocalTime": "09:00", "CloseLocalTime": "15:00", "SpansMidnight": False, "TradingWeekdays": (0, 1, 2, 3, 4)},
        ),
        "Holidays": (),
    }
    feeSchedules = (
        {"FeeScheduleId": "fee-cny-etf", "Version": "2026.1", "Currency": "CNY", "BrokerFeeRate": Decimal("0.0002"), "ExchangeFeeRate": Decimal("0"), "TaxRate": Decimal("0"), "MinimumFee": Decimal("5"), "EffectiveFrom": date(2026, 1, 1)},
        {"FeeScheduleId": "fee-cny-futures", "Version": "2026.1", "Currency": "CNY", "BrokerFeeRate": Decimal("0.00002"), "ExchangeFeeRate": Decimal("0.00001"), "TaxRate": Decimal("0"), "MinimumFee": Decimal("0"), "EffectiveFrom": date(2026, 1, 1)},
    )
    etf = {
        "InstrumentId": "sse-518880", "Symbol": "518880", "Market": Market.Sse, "AssetClass": AssetClass.EquityEtf,
        "Currency": "CNY", "MetadataVersion": "2026.1", "CalendarId": "calendar-sse", "FeeScheduleId": "fee-cny-etf",
        "TickSize": Decimal("0.001"), "LotSize": Decimal("100"), "SettlementRule": "SECURITY_T_PLUS_1",
    }
    future = {
        "InstrumentId": "shfe-au2608", "Symbol": "AU2608", "Market": Market.Shfe, "AssetClass": AssetClass.Futures,
        "Currency": "CNY", "MetadataVersion": "2026.1", "CalendarId": "calendar-shfe", "FeeScheduleId": "fee-cny-futures",
        "TickSize": Decimal("0.02"), "LotSize": Decimal("1"), "SettlementRule": "FUTURES_DAILY_MARK_TO_MARKET",
        "ContractMultiplier": Decimal("1000"), "InitialMarginRate": Decimal("0.12"), "ExpiryDate": date(2026, 8, 31),
    }
    return InstrumentRegistryV1.model_validate({
        "RegistryVersion": "2026.1",
        "Instruments": (etf, future),
        "Calendars": (sseCalendar, shfeCalendar),
        "FeeSchedules": feeSchedules,
        "Capabilities": (
            _capability("sse-518880", AssetClass.EquityEtf, Market.Sse, "2026.1", "2026.1", "2026.1"),
            _capability("shfe-au2608", AssetClass.Futures, Market.Shfe, "2026.1", "2026.1", "2026.1"),
        ),
    })


def test_registry_contains_complete_etf_and_gold_futures_metadata() -> None:
    registry = makeRegistry()
    etf = registry.requireTradable("518880", Market.Sse, ExecutionMode.Backtest)
    future = registry.requireTradable("AU2608", Market.Shfe, ExecutionMode.Backtest)
    assert etf.lotSize == Decimal("100")
    assert etf.tickSize == Decimal("0.001")
    assert future.contractMultiplier == Decimal("1000")
    assert future.initialMarginRate == Decimal("0.12")
    assert future.expiryDate == date(2026, 8, 31)


def test_registry_rejects_missing_trade_metadata_and_unapproved_mode() -> None:
    with pytest.raises(ValidationError):
        InstrumentRegistryV1.model_validate({**makeRegistry().model_dump(by_alias=True), "Instruments": ({"InstrumentId": "bad"},)})
    registry = makeRegistry()
    with pytest.raises(InstrumentContractError, match="未获当前执行模式"):
        registry.requireTradable("518880", Market.Sse, ExecutionMode.Simulation)


def test_futures_rejects_missing_margin_or_expiry_and_manifest_rejects_live() -> None:
    payload = makeRegistry().instruments[1].model_dump(by_alias=True)
    payload.pop("InitialMarginRate")
    with pytest.raises(ValidationError):
        type(makeRegistry().instruments[1]).model_validate(payload)
    capability = _capability("sse-518880", AssetClass.EquityEtf, Market.Sse, "2026.1", "2026.1", "2026.1")
    capability["AllowedExecutionModes"] = (ExecutionMode.Live,)
    with pytest.raises(ValidationError):
        AssetCapabilityManifestV1.model_validate(capability)
