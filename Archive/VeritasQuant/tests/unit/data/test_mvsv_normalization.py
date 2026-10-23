from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal
from zoneinfo import ZoneInfo

import pytest
from pydantic import ValidationError

from veritasquant.core.Time import TsPrecision
from veritasquant.data.Mvsv import MvsvHeaderV1, MvsvRecordV1
from veritasquant.data.MvsvNormalization import (
    TURNOVER_SCALE_UNKNOWN,
    MvsvImportPolicyV1,
    MvsvNormalizationError,
    MvsvNormalizerV1,
)
from veritasquant.instruments.Registry import InstrumentV1


def makeInstrument() -> InstrumentV1:
    return InstrumentV1.model_validate({"InstrumentId": "sse-518880", "Symbol": "518880", "Market": "SSE", "AssetClass": "EQUITY_ETF", "Currency": "CNY", "MetadataVersion": "v1", "CalendarId": "c", "FeeScheduleId": "f", "TickSize": Decimal("0.001"), "LotSize": Decimal("100"), "SettlementRule": "SECURITY_T_PLUS_1"})


def makeHeader() -> MvsvHeaderV1:
    return MvsvHeaderV1({"Code": "518880"}, 1, ZoneInfo("UTC"))


def makeRecord(**overrides: object) -> MvsvRecordV1:
    values: dict[str, object] = {"sourceTs": datetime(2026, 7, 31, 2, 30, tzinfo=timezone.utc), "sourceLocalTime": datetime(2026, 7, 31, 2, 30, tzinfo=timezone.utc), "open": Decimal("1.001"), "close": Decimal("1.004"), "low": Decimal("1.000"), "high": Decimal("1.005"), "volume": Decimal("100"), "turnover": Decimal("100.4"), "change": Decimal("0.004"), "changeRate": Decimal("0.3984063745019920318725099602"), "previousClose": Decimal("1.000"), "sourceLine": 20, "sourceSequence": 1}
    values.update(overrides)
    return MvsvRecordV1(**values)


def policy(**overrides: object) -> MvsvImportPolicyV1:
    values: dict[str, object] = {"Source": "fixture", "BarLabelMeaning": "Start", "BarIntervalSeconds": 60, "AvailabilityDelaySeconds": 0, "TurnoverScale": None, "ChangeTolerance": Decimal("0"), "ChangeRateTolerance": Decimal("0.0000000001")}
    values.update(overrides)
    return MvsvImportPolicyV1.model_validate(values)


def test_normalizer_preserves_provenance_and_unknown_turnover_scale() -> None:
    bar = MvsvNormalizerV1(policy(), makeInstrument(), TsPrecision.Second).normalize(makeHeader(), makeRecord(), "a" * 64, "Data/sample.mvsv")
    assert bar.amount is None
    assert bar.qualityFlags & TURNOVER_SCALE_UNKNOWN
    assert bar.sourceRecordId.endswith(":Data/sample.mvsv:20")


def test_normalizer_requires_explicit_label_and_validates_cp_cr() -> None:
    with pytest.raises(ValidationError):
        MvsvImportPolicyV1.model_validate({"Source": "x"})
    normalizer = MvsvNormalizerV1(policy(TurnoverScale=Decimal("10")), makeInstrument(), TsPrecision.Second)
    with pytest.raises(MvsvNormalizationError, match="cp"):
        normalizer.normalize(makeHeader(), makeRecord(change=Decimal("0")), "a" * 64, "sample.mvsv")
    with pytest.raises(MvsvNormalizationError, match="cr"):
        normalizer.normalize(makeHeader(), makeRecord(changeRate=Decimal("0")), "a" * 64, "sample.mvsv")
