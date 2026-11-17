from __future__ import annotations

import pytest

from veritasquant.core.CrashInjection import CrashInjectedError, CrashInjectorV1, CrashPoint


@pytest.mark.parametrize("point", tuple(CrashPoint))
def test_each_commit_boundary_can_be_deterministically_injected(point: CrashPoint) -> None:
    injector = CrashInjectorV1(point)
    with pytest.raises(CrashInjectedError):
        injector.hit(point)
    assert injector.records[0].point is point


def test_injector_uses_configured_hit_count_without_clock_or_randomness() -> None:
    injector = CrashInjectorV1(CrashPoint.BeforeOutbox, triggerHit=2)
    injector.hit(CrashPoint.BeforeOutbox)
    with pytest.raises(CrashInjectedError, match="第 2 次"):
        injector.hit(CrashPoint.BeforeOutbox)
