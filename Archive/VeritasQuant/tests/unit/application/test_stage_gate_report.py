from __future__ import annotations


import pytest

from veritasquant.application.StageGatePolicy import GateVerdict
from veritasquant.application.StageGateReport import (
    GateCheckItemV1,
    GateItemStatus,
    StageGateError,
    StageGateReportBuilderV1,
)


def _passingChecks() -> tuple[GateCheckItemV1, ...]:
    return tuple(
        GateCheckItemV1(f"check-{index}", f"强制项 {index}", GateItemStatus.Pass, f"hash-{index}")
        for index in range(1, 8)
    )


def _build(**overrides: object):
    values: dict[str, object] = {
        "gateName": "M1",
        "stageGatePolicyVersion": "StageGatePolicyVersion-1",
        "checks": _passingChecks(),
        "openS0": 0,
        "openS1": 0,
        "lookaheadHits": 0,
        "propertySequences": 10_000,
        "signer": None,
    }
    values.update(overrides)
    return StageGateReportBuilderV1().build(**values)  # type: ignore[arg-type]


def test_all_mandatory_passed_and_zero_open_s0s1_is_pass() -> None:
    report = _build()
    assert report.verdict is GateVerdict.Pass
    assert report.mandatoryPassed
    assert report.assertUniqueConclusion() is None


def test_any_mandatory_failure_is_fail() -> None:
    checks = (GateCheckItemV1("c1", "x", GateItemStatus.Fail, "hash"),)
    report = _build(checks=checks)
    assert report.verdict is GateVerdict.Fail
    with pytest.raises(StageGateError, match="不得进入阶段"):
        report.assertUniqueConclusion()


def test_open_s0_or_s1_blocks_pass() -> None:
    assert _build(openS0=1).verdict is GateVerdict.Fail
    assert _build(openS1=2).verdict is GateVerdict.Fail


def test_lookahead_hits_block_pass() -> None:
    assert _build(lookaheadHits=1).verdict is GateVerdict.Fail


def test_insufficient_property_sequences_is_insufficient_evidence() -> None:
    report = _build(propertySequences=5_000)
    assert report.verdict is GateVerdict.InsufficientEvidence


def test_report_hash_is_deterministic() -> None:
    first = _build()
    second = _build()
    assert first.reportHash == second.reportHash
    assert len(first.reportHash) == 64
    changed = _build(openS0=1)
    assert first.reportHash != changed.reportHash


def test_rejects_empty_checks_and_negative_counts() -> None:
    with pytest.raises(StageGateError, match="检查项"):
        _build(checks=())
    with pytest.raises(StageGateError, match="不得为负"):
        _build(openS0=-1)


def test_m1_mandatory_checklist_is_complete() -> None:
    checks = StageGateReportBuilderV1.m1MandatoryChecks()
    assert len(checks) == 7
    assert all(item.status is GateItemStatus.Pass for item in checks)
    assert checks[0].checkId == "M1-001"
