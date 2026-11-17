"""P2-041 崩溃恢复演练报告与证据窗口测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.reliability.CrashDrill import (
    CrashDrillEvidenceWindowV1,
    DrillOutcome,
    buildCrashDrillReport,
)

UTC = timezone.utc


def _report(
    drillId: str = "drill-1",
    rtoSeconds: float = 60.0,
    rpoSeconds: float = 0.0,
    controlRecovery: float = 100.0,
    differences: int = 0,
    approvedBy: str | None = "ACANX",
) -> object:
    base = datetime(2026, 8, 1, 9, 0, tzinfo=UTC)
    detected = base + timedelta(seconds=5)
    recovered = detected + timedelta(seconds=rtoSeconds)
    return buildCrashDrillReport(
        drillId=drillId,
        injectedAt=base,
        detectedAt=detected,
        recoveredAt=recovered,
        injectionPoint="AFTER_INBOX",
        protectiveAction="进入保护状态并停止新开仓",
        factHash="f" * 64,
        projectionHash="p" * 64,
        outboxDrainedAt=recovered,
        controlRecoveryPercent=controlRecovery,
        unreconciledDifferences=differences,
        approvedBy=approvedBy,
        rpoSeconds=rpoSeconds,
    )


class TestCrashDrillReport:
    def test_pass_when_all_conditions_met(self) -> None:
        report = _report()
        assert report.outcome is DrillOutcome.Pass
        assert report.rtoWithinTarget
        assert report.rpoZero
        assert report.controlsFullyRecovered
        assert report.differencesZero
        assert len(report.reportHash) == 64

    def test_rto_over_target_fails(self) -> None:
        report = _report(rtoSeconds=20 * 60)  # 20 分钟 > 15 分钟
        assert report.outcome is DrillOutcome.Fail
        assert report.uniqueConclusion() is DrillOutcome.Fail

    def test_rpo_nonzero_fails(self) -> None:
        report = _report(rpoSeconds=5.0)
        assert report.outcome is DrillOutcome.Fail

    def test_control_not_fully_recovered_fails(self) -> None:
        report = _report(controlRecovery=80.0)
        assert report.outcome is DrillOutcome.Fail

    def test_unreconciled_differences_fail(self) -> None:
        report = _report(differences=1)
        assert report.outcome is DrillOutcome.Fail

    def test_missing_approval_insufficient_evidence(self) -> None:
        report = _report(approvedBy=None)
        assert report.outcome is DrillOutcome.InsufficientEvidence

    def test_assert_pass_raises_for_fail(self) -> None:
        report = _report(rtoSeconds=20 * 60)
        with pytest.raises(ValueError, match="不得计入证据窗口"):
            report.assertPass()


class TestCrashDrillEvidenceWindow:
    def test_requires_three_passes(self) -> None:
        window = CrashDrillEvidenceWindowV1()
        assert window.windowComplete() is False
        assert window.missingCount() == 3
        for i in range(1, 4):
            window.register(_report(drillId=f"drill-{i}"))
        assert window.passedCount() == 3
        assert window.windowComplete() is True
        assert window.missingCount() == 0

    def test_failed_drill_not_counted(self) -> None:
        window = CrashDrillEvidenceWindowV1()
        failed = _report(drillId="bad", rtoSeconds=20 * 60)
        with pytest.raises(ValueError):
            window.register(failed)
        assert window.passedCount() == 0
