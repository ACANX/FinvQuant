"""理想/真实双轨对比与完整报告（技术方案 7.3 节）。

报告明确模式、版本、数据缺口和摩擦；不能把理想结果标为实盘预期。
同一次回测同时产出理想与真实模式的权益、收益、回撤、夏普等指标，
并显示真实相对理想的摩擦损耗。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.reporting.Performance import (
    EquityPointV1,
    PerformanceCalculatorV1,
    PerformanceMetricsV1,
    TradeRecordV1,
)


class ReportError(ValueError):
    """双轨报告违反模式标识或版本契约时抛出。"""


@dataclass(frozen=True, slots=True)
class TrackPerformanceV1:
    """单轨绩效结果。"""

    mode: str
    metrics: PerformanceMetricsV1


@dataclass(frozen=True, slots=True)
class FrictionSummaryV1:
    """真实相对理想的摩擦损耗。"""

    returnGap: Decimal
    drawdownGap: Decimal
    fillRate: Decimal
    averageSlippage: Decimal


@dataclass(frozen=True, slots=True)
class DualTrackReportV1:
    """理想/真实双轨完整报告。"""

    reportId: str
    executionModelVersion: str
    barPathModelVersion: str
    liquidityAllocationVersion: str
    ideal: TrackPerformanceV1
    real: TrackPerformanceV1
    friction: FrictionSummaryV1
    dataGaps: tuple[str, ...]
    reportHash: str

    def assertIdealNotMarkedAsLive(self) -> None:
        """理想结果不得标为实盘预期。"""
        if self.ideal.mode == "IDEAL" and "LIVE" in self.reportId.upper():
            raise ReportError("理想结果被错误标识为实盘预期")


class DualTrackReportBuilderV1:
    """构建理想/真实双轨报告。"""

    def __init__(
        self,
        *,
        executionModelVersion: str = "DELAYED_SLIPPAGE_PARTIAL_V1",
        barPathModelVersion: str = "DIRECTIONAL_OHLC_V1",
        liquidityAllocationVersion: str = "V1",
    ) -> None:
        if not all((executionModelVersion, barPathModelVersion, liquidityAllocationVersion)):
            raise ReportError("报告必须记录全部模式与版本")
        self._executionModelVersion = executionModelVersion
        self._barPathModelVersion = barPathModelVersion
        self._liquidityAllocationVersion = liquidityAllocationVersion
        self._calculator = PerformanceCalculatorV1()

    def build(
        self,
        *,
        reportId: str,
        idealEquity: tuple[EquityPointV1, ...],
        realEquity: tuple[EquityPointV1, ...],
        trades: tuple[TradeRecordV1, ...],
        dataGaps: tuple[str, ...] = (),
    ) -> DualTrackReportV1:
        """从双轨净值与成交构建完整报告。"""
        if not reportId:
            raise ReportError("报告 ID 不能为空")
        idealMetrics = self._calculator.calculate(equityCurve=idealEquity, cashFlows=(), trades=trades)
        realMetrics = self._calculator.calculate(equityCurve=realEquity, cashFlows=(), trades=trades)
        friction = FrictionSummaryV1(
            returnGap=realMetrics.totalReturn - idealMetrics.totalReturn,
            drawdownGap=realMetrics.maxDrawdown - idealMetrics.maxDrawdown,
            fillRate=realMetrics.fillRate,
            averageSlippage=realMetrics.averageSlippage,
        )
        reportHash = canonicalHash(
            {
                "report_id": reportId,
                "execution_model_version": self._executionModelVersion,
                "bar_path_model_version": self._barPathModelVersion,
                "liquidity_allocation_version": self._liquidityAllocationVersion,
                "ideal_return": idealMetrics.totalReturn,
                "real_return": realMetrics.totalReturn,
                "ideal_max_drawdown": idealMetrics.maxDrawdown,
                "real_max_drawdown": realMetrics.maxDrawdown,
                "return_gap": friction.returnGap,
                "data_gaps": list(dataGaps),
            }
        )
        return DualTrackReportV1(
            reportId=reportId,
            executionModelVersion=self._executionModelVersion,
            barPathModelVersion=self._barPathModelVersion,
            liquidityAllocationVersion=self._liquidityAllocationVersion,
            ideal=TrackPerformanceV1("IDEAL", idealMetrics),
            real=TrackPerformanceV1("REAL", realMetrics),
            friction=friction,
            dataGaps=dataGaps,
            reportHash=reportHash,
        )
