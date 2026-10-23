"""P2-016 入金与预算单元测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.funds.InvestmentBudget import (
    BudgetError,
    DepositLedgerV1,
    ExternalDepositV1,
    InsufficientFundsPolicyV1,
    InvestmentBudgetV1,
)


class TestDepositLedger:
    def test_deposit_idempotent_and_independent(self) -> None:
        ledger = DepositLedgerV1()
        deposit = ExternalDepositV1("d-1", "a1", "CNY", Decimal("10000"))
        ledger.record(deposit)
        ledger.record(deposit)  # 幂等
        assert ledger.depositCount == 1
        assert ledger.totalDeposited("a1") == Decimal("10000")

    def test_deposits_excluded_from_returns(self) -> None:
        """入金独立记账：总额单独统计，不参与收益计算。"""
        ledger = DepositLedgerV1()
        ledger.record(ExternalDepositV1("d-1", "a1", "CNY", Decimal("10000")))
        ledger.record(ExternalDepositV1("d-2", "a1", "CNY", Decimal("5000")))
        assert ledger.totalDeposited("a1") == Decimal("15000")

    def test_invalid_deposit_rejected(self) -> None:
        ledger = DepositLedgerV1()
        with pytest.raises(BudgetError):
            ledger.record(ExternalDepositV1("d-0", "a1", "CNY", Decimal("0")))


class TestInvestmentBudget:
    def test_reject_policy_blocks_insufficient(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"), InsufficientFundsPolicyV1.Reject)
        decision = budget.allocate(Decimal("1000"), availableCash=Decimal("100"))
        assert decision.allocatedAmount == Decimal("0")
        assert decision.policy is InsufficientFundsPolicyV1.Reject
        assert budget.remainingBudget == Decimal("1000")  # 拒绝不消耗预算

    def test_cap_policy_caps_to_available(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"), InsufficientFundsPolicyV1.Cap)
        decision = budget.allocate(Decimal("1000"), availableCash=Decimal("300"))
        assert decision.allocatedAmount == Decimal("300")
        assert budget.remainingBudget == Decimal("700")

    def test_cap_policy_caps_to_budget_remaining(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"), InsufficientFundsPolicyV1.Cap)
        budget.allocate(Decimal("800"), availableCash=Decimal("900"))
        decision = budget.allocate(Decimal("500"), availableCash=Decimal("900"))
        assert decision.allocatedAmount == Decimal("200")  # 预算剩余 200

    def test_skip_policy_skips_without_consuming(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"), InsufficientFundsPolicyV1.Skip)
        decision = budget.allocate(Decimal("1000"), availableCash=Decimal("50"))
        assert decision.allocatedAmount == Decimal("0")
        assert budget.remainingBudget == Decimal("1000")  # 跳过保留预算

    def test_allocate_consumes_budget(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"), InsufficientFundsPolicyV1.Cap)
        budget.allocate(Decimal("400"), availableCash=Decimal("1000"))
        assert budget.remainingBudget == Decimal("600")

    def test_reset_period(self) -> None:
        budget = InvestmentBudgetV1(Decimal("1000"))
        budget.allocate(Decimal("700"), availableCash=Decimal("1000"))
        budget.resetPeriod()
        assert budget.remainingBudget == Decimal("1000")

    def test_invalid_inputs_rejected(self) -> None:
        with pytest.raises(BudgetError):
            InvestmentBudgetV1(Decimal("0"))
        budget = InvestmentBudgetV1(Decimal("1000"))
        with pytest.raises(BudgetError):
            budget.allocate(Decimal("0"), availableCash=Decimal("100"))
        with pytest.raises(BudgetError):
            budget.allocate(Decimal("100"), availableCash=Decimal("-1"))
