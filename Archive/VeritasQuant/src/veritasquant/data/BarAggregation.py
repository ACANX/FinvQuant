"""P1-023 小时/日 Bar 完成后聚合与受限只读窗口。

聚合器只基于已完成分钟 Bar 构建小时线与日线：未完成周期不可查询；
日线仅在会话收盘后的下一有效时点可用；覆盖半日市与会话边界。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, time as datetime_time, timedelta, timezone
from decimal import Decimal
from enum import StrEnum
from zoneinfo import ZoneInfo

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, parseUtcTimestamp
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.instruments.Registry import TradingCalendarV1, TradingSessionRuleV1


class AggregationError(ValueError):
    """聚合窗口或查询违反已完成 Bar 约束。"""


class BarPeriod(StrEnum):
    """阶段 1 支持的聚合周期。"""

    Hour = "Hour"
    Day = "Day"


class AggregatedBarV1(StrictModel):
    """已完成周期的不可变 OHLCV 聚合。"""

    period: BarPeriod = PascalAlias("Period")
    symbol: str = PascalAlias("Symbol", min_length=1)
    market: str = PascalAlias("Market", min_length=1)
    periodStart: datetime = PascalAlias("PeriodStart")
    periodEnd: datetime = PascalAlias("PeriodEnd")
    open: Decimal = PascalAlias("Open", gt=Decimal("0"))
    high: Decimal = PascalAlias("High", gt=Decimal("0"))
    low: Decimal = PascalAlias("Low", gt=Decimal("0"))
    close: Decimal = PascalAlias("Close", gt=Decimal("0"))
    volume: Decimal = PascalAlias("Volume", ge=Decimal("0"))
    source: str = PascalAlias("Source", min_length=1)
    completedAt: datetime = PascalAlias("CompletedAt")

    @field_validator("period", mode="before")
    @classmethod
    def parsePeriod(cls, value: object) -> BarPeriod:
        if isinstance(value, BarPeriod):
            return value
        if not isinstance(value, str):
            raise AggregationError("周期必须是受控字符串")
        try:
            return BarPeriod(value)
        except ValueError as error:
            raise AggregationError(f"未知周期: {value}") from error

    @model_validator(mode="after")
    def validateWindow(self) -> "AggregatedBarV1":
        if not self.periodStart < self.periodEnd <= self.completedAt:
            raise AggregationError("必须满足 periodStart < periodEnd <= completedAt")
        if not self.low <= self.open <= self.high or not self.low <= self.close <= self.high:
            raise AggregationError("OHLC 必须满足 low <= open/close <= high")
        return self


def _periodStartOf(bar: MinuteBarSchemaV1, period: BarPeriod) -> datetime:
    """计算分钟 Bar 所属聚合周期的起点（UTC）。"""
    local = bar.barStart
    if period is BarPeriod.Hour:
        return local.replace(minute=0, second=0, microsecond=0)
    return local.replace(hour=0, minute=0, second=0, microsecond=0)


def _parseLocalTime(value: str) -> datetime_time:
    hour, minute = value.split(":")
    return datetime_time(int(hour), int(minute))


def _sessionTimesUtc(day: datetime, session: TradingSessionRuleV1, calendar: TradingCalendarV1) -> tuple[datetime, datetime]:
    """将会话开闭本地时间换算为 UTC 时刻（处理跨午夜）。"""
    zone = ZoneInfo(calendar.timeZone)
    openLocal = datetime.combine(day.date(), _parseLocalTime(session.openLocalTime), tzinfo=zone)
    closeLocal = datetime.combine(day.date(), _parseLocalTime(session.closeLocalTime), tzinfo=zone)
    if session.spansMidnight:
        closeLocal += timedelta(days=1)
    return openLocal.astimezone(timezone.utc), closeLocal.astimezone(timezone.utc)


def _dayPeriodEnd(periodStart: datetime, calendar: TradingCalendarV1 | None) -> datetime:
    """日线周期结束 = 当日全部会话收盘（UTC）；无日历时取次日零点。"""
    if calendar is None:
        return _periodEndOf(periodStart, BarPeriod.Day)
    closes = [
        _sessionTimesUtc(periodStart, session, calendar)[1] for session in calendar.sessions
    ]
    return max(closes)


def _nextValidOpen(periodStart: datetime, calendar: TradingCalendarV1 | None) -> datetime:
    """日线可查询的下一有效时点 = 次一交易日首会话开盘（UTC）。"""
    if calendar is None:
        return _periodEndOf(periodStart, BarPeriod.Day)
    nextDay = periodStart + timedelta(days=1)
    opens = [
        _sessionTimesUtc(nextDay, session, calendar)[0] for session in calendar.sessions
    ]
    return min(opens)


def _periodEndOf(periodStart: datetime, period: BarPeriod) -> datetime:
    if period is BarPeriod.Hour:
        return periodStart + timedelta(hours=1)
    return periodStart + timedelta(days=1)


@dataclass(frozen=True, slots=True)
class _Window:
    """聚合窗口的增量状态。"""

    periodStart: datetime
    periodEnd: datetime
    open: Decimal
    high: Decimal
    low: Decimal
    close: Decimal
    volume: Decimal
    count: int

    def merge(self, bar: MinuteBarSchemaV1) -> "_Window":
        return _Window(
            self.periodStart,
            self.periodEnd,
            self.open,
            max(self.high, bar.high),
            min(self.low, bar.low),
            bar.close,
            self.volume + bar.volume,
            self.count + 1,
        )

    def toBar(self, period: BarPeriod, symbol: str, market: str, source: str) -> AggregatedBarV1:
        return AggregatedBarV1.model_validate({
            "Period": period.value,
            "Symbol": symbol,
            "Market": market,
            "PeriodStart": self.periodStart,
            "PeriodEnd": self.periodEnd,
            "Open": self.open,
            "High": self.high,
            "Low": self.low,
            "Close": self.close,
            "Volume": self.volume,
            "Source": source,
            "CompletedAt": self.periodEnd,
        })


class BarAggregatorV1:
    """增量构建已完成小时/日线；只暴露已完成周期。

    append 必须按 ts 升序提交已完成分钟 Bar；跨周期推进时旧窗口自动
    归档为 completed。聚合器不校验分钟 Bar 的完成性（由上游保证）。
    """

    def __init__(self, tsPrecision: TsPrecision, calendar: TradingCalendarV1 | None = None) -> None:
        self._tsPrecision = tsPrecision
        self._calendar = calendar
        self._completed: dict[tuple[BarPeriod, str, str], list[AggregatedBarV1]] = {}
        self._active: dict[tuple[BarPeriod, str, str], _Window] = {}
        self._activeMarket: dict[tuple[BarPeriod, str, str], str] = {}
        self._lastBarEnd: datetime | None = None

    def append(self, bar: MinuteBarSchemaV1) -> None:
        """追加一条已完成分钟 Bar；违反升序时拒绝。"""
        if self._lastBarEnd is not None and bar.barStart < self._lastBarEnd:
            raise AggregationError("分钟 Bar 必须按 barStart 升序追加")
        self._lastBarEnd = bar.barEnd
        for period in (BarPeriod.Hour, BarPeriod.Day):
            key = (period, bar.symbol, bar.source)
            periodStart = _periodStartOf(bar, period)
            periodEnd = (
                _dayPeriodEnd(periodStart, self._calendar)
                if period is BarPeriod.Day
                else _periodEndOf(periodStart, period)
            )
            active = self._active.get(key)
            if active is None or active.periodStart != periodStart:
                # 归档上一个已完成的窗口
                if active is not None:
                    market = self._activeMarket.get(key, bar.market.value)
                    self._completed.setdefault(key, []).append(
                        active.toBar(period, bar.symbol, market, bar.source)
                    )
                self._active[key] = _Window(
                    periodStart,
                    periodEnd,
                    bar.open,
                    bar.high,
                    bar.low,
                    bar.close,
                    bar.volume,
                    1,
                )
                self._activeMarket[key] = bar.market.value
            else:
                self._active[key] = active.merge(bar)

    def _completedAndActive(self, period: BarPeriod, symbol: str, source: str) -> list[AggregatedBarV1]:
        key = (period, symbol, source)
        result = list(self._completed.get(key, ()))
        active = self._active.get(key)
        if active is not None:
            market = self._activeMarket.get(key, "")
            result.append(active.toBar(period, symbol, market, source))
        return result

    def query(
        self,
        period: BarPeriod,
        symbol: str,
        source: str,
        at: datetime,
        limit: int = 5,
    ) -> list[AggregatedBarV1]:
        """在 `at` 时点查询已完成的最近 n 根聚合。

        未完成周期不可查询；日线仅在会话收盘后的下一有效时点可用。
        """
        if limit < 1:
            raise AggregationError("limit 必须为正数")
        normalizedAt = parseUtcTimestamp(at, self._tsPrecision)
        key = (period, symbol, source)
        active = self._active.get(key)
        if active is None:
            return []
        if normalizedAt < active.periodEnd:
            raise AggregationError("未完成周期不可查询")
        if period is BarPeriod.Day and normalizedAt < _nextValidOpen(active.periodStart, self._calendar):
            raise AggregationError("日线仅在会话收盘后的下一有效时点可用")
        bars = self._completedAndActive(period, symbol, source)
        return bars[-limit:]


class ClosedWindowViewV1:
    """只读历史窗口：只包含已提交的已完成聚合，绝不暴露进行中数据。"""

    def __init__(self, aggregator: BarAggregatorV1) -> None:
        self._aggregator = aggregator

    def getDailyBars(self, symbol: str, source: str, at: datetime, n: int = 5) -> list[AggregatedBarV1]:
        """策略可用的日线查询入口；只读且受限窗口。"""
        return self._aggregator.query(BarPeriod.Day, symbol, source, at, limit=n)

    def getHourlyBars(self, symbol: str, source: str, at: datetime, n: int = 5) -> list[AggregatedBarV1]:
        """策略可用的小时线查询入口；只读且受限窗口。"""
        return self._aggregator.query(BarPeriod.Hour, symbol, source, at, limit=n)
