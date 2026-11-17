"""P2-010/011 基金净值 Schema、导入与 manifest 单元测试。

验收标准映射：
- nav_date 与可用 ts 分离；未知发布时间按保守策略；修订生成新版本。
"""

from __future__ import annotations

from datetime import date, datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.data.FundNav import (
    FundCalendarV1,
    FundNavSchemaV1,
    FundRateScheduleV1,
    FundStatusV1,
    FundTradingStatus,
    FundTypeV1,
    NavAvailabilityPolicyV1,
)
from veritasquant.data.FundNavImporter import (
    FundNavImporterV1,
    NavQualityFlags,
    RawFundNavRecordV1,
)


def _record(
    navDate: date = date(2026, 8, 3),
    publishedAt: datetime | None = None,
    symbol: str = "FUND-001",
    sourceSequence: int = 1,
    unitNav: Decimal = Decimal("1.2345"),
) -> RawFundNavRecordV1:
    return RawFundNavRecordV1(
        navDate=navDate,
        symbol=symbol,
        fundType=FundTypeV1.Equity,
        currency="CNY",
        unitNav=unitNav,
        accumulatedNav=None,
        subscriptionStatus=FundTradingStatus.Open,
        redemptionStatus=FundTradingStatus.Open,
        source="fund-source",
        sourceSequence=sourceSequence,
        fundMetadataVersion="V1",
        publishedAt=publishedAt,
    )


class TestFundNavSchema:
    def test_nav_date_separated_from_available_ts(self) -> None:
        nav = FundNavSchemaV1.model_validate(
            {
                "Ts": datetime(2026, 8, 4, 1, 30, tzinfo=timezone.utc),
                "NavDate": date(2026, 8, 3),
                "PublishedAt": datetime(2026, 8, 4, 1, 0, tzinfo=timezone.utc),
                "IngestedAt": datetime(2026, 8, 4, 0, 55, tzinfo=timezone.utc),
                "Symbol": "FUND-001",
                "FundType": "股票",
                "Currency": "CNY",
                "UnitNav": Decimal("1.23"),
                "SubscriptionStatus": "OPEN",
                "RedemptionStatus": "OPEN",
                "Source": "s",
                "SourceSequence": 1,
                "FundMetadataVersion": "V1",
                "QualityFlags": 0,
            }
        )
        assert nav.navDate.isoformat() == "2026-08-03"
        assert nav.ts.isoformat().startswith("2026-08-04")

    def test_ts_before_ingested_rejected(self) -> None:
        with pytest.raises(ValidationError):
            FundNavSchemaV1.model_validate(
                {
                    "Ts": datetime(2026, 8, 4, 0, 50, tzinfo=timezone.utc),
                    "NavDate": date(2026, 8, 3),
                    "IngestedAt": datetime(2026, 8, 4, 0, 55, tzinfo=timezone.utc),
                    "Symbol": "FUND-001",
                    "FundType": "股票",
                    "Currency": "CNY",
                    "UnitNav": Decimal("1.23"),
                    "SubscriptionStatus": "OPEN",
                    "RedemptionStatus": "OPEN",
                    "Source": "s",
                    "SourceSequence": 1,
                    "FundMetadataVersion": "V1",
                    "QualityFlags": 0,
                }
            )

    def test_ts_before_published_rejected(self) -> None:
        with pytest.raises(ValidationError):
            FundNavSchemaV1.model_validate(
                {
                    "Ts": datetime(2026, 8, 4, 1, 0, tzinfo=timezone.utc),
                    "NavDate": date(2026, 8, 3),
                    "PublishedAt": datetime(2026, 8, 4, 1, 30, tzinfo=timezone.utc),
                    "IngestedAt": datetime(2026, 8, 4, 0, 55, tzinfo=timezone.utc),
                    "Symbol": "FUND-001",
                    "FundType": "股票",
                    "Currency": "CNY",
                    "UnitNav": Decimal("1.23"),
                    "SubscriptionStatus": "OPEN",
                    "RedemptionStatus": "OPEN",
                    "Source": "s",
                    "SourceSequence": 1,
                    "FundMetadataVersion": "V1",
                    "QualityFlags": 0,
                }
            )

    def test_float_nav_rejected(self) -> None:
        with pytest.raises(ValidationError):
            FundNavSchemaV1.model_validate(
                {
                    "Ts": datetime(2026, 8, 4, 1, 30, tzinfo=timezone.utc),
                    "NavDate": date(2026, 8, 3),
                    "IngestedAt": datetime(2026, 8, 4, 0, 55, tzinfo=timezone.utc),
                    "Symbol": "FUND-001",
                    "FundType": "股票",
                    "Currency": "CNY",
                    "UnitNav": 1.23,  # float 禁止
                    "SubscriptionStatus": "OPEN",
                    "RedemptionStatus": "OPEN",
                    "Source": "s",
                    "SourceSequence": 1,
                    "FundMetadataVersion": "V1",
                    "QualityFlags": 0,
                }
            )


