"""P2-039 M2A 候选版本冻结与容量预演单元测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.reliability.CandidateFreeze import (
    CandidateFreezeStoreV1,
    CapacityForecasterV1,
    CapacityObservationV1,
    CapacityResource,
    SeverityLevel,
    buildCandidateFreeze,
)

UTC = timezone.utc


def _obs(
    resource: CapacityResource,
    usedBytes: int,
    capacityBytes: int,
    daysAgo: int = 1,
) -> CapacityObservationV1:
    now = datetime.now(UTC)
    now = now.replace(microsecond=(now.microsecond // 1_000) * 1_000)
    return CapacityObservationV1(
        resource=resource,
        usedBytes=usedBytes,
        capacityBytes=capacityBytes,
        observedAt=now - timedelta(days=daysAgo),
    )


class TestCapacityForecaster:
    def test_adequate_capacity_passes(self) -> None:
        forecaster = CapacityForecasterV1(evidenceWindowDays=60)
        # 已用 10GB，容量 1TB → 2x 覆盖充足
        forecaster.record(_obs(CapacityResource.Disk, 10_000_000_000, 1_000_000_000_000))
        forecaster.record(_obs(CapacityResource.Database, 5_000_000_000, 500_000_000_000))
        result = forecaster.runPreflight()
        assert result.passed is True
        assert result.worstSeverity() is SeverityLevel.None_

    def test_s1_blocking_when_capacity_tight(self) -> None:
        forecaster = CapacityForecasterV1(evidenceWindowDays=60)
        # 已用 800GB，容量 2TB → 外推后需求/当前 ≈ 0.57 → S1
        forecaster.record(_obs(CapacityResource.Disk, 800_000_000_000, 2_000_000_000_000))
        result = forecaster.runPreflight()
        assert result.passed is False
        assert result.worstSeverity() is SeverityLevel.S1
        assert result.blockingFindings

    def test_s0_when_extreme_overcommit(self) -> None:
        forecaster = CapacityForecasterV1(evidenceWindowDays=60)
        # 已用接近容量且容量很小 → 需求/当前 < 0.5 → S0
        forecaster.record(_obs(CapacityResource.Queue, 900, 1000))
        result = forecaster.runPreflight()
        assert result.worstSeverity() is SeverityLevel.S0

    def test_unknown_resource_skipped(self) -> None:
        forecaster = CapacityForecasterV1(evidenceWindowDays=60)
        assert forecaster.forecast(CapacityResource.Database) is None
        result = forecaster.runPreflight()
        assert result.passed is True  # 无观测 → 无阻断

    def test_evidence_window_validation(self) -> None:
        with pytest.raises(ValueError):
            CapacityForecasterV1(evidenceWindowDays=0)


class TestCandidateFreeze:
    def _preflight(self, passed: bool) -> object:
        from veritasquant.reliability.CandidateFreeze import CapacityPreflightResultV1

        if passed:
            return CapacityPreflightResultV1(
                forecasts=(),
                passed=True,
                blockingFindings=(),
            )
        return CapacityPreflightResultV1(
            forecasts=(),
            passed=False,
            blockingFindings=("DISK 容量不足"),
        )

    def test_freeze_records_and_hash(self) -> None:
        candidate = buildCandidateFreeze(
            codeVersion="0.9.0",
            dependencyLockHash="a" * 64,
            eventSchemaRegistryHash="b" * 64,
            frozenBy="BeeAgent",
            preflight=self._preflight(True),
        )
        store = CandidateFreezeStoreV1()
        record = store.freeze(candidate)
        assert record.state == "FROZEN"
        assert len(record.freezeHash) == 64
        assert store.current() is record
        assert store.hashOf(record.freezeHash) is record

    def test_freeze_rejected_when_preflight_failed(self) -> None:
        candidate = buildCandidateFreeze(
            codeVersion="0.9.0",
            dependencyLockHash="a" * 64,
            eventSchemaRegistryHash="b" * 64,
            frozenBy="BeeAgent",
            preflight=self._preflight(False),
        )
        store = CandidateFreezeStoreV1()
        with pytest.raises(ValueError, match="容量预演未通过"):
            store.freeze(candidate)
        assert store.current() is None

    def test_freeze_hash_changes_with_fields(self) -> None:
        base = buildCandidateFreeze(
            codeVersion="0.9.0",
            dependencyLockHash="a" * 64,
            eventSchemaRegistryHash="b" * 64,
            frozenBy="BeeAgent",
            preflight=self._preflight(True),
        )
        changed = buildCandidateFreeze(
            codeVersion="0.9.1",
            dependencyLockHash="a" * 64,
            eventSchemaRegistryHash="b" * 64,
            frozenBy="BeeAgent",
            preflight=self._preflight(True),
        )
        assert base.freezeHash() != changed.freezeHash()

    def test_no_s0_s1_check(self) -> None:
        candidate = buildCandidateFreeze(
            codeVersion="0.9.0",
            dependencyLockHash="a" * 64,
            eventSchemaRegistryHash="b" * 64,
            frozenBy="BeeAgent",
            preflight=self._preflight(True),
        )
        assert candidate.containsNoS0S1() is True

    def test_supersede_keeps_history(self) -> None:
        store = CandidateFreezeStoreV1()
        first = store.freeze(
            buildCandidateFreeze(
                codeVersion="0.9.0",
                dependencyLockHash="a" * 64,
                eventSchemaRegistryHash="b" * 64,
                frozenBy="BeeAgent",
                preflight=self._preflight(True),
            )
        )
        second = store.freeze(
            buildCandidateFreeze(
                codeVersion="0.9.1",
                dependencyLockHash="a" * 64,
                eventSchemaRegistryHash="b" * 64,
                frozenBy="BeeAgent",
                preflight=self._preflight(True),
            )
        )
        assert len(store.all()) == 2
        assert store.current() is second
        assert store.hashOf(first.freezeHash) is first
