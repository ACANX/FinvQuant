"""P2-016 定投入金、预算与资金不足策略。

验收标准：
- ExternalDeposit 独立幂等记账（同 depositId 不重复；入金不计收益）；
- 资金不足策略 Reject/Cap/Skip 行为固定；
- 预算裁剪：分配额不超过预算剩余与可用现金。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from enum import StrEnum


class BudgetError(ValueError):
    """入金或预算分配不满足契约。"""


class InsufficientFundsPolicyV1(StrEnum):
    Reject = "REJECT"  # 资金不足时拒绝本次定投
    Cap = "CAP"  # 封顶到可用资金/预算剩余
    Skip = "SKIP"  # 跳过本次，保留预算


@dataclass(frozen=True, slots=True)
class ExternalDepositV1:
    """一次外部入金：独立幂等记账，入金不计收益。"""

    depositId: str
    accountId: str
    currency: str
    amount: Decimal
    ts: datetime | None = None


@dataclass(frozen=True, slots=True)
class AllocationDecisionV1:
    """一次定投分配的决策结果。"""

    requestedAmount: Decimal
    allocatedAmount: Decimal
    policy: InsufficientFundsPolicyV1
    reason: str


class DepositLedgerV1:
    """入金账本：外部入金独立记账（不计收益，幂等）。"""

    def __init__(self) -> None:
        self._deposits: dict[str, ExternalDepositV1] = {}
        self._totalByAccount: dict[str, Decimal] = {}

    def record(self, deposit: ExternalDepositV1) -> ExternalDepositV1:
        """幂等登记入金；同 depositId 返回原记录。"""
        if deposit.amount <= 0:
            raise BudgetError("入金金额必须为正")
        existing = self._deposits.get(deposit.depositId)
        if existing is not None:
            return existing
        self._deposits[deposit.depositId] = deposit
        self._totalByAccount[deposit.accountId] = (
            self._totalByAccount.get(deposit.accountId, Decimal("0")) + deposit.amount
        )
        return deposit

    def totalDeposited(self, accountId: str) -> Decimal:
        """累计入金（独立于收益计算）。"""
        return self._totalByAccount.get(accountId, Decimal("0"))

    @property
    def depositCount(self) -> int:
        return len(self._deposits)


class InvestmentBudgetV1:
    """定投预算：月度预算 + 资金不足策略。"""

    def __init__(
        self,
        monthlyBudget: Decimal,
        policy: InsufficientFundsPolicyV1 = InsufficientFundsPolicyV1.Cap,
        currency: str = "CNY",
    ) -> None:
        if monthlyBudget <= 0:
            raise BudgetError("月度预算必须为正")
        self._monthlyBudget = monthlyBudget
        self._policy = policy
        self._currency = currency
        self._allocated = Decimal("0")

    @property
    def monthlyBudget(self) -> Decimal:
        return self._monthlyBudget

    @property
    def remainingBudget(self) -> Decimal:
        return self._monthlyBudget - self._allocated

    def allocate(self, requestedAmount: Decimal, availableCash: Decimal) -> AllocationDecisionV1:
        """按预算与资金不足策略决定实际分配额。"""
        if requestedAmount <= 0:
            raise BudgetError("申请金额必须为正")
        if availableCash < 0:
            raise BudgetError("可用现金不得为负")
        remaining = self.remainingBudget
        if self._policy is InsufficientFundsPolicyV1.Reject:
            if requestedAmount > remaining or requestedAmount > availableCash:
                return AllocationDecisionV1(requestedAmount, Decimal("0"), self._policy, "资金或预算不足，拒绝")
            self._allocated += requestedAmount
            return AllocationDecisionV1(requestedAmount, requestedAmount, self._policy, "")
        if self._policy is InsufficientFundsPolicyV1.Skip:
            if requestedAmount > remaining or requestedAmount > availableCash:
                return AllocationDecisionV1(requestedAmount, Decimal("0"), self._policy, "资金或预算不足，跳过")
            self._allocated += requestedAmount
            return AllocationDecisionV1(requestedAmount, requestedAmount, self._policy, "")
        # CAP：封顶到 min(申请额, 预算剩余, 可用现金)
        capped = min(requestedAmount, remaining, availableCash)
        self._allocated += capped
        reason = "封顶到预算/现金上限" if capped < requestedAmount else ""
        return AllocationDecisionV1(requestedAmount, capped, self._policy, reason)

    def resetPeriod(self) -> None:
        """新预算周期（如月度）重置已分配额。"""
        self._allocated = Decimal("0")
