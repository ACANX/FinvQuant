"""P2-019 六类内置智能定投方案。

每类方案有固定参数 Schema、基准数据和逐期预期决定；决策为纯函数，
只用当时可用数据（禁止未来变量/未来净值）。

六类：固定金额、均线偏离、估值分位、回撤倍增、目标价值、目标收益。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import ROUND_HALF_EVEN, Decimal
from typing import Protocol

from veritasquant.core.CanonicalJson import canonicalHash


class SmartPlanError(ValueError):
    """智能定投方案参数或决策不满足契约。"""


@dataclass(frozen=True, slots=True)
class SmartPlanContextV1:
    """决策上下文：只用当时可用数据。"""

    fundSymbol: str
    planDate: date
    availableNav: Decimal
    availableCash: Decimal
    navHistory: tuple[Decimal, ...] = ()  # 截止当前日已发布净值（升序）
    currentDrawdown: Decimal | None = None  # 当前回撤（0~1）
    targetValuePath: Decimal | None = None  # 目标价值路径（价值平均）
    currentValue: Decimal | None = None  # 当前组合价值


@dataclass(frozen=True, slots=True)
class SmartPlanDecisionV1:
    """逐期决定：金额 + 固定原因。"""

    amount: Decimal
    reason: str


class SmartPlanV1(Protocol):
    """智能定投方案协议。"""

    @property
    def planHash(self) -> str: ...

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1: ...


def _usableHistory(context: SmartPlanContextV1) -> tuple[Decimal, ...]:
    """防前视：只用不晚于当前时点的净值。

    navHistory 为截止当前日已发布净值（升序）；若调用方误将未来
    净值混入序列尾部，以 availableNav（当前时点净值）最后一次
    出现位置为界截断，未来数据不得参与均线计算。
    """
    history = context.navHistory
    if not history:
        return history
    # 找到 availableNav 在历史中的最后一次出现位置（含当前净值本身）
    anchor = -1
    for index in range(len(history) - 1, -1, -1):
        if history[index] == context.availableNav:
            anchor = index
            break
    if anchor < 0:
        # 当前净值不在历史中：历史全部视为可用（已发布数据）
        return history
    return history[: anchor + 1]


@dataclass(frozen=True, slots=True)
class FixedAmountPlanV1:
    """方案 1：固定金额定投。"""

    fundSymbol: str
    amount: Decimal

    def __post_init__(self) -> None:
        if self.amount <= 0:
            raise SmartPlanError("固定金额必须为正")

    @property
    def planHash(self) -> str:
        return canonicalHash({"kind": "FIXED_AMOUNT", "fund": self.fundSymbol, "amount": str(self.amount)})

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        return SmartPlanDecisionV1(min(self.amount, context.availableCash), "固定金额")


@dataclass(frozen=True, slots=True)
class MaDeviationPlanV1:
    """方案 2：均线偏离定投（低于均线多投）。"""

    fundSymbol: str
    baseAmount: Decimal
    maWindow: int
    adjustmentFactor: Decimal  # 每偏离 1% 调整 factor%

    def __post_init__(self) -> None:
        if self.baseAmount <= 0 or self.maWindow <= 1:
            raise SmartPlanError("基准金额与均线窗口必须有效")

    @property
    def planHash(self) -> str:
        return canonicalHash(
            {
                "kind": "MA_DEVIATION",
                "fund": self.fundSymbol,
                "baseAmount": str(self.baseAmount),
                "maWindow": self.maWindow,
                "adjustmentFactor": str(self.adjustmentFactor),
            }
        )

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        history = _usableHistory(context)
        if len(history) < self.maWindow:
            return SmartPlanDecisionV1(Decimal("0"), "均线窗口数据不足，跳过")
        window = history[-self.maWindow :]
        average = sum(window, Decimal("0")) / Decimal(len(window))
        if average <= 0:
            raise SmartPlanError("均线必须为正")
        deviation = (context.availableNav - average) / average
        amount = self.baseAmount * (Decimal("1") - self.adjustmentFactor * deviation)
        amount = max(amount, Decimal("0")).quantize(Decimal("0.01"), rounding=ROUND_HALF_EVEN)
        return SmartPlanDecisionV1(min(amount, context.availableCash), "均线偏离调整")


@dataclass(frozen=True, slots=True)
class ValuationPercentilePlanV1:
    """方案 3：估值分位定投（低估多投）。"""

    fundSymbol: str
    baseAmount: Decimal
    percentileHistory: tuple[Decimal, ...]  # 历史估值分位（0~1）
    lowThreshold: Decimal  # 低于该分位视为低估

    def __post_init__(self) -> None:
        if self.baseAmount <= 0 or not self.percentileHistory:
            raise SmartPlanError("基准金额与分位历史必须有效")
        if not 0 <= self.lowThreshold <= 1:
            raise SmartPlanError("低估阈值必须在 0~1")

    @property
    def planHash(self) -> str:
        return canonicalHash(
            {
                "kind": "VALUATION_PERCENTILE",
                "fund": self.fundSymbol,
                "baseAmount": str(self.baseAmount),
                "lowThreshold": str(self.lowThreshold),
                "percentiles": [str(item) for item in self.percentileHistory],
            }
        )

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        if not context.navHistory:
            return SmartPlanDecisionV1(Decimal("0"), "无历史数据，跳过")
        currentPercentile = self.percentileHistory[-1] if self.percentileHistory else Decimal("1")
        if currentPercentile < self.lowThreshold:
            amount = self.baseAmount * Decimal("2")  # 低估加倍
            return SmartPlanDecisionV1(min(amount, context.availableCash), "估值低估加倍")
        return SmartPlanDecisionV1(min(self.baseAmount, context.availableCash), "估值正常")


@dataclass(frozen=True, slots=True)
class DrawdownMultiplierPlanV1:
    """方案 4：回撤倍增（回撤大时按倍数加投）。"""

    fundSymbol: str
    baseAmount: Decimal
    maxMultiplier: Decimal
    drawdownScale: Decimal  # 回撤 100% 对应倍数

    def __post_init__(self) -> None:
        if self.baseAmount <= 0 or self.maxMultiplier < 1:
            raise SmartPlanError("基准金额与最大倍数必须有效")

    @property
    def planHash(self) -> str:
        return canonicalHash(
            {
                "kind": "DRAWDOWN_MULTIPLIER",
                "fund": self.fundSymbol,
                "baseAmount": str(self.baseAmount),
                "maxMultiplier": str(self.maxMultiplier),
                "drawdownScale": str(self.drawdownScale),
            }
        )

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        if context.currentDrawdown is None or context.currentDrawdown <= 0:
            return SmartPlanDecisionV1(min(self.baseAmount, context.availableCash), "无回撤")
        multiplier = Decimal("1") + self.drawdownScale * context.currentDrawdown
        multiplier = min(multiplier, self.maxMultiplier)
        amount = (self.baseAmount * multiplier).quantize(Decimal("0.01"), rounding=ROUND_HALF_EVEN)
        return SmartPlanDecisionV1(min(amount, context.availableCash), "回撤倍增")


@dataclass(frozen=True, slots=True)
class ValueAveragingPlanV1:
    """方案 5：目标价值定投（按目标价值路径补差）。"""

    fundSymbol: str
    initialTarget: Decimal
    monthlyIncrement: Decimal

    def __post_init__(self) -> None:
        if self.initialTarget <= 0 or self.monthlyIncrement < 0:
            raise SmartPlanError("目标价值参数必须有效")

    @property
    def planHash(self) -> str:
        return canonicalHash(
            {
                "kind": "VALUE_AVERAGING",
                "fund": self.fundSymbol,
                "initialTarget": str(self.initialTarget),
                "monthlyIncrement": str(self.monthlyIncrement),
            }
        )

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        if context.targetValuePath is None or context.currentValue is None:
            return SmartPlanDecisionV1(Decimal("0"), "缺少目标价值路径或当前价值")
        gap = context.targetValuePath - context.currentValue
        if gap <= 0:
            return SmartPlanDecisionV1(Decimal("0"), "已超过目标价值，不投")
        return SmartPlanDecisionV1(min(gap, context.availableCash), "目标价值补差")


@dataclass(frozen=True, slots=True)
class TargetReturnPlanV1:
    """方案 6：目标收益（达到目标收益率后止盈/减投）。"""

    fundSymbol: str
    baseAmount: Decimal
    targetReturn: Decimal  # 0~1
    returnRate: Decimal | None = None  # 当前已实现收益率

    def __post_init__(self) -> None:
        if self.baseAmount <= 0 or not 0 < self.targetReturn <= 1:
            raise SmartPlanError("目标收益参数必须有效")

    @property
    def planHash(self) -> str:
        return canonicalHash(
            {
                "kind": "TARGET_RETURN",
                "fund": self.fundSymbol,
                "baseAmount": str(self.baseAmount),
                "targetReturn": str(self.targetReturn),
            }
        )

    def decisionFor(self, context: SmartPlanContextV1) -> SmartPlanDecisionV1:
        if context.currentDrawdown is not None:
            pass  # 保留：回撤参数也可用于止盈判断
        rate = self.returnRate
        if rate is None:
            return SmartPlanDecisionV1(min(self.baseAmount, context.availableCash), "收益率未知，常规定投")
        if rate >= self.targetReturn:
            return SmartPlanDecisionV1(Decimal("0"), "达到目标收益，止盈")
        return SmartPlanDecisionV1(min(self.baseAmount, context.availableCash), "未达目标收益")
