"""不可变、可复现的运行清单。"""

from __future__ import annotations

from datetime import datetime
from typing import Mapping

from pydantic import field_validator

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class RunManifestV1(StrictModel):
    """阶段 1 所需的运行身份、版本与结果计数。"""

    codeVersion: str = PascalAlias("CodeVersion", min_length=1)
    eventSchemaRegistryHash: str = PascalAlias("EventSchemaRegistryHash", min_length=64, max_length=64)
    strategyVersion: str = PascalAlias("StrategyVersion", min_length=1)
    strategySourceHash: str = PascalAlias("StrategySourceHash", min_length=64, max_length=64)
    dependencyLockHash: str = PascalAlias("DependencyLockHash", min_length=64, max_length=64)
    interpreterVersion: str = PascalAlias("InterpreterVersion", min_length=1)
    sandboxImageDigest: str = PascalAlias("SandboxImageDigest", min_length=1)
    strategySandboxPolicyVersion: str = PascalAlias("StrategySandboxPolicyVersion", min_length=1)
    strategyDslSchemaVersion: str = PascalAlias("StrategyDslSchemaVersion", min_length=1)
    investmentPlanSchemaVersion: str = PascalAlias("InvestmentPlanSchemaVersion", min_length=1)
    configHash: str = PascalAlias("ConfigHash", min_length=64, max_length=64)
    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion", min_length=1)
    dataVersionId: str = PascalAlias("DataVersionId", min_length=1)
    assetCapabilityVersion: str = PascalAlias("AssetCapabilityVersion", min_length=1)
    accountGroupId: str = PascalAlias("AccountGroupId", min_length=1)
    accountRanks: Mapping[str, int] = PascalAlias("AccountRanks")
    randomSeed: int = PascalAlias("RandomSeed", ge=0)
    tsPrecision: TsPrecision = PascalAlias("TsPrecision")
    eventOrderingVersion: str = PascalAlias("EventOrderingVersion", min_length=1)
    executionModelVersion: str = PascalAlias("ExecutionModelVersion", min_length=1)
    fundExecutionModelVersion: str = PascalAlias("FundExecutionModelVersion", min_length=1)
    navAvailabilityPolicyVersion: str = PascalAlias("NavAvailabilityPolicyVersion", min_length=1)
    barPathModelVersion: str = PascalAlias("BarPathModelVersion", min_length=1)
    liquidityAllocationVersion: str = PascalAlias("LiquidityAllocationVersion", min_length=1)
    riskPolicyVersion: str = PascalAlias("RiskPolicyVersion", min_length=1)
    reliabilityPolicyVersion: str = PascalAlias("ReliabilityPolicyVersion", min_length=1)
    startedAt: datetime = PascalAlias("StartedAt")
    completedAt: datetime | None = PascalAlias("CompletedAt", default=None)
    eventCount: int = PascalAlias("EventCount", ge=0)
    orderCount: int = PascalAlias("OrderCount", ge=0)
    fundSubscriptionCount: int = PascalAlias("FundSubscriptionCount", ge=0)
    fundConfirmationCount: int = PascalAlias("FundConfirmationCount", ge=0)
    executionCount: int = PascalAlias("ExecutionCount", ge=0)
    reportPath: str | None = PascalAlias("ReportPath", default=None)

    @field_validator("startedAt", "completedAt")
    @classmethod
    def validateManifestTime(cls, value: datetime | None) -> datetime | None:
        if value is None:
            return None
        return validateUtcTimestamp(value, TsPrecision.Millisecond)

    @field_validator("accountRanks")
    @classmethod
    def validateAccountRanks(cls, value: Mapping[str, int]) -> Mapping[str, int]:
        if not value or any(not accountId or rank < 0 for accountId, rank in value.items()):
            raise ValueError("AccountRanks 必须包含非空账户与非负排名")
        if len(set(value.values())) != len(value):
            raise ValueError("同一账户组内 AccountRanks 不得重复")
        return value

    def identityHash(self) -> str:
        """计算不受时长、计数和报告位置影响的可复现运行身份。"""
        identity = self.model_dump(mode="python", by_alias=False, exclude_none=False)
        for key in (
            "startedAt",
            "completedAt",
            "eventCount",
            "orderCount",
            "fundSubscriptionCount",
            "fundConfirmationCount",
            "executionCount",
            "reportPath",
        ):
            identity.pop(key)
        return canonicalHash(identity, self.tsPrecision)
