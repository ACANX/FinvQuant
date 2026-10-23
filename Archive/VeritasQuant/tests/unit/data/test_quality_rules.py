"""P1-021 质量规则、隔离记录与 dry-run 摘要验证。"""

from __future__ import annotations

from datetime import date, datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.core.Time import TsPrecision
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.data.QualityRules import (
    QualityRuleConfigV1,
    QualityRuleEngineV1,
    QualityRuleKind,
)
from veritasquant.instruments.Registry import (
    AssetClass,
    ExecutionMode,
    InstrumentRegistryV1,
    Market,
)


def _instrument() -> "object":
    """构建一个 SSE ETF 标的（含能力清单），与测试注册表一致。"""
    registry = InstrumentRegistryV1.model_validate({
        "RegistryVersion": "1.0",
        "Instruments": (
            {
                "InstrumentId": "sse-518880",
                "Symbol": "518880",
                "Market": Market.Sse,
                "AssetClass": AssetClass.EquityEtf,
                "Currency": "CNY",
                "MetadataVersion": "2026.1",
                "CalendarId": "calendar-sse",
                "FeeScheduleId": "fee-cny-etf",
                "TickSize": Decimal("0.001"),
                "LotSize": Decimal("100"),
                "SettlementRule": "SECURITY_T_PLUS_1",
            },
        ),
        "Calendars": (
            {
                "CalendarId": "calendar-sse",
                "Version": "2026.1",
                "Market": Market.Sse,
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
            },
        ),
        "FeeSchedules": (
            {
                "FeeScheduleId": "fee-cny-etf",
                "Version": "2026.1",
                "Currency": "CNY",
                "BrokerFeeRate": Decimal("0.0002"),
                "ExchangeFeeRate": Decimal("0"),
                "TaxRate": Decimal("0"),
                "MinimumFee": Decimal("5"),
                "EffectiveFrom": date(2026, 1, 1),
            },
        ),
        "Capabilities": (
            {
                "CapabilityVersion": "A-1",
                "InstrumentId": "sse-518880",
                "AssetClass": AssetClass.EquityEtf,
                "Market": Market.Sse,
                "AllowedExecutionModes": (ExecutionMode.Backtest,),
                "MinuteBarSchemaId": "MinuteBarSchemaV1",
                "CalendarVersion": "2026.1",
                "InstrumentMetadataVersion": "2026.1",
                "FeeScheduleVersion": "2026.1",
                "ExecutionAdapterId": "ideal-v1",
                "ContractTestHashes": ("a" * 64,),
            },
        ),
    })
    return registry.instruments[0]


_INSTRUMENT = _instrument()


def _config(**overrides: object) -> QualityRuleConfigV1:
    base: dict[str, object] = {
        "QualityRuleVersion": "q-v1",
        "MaxGapSeconds": 180,
        "AllowUnknownTurnoverScale": True,
    }
    base.update(overrides)
    return QualityRuleConfigV1.model_validate(base)


def _bar(
    minute: int,
    *,
    session: str = "day",
    symbol: str = "518880",
    market: str = "SSE",
    amount: Decimal | None = Decimal("100.00000000"),
    sourceSequence: int | None = None,
    sessionUnverified: bool = False,
    gap: bool = False,
) -> MinuteBarSchemaV1:
    start = datetime(2026, 8, 3, 9, minute, tzinfo=timezone.utc)
    end = datetime(2026, 8, 3, 9, minute + 1, tzinfo=timezone.utc)
    if gap:
        start = datetime(2026, 8, 3, 10, minute, tzinfo=timezone.utc)
        end = datetime(2026, 8, 3, 10, minute + 1, tzinfo=timezone.utc)
    return MinuteBarSchemaV1.model_validate({
        "Ts": end,
        "BarStart": start,
        "BarEnd": end,
        "Symbol": symbol,
        "Market": market,
        "Open": Decimal("10.000"),
        "High": Decimal("10.500"),
        "Low": Decimal("9.900"),
        "Close": Decimal("10.200"),
        "Volume": Decimal("1000"),
        "Amount": amount,
        "TradeCount": 5,
        "Currency": "CNY",
        "SessionId": "source-unverified" if sessionUnverified else session,
        "Source": "fixture",
        "SourceRecordId": f"fixture:{minute}",
        "SourceSequence": minute if sourceSequence is None else sourceSequence,
        "IsAdjusted": False,
        "AdjustmentVersion": None,
        "InstrumentMetadataVersion": "2026.1",
        "QualityFlags": 0,
    })


