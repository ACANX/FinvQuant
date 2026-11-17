"""P2-042 每周复核工具测试。"""

from __future__ import annotations

import pytest

from veritasquant.reliability.WeeklyReview import (
    DailyReconciliationResultV1,
    DataGapV1,
    WeeklyReviewStatus,
    WeeklyReviewerV1,
)


class TestWeeklyReview:
    def test_clean_when_no_findings(self) -> None:
        reviewer = WeeklyReviewerV1()
        for day in range(1, 8):
            reviewer.addDaily(
                DailyReconciliationResultV1(
                    tradingDay=f"2026-08-{day:02d}",
                    ledgerDifferences=0,
                    orderDifferences=0,
                    positionDifferences=0,
                )
            )
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.Clean
        assert report.totalDifferences == 0
        assert len(report.reviewHash) == 64
        report.assertClean()  # 不抛

    def test_has_findings_when_differences(self) -> None:
        reviewer = WeeklyReviewerV1()
        reviewer.addDaily(
            DailyReconciliationResultV1(
                tradingDay="2026-08-01",
                ledgerDifferences=1,
                orderDifferences=0,
                positionDifferences=0,
            )
        )
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.HasFindings
        with pytest.raises(ValueError, match="未通过"):
            report.assertClean()

    def test_has_findings_when_unresolved_gap(self) -> None:
        reviewer = WeeklyReviewerV1()
        reviewer.addDaily(
            DailyReconciliationResultV1("2026-08-01", 0, 0, 0)
        )
        reviewer.addGap(DataGapV1("gap-1", "518880", "2026-08-01", resolved=False))
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.HasFindings
        assert report.unresolvedGaps == 1

    def test_resolved_gap_not_finding(self) -> None:
        reviewer = WeeklyReviewerV1()
        reviewer.addDaily(
            DailyReconciliationResultV1("2026-08-01", 0, 0, 0)
        )
        reviewer.addGap(DataGapV1("gap-1", "518880", "2026-08-01", resolved=True))
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.Clean

    def test_duplicate_side_effects_finding(self) -> None:
        reviewer = WeeklyReviewerV1()
        reviewer.addDaily(
            DailyReconciliationResultV1("2026-08-01", 0, 0, 0)
        )
        reviewer.recordDuplicateSideEffects(2)
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.HasFindings

    def test_insufficient_evidence_when_no_daily(self) -> None:
        reviewer = WeeklyReviewerV1()
        report = reviewer.review()
        assert report.status is WeeklyReviewStatus.InsufficientEvidence

    def test_window_complete_requires_60_clean_days(self) -> None:
        reviewer = WeeklyReviewerV1(windowDays=60)
        for day in range(1, 60):
            reviewer.addDaily(DailyReconciliationResultV1(f"day-{day}", 0, 0, 0))
        assert reviewer.windowComplete() is False  # 59 天不足
        reviewer.addDaily(DailyReconciliationResultV1("day-60", 0, 0, 0))
        assert reviewer.windowComplete() is True

    def test_window_complete_false_with_differences(self) -> None:
        reviewer = WeeklyReviewerV1(windowDays=3)
        reviewer.addDaily(DailyReconciliationResultV1("d1", 0, 0, 0))
        reviewer.addDaily(DailyReconciliationResultV1("d2", 1, 0, 0))
        reviewer.addDaily(DailyReconciliationResultV1("d3", 0, 0, 0))
        assert reviewer.windowComplete() is False
