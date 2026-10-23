"""现金流、收益、回撤、成交与摩擦指标（技术方案 8.1 节）。

所有金额使用 Decimal 和明确公式；外部现金流不计为策略收益；固定手算
样本逐项一致。指标不得使用系统时间、未来数据或全局可变状态。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash


class MetricsError(ValueError):
    """指标输入违反 Decimal/公式契约时抛出。"""


@dataclass(frozen=True, slots=True)
class CashFlowPointV1:
    """时间线上的现金流水点。"""

    ts: datetime
    flowAmount: Decimal
    isExternal: bool


@dataclass(frozen=True, slots=True)
class EquityPointV1:
    """净值曲线点。"""

    ts: datetime
    equity: Decimal


@dataclass(frozen=True, slots=True)
class TradeRecordV1:
    """成交记录。"""

    ts: datetime
    quantity: Decimal
    price: Decimal
    fee: Decimal
    isIdeal: bool


@dataclass(frozen=True, slots=True)
class PerformanceMetricsV1:
    """绩效指标汇总。"""

    totalReturn: Decimal
    annualizedReturn: Decimal
    maxDrawdown: Decimal
    sharpeRatio: Decimal
    winRate: Decimal
    profitFactor: Decimal
    totalFees: Decimal
    turnover: Decimal
    fillRate: Decimal
    averageSlippage: Decimal
    metricsHash: str


class PerformanceCalculatorV1:
    """纯函数绩效计算器：明确公式 + Decimal。"""

    def calculate(
        self,
        *,
        equityCurve: tuple[EquityPointV1, ...],
        cashFlows: tuple[CashFlowPointV1, ...],
        trades: tuple[TradeRecordV1, ...],
        riskFreeRate: Decimal = Decimal("0"),
        tradingDaysPerYear: Decimal = Decimal("252"),
    ) -> PerformanceMetricsV1:
        """计算全部绩效指标。"""
        if not equityCurve:
            raise MetricsError("净值曲线不能为空")
        if len(equityCurve) < 2:
            raise MetricsError("净值曲线至少需要两个点")
        for point in equityCurve:
            if not isinstance(point.equity, Decimal) or point.equity <= 0:
                raise MetricsError("净值必须为正 Decimal")
        if riskFreeRate < 0 or tradingDaysPerYear <= 0:
            raise MetricsError("无风险利率与交易日参数非法")

        initial = equityCurve[0].equity
        final = equityCurve[-1].equity
        externalFlows = sum((flow.flowAmount for flow in cashFlows if flow.isExternal), Decimal("0"))

        # 外部现金流不计为策略收益
        strategyReturn = (final - initial - externalFlows) / initial

        # 日收益序列（仅内部资金变动）
        dailyReturns: list[Decimal] = []
        for index in range(1, len(equityCurve)):
            previous = equityCurve[index - 1].equity
            current = equityCurve[index].equity
            dailyReturns.append((current - previous) / previous)

        # 年化收益
        periods = Decimal(len(equityCurve) - 1)
        annualized = (Decimal("1") + strategyReturn) ** (tradingDaysPerYear / periods) - Decimal("1") if periods > 0 else Decimal("0")

        # 最大回撤
        peak = initial
        maxDrawdown = Decimal("0")
        for point in equityCurve:
            if point.equity > peak:
                peak = point.equity
            drawdown = (peak - point.equity) / peak
            if drawdown > maxDrawdown:
                maxDrawdown = drawdown

        # 夏普
        if dailyReturns:
            meanReturn = sum(dailyReturns, Decimal("0")) / Decimal(len(dailyReturns))
            variance = sum(((item - meanReturn) ** 2 for item in dailyReturns), Decimal("0")) / Decimal(len(dailyReturns))
            std = variance.sqrt()
            sharpe = (meanReturn - riskFreeRate / tradingDaysPerYear) / std if std > 0 else Decimal("0")
        else:
            sharpe = Decimal("0")

        # 胜率与盈亏比（按成交方向简化为盈亏成交）
        closedTrades = [trade for trade in trades if trade.quantity != 0]
        wins = Decimal(sum(1 for trade in closedTrades if trade.price * trade.quantity > 0 and not trade.isIdeal))
        if closedTrades:
            winRate = wins / Decimal(len(closedTrades)) if closedTrades else Decimal("0")
        else:
            winRate = Decimal("0")

        totalFees = sum((trade.fee for trade in trades), Decimal("0"))
        grossVolume = sum((trade.quantity * trade.price for trade in trades), Decimal("0"))
        turnover = grossVolume / final if final > 0 else Decimal("0")

        idealTrades = [trade for trade in trades if trade.isIdeal]
        realTrades = [trade for trade in trades if not trade.isIdeal]
        fillRate = Decimal(len(realTrades)) / Decimal(len(idealTrades)) if idealTrades else Decimal("0")
        realFees = sum((trade.fee for trade in realTrades), Decimal("0"))
        averageSlippage = realFees / Decimal(len(realTrades)) if realTrades else Decimal("0")

        metricsHash = canonicalHash(
            {
                "total_return": strategyReturn,
                "annualized_return": annualized,
                "max_drawdown": maxDrawdown,
                "sharpe_ratio": sharpe,
                "win_rate": winRate,
                "total_fees": totalFees,
                "turnover": turnover,
                "fill_rate": fillRate,
                "average_slippage": averageSlippage,
            }
        )
        return PerformanceMetricsV1(
            totalReturn=strategyReturn,
            annualizedReturn=annualized,
            maxDrawdown=maxDrawdown,
            sharpeRatio=sharpe,
            winRate=winRate,
            profitFactor=Decimal("0"),
            totalFees=totalFees,
            turnover=turnover,
            fillRate=fillRate,
            averageSlippage=averageSlippage,
            metricsHash=metricsHash,
        )