def _engine(**configOverrides: object) -> QualityRuleEngineV1:
    return QualityRuleEngineV1(
        _config(**configOverrides),
        _INSTRUMENT,
        TsPrecision.Millisecond,
        sessionIds={"day"},
    )


def test_all_rules_pass_for_clean_bars() -> None:
    engine = _engine()
    assert engine.check(_bar(1))
    assert engine.check(_bar(2))
    summary = engine.dryRun([_bar(1), _bar(2)], "0" * 64)
    assert summary.acceptedCount == 2
    assert summary.isolatedCount == 0
    assert len(summary.configHash) == 64
    assert len(summary.inputFileHash) == 64


def test_time_order_reversal_is_isolated_not_silently_fixed() -> None:
    engine = _engine()
    engine.check(_bar(2))
    assert not engine.check(_bar(1))
    summary = engine.dryRun([_bar(2), _bar(1)], "0" * 64)
    assert summary.isolatedCount == 1
    assert summary.isolationRecords[0].ruleKind is QualityRuleKind.TimeOrder
    assert summary.acceptedCount == 1


def test_duplicate_primary_key_is_isolated() -> None:
    engine = _engine()
    assert engine.check(_bar(1))
    assert not engine.check(_bar(1))
    kinds = [record.ruleKind for record in engine._isolated]
    assert QualityRuleKind.Duplicate in kinds


def test_gap_over_threshold_is_isolated() -> None:
    engine = _engine()
    engine.check(_bar(1))
    # 10:00 与 09:01 差距远超 180 秒
    assert not engine.check(_bar(2, gap=True))
    kinds = [record.ruleKind for record in engine._isolated]
    assert QualityRuleKind.Gap in kinds


def test_session_mismatch_and_unverified_session_are_isolated() -> None:
    engine = _engine()
    assert not engine.check(_bar(1, session="night"))
    assert not engine.check(_bar(2, sessionUnverified=True))
    kinds = [record.ruleKind for record in engine._isolated]
    assert kinds.count(QualityRuleKind.Session) == 2


def test_instrument_mapping_mismatch_is_isolated() -> None:
    engine = _engine()
    assert not engine.check(_bar(1, symbol="OTHER", market="SSE"))
    kinds = [record.ruleKind for record in engine._isolated]
    assert QualityRuleKind.InstrumentMapping in kinds


def test_source_quality_rule_blocks_unknown_turnover_when_disallowed() -> None:
    engine = _engine(AllowUnknownTurnoverScale=False)
    assert not engine.check(_bar(1, amount=None))
    kinds = [record.ruleKind for record in engine._isolated]
    assert QualityRuleKind.SourceQuality in kinds


def test_dry_run_is_reviewable_and_deterministic() -> None:
    first = _engine().dryRun([_bar(1), _bar(2), _bar(1)], "0" * 64)
    second = _engine().dryRun([_bar(1), _bar(2), _bar(1)], "0" * 64)
    assert first.isolationRecordHash == second.isolationRecordHash
    # 第二次 bar(1) 同时违反 TimeOrder 与 Duplicate
    assert first.toDict()["isolated_count"] == 2
    assert first.toDict()["accepted_count"] == 2
    assert first.toDict()["contract_hash"] == second.toDict()["contract_hash"]
    kinds = [record.ruleKind for record in first.isolationRecords]
    assert QualityRuleKind.TimeOrder in kinds and QualityRuleKind.Duplicate in kinds


def test_config_version_must_not_be_empty() -> None:
    with pytest.raises(ValidationError, match="版本不得为空"):
        QualityRuleConfigV1.model_validate({
            "QualityRuleVersion": "  ",
            "MaxGapSeconds": 180,
            "AllowUnknownTurnoverScale": True,
        })