class TestConservativeAvailability:
    def test_missing_published_at_uses_conservative_policy(self) -> None:
        ingested = datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc)
        result = FundNavImporterV1().importRecords((_record(publishedAt=None),), ingested)
        assert len(result.normalized) == 1
        nav = result.normalized[0]
        # 保守策略：归属日下一交易日开盘，而非归属日盘前已知
        assert nav.ts >= ingested
        assert nav.qualityFlags & NavQualityFlags.MissingPublishedAt
        assert result.appliedPolicy is NavAvailabilityPolicyV1.NextTradingDayOpen

    def test_published_at_used_when_available(self) -> None:
        published = datetime(2026, 8, 3, 20, 0, tzinfo=timezone.utc)
        ingested = datetime(2026, 8, 3, 20, 5, tzinfo=timezone.utc)
        result = FundNavImporterV1().importRecords((_record(publishedAt=published),), ingested)
        nav = result.normalized[0]
        assert nav.publishedAt == published
        assert nav.ts == ingested  # max(published, ingested)
        assert nav.qualityFlags == 0


class TestRevisionAndIsolation:
    def test_revision_requires_supersedes_chain(self) -> None:
        ingested = datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc)
        with pytest.raises(Exception):
            FundNavImporterV1().importRecords(
                (_record(),), ingested, supersedesDataVersionId=None, revisionReason="修订"
            )

    def test_revision_creates_new_version(self) -> None:
        ingested = datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc)
        first = FundNavImporterV1().importRecords((_record(),), ingested)
        revised = FundNavImporterV1().importRecords(
            (_record(unitNav=Decimal("1.3000")),),
            ingested,
            supersedesDataVersionId="b" * 64,
            revisionReason="供应商修订净值",
        )
        assert first.normalized[0].unitNav == Decimal("1.2345")
        assert revised.normalized[0].unitNav == Decimal("1.3000")
        assert revised.manifest.supersedesDataVersionId == "b" * 64

    def test_duplicate_symbol_nav_date_isolated(self) -> None:
        ingested = datetime(2026, 8, 3, 10, 0, tzinfo=timezone.utc)
        result = FundNavImporterV1().importRecords(
            (_record(sourceSequence=1), _record(sourceSequence=2)), ingested
        )
        assert len(result.normalized) == 1
        assert len(result.isolated) == 1


class TestCalendarAndRates:
    def test_calendar_requires_sorted_unique_days(self) -> None:
        calendar = FundCalendarV1.model_validate(
            {
                "CalendarVersion": "V1",
                "TradingDays": ("2026-08-03", "2026-08-04", "2026-08-05"),
            }
        )
        assert calendar.isTradingDay(date(2026, 8, 4))
        assert calendar.nextTradingDayAfter(date(2026, 8, 3)) == date(2026, 8, 4)
        with pytest.raises(ValidationError):
            FundCalendarV1.model_validate(
                {"CalendarVersion": "V1", "TradingDays": ("2026-08-04", "2026-08-03")}
            )

    def test_rate_schedule_validation(self) -> None:
        schedule = FundRateScheduleV1.model_validate(
            {
                "RateScheduleVersion": "V1",
                "SubscriptionRate": Decimal("0.0015"),
                "RedemptionRate": Decimal("0.005"),
                "ManagementRate": Decimal("0.01"),
                "EffectiveFrom": datetime(2026, 8, 3, tzinfo=timezone.utc),
            }
        )
        assert schedule.subscriptionRate == Decimal("0.0015")

    def test_fund_status_requires_effective_time(self) -> None:
        status = FundStatusV1.model_validate(
            {
                "SubscriptionStatus": "SUSPENDED",
                "RedemptionStatus": "OPEN",
                "EffectiveFrom": datetime(2026, 8, 3, tzinfo=timezone.utc),
                "SourceVersion": "V1",
            }
        )
        assert status.subscriptionStatus is FundTradingStatus.Suspended
