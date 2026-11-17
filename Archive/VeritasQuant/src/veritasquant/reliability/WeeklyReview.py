"""P2-042 每周复核：数据缺口、账本/订单/持仓对账与重复副作用。

对齐 TechSpec 13 阶段 2 验收：
- 60 日每日对账差异为 0；
- 所有数据缺口有隔离或修复证据；
- 重复副作用计数为 0（inbox 幂等、命令幂等、任务执行键幂等）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date, timedelta
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash


class WeeklyReviewStatus(StrEnum):
    Clean = "CLEAN"
    HasFindings = "HAS_FINDINGS"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class DataGapV1:
    """数据缺口记录：必须隔离或修复。"""

    gapId: str
    symbol: str
    gapDate: str
    resolved: bool  # True=已修复；False=已隔离
    isolationNote: str = ""


@dataclass(frozen=True, slots=True)
class DailyReconciliationResultV1:
    """单日对账结果。"""

    tradingDay: str
    ledgerDifferences: int
    orderDifferences: int
    positionDifferences: int

    @property
    def totalDifferences(self) -> int:
        return self.ledgerDifferences + self.orderDifferences + self.positionDifferences


@dataclass(frozen=True, slots=True)
class WeeklyReviewReportV1:
    """周复核报告：窗口内所有差异与缺口必须归零。"""

    weekStart: str
    weekEnd: str
    dailyResults: tuple[DailyReconciliationResultV1, ...]
    dataGaps: tuple[DataGapV1, ...]
    duplicateSideEffects: int
    reviewHash: str

    @property
    def totalDifferences(self) -> int:
        return sum(d.totalDifferences for d in self.dailyResults)

    @property
    def unresolvedGaps(self) -> int:
        return sum(1 for g in self.dataGaps if not g.resolved)

    @property
    def status(self) -> WeeklyReviewStatus:
        if not self.dailyResults:
            return WeeklyReviewStatus.InsufficientEvidence
        if self.totalDifferences > 0 or self.duplicateSideEffects > 0 or self.unresolvedGaps > 0:
            return WeeklyReviewStatus.HasFindings
        return WeeklyReviewStatus.Clean

    def assertClean(self) -> None:
        """CLEAN 才能继续证据窗口；否则抛出。"""
        if self.status is not WeeklyReviewStatus.Clean:
            raise ValueError(
                f"周复核 {self.weekStart}~{self.weekEnd} 未通过: "
                f"差异 {self.totalDifferences}，缺口未解决 {self.unresolvedGaps}，"
                f"重复副作用 {self.duplicateSideEffects}"
            )


class WeeklyReviewerV1:
    """周复核用例：聚合每日对账、缺口与副作用计数。"""

    def __init__(self, windowDays: int = 60) -> None:
        self._windowDays = windowDays
        self._daily: list[DailyReconciliationResultV1] = []
        self._gaps: list[DataGapV1] = []
        self._duplicateSideEffects = 0

    def addDaily(self, result: DailyReconciliationResultV1) -> None:
        self._daily.append(result)

    def addGap(self, gap: DataGapV1) -> None:
        self._gaps.append(gap)

    def recordDuplicateSideEffects(self, count: int) -> None:
        self._duplicateSideEffects += count

    def review(self, weekStart: str | None = None) -> WeeklyReviewReportV1:
        """生成周复核报告。"""
        today = date.today()
        start = weekStart or (today - timedelta(days=7)).isoformat()
        end = today.isoformat()
        report = WeeklyReviewReportV1(
            weekStart=start,
            weekEnd=end,
            dailyResults=tuple(self._daily),
            dataGaps=tuple(self._gaps),
            duplicateSideEffects=self._duplicateSideEffects,
            reviewHash="",
        )
        payload = {
            "week_start": start,
            "week_end": end,
            "daily": [
                {
                    "trading_day": d.tradingDay,
                    "ledger": d.ledgerDifferences,
                    "orders": d.orderDifferences,
                    "positions": d.positionDifferences,
                }
                for d in self._daily
            ],
            "gaps": [
                {"gap_id": g.gapId, "resolved": g.resolved} for g in self._gaps
            ],
            "duplicate_side_effects": self._duplicateSideEffects,
            "status": report.status.value,
        }
        return WeeklyReviewReportV1(
            weekStart=start,
            weekEnd=end,
            dailyResults=tuple(self._daily),
            dataGaps=tuple(self._gaps),
            duplicateSideEffects=self._duplicateSideEffects,
            reviewHash=canonicalHash(payload),
        )

    def windowComplete(self) -> bool:
        """证据窗口：至少 windowDays 个有效交易日且全部 CLEAN。"""
        if len(self._daily) < self._windowDays:
            return False
        return all(d.totalDifferences == 0 for d in self._daily) and self._duplicateSideEffects == 0
