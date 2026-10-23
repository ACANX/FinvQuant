"""P2-021 多基金分配单元测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.funds.PortfolioAllocation import (
    AccountIsolatedAllocatorV1,
    AllocationError,
    AllocationResult,
    FundWeightV1,
    PortfolioAllocatorV1,
)


def _weights() -> tuple[FundWeightV1, ...]:
    return (
        FundWeightV1("FUND-A", Decimal("0.5")),
        FundWeightV1("FUND-B", Decimal("0.3")),
        FundWeightV1("FUND-C", Decimal("0.2")),
    )


class TestPortfolioAllocator:
    def test_total_never_exceeds_budget_or_cash(self) -> None:
        allocator = PortfolioAllocatorV1()
        plan = allocator.allocate("a1", _weights(), Decimal("1000"), Decimal("1000"))
        assert plan.totalAllocated == Decimal("1000")  # 预算 1000
        assert sum(item.allocatedAmount for item in plan.allocations) == Decimal("1000")
        assert plan.result is AllocationResult.FullyAllocated

    def test_limited_cash_caps_total(self) -> None:
        allocator = PortfolioAllocatorV1()
        plan = allocator.allocate("a1", _weights(), Decimal("1000"), Decimal("300"))
        assert plan.totalAllocated == Decimal("300")  # 现金上限
        assert plan.result is AllocationResult.BudgetCapped

    def test_single_fund_risk_cap_enforced(self) -> None:
        allocator = PortfolioAllocatorV1(maxSingleFundRatio=Decimal("0.4"))
        plan = allocator.allocate("a1", _weights(), Decimal("1000"), Decimal("1000"))
        # FUND-A 权重 50% 但风险上限 40% -> 400
        assert plan.amountFor("FUND-A") == Decimal("400")
        assert plan.totalAllocated <= Decimal("1000")

    def test_deterministic_order_weight_desc(self) -> None:
        allocator = PortfolioAllocatorV1()
        plan = allocator.allocate("a1", _weights(), Decimal("1000"), Decimal("1000"))
        symbols = [item.fundSymbol for item in plan.allocations]
        assert symbols == ["FUND-A", "FUND-B", "FUND-C"]  # 权重降序

    def test_tie_broken_by_symbol(self) -> None:
        allocator = PortfolioAllocatorV1()
        weights = (FundWeightV1("FUND-B", Decimal("0.5")), FundWeightV1("FUND-A", Decimal("0.5")))
        plan = allocator.allocate("a1", weights, Decimal("100"), Decimal("100"))
        assert [item.fundSymbol for item in plan.allocations] == ["FUND-A", "FUND-B"]

    def test_invalid_inputs_rejected(self) -> None:
        allocator = PortfolioAllocatorV1()
        with pytest.raises(AllocationError):
            allocator.allocate("a1", (), Decimal("100"), Decimal("100"))
        with pytest.raises(AllocationError):
            allocator.allocate("a1", _weights(), Decimal("0"), Decimal("100"))
        with pytest.raises(AllocationError):
            PortfolioAllocatorV1(maxSingleFundRatio=Decimal("1.5"))


class TestAccountIsolation:
    def test_accounts_allocated_independently(self) -> None:
        allocator = AccountIsolatedAllocatorV1(PortfolioAllocatorV1())
        plans = allocator.allocateAccounts(
            {"a1": _weights(), "a2": (FundWeightV1("FUND-X", Decimal("1")),)},
            {"a1": Decimal("1000"), "a2": Decimal("500")},
            {"a1": Decimal("1000"), "a2": Decimal("200")},
        )
        assert plans["a1"].totalAllocated == Decimal("1000")
        assert plans["a2"].totalAllocated == Decimal("100")  # a2 现金 200，单基金风险上限 50% -> 100
        assert plans["a1"].accountId == "a1"
        assert plans["a2"].accountId == "a2"

    def test_missing_account_rejected(self) -> None:
        allocator = AccountIsolatedAllocatorV1(PortfolioAllocatorV1())
        with pytest.raises(AllocationError):
            allocator.allocateAccounts(
                {"a1": _weights(), "a2": (FundWeightV1("FUND-X", Decimal("1")),)},
                {"a1": Decimal("1000")},  # a2 无预算
                {"a1": Decimal("1000"), "a2": Decimal("200")},
            )
