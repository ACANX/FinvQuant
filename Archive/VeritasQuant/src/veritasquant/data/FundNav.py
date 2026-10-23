"""P2-010 FundNavSchemaV1、基金状态、费率和日历版本。

TechSpec 5.5：
- `ts`（发布可用时刻）与 `nav_date`（净值归属日）严格分离；
- 缺少 `published_at` 的历史来源必须使用 manifest 固定的保守
  `NavAvailabilityPolicy`（例如下一基金交易日开盘）并在报告中披露；
- 供应商修订产生新的原始对象、规范化文件和 manifest，通过
  `SupersedesDataVersionId` 关联旧版本，禁止覆盖已用于回测的净值。
"""

from __future__ import annotations

from datetime import date, datetime, timedelta, timezone
from decimal import Decimal
from enum import StrEnum

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class FundNavError(ValueError):
    """基金净值、状态、费率或日历版本契约失败。"""


class FundTypeV1(StrEnum):
    """受控基金能力类型。"""

    Etf = "ETF"
    Lof = "LOF"
    Equity = "股票"
    Mixed = "混合"
    Bond = "债券"
    Index = "指数"
    Feeder = "联接"
    Money = "货币"
    Qdii = "QDII"


class FundTradingStatus(StrEnum):
    """申赎状态：开放、暂停、限额或关闭。"""

    Open = "OPEN"
    Suspended = "SUSPENDED"
    Limited = "LIMITED"
    Closed = "CLOSED"


class NavAvailabilityPolicyV1(StrEnum):
    """保守净值可用策略（缺发布时间时应用）。"""

    NextTradingDayOpen = "NEXT_TRADING_DAY_OPEN"
    SameTradingDayClose = "SAME_TRADING_DAY_CLOSE"

    def apply(self, navDate: date) -> datetime:
        """将归属日转换为保守可用时刻（UTC）。"""
        if self is NavAvailabilityPolicyV1.NextTradingDayOpen:
            # 保守：归属日下一交易日开盘（此处以 +1 天 09:30 表示；日历精确计算
            # 由基金日历版本负责，策略本身固定语义）
            nextDay = navDate + timedelta(days=1)
            return datetime.combine(nextDay, datetime.min.time().replace(hour=9, minute=30), tzinfo=timezone.utc)
        return datetime.combine(navDate, datetime.min.time().replace(hour=15, minute=0), tzinfo=timezone.utc)


class FundStatusV1(StrictModel):
    """申赎状态快照：必须带状态生效时间和来源版本。"""

    subscriptionStatus: FundTradingStatus = PascalAlias("SubscriptionStatus")
    redemptionStatus: FundTradingStatus = PascalAlias("RedemptionStatus")
    effectiveFrom: datetime = PascalAlias("EffectiveFrom")
    sourceVersion: str = PascalAlias("SourceVersion", min_length=1)

    @field_validator("subscriptionStatus", "redemptionStatus", mode="before")
    @classmethod
    def parseStatus(cls, value: object) -> FundTradingStatus:
        if isinstance(value, FundTradingStatus):
            return value
        if not isinstance(value, str):
            raise FundNavError("基金状态必须是受控字符串")
        try:
            return FundTradingStatus(value)
        except ValueError as error:
            raise FundNavError(f"未知基金状态: {value}") from error

    @field_validator("effectiveFrom", mode="before")
    @classmethod
    def parseTime(cls, value: object) -> datetime:
        if not isinstance(value, datetime):
            raise FundNavError("状态生效时间必须是 UTC datetime")
        return validateUtcTimestamp(value, TsPrecision.Millisecond)


class FundRateScheduleV1(StrictModel):
    """版本化费率计划：申购/赎回/管理费率。"""

    rateScheduleVersion: str = PascalAlias("RateScheduleVersion", min_length=1)
    subscriptionRate: Decimal = PascalAlias("SubscriptionRate", ge=Decimal("0"), le=Decimal("1"))
    redemptionRate: Decimal = PascalAlias("RedemptionRate", ge=Decimal("0"), le=Decimal("1"))
    managementRate: Decimal = PascalAlias("ManagementRate", ge=Decimal("0"), le=Decimal("1"))
    effectiveFrom: datetime = PascalAlias("EffectiveFrom")

    @field_validator("subscriptionRate", "redemptionRate", "managementRate", mode="before")
    @classmethod
    def parseRate(cls, value: object) -> Decimal:
        if not isinstance(value, Decimal):
            raise FundNavError("费率必须为 Decimal")
        return value

    @field_validator("effectiveFrom", mode="before")
    @classmethod
    def parseTime(cls, value: object) -> datetime:
        if not isinstance(value, datetime):
            raise FundNavError("费率生效时间必须是 UTC datetime")
        return validateUtcTimestamp(value, TsPrecision.Millisecond)


