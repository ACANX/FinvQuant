"""P2-022 基金业绩报告单元测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.funds.FundPerformance import (
    CashFlowV1,
    PerformanceError,
    PrincipalReporterV1,
    RuleContributionReporterV1,
    TwrCalculatorV1,
    XirrCalculatorV1,
)
from veritasquant.funds.InvestmentBudget import DepositLedgerV1, ExternalDepositV1


class TestTwr:
    def test_no_cashflow_simple_return(self) -> None:
        result = TwrCalculatorV1().calculate(Decimal("100"), Decimal("110"), ())
        assert result.twr == Decimal("0.1")

    def test_with_cashflow_base_adjusted(self) -> None:
        """现金流（投入）计入基数：100 期初 + 50 投入 -> 期末 165 => 10%"""
        flows = (CashFlowV1(date(2026, 8, 1), Decimal("-50")),)
        result = TwrCalculatorV1().calculate(Decimal("100"), Decimal("165"), flows)
        assert result.twr == Decimal("0.1")

    def test_invalid_inputs_rejected(self) -> None:
        with pytest.raises(PerformanceError):
            TwrCalculatorV1().calculate(Decimal("-1"), Decimal("100"), ())


class TestXirr:
    def test_simple_loan_irr(self) -> None:
        """投入 1000，一年后回收 1100 -> 约 10%"""
        flows = (
            CashFlowV1(date(2026, 1, 1), Decimal("-1000")),
            CashFlowV1(date(2027, 1, 1), Decimal("1100")),
        )
        rate = XirrCalculatorV1().calculate(flows)
        assert abs(rate - Decimal("0.1")) < Decimal("0.01")

    def test_requires_both_signs(self) -> None:
        with pytest.raises(PerformanceError):
            XirrCalculatorV1().calculate(
                (CashFlowV1(date(2026, 1, 1), Decimal("100")), CashFlowV1(date(2027, 1, 1), Decimal("200")))
            )

    def test_requires_two_flows(self) -> None:
        with pytest.raises(PerformanceError):
            XirrCalculatorV1().calculate((CashFlowV1(date(2026, 1, 1), Decimal("-100")),))


class TestPrincipalAndContribution:
    def test_principal_excludes_returns(self) -> None:
        ledger = DepositLedgerV1()
        ledger.record(ExternalDepositV1("d-1", "a1", "CNY", Decimal("10000")))
        reporter = PrincipalReporterV1(ledger)
        assert reporter.investedPrincipal("a1") == Decimal("10000")

    def test_rule_contribution_report(self) -> None:
        reporter = RuleContributionReporterV1()
        reporter.record("固定金额", Decimal("1000"), Decimal("800"))
        reporter.record("均线偏离", Decimal("500"), Decimal("400"))
        assert reporter.totalInvested() == Decimal("1500")
        assert len(reporter.report()) == 2

    def test_invalid_contribution_rejected(self) -> None:
        reporter = RuleContributionReporterV1()
        with pytest.raises(PerformanceError):
            reporter.record("规则", Decimal("-1"), Decimal("0"))
