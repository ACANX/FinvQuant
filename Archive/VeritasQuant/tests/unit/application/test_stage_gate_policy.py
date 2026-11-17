from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.application.StageGatePolicy import (
    GateVerdict,
    StageGatePolicyError,
    StageGatePolicyStoreV1,
    StrategyAcceptancePolicyV1,
)

UTC = timezone.utc


def _policy(**overrides: object) -> StrategyAcceptancePolicyV1:
    values: dict[str, object] = {
        "policyVersion": "StageGatePolicyVersion-1",
        "sampleMonths": 24,
        "minimumClosedTrades": 100,
        "bootstrapSeed": 20260802,
        "bootstrapPercentile": Decimal("0.95"),
        "netReturnLowerBound": Decimal("0"),
        "feeSlippageStressMultiple": Decimal("2"),
        "maxDrawdownLimit": Decimal("0.30"),
        "datasetRef": "13 章样本",
        "statisticalMethod": "bootstrap",
        "windowInterruptionRule": "重置窗口",
        "signer": "ACANX",
    }
    values.update(overrides)
    return StrategyAcceptancePolicyV1(**values)  # type: ignore[call-arg]


def test_freeze_records_version_hash_and_signer() -> None:
    store = StageGatePolicyStoreV1()
    record = store.freeze(_policy())
    assert record.policyVersion == "StageGatePolicyVersion-1"
    assert len(record.policyHash) == 64
    assert record.signer == "ACANX"
    assert isinstance(record.frozenAt, datetime)


def test_frozen_policy_cannot_be_modified() -> None:
    store = StageGatePolicyStoreV1()
    store.freeze(_policy())
    with pytest.raises(StageGatePolicyError, match="不得修改"):
        store.freeze(_policy(sampleMonths=36))


def test_idempotent_freeze_of_identical_policy() -> None:
    store = StageGatePolicyStoreV1()
    first = store.freeze(_policy())
    second = store.freeze(_policy())
    assert first.policyHash == second.policyHash
    assert len(store.frozenVersions) == 1


def test_policy_hash_sensitive_to_all_parameters() -> None:
    assert _policy().policyHash() != _policy(sampleMonths=12).policyHash()
    assert _policy().policyHash() != _policy(bootstrapSeed=1).policyHash()
    assert _policy().policyHash() != _policy(maxDrawdownLimit=Decimal("0.5")).policyHash()


def test_evaluate_pass_fail_insufficient() -> None:
    store = StageGatePolicyStoreV1()
    store.freeze(_policy())
    assert store.evaluate(policyVersion="StageGatePolicyVersion-1", netReturn=Decimal("0.05"), maxDrawdown=Decimal("0.1"), closedTrades=150) is GateVerdict.Pass
    assert store.evaluate(policyVersion="StageGatePolicyVersion-1", netReturn=Decimal("-0.02"), maxDrawdown=Decimal("0.1"), closedTrades=150) is GateVerdict.Fail
    assert store.evaluate(policyVersion="StageGatePolicyVersion-1", netReturn=Decimal("0.05"), maxDrawdown=Decimal("0.1"), closedTrades=50) is GateVerdict.InsufficientEvidence
    assert store.evaluate(policyVersion="StageGatePolicyVersion-1", netReturn=Decimal("0.05"), maxDrawdown=Decimal("0.40"), closedTrades=150) is GateVerdict.Fail


def test_evaluate_unfrozen_policy_rejected() -> None:
    store = StageGatePolicyStoreV1()
    with pytest.raises(StageGatePolicyError, match="尚未冻结"):
        store.evaluate(policyVersion="ghost", netReturn=Decimal("0.1"), maxDrawdown=Decimal("0.1"), closedTrades=200)


def test_rejects_invalid_policy_parameters() -> None:
    store = StageGatePolicyStoreV1()
    with pytest.raises(StageGatePolicyError, match="签署人"):
        store.freeze(_policy(signer=""))
    with pytest.raises(StageGatePolicyError, match="分位数"):
        store.freeze(_policy(bootstrapPercentile=Decimal("1.5")))
    with pytest.raises(StageGatePolicyError, match="数据集"):
        store.freeze(_policy(datasetRef=""))
