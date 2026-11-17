from __future__ import annotations

from datetime import datetime, timezone

import pytest
from pydantic import ValidationError

from veritasquant.core.RunManifest import RunManifestV1
from veritasquant.core.Time import TsPrecision


def makeManifest(**overrides: object) -> RunManifestV1:
    hashValue = "a" * 64
    values: dict[str, object] = {
        "CodeVersion": "git:abc",
        "EventSchemaRegistryHash": hashValue,
        "StrategyVersion": "1.0",
        "StrategySourceHash": hashValue,
        "DependencyLockHash": hashValue,
        "InterpreterVersion": "Python 3.13",
        "SandboxImageDigest": "sha256:test",
        "StrategySandboxPolicyVersion": "V1",
        "StrategyDslSchemaVersion": "V1",
        "InvestmentPlanSchemaVersion": "V1",
        "ConfigHash": hashValue,
        "ConfigSchemaVersion": "V1",
        "DataVersionId": "data-1",
        "AssetCapabilityVersion": "V1",
        "AccountGroupId": "group-1",
        "AccountRanks": {"account-1": 0},
        "RandomSeed": 7,
        "TsPrecision": TsPrecision.Second,
        "EventOrderingVersion": "V1",
        "ExecutionModelVersion": "V1",
        "FundExecutionModelVersion": "V1",
        "NavAvailabilityPolicyVersion": "V1",
        "BarPathModelVersion": "V1",
        "LiquidityAllocationVersion": "V1",
        "RiskPolicyVersion": "V1",
        "ReliabilityPolicyVersion": "V1",
        "StartedAt": datetime(2026, 7, 31, 8, 15, 30, tzinfo=timezone.utc),
        "EventCount": 1,
        "OrderCount": 0,
        "FundSubscriptionCount": 0,
        "FundConfirmationCount": 0,
        "ExecutionCount": 0,
    }
    values.update(overrides)
    return RunManifestV1.model_validate(values)


def test_identity_fields_are_stable_when_only_result_counters_change() -> None:
    first = makeManifest(EventCount=1)
    second = makeManifest(EventCount=2, OrderCount=1)
    assert first.identityHash() == second.identityHash()


def test_missing_required_version_prevents_run_manifest_creation() -> None:
    values = makeManifest().model_dump(by_alias=True)
    values.pop("RiskPolicyVersion")
    with pytest.raises(ValidationError):
        RunManifestV1.model_validate(values)