class FundCalendarV1(StrictModel):
    """版本化基金交易日历：交易日集合 + 版本号。"""

    calendarVersion: str = PascalAlias("CalendarVersion", min_length=1)
    tradingDays: tuple[date, ...] = PascalAlias("TradingDays", min_length=1)

    @field_validator("tradingDays", mode="before")
    @classmethod
    def parseDays(cls, value: object) -> tuple[date, ...]:
        if not isinstance(value, (tuple, list)) or not value:
            raise FundNavError("基金日历必须包含至少一个交易日")
        days = tuple(datetime.fromisoformat(item).date() if isinstance(item, str) else item for item in value)
        if days != tuple(sorted(days)):
            raise FundNavError("基金交易日必须升序且唯一")
        return days

    def isTradingDay(self, day: date) -> bool:
        return day in self.tradingDays

    def nextTradingDayAfter(self, day: date) -> date:
        for candidate in self.tradingDays:
            if candidate > day:
                return candidate
        raise FundNavError(f"日历中没有 {day} 之后的交易日")


class FundNavSchemaV1(StrictModel):
    """不可变基金净值记录：nav_date 与可用 ts 分离。"""

    ts: datetime = PascalAlias("Ts")
    navDate: date = PascalAlias("NavDate")
    publishedAt: datetime | None = PascalAlias("PublishedAt", default=None)
    ingestedAt: datetime = PascalAlias("IngestedAt")
    symbol: str = PascalAlias("Symbol", min_length=1)
    fundType: FundTypeV1 = PascalAlias("FundType")
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    unitNav: Decimal = PascalAlias("UnitNav", gt=Decimal("0"))
    accumulatedNav: Decimal | None = PascalAlias("AccumulatedNav", default=None, gt=Decimal("0"))
    subscriptionStatus: FundTradingStatus = PascalAlias("SubscriptionStatus")
    redemptionStatus: FundTradingStatus = PascalAlias("RedemptionStatus")
    source: str = PascalAlias("Source", min_length=1)
    sourceSequence: int = PascalAlias("SourceSequence", ge=0)
    fundMetadataVersion: str = PascalAlias("FundMetadataVersion", min_length=1)
    qualityFlags: int = PascalAlias("QualityFlags", ge=0)

    @field_validator("fundType", mode="before")
    @classmethod
    def parseFundType(cls, value: object) -> FundTypeV1:
        if isinstance(value, FundTypeV1):
            return value
        if not isinstance(value, str):
            raise FundNavError("基金类型必须是受控字符串")
        try:
            return FundTypeV1(value)
        except ValueError as error:
            raise FundNavError(f"未知基金类型: {value}") from error

    @field_validator("subscriptionStatus", "redemptionStatus", mode="before")
    @classmethod
    def parseStatus(cls, value: object) -> FundTradingStatus:
        if isinstance(value, FundTradingStatus):
            return value
        if not isinstance(value, str):
            raise FundNavError("基金状态必须是受控字符串")
        try:
            return FundTradingStatus(value)
        except ValueError as error:
            raise FundNavError(f"未知基金状态: {value}") from error

    @field_validator("ts", "publishedAt", "ingestedAt")
    @classmethod
    def parseTime(cls, value: datetime | None) -> datetime | None:
        if value is None:
            return None
        return validateUtcTimestamp(value, TsPrecision.Millisecond)

    @field_validator("unitNav", "accumulatedNav", mode="before")
    @classmethod
    def parseNav(cls, value: object) -> Decimal | None:
        if value is None:
            return None
        if not isinstance(value, Decimal):
            raise FundNavError("净值必须为 Decimal，禁止 float")
        return value

    @model_validator(mode="after")
    def validateFundNav(self) -> "FundNavSchemaV1":
        if self.ts < self.ingestedAt:
            raise FundNavError("可用 ts 不得早于平台接收时间 ingested_at")
        if self.publishedAt is not None and self.ts < self.publishedAt:
            raise FundNavError("可用 ts 不得早于来源公布时间 published_at")
        return self
