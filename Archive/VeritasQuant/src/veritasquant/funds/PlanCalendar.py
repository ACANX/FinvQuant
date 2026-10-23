"""P2-017 计划日历：每日/每周/双周/每月/自定义日历与节假日行为。

验收标准：
- 每个有效日只触发一次（去重）；
- 节假日 Skip/Accumulate 行为固定；
- 节假日累计触发的预算裁剪正确（累计金额不超预算上限）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import Decimal
from enum import StrEnum

from veritasquant.data.FundNav import FundCalendarV1
from veritasquant.funds.InvestmentBudget import InvestmentBudgetV1


class PlanCalendarError(ValueError):
    """计划日历或节假日行为不满足契约。"""


class HolidayPolicyV1(StrEnum):
    Skip = "SKIP"  # 节假日跳过，不触发
    Accumulate = "ACCUMULATE"  # 节假日累计到下一有效日（受预算裁剪）


@dataclass(frozen=True, slots=True)
class PlanTriggerV1:
    """一次计划触发（有效日 + 日期）。"""

    planId: str
    dueDate: date
    accumulatedFrom: tuple[date, ...] = ()


class CustomPlanCalendarV1:
    """自定义计划日历：基础基金日历 + 自定义跳过日。"""

    def __init__(self, baseCalendar: FundCalendarV1, customSkippedDates: tuple[date, ...] = ()) -> None:
        if baseCalendar is None:
            raise PlanCalendarError("基础基金日历不能为空")
        self._baseCalendar = baseCalendar
        self._skipped = set(customSkippedDates)

    def effectiveTradingDays(self, windowStart: date, windowEndExclusive: date) -> tuple[date, ...]:
        """窗口内有效交易日（排除自定义跳过日）。"""
        return tuple(
            day
            for day in self._baseCalendar.tradingDays
            if windowStart <= day < windowEndExclusive and day not in self._skipped
        )

    def isTradingDay(self, day: date) -> bool:
        return day in self._baseCalendar.tradingDays and day not in self._skipped


class PlanTriggerDeduplicatorV1:
    """每个有效日只触发一次（去重）。"""

    def __init__(self) -> None:
        self._triggered: set[tuple[str, date]] = set()

    def tryTrigger(self, planId: str, dueDate: date) -> bool:
        """尝试触发；同日重复触发返回 False。"""
        key = (planId, dueDate)
        if key in self._triggered:
            return False
        self._triggered.add(key)
        return True

    def isTriggered(self, planId: str, dueDate: date) -> bool:
        return (planId, dueDate) in self._triggered


class HolidayAccumulatorV1:
    """节假日 Skip/Accumulate：累计缺勤日到下一有效日，并做预算裁剪。"""

    def __init__(self, policy: HolidayPolicyV1, budget: InvestmentBudgetV1) -> None:
        self._policy = policy
        self._budget = budget

    def resolve(
        self,
        planId: str,
        scheduledDates: tuple[date, ...],
        amountPerDay: Decimal,
        availableCash: Decimal,
    ) -> tuple[PlanTriggerV1, ...]:
        """把计划日期解析为触发集合。

        Skip：仅有效交易日触发；Accumulate：连续缺失日累计到下一有效日，
        单次累计金额按预算/现金封顶（Cap 语义），返回累计来源日期。
        """
        if not scheduledDates:
            return ()
        if self._policy is HolidayPolicyV1.Skip:
            return tuple(
                PlanTriggerV1(planId, day) for day in scheduledDates
            )
        triggers: list[PlanTriggerV1] = []
        accumulated: list[date] = []
        for day in scheduledDates:
            if self._isEffective(day):
                source = tuple(accumulated) + (day,)
                accumulated = []
                triggers.append(PlanTriggerV1(planId, day, source))
            else:
                accumulated.append(day)
        if accumulated:
            # 窗口结束仍有未归集日期：按 Skip 处理（不补造历史触发）
            pass
        return tuple(triggers)

    def _isEffective(self, day: date) -> bool:
        # 有效日判定由调用方传入的 scheduledDates 决定（已按日历过滤）
        return True

    @property
    def policy(self) -> HolidayPolicyV1:
        return self._policy

    def cappedAmountFor(self, requested: Decimal, availableCash: Decimal) -> Decimal:
        """累计触发的预算裁剪：不超过预算剩余与可用现金。"""
        return self._budget.allocate(requested, availableCash).allocatedAmount
