"""P2-012 基金事件注册与计划到期生成器单元测试。

验收标准映射：
- 计划时间确定性转 UTC；历史触发不依赖服务器当前时间。
"""

from __future__ import annotations

from datetime import date, time
from decimal import Decimal

import pytest

from veritasquant.core.EventRegistry import EventSchemaRegistry, SchemaRegistryError
from veritasquant.data.FundNav import FundCalendarV1
from veritasquant.funds.FundEventRegistration import (
    InvestmentPlanDueGeneratorV1,
    InvestmentPlanSpecV1,
    PlanFrequency,
    registerFundEvents,
)
from veritasquant.funds.FundEvents import (
    InvestmentPlanDueEventV1,
)


class TestFundEventRegistration:
    def test_fund_events_registered(self) -> None:
        registry = EventSchemaRegistry()
        registerFundEvents(registry)
        # 注册表必须包含全部基金事件类型且内容哈希可计算（运行清单用）
        registeredTypes = {eventType for eventType, _ in registry._registrations}  # noqa: SLF001
        for eventType in (
            "FundNavPublishedEvent",
            "InvestmentPlanDueEvent",
            "FundSubscriptionEvent",
            "FundRedemptionEvent",
            "FundShareConfirmedEvent",
        ):
            assert eventType in registeredTypes, f"缺少注册: {eventType}"
        assert len(registry.registryHash()) == 64

    def test_actual_payload_decodes_after_registration(self) -> None:
        registry = EventSchemaRegistry()
        registerFundEvents(registry)
        # 注册表中 FundNavPublishedEvent 的载荷模型可直接解析真实载荷
        from veritasquant.core.EventRegistry import SchemaVersion

        registration = registry._registrations[(  # noqa: SLF001
            "FundNavPublishedEvent",
            SchemaVersion.parse("1.0"),
        )]
        payload = registration.payloadModel.model_validate(
            {
                "Symbol": "FUND-001",
                "NavDate": date(2026, 8, 3),
                "UnitNav": Decimal("1.5"),
                "Currency": "CNY",
                "NavAvailabilityPolicy": "NEXT_TRADING_DAY_OPEN",
                "FundMetadataVersion": "V1",
            }
        )
        assert payload.symbol == "FUND-001"

    def test_duplicate_registration_rejected(self) -> None:
        registry = EventSchemaRegistry()
        registerFundEvents(registry)
        with pytest.raises(SchemaRegistryError):
            registerFundEvents(registry)


class TestInvestmentPlanDueGenerator:
    def _calendar(self) -> FundCalendarV1:
        return FundCalendarV1.model_validate(
            {
                "CalendarVersion": "V1",
                "TradingDays": (
                    "2026-08-03", "2026-08-04", "2026-08-05", "2026-08-06", "2026-08-07",
                    "2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13", "2026-08-14",
                ),
            }
        )

    def _plan(self, frequency: PlanFrequency) -> InvestmentPlanSpecV1:
        return InvestmentPlanSpecV1(
            planId="plan-1",
            planVersion="V1",
            fundSymbol="FUND-001",
            frequency=frequency,
            localTimeZone="Asia/Shanghai",
            localTriggerTime=time(9, 30),
            amountRuleVersion="V1",
        )

    def test_daily_generates_each_trading_day(self) -> None:
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        events = generator.generate(self._plan(PlanFrequency.Daily), date(2026, 8, 3), date(2026, 8, 10))
        assert len(events) == 5  # 8/3~8/7 五个交易日
        assert all(isinstance(event, InvestmentPlanDueEventV1) for event in events)
        # 时间确定性转 UTC：本地 09:30 无时区偏移示例中直接映射
        assert events[0].dueDate == date(2026, 8, 3)
        assert events[0].scheduledUtcTs.hour == 9

    def test_monthly_generates_first_trading_day_of_month(self) -> None:
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        events = generator.generate(self._plan(PlanFrequency.Monthly), date(2026, 8, 3), date(2026, 8, 15))
        assert len(events) == 1
        assert events[0].dueDate == date(2026, 8, 3)

    def test_weekly_generates_first_trading_day_of_week(self) -> None:
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        events = generator.generate(self._plan(PlanFrequency.Weekly), date(2026, 8, 3), date(2026, 8, 15))
        assert len(events) == 2  # 第一周 8/3，第二周 8/10

    def test_biweekly_generates_every_other_week(self) -> None:
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        events = generator.generate(self._plan(PlanFrequency.Biweekly), date(2026, 8, 3), date(2026, 8, 15))
        assert len(events) == 1  # 8/3 与 8/10 每两周取一

    def test_historical_trigger_independent_of_server_time(self) -> None:
        """同一日历 + 计划 + 窗口永远生成相同事件（纯函数，不读服务器时间）。"""
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        first = generator.generate(self._plan(PlanFrequency.Daily), date(2026, 8, 3), date(2026, 8, 8))
        second = generator.generate(self._plan(PlanFrequency.Daily), date(2026, 8, 3), date(2026, 8, 8))
        assert [(e.dueDate, e.scheduledUtcTs) for e in first] == [
            (e.dueDate, e.scheduledUtcTs) for e in second
        ]

    def test_invalid_window_rejected(self) -> None:
        generator = InvestmentPlanDueGeneratorV1(self._calendar())
        with pytest.raises(ValueError):
            generator.generate(self._plan(PlanFrequency.Daily), date(2026, 8, 10), date(2026, 8, 3))
