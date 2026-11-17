"""P2-018 金额规则单元测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.funds.AmountRules import (
    AmountContextV1,
    AmountRuleError,
    ExplicitSeriesAmountRuleV1,
    FixedAmountRuleV1,
    MissingDayPolicyV1,
    RuleBasedAmountRuleV1,
)


class TestFixedAmountRule:
    def test_fixed_amount_per_fund(self) -> None:
        rule = FixedAmountRuleV1("FUND-A", Decimal("500"))
        context = AmountContextV1("FUND-A", date(2026, 8, 3), availableNav=Decimal("1.2"))
        assert rule.amountFor(context) == Decimal("500")

    def test_budget_boundary_caps_amount(self) -> None:
        rule = FixedAmountRuleV1("FUND-A", Decimal("500"), maxAmount=Decimal("300"))
        context = AmountContextV1("FUND-A", date(2026, 8, 3), availableNav=Decimal("1.2"))
        assert rule.amountFor(context) == Decimal("300")

    def test_wrong_fund_rejected(self) -> None:
        rule = FixedAmountRuleV1("FUND-A", Decimal("500"))
        with pytest.raises(AmountRuleError):
            rule.amountFor(AmountContextV1("FUND-B", date(2026, 8, 3), availableNav=Decimal("1")))

    def test_invalid_amount_rejected(self) -> None:
        with pytest.raises(AmountRuleError):
            FixedAmountRuleV1("FUND-A", Decimal("0"))


class TestRuleBasedAmountRule:
    def test_nav_deviation_adjusts_amount(self) -> None:
        rule = RuleBasedAmountRuleV1(
            fundSymbol="FUND-A",
            baseAmount=Decimal("1000"),
            navAdjustmentFactor=Decimal("0.5"),  # 每偏离 1% 调整 0.5%
            referenceNav=Decimal("1.00"),
        )
        below = AmountContextV1("FUND-A", date(2026, 8, 3), availableNav=Decimal("0.95"))
        above = AmountContextV1("FUND-A", date(2026, 8, 4), availableNav=Decimal("1.05"))
        assert rule.amountFor(below) > Decimal("1000")  # 低于参考净值多投
        assert rule.amountFor(above) < Decimal("1000")  # 高于参考净值少投

    def test_missing_day_use_previous(self) -> None:
        rule = RuleBasedAmountRuleV1(
            fundSymbol="FUND-A", baseAmount=Decimal("1000"),
            navAdjustmentFactor=Decimal("0"), referenceNav=Decimal("1.00"),
            missingDayPolicy=MissingDayPolicyV1.UsePrevious,
        )
        context = AmountContextV1("FUND-A", date(2026, 8, 3), availableNav=None, previousAmount=Decimal("800"))
        assert rule.amountFor(context) == Decimal("800")

    def test_missing_day_skip_returns_zero(self) -> None:
        rule = RuleBasedAmountRuleV1(
            fundSymbol="FUND-A", baseAmount=Decimal("1000"),
            navAdjustmentFactor=Decimal("0"), referenceNav=Decimal("1.00"),
        )
        assert rule.amountFor(AmountContextV1("FUND-A", date(2026, 8, 3), availableNav=None)) == Decimal("0")


class TestExplicitSeriesRule:
    def test_series_lookup(self) -> None:
        rule = ExplicitSeriesAmountRuleV1(
            "FUND-A",
            ((date(2026, 8, 3), Decimal("300")), (date(2026, 8, 4), Decimal("500"))),
        )
        context = AmountContextV1("FUND-A", date(2026, 8, 4), availableNav=Decimal("1"))
        assert rule.amountFor(context) == Decimal("500")

    def test_missing_day_use_previous(self) -> None:
        rule = ExplicitSeriesAmountRuleV1(
            "FUND-A",
            ((date(2026, 8, 3), Decimal("300")),),
            missingDayPolicy=MissingDayPolicyV1.UsePrevious,
        )
        context = AmountContextV1("FUND-A", date(2026, 8, 5), availableNav=Decimal("1"))
        assert rule.amountFor(context) == Decimal("300")

    def test_duplicate_dates_rejected(self) -> None:
        with pytest.raises(AmountRuleError):
            ExplicitSeriesAmountRuleV1(
                "FUND-A",
                ((date(2026, 8, 3), Decimal("300")), (date(2026, 8, 3), Decimal("500"))),
            )


class TestRuleHash:
    def test_same_config_same_hash(self) -> None:
        first = FixedAmountRuleV1("FUND-A", Decimal("500"))
        second = FixedAmountRuleV1("FUND-A", Decimal("500"))
        assert first.ruleHash == second.ruleHash == FixedAmountRuleV1("FUND-A", Decimal("500")).ruleHash

    def test_different_config_different_hash(self) -> None:
        assert FixedAmountRuleV1("FUND-A", Decimal("500")).ruleHash != FixedAmountRuleV1("FUND-A", Decimal("600")).ruleHash
        assert FixedAmountRuleV1("FUND-A", Decimal("500")).ruleHash != FixedAmountRuleV1("FUND-B", Decimal("500")).ruleHash

    def test_explicit_series_hash_deterministic(self) -> None:
        rule = ExplicitSeriesAmountRuleV1(
            "FUND-A", ((date(2026, 8, 3), Decimal("300")),)
        )
        assert len(rule.ruleHash) == 64
