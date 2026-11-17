"""P2-017 计划日历单元测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.data.FundNav import FundCalendarV1
from veritasquant.funds.InvestmentBudget import InsufficientFundsPolicyV1, InvestmentBudgetV1
from veritasquant.funds.PlanCalendar import (
    CustomPlanCalendarV1,
    HolidayAccumulatorV1,
    HolidayPolicyV1,
    PlanCalendarError,
    PlanTriggerDeduplicatorV1,
)


def _calendar() -> FundCalendarV1:
    return FundCalendarV1.model_validate(
        {
            "CalendarVersion": "V1",
            "TradingDays": (
                "2026-08-03", "2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07",
                "2026-08-10", "2026-08-11",
            ),
        }
    )


class TestCustomPlanCalendar:
    def test_custom_skipped_dates_excluded(self) -> None:
        calendar = CustomPlanCalendarV1(_calendar(), customSkippedDates=(date(2026, 8, 5),))
        days = calendar.effectiveTradingDays(date(2026, 8, 3), date(2026, 8, 8))
        assert days == (date(2026, 8, 3), date(2026, 8, 4), date(2026, 8, 6), date(2026, 8, 7))
        assert not calendar.isTradingDay(date(2026, 8, 5))

    def test_invalid_calendar_rejected(self) -> None:
        with pytest.raises(PlanCalendarError):
            CustomPlanCalendarV1(None)  # type: ignore[arg-type]


class TestTriggerDeduplicator:
    def test_same_day_triggered_once(self) -> None:
        dedup = PlanTriggerDeduplicatorV1()
        assert dedup.tryTrigger("plan-1", date(2026, 8, 3))
        assert not dedup.tryTrigger("plan-1", date(2026, 8, 3))  # 每个有效日只触发一次
        assert dedup.tryTrigger("plan-1", date(2026, 8, 4))  # 不同日可触发
        assert dedup.tryTrigger("plan-2", date(2026, 8, 3))  # 不同计划同日可触发


class TestHolidayAccumulator:
    def test_skip_policy_drops_holidays(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"))
        accumulator = HolidayAccumulatorV1(HolidayPolicyV1.Skip, budget)
        triggers = accumulator.resolve(
            "plan-1",
            (date(2026, 8, 3), date(2026, 8, 4), date(2026, 8, 5)),
            Decimal("100"),
            Decimal("1000"),
        )
        assert [t.dueDate for t in triggers] == [
            date(2026, 8, 3), date(2026, 8, 4), date(2026, 8, 5)
        ]  # Skip：传入日期即触发（日历层已过滤节假日）

    def test_accumulate_groups_consecutive_missed_days(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"))
        accumulator = HolidayAccumulatorV1(HolidayPolicyV1.Accumulate, budget)
        # 8/4 缺失（节假日），累计到 8/5 触发
        triggers = accumulator.resolve(
            "plan-1",
            (date(2026, 8, 3), date(2026, 8, 5)),
            Decimal("100"),
            Decimal("1000"),
        )
        assert len(triggers) == 2
        assert triggers[1].dueDate == date(2026, 8, 5)
        assert triggers[1].accumulatedFrom == (date(2026, 8, 5),)

    def test_budget_clipping_caps_accumulated_amount(self) -> None:
        budget = InvestmentBudgetV1(Decimal("150"), InsufficientFundsPolicyV1.Cap)
        accumulator = HolidayAccumulatorV1(HolidayPolicyV1.Accumulate, budget)
        # 累计 2 天 100/天 = 200，但预算剩余 150 -> 裁剪到 150
        capped = accumulator.cappedAmountFor(Decimal("200"), availableCash=Decimal("500"))
        assert capped == Decimal("150")
        assert budget.remainingBudget == Decimal("0")

    def test_capped_amount_limited_by_cash(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"))
        accumulator = HolidayAccumulatorV1(HolidayPolicyV1.Accumulate, budget)
        capped = accumulator.cappedAmountFor(Decimal("300"), availableCash=Decimal("120"))
        assert capped == Decimal("120")
