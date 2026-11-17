"""P2-019 六类智能定投方案单元测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.funds.SmartPlans import (
    DrawdownMultiplierPlanV1,
    FixedAmountPlanV1,
    MaDeviationPlanV1,
    SmartPlanContextV1,
    SmartPlanError,
    TargetReturnPlanV1,
    ValuationPercentilePlanV1,
    ValueAveragingPlanV1,
)


def _context(**overrides) -> SmartPlanContextV1:  # noqa: ANN003
    base = dict(
        fundSymbol="FUND-A",
        planDate=date(2026, 8, 3),
        availableNav=Decimal("1.00"),
        availableCash=Decimal("10000"),
        navHistory=(Decimal("1.00"), Decimal("1.02"), Decimal("0.98")),
    )
    base.update(overrides)
    return SmartPlanContextV1(**base)


class TestFixedAmountPlan:
    def test_fixed_amount(self) -> None:
        plan = FixedAmountPlanV1("FUND-A", Decimal("500"))
        assert plan.decisionFor(_context()).amount == Decimal("500")
        assert len(plan.planHash) == 64

    def test_capped_by_cash(self) -> None:
        plan = FixedAmountPlanV1("FUND-A", Decimal("500"))
        assert plan.decisionFor(_context(availableCash=Decimal("100"))).amount == Decimal("100")


class TestMaDeviationPlan:
    def test_below_ma_invests_more(self) -> None:
        plan = MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=3, adjustmentFactor=Decimal("2"))
        context = _context(availableNav=Decimal("0.90"), navHistory=(Decimal("1.0"), Decimal("1.1"), Decimal("1.2")))
        # 均线 1.1，当前 0.9，偏离 -18.2% -> 加倍
        assert plan.decisionFor(context).amount > Decimal("1000")

    def test_insufficient_window_skips(self) -> None:
        plan = MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=5, adjustmentFactor=Decimal("2"))
        assert plan.decisionFor(_context(navHistory=(Decimal("1.0"),))).amount == Decimal("0")


class TestValuationPercentilePlan:
    def test_low_valuation_doubles(self) -> None:
        plan = ValuationPercentilePlanV1(
            "FUND-A", Decimal("1000"),
            percentileHistory=(Decimal("0.3"),), lowThreshold=Decimal("0.4"),
        )
        assert plan.decisionFor(_context()).amount == Decimal("2000")

    def test_normal_valuation_base_amount(self) -> None:
        plan = ValuationPercentilePlanV1(
            "FUND-A", Decimal("1000"),
            percentileHistory=(Decimal("0.6"),), lowThreshold=Decimal("0.4"),
        )
        assert plan.decisionFor(_context()).amount == Decimal("1000")


class TestDrawdownMultiplierPlan:
    def test_deep_drawdown_increases_amount(self) -> None:
        plan = DrawdownMultiplierPlanV1("FUND-A", Decimal("1000"), maxMultiplier=Decimal("3"), drawdownScale=Decimal("5"))
        decision = plan.decisionFor(_context(currentDrawdown=Decimal("0.2")))
        assert decision.amount == Decimal("2000")  # 1 + 5*0.2 = 2x
        assert decision.amount <= Decimal("3000")

    def test_no_drawdown_base_amount(self) -> None:
        plan = DrawdownMultiplierPlanV1("FUND-A", Decimal("1000"), maxMultiplier=Decimal("3"), drawdownScale=Decimal("5"))
        assert plan.decisionFor(_context(currentDrawdown=Decimal("0"))).amount == Decimal("1000")


class TestValueAveragingPlan:
    def test_gap_filled(self) -> None:
        plan = ValueAveragingPlanV1("FUND-A", Decimal("10000"), Decimal("1000"))
        decision = plan.decisionFor(
            _context(targetValuePath=Decimal("12000"), currentValue=Decimal("11000"))
        )
        assert decision.amount == Decimal("1000")

    def test_above_target_no_invest(self) -> None:
        plan = ValueAveragingPlanV1("FUND-A", Decimal("10000"), Decimal("1000"))
        decision = plan.decisionFor(
            _context(targetValuePath=Decimal("12000"), currentValue=Decimal("12500"))
        )
        assert decision.amount == Decimal("0")


class TestTargetReturnPlan:
    def test_target_reached_stops(self) -> None:
        plan = TargetReturnPlanV1("FUND-A", Decimal("1000"), Decimal("0.2"), returnRate=Decimal("0.25"))
        assert plan.decisionFor(_context()).amount == Decimal("0")

    def test_below_target_normal(self) -> None:
        plan = TargetReturnPlanV1("FUND-A", Decimal("1000"), Decimal("0.2"), returnRate=Decimal("0.05"))
        assert plan.decisionFor(_context()).amount == Decimal("1000")


class TestPlanValidation:
    def test_invalid_params_rejected(self) -> None:
        with pytest.raises(SmartPlanError):
            FixedAmountPlanV1("FUND-A", Decimal("0"))
        with pytest.raises(SmartPlanError):
            MaDeviationPlanV1("FUND-A", Decimal("1000"), maWindow=1, adjustmentFactor=Decimal("2"))
        with pytest.raises(SmartPlanError):
            TargetReturnPlanV1("FUND-A", Decimal("1000"), Decimal("1.5"))

    def test_plan_hashes_deterministic_and_distinct(self) -> None:
        fixed = FixedAmountPlanV1("FUND-A", Decimal("500"))
        ma = MaDeviationPlanV1("FUND-A", Decimal("500"), 3, Decimal("2"))
        assert fixed.planHash == FixedAmountPlanV1("FUND-A", Decimal("500")).planHash
        assert fixed.planHash != ma.planHash
