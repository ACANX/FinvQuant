"""P2-012 基金事件 Schema 注册与计划到期生成器。

- `registerFundEvents`：把五个基金事件类型注册进 `EventSchemaRegistry`；
- `InvestmentPlanDueGeneratorV1`：使用版本化基金交易日历，将本地计划时间
  确定性转换为 UTC `InvestmentPlanDueEvent`；历史触发不依赖服务器当前时间
  （纯函数输入：日历 + 计划 + 时间范围）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime, time, timezone
from enum import StrEnum

from veritasquant.core.EventRegistry import EventSchemaRegistry, SchemaRegistration, SchemaVersion
from veritasquant.core.Models import EventPayloadV1
from veritasquant.data.FundNav import FundCalendarV1
from veritasquant.funds.FundEvents import (
    FundNavPublishedEventV1,
    FundRedemptionEventV1,
    FundShareConfirmedEventV1,
    FundSubscriptionEventV1,
    InvestmentPlanDueEventV1,
)

_FUND_EVENT_TYPES = (
    "FundNavPublishedEvent",
    "InvestmentPlanDueEvent",
    "FundSubscriptionEvent",
    "FundRedemptionEvent",
    "FundShareConfirmedEvent",
)

_FUND_PAYLOAD_MODELS: dict[str, type[EventPayloadV1]] = {
    "FundNavPublishedEvent": FundNavPublishedEventV1,
    "InvestmentPlanDueEvent": InvestmentPlanDueEventV1,
    "FundSubscriptionEvent": FundSubscriptionEventV1,
    "FundRedemptionEvent": FundRedemptionEventV1,
    "FundShareConfirmedEvent": FundShareConfirmedEventV1,
}


def registerFundEvents(registry: EventSchemaRegistry) -> None:
    """把基金事件 Schema 注册进注册表（幂等：重复注册抛 SchemaRegistryError）。"""
    for eventType, payloadModel in _FUND_PAYLOAD_MODELS.items():
        registry.register(
            SchemaRegistration(
                eventType=eventType,
                schemaVersion=SchemaVersion(1, 0),
                payloadModel=payloadModel,
                ownerModule="veritasquant.funds",
                compatibleConsumerRange=">=1.0,<2.0",
            )
        )


class PlanFrequency(StrEnum):
    Daily = "DAILY"
    Weekly = "WEEKLY"
    Biweekly = "BIWEEKLY"
    Monthly = "MONTHLY"


@dataclass(frozen=True, slots=True)
class InvestmentPlanSpecV1:
    """一份定投计划：频率 + 本地时区 + 本地触发时间。"""

    planId: str
    planVersion: str
    fundSymbol: str
    frequency: PlanFrequency
    localTimeZone: str
    localTriggerTime: time
    amountRuleVersion: str

    def __post_init__(self) -> None:
        if not self.planId or not self.fundSymbol:
            raise ValueError("计划 ID 与基金代码不能为空")
        if self.frequency is not PlanFrequency.Daily and self.localTriggerTime is None:
            raise ValueError("非日频计划必须指定本地触发时间")


class InvestmentPlanDueGeneratorV1:
    """确定性计划到期生成器：日历 + 计划 + 范围 -> UTC 到期事件。"""

    def __init__(self, calendar: FundCalendarV1) -> None:
        if calendar is None:
            raise ValueError("基金日历不能为空")
        self._calendar = calendar

    def generate(
        self,
        plan: InvestmentPlanSpecV1,
        windowStart: date,
        windowEndExclusive: date,
    ) -> tuple[InvestmentPlanDueEventV1, ...]:
        """生成窗口内计划到期事件；时间确定性转 UTC，不依赖服务器时间。"""
        if windowEndExclusive <= windowStart:
            raise ValueError("时间窗口必须为正")
        dueDates = self._selectDueDates(plan.frequency, windowStart, windowEndExclusive)
        events: list[InvestmentPlanDueEventV1] = []
        for dueDate in dueDates:
            local = datetime.combine(dueDate, plan.localTriggerTime)
            scheduledUtc = local.replace(tzinfo=timezone.utc)
            events.append(
                InvestmentPlanDueEventV1.model_validate(
                    {
                        "PlanId": plan.planId,
                        "PlanVersion": plan.planVersion,
                        "FundSymbol": plan.fundSymbol,
                        "DueDate": dueDate,
                        "ScheduledUtcTs": scheduledUtc,
                        "AmountRuleVersion": plan.amountRuleVersion,
                    }
                )
            )
        return tuple(events)

    def _selectDueDates(
        self,
        frequency: PlanFrequency,
        windowStart: date,
        windowEndExclusive: date,
    ) -> tuple[date, ...]:
        tradingDays = [
            day for day in self._calendar.tradingDays if windowStart <= day < windowEndExclusive
        ]
        if frequency is PlanFrequency.Daily:
            return tuple(tradingDays)
        if frequency is PlanFrequency.Monthly:
            return tuple(self._firstTradingDayOfMonth(tradingDays))
        if frequency is PlanFrequency.Weekly:
            return tuple(self._firstTradingDayOfWeek(tradingDays))
        if frequency is PlanFrequency.Biweekly:
            weekly = self._firstTradingDayOfWeek(tradingDays)
            return tuple(weekly[index] for index in range(0, len(weekly), 2))
        return ()

    @staticmethod
    def _firstTradingDayOfMonth(days: list[date]) -> list[date]:
        selected: list[date] = []
        seenMonths: set[tuple[int, int]] = set()
        for day in days:
            key = (day.year, day.month)
            if key not in seenMonths:
                seenMonths.add(key)
                selected.append(day)
        return selected

    @staticmethod
    def _firstTradingDayOfWeek(days: list[date]) -> list[date]:
        selected: list[date] = []
        seenWeeks: set[tuple[int, int]] = set()
        for day in days:
            iso = day.isocalendar()
            key = (iso.year, iso.week)
            if key not in seenWeeks:
                seenWeeks.add(key)
                selected.append(day)
        return selected
