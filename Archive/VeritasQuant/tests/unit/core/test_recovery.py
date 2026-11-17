from __future__ import annotations

import pytest

from veritasquant.core.CrashInjection import CrashPoint
from veritasquant.core.Recovery import verifyRecoveryInvariant


@pytest.mark.parametrize("point", tuple(CrashPoint))
def test_crash_points_preserve_atomic_facts_and_rebuildable_projection(point: CrashPoint) -> None:
    report = verifyRecoveryInvariant(point)
    assert report.randomSeed == 20260801
    assert report.committedFactCount == 5
    assert report.committedOutboxCount == 1
    assert report.projectionHash is not None
    assert report.controlHash == "c" * 64
    if point is CrashPoint.AfterOutbox:
        assert report.replayCount == 0
    else:
        assert report.replayCount == 1


def test_all_crash_points_recover_to_the_same_deterministic_state() -> None:
    reports = tuple(verifyRecoveryInvariant(point) for point in CrashPoint)
    assert len({report.factsHash for report in reports}) == 1
    assert len({report.projectionHash for report in reports}) == 1
