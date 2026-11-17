"""P2-018 Fixed/RuleBased/ExplicitSeries 三种日频金额模式。

验收标准：
- 缺日策略（Skip/UsePrevious/Zero）行为固定；
- 不同基金额度独立；来源哈希确定性（相同配置 -> 相同哈希）；
- 预算边界：单日金额不超过预算上限。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import Decimal
from enum import StrEnum
from typing import Protocol

from veritasquant.core.CanonicalJson import canonicalHash


class AmountRuleError(ValueError):
    """金额规则配置或求值不满足契约。"""


class MissingDayPolicyV1(StrEnum):
    Skip = "SKIP"  # 缺失日跳过（不触发）
    UsePrevious = "USE_PREVIOUS"  # 缺失日沿用上一有效日金额
    Zero = "ZERO"  # 缺失日金额为 0


@dataclass(frozen=True, slots=True)
class AmountContextV1:
    """金额规则求值上下文（只用当时可用数据）。"""

    fundSymbol: str
    planDate: date
    availableNav: Decimal | None = None
    previousAmount: Decimal | None = None


class AmountRuleV1(Protocol):
    """日频金额规则协议。"""

    @property
    def ruleHash(self) -> str: ...

    def amountFor(self, context: AmountContextV1) -> Decimal: ...

    @property
    def missingDayPolicy(self) -> MissingDayPolicyV1: ...


@dataclass(frozen=True, slots=True)
class FixedAmountRuleV1:
    """固定金额：每基金独立额度。"""

    fundSymbol: str
    amount: Decimal
    missingDayPolicy: MissingDayPolicyV1 = MissingDayPolicyV1.Skip
    maxAmount: Decimal | None = None  # 预算边界

    def __post_init__(self) -> None:
        if self.amount <= 0:
            raise AmountRuleError("固定金额必须为正")
        if self.maxAmount is not None and self.maxAmount <= 0:
            raise AmountRuleError("预算上限必须为正")

    @property
    def ruleHash(self) -> str:
        return canonicalHash(
            {
                "kind": "FIXED",
                "fund": self.fundSymbol,
                "amount": str(self.amount),
                "missingDayPolicy": self.missingDayPolicy.value,
            }
        )

    def amountFor(self, context: AmountContextV1) -> Decimal:
        if context.fundSymbol != self.fundSymbol:
            raise AmountRuleError("规则基金与上下文基金不一致")
        if self.missingDayPolicy is MissingDayPolicyV1.Skip and context.availableNav is None:
            # 无当日可用数据且策略为 Skip 时视为缺日
            return Decimal("0")
        amount = self.amount
        if self.maxAmount is not None:
            amount = min(amount, self.maxAmount)  # 预算边界裁剪
        return amount


@dataclass(frozen=True, slots=True)
class RuleBasedAmountRuleV1:
    """规则金额：按可用净值偏离基准调整（只用当时数据）。"""

    fundSymbol: str
    baseAmount: Decimal
    navAdjustmentFactor: Decimal  # 每偏离 1% 的调整比例
    referenceNav: Decimal
    missingDayPolicy: MissingDayPolicyV1 = MissingDayPolicyV1.Skip
    minAmount: Decimal = Decimal("0")
    maxAmount: Decimal | None = None

    def __post_init__(self) -> None:
        if self.baseAmount <= 0:
            raise AmountRuleError("基准金额必须为正")
        if self.referenceNav <= 0:
            raise AmountRuleError("参考净值必须为正")

    @property
    def ruleHash(self) -> str:
        return canonicalHash(
            {
                "kind": "RULE_BASED",
                "fund": self.fundSymbol,
                "baseAmount": str(self.baseAmount),
                "navAdjustmentFactor": str(self.navAdjustmentFactor),
                "referenceNav": str(self.referenceNav),
            }
        )

    def amountFor(self, context: AmountContextV1) -> Decimal:
        if context.fundSymbol != self.fundSymbol:
            raise AmountRuleError("规则基金与上下文基金不一致")
        if context.availableNav is None:
            if self.missingDayPolicy is MissingDayPolicyV1.Skip:
                return Decimal("0")
            if self.missingDayPolicy is MissingDayPolicyV1.Zero:
                return Decimal("0")
            if self.missingDayPolicy is MissingDayPolicyV1.UsePrevious:
                return context.previousAmount or self.baseAmount
        deviation = (context.availableNav - self.referenceNav) / self.referenceNav
        # 低于参考净值（低估）多投，高于参考（高估）少投
        amount = self.baseAmount * (Decimal("1") - self.navAdjustmentFactor * deviation)
        amount = max(amount, self.minAmount)
        if self.maxAmount is not None:
            amount = min(amount, self.maxAmount)
        return amount.quantize(Decimal("0.01"))


@dataclass(frozen=True, slots=True)
class ExplicitSeriesAmountRuleV1:
    """显式序列：日期 -> 金额（不同基金独立额度）。"""

    fundSymbol: str
    series: tuple[tuple[date, Decimal], ...]
    missingDayPolicy: MissingDayPolicyV1 = MissingDayPolicyV1.Skip

    def __post_init__(self) -> None:
        if not self.series:
            raise AmountRuleError("显式序列不能为空")
        dates = [day for day, _ in self.series]
        if len(dates) != len(set(dates)):
            raise AmountRuleError("显式序列日期不得重复")
        if any(amount <= 0 for _, amount in self.series):
            raise AmountRuleError("序列金额必须为正")

    @property
    def ruleHash(self) -> str:
        return canonicalHash(
            {
                "kind": "EXPLICIT_SERIES",
                "fund": self.fundSymbol,
                "series": [
                    {"date": day.isoformat(), "amount": str(amount)}
                    for day, amount in self.series
                ],
            }
        )

    def amountFor(self, context: AmountContextV1) -> Decimal:
        if context.fundSymbol != self.fundSymbol:
            raise AmountRuleError("规则基金与上下文基金不一致")
        for day, amount in self.series:
            if day == context.planDate:
                return amount
        if self.missingDayPolicy is MissingDayPolicyV1.UsePrevious:
            previous = [amount for day, amount in self.series if day < context.planDate]
            if previous:
                return previous[-1]
        return Decimal("0")  # Skip/Zero 缺日
