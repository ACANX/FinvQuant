"""P2-021 多基金分配、权重竞争与组合预算。

验收标准：
- 同日分配总额不超预算/现金/风险上限（单基金风险上限按权重约束）；
- 分配顺序确定（权重降序，权重相同按基金代码升序）；
- 多账户隔离（每个账户独立分配，互不串扰）。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import ROUND_HALF_EVEN, Decimal
from enum import StrEnum


class AllocationError(ValueError):
    """多基金分配不满足预算、权重或风险上限契约。"""


class AllocationResult(StrEnum):
    FullyAllocated = "FULLY_ALLOCATED"  # 全部按权重分配
    BudgetCapped = "BUDGET_CAPPED"  # 总预算不足，按可用资金裁剪
    RiskCapped = "RISK_CAPPED"  # 单基金风险上限约束


@dataclass(frozen=True, slots=True)
class FundWeightV1:
    """一只基金的权重（比例 0~1）。"""

    fundSymbol: str
    weight: Decimal

    def __post_init__(self) -> None:
        if not self.fundSymbol:
            raise AllocationError("基金代码不能为空")
        if not 0 <= self.weight <= 1:
            raise AllocationError("权重必须在 0~1")


@dataclass(frozen=True, slots=True)
class FundAllocationV1:
    """单只基金的分配结果。"""

    fundSymbol: str
    weight: Decimal
    allocatedAmount: Decimal
    reason: str = ""


@dataclass(frozen=True, slots=True)
class PortfolioAllocationPlanV1:
    """一次组合分配计划：总额不超预算/现金/风险上限。"""

    accountId: str
    allocations: tuple[FundAllocationV1, ...]
    totalAllocated: Decimal
    result: AllocationResult

    def amountFor(self, fundSymbol: str) -> Decimal:
        for allocation in self.allocations:
            if allocation.fundSymbol == fundSymbol:
                return allocation.allocatedAmount
        return Decimal("0")


class PortfolioAllocatorV1:
    """确定性多基金分配器：权重降序、总额受预算/现金/风险上限约束。"""

    def __init__(self, maxSingleFundRatio: Decimal = Decimal("0.5")) -> None:
        """maxSingleFundRatio：单基金风险上限（占总预算比例）。"""
        if not 0 < maxSingleFundRatio <= 1:
            raise AllocationError("单基金风险上限必须在 0~1")
        self._maxSingleFundRatio = maxSingleFundRatio

    def allocate(
        self,
        accountId: str,
        weights: tuple[FundWeightV1, ...],
        totalBudget: Decimal,
        availableCash: Decimal,
    ) -> PortfolioAllocationPlanV1:
        """按权重分配；总额不超过 min(总预算, 可用现金, 各基金风险上限)。"""
        if not weights:
            raise AllocationError("权重列表不能为空")
        if totalBudget <= 0:
            raise AllocationError("总预算必须为正")
        if availableCash < 0:
            raise AllocationError("可用现金不得为负")
        weightSum = sum((weight.weight for weight in weights), Decimal("0"))
        if weightSum <= 0:
            raise AllocationError("权重之和必须为正")
        effectiveBudget = min(totalBudget, availableCash)
        # 现金不足（低于总预算）视为预算裁剪
        result = AllocationResult.BudgetCapped if availableCash < totalBudget else AllocationResult.FullyAllocated
        # 确定性顺序：权重降序，同权重按基金代码升序
        ordered = sorted(
            weights,
            key=lambda item: (-item.weight, item.fundSymbol),
        )
        allocations: list[FundAllocationV1] = []
        remaining = effectiveBudget
        for index, weightItem in enumerate(ordered):
            if index == len(ordered) - 1:
                rawAmount = remaining  # 末位吸收舍入余量
            else:
                rawAmount = (effectiveBudget * weightItem.weight / weightSum).quantize(
                    Decimal("0.01"), rounding=ROUND_HALF_EVEN
                )
            riskCap = (effectiveBudget * self._maxSingleFundRatio).quantize(
                Decimal("0.01"), rounding=ROUND_HALF_EVEN
            )
            amount = min(rawAmount, remaining, riskCap)
            if amount < rawAmount:
                result = AllocationResult.RiskCapped if amount == riskCap else AllocationResult.BudgetCapped
            remaining -= amount
            allocations.append(
                FundAllocationV1(weightItem.fundSymbol, weightItem.weight, amount)
            )
        total = effectiveBudget - remaining
        if total < effectiveBudget and result is AllocationResult.FullyAllocated:
            result = AllocationResult.BudgetCapped
        return PortfolioAllocationPlanV1(accountId, tuple(allocations), total, result)


class AccountIsolatedAllocatorV1:
    """多账户隔离分配：每个账户独立分配，互不串扰。"""

    def __init__(self, allocator: PortfolioAllocatorV1) -> None:
        self._allocator = allocator

    def allocateAccounts(
        self,
        accounts: dict[str, tuple[FundWeightV1, ...]],
        budgetPerAccount: dict[str, Decimal],
        cashPerAccount: dict[str, Decimal],
    ) -> dict[str, PortfolioAllocationPlanV1]:
        """按账户独立分配；缺失预算/现金的账户被拒绝。"""
        plans: dict[str, PortfolioAllocationPlanV1] = {}
        for accountId, weights in accounts.items():
            if accountId not in budgetPerAccount or accountId not in cashPerAccount:
                raise AllocationError(f"账户 {accountId} 缺少预算或现金")
            plans[accountId] = self._allocator.allocate(
                accountId, weights, budgetPerAccount[accountId], cashPerAccount[accountId]
            )
        return plans
