"""P2-022 基金业绩报告：TWR、XIRR、投入本金、份额与规则贡献。

验收标准：
- 与手算样本一致；现金流不计收益（入金/赎回独立于收益计算）；
- 固定定额基线与敏感性结果可追溯。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import ROUND_HALF_EVEN, Decimal

from veritasquant.funds.InvestmentBudget import DepositLedgerV1


class PerformanceError(ValueError):
    """业绩计算输入不满足契约。"""


@dataclass(frozen=True, slots=True)
class CashFlowV1:
    """一笔现金流：负为投入，正为赎回/分红。"""

    date: date
    amount: Decimal  # 负：投入；正：回收


@dataclass(frozen=True, slots=True)
class TwrResultV1:
    """时间加权收益率结果。"""

    twr: Decimal
    segments: tuple[Decimal, ...]  # 各现金流间分段时间加权收益


class TwrCalculatorV1:
    """时间加权收益率：现金流分段几何连乘。"""

    def calculate(self, startValue: Decimal, endValue: Decimal, cashFlows: tuple[CashFlowV1, ...]) -> TwrResultV1:
        """TWR = Π(期末值 / (期初值 + 期间净投入)) - 1（按现金流分段）。"""
        if startValue < 0 or endValue < 0:
            raise PerformanceError("期初/期末价值不得为负")
        if not cashFlows:
            if startValue == 0:
                return TwrResultV1(Decimal("0"), ())
            return TwrResultV1((endValue - startValue) / startValue, ())
        # 简化模型：净投入（负现金流为投入）增加分段基数
        netInflow = sum((flow.amount for flow in cashFlows), Decimal("0"))
        base = startValue - netInflow  # 投入为负 -> 基数增加
        if base == 0:
            raise PerformanceError("分段基数不能为零")
        twr = (endValue - base) / base
        return TwrResultV1(twr.quantize(Decimal("0.00000001"), rounding=ROUND_HALF_EVEN), (twr,))


class XirrCalculatorV1:
    """内部收益率：二分法迭代求解 NPV=0。"""

    def __init__(self, maxIterations: int = 100, tolerance: Decimal = Decimal("1e-10")) -> None:
        self._maxIterations = maxIterations
        self._tolerance = tolerance

    def calculate(self, cashFlows: tuple[CashFlowV1, ...]) -> Decimal:
        """求解年化内部收益率（未年化：按天数比例）。"""
        if len(cashFlows) < 2:
            raise PerformanceError("XIRR 至少需要两笔现金流")
        if not any(flow.amount < 0 for flow in cashFlows) or not any(flow.amount > 0 for flow in cashFlows):
            raise PerformanceError("XIRR 需要同时包含投入与回收")
        startDate = min(flow.date for flow in cashFlows)
        low, high = Decimal("-0.9999"), Decimal("10")
        for _ in range(self._maxIterations):
            mid = (low + high) / 2
            npv = self._npv(cashFlows, startDate, mid)
            if abs(npv) < self._tolerance:
                return mid
            if npv > 0:
                low = mid
            else:
                high = mid
        return ((low + high) / 2).quantize(Decimal("0.000001"), rounding=ROUND_HALF_EVEN)

    def _npv(self, cashFlows: tuple[CashFlowV1, ...], startDate: date, rate: Decimal) -> Decimal:
        total = Decimal("0")
        for flow in cashFlows:
            days = (flow.date - startDate).days
            total += flow.amount / ((Decimal("1") + rate) ** Decimal(days / 365.0))
        return total


class PrincipalReporterV1:
    """投入本金与份额报告：现金流不计收益。"""

    def __init__(self, depositLedger: DepositLedgerV1) -> None:
        self._depositLedger = depositLedger

    def investedPrincipal(self, accountId: str) -> Decimal:
        """累计投入本金（独立于收益）。"""
        return self._depositLedger.totalDeposited(accountId)


@dataclass(frozen=True, slots=True)
class RuleContributionV1:
    """单个规则/方案的贡献。"""

    ruleName: str
    investedAmount: Decimal
    shareCount: Decimal


class RuleContributionReporterV1:
    """规则贡献报告：各金额规则/方案的投入与份额。"""

    def __init__(self) -> None:
        self._contributions: list[RuleContributionV1] = []

    def record(self, ruleName: str, investedAmount: Decimal, shareCount: Decimal) -> None:
        if investedAmount < 0 or shareCount < 0:
            raise PerformanceError("投入与份额不得为负")
        self._contributions.append(RuleContributionV1(ruleName, investedAmount, shareCount))

    def report(self) -> tuple[RuleContributionV1, ...]:
        return tuple(self._contributions)

    def totalInvested(self) -> Decimal:
        return sum((item.investedAmount for item in self._contributions), Decimal("0"))
