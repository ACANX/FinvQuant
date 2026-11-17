"""P2-039 M2A 候选版本冻结与容量/故障预演。

对齐 TechSpec 13 阶段 2 验收：
- 冻结 M2A 候选版本：代码版本、依赖锁、Schema 注册表、策略/风控/可靠性政策版本、
  事件 Schema 哈希等不可变标识，冻结后禁止未审批变更；
- 容量预演：队列/磁盘/数据库容量至少覆盖证据窗口（60 个有效交易日）2 倍预测峰值；
- 预演输出 S0/S1 判定：任何 S0/S1 风险阻止候选版本冻结。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum
from typing import Any

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import StrictModel, PascalAlias


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class CapacityResource(StrEnum):
    Queue = "QUEUE"
    Disk = "DISK"
    Database = "DATABASE"


class SeverityLevel(StrEnum):
    S0 = "S0"  # 阻断发布：数据丢失或交易安全风险
    S1 = "S1"  # 高优先级：证据窗口内必然容量耗尽
    S2 = "S2"  # 观察项：接近阈值，需要监控
    None_ = "NONE"


@dataclass(frozen=True, slots=True)
class CapacityObservationV1:
    """一次容量观测：资源用量与时间。"""

    resource: CapacityResource
    usedBytes: int
    capacityBytes: int
    observedAt: datetime = field(default_factory=lambda: datetime.now(timezone.utc))


@dataclass(frozen=True, slots=True)
class PeakForecastV1:
    """按资源预测的证据窗口峰值。"""

    resource: CapacityResource
    forecastPeakBytes: int  # 证据窗口内预测峰值
    safetyFactor: float  # 必须 >= 2.0
    requiredCapacityBytes: int  # forecastPeak * safetyFactor
    currentCapacityBytes: int
    adequate: bool
    severity: SeverityLevel


@dataclass(frozen=True, slots=True)
class CapacityPreflightResultV1:
    """容量预演结果：全部资源是否满足 2x 覆盖。"""

    forecasts: tuple[PeakForecastV1, ...] = ()
    passed: bool = False
    blockingFindings: tuple[str, ...] = ()

    def worstSeverity(self) -> SeverityLevel:
        levels = [f.severity for f in self.forecasts]
        if SeverityLevel.S0 in levels:
            return SeverityLevel.S0
        if SeverityLevel.S1 in levels:
            return SeverityLevel.S1
        if SeverityLevel.S2 in levels:
            return SeverityLevel.S2
        return SeverityLevel.None_


class CapacityForecasterV1:
    """基于历史观测外推证据窗口峰值（线性增长率上限）。"""

    MIN_SAFETY_FACTOR = 2.0

    def __init__(self, evidenceWindowDays: int = 60) -> None:
        """evidenceWindowDays：阶段 2 证据窗口（60 个有效交易日）。"""
        if evidenceWindowDays <= 0:
            raise ValueError("证据窗口天数必须为正")
        self._windowDays = evidenceWindowDays
        self._observations: list[CapacityObservationV1] = []

    def record(self, observation: CapacityObservationV1) -> None:
        self._observations.append(observation)

    def forecast(self, resource: CapacityResource, growthMultiplier: float = 1.0) -> PeakForecastV1 | None:
        """预测证据窗口峰值；growthMultiplier 为业务量增长系数（默认 1.0）。"""
        matching = [o for o in self._observations if o.resource is resource]
        if not matching:
            return None
        # 取观测样本的日增长率上限，外推 windowDays 天
        used = [o.usedBytes for o in matching]
        peakObserved = max(used)
        # 保守估计：观测最大值 * (1 + 窗口天数 * 2% 日增长) * growthMultiplier
        dailyGrowth = 0.02
        forecastPeak = int(peakObserved * (1 + dailyGrowth * self._windowDays) * growthMultiplier)
        required = int(forecastPeak * self.MIN_SAFETY_FACTOR)
        current = min(o.capacityBytes for o in matching)
        adequate = current >= required
        ratio = current / max(1, required)
        if ratio < 1.0:
            severity = SeverityLevel.S0 if ratio < 0.5 else SeverityLevel.S1
        elif ratio < self.MIN_SAFETY_FACTOR:
            severity = SeverityLevel.S2
        else:
            severity = SeverityLevel.None_
        return PeakForecastV1(
            resource=resource,
            forecastPeakBytes=forecastPeak,
            safetyFactor=self.MIN_SAFETY_FACTOR,
            requiredCapacityBytes=required,
            currentCapacityBytes=current,
            adequate=adequate,
            severity=severity,
        )

    def runPreflight(
        self, growthMultiplier: float = 1.0
    ) -> CapacityPreflightResultV1:
        """对所有观测过的资源执行预演；S0/S1 视为阻断。"""
        forecasts: list[PeakForecastV1] = []
        findings: list[str] = []
        for resource in CapacityResource:
            forecast = self.forecast(resource, growthMultiplier)
            if forecast is None:
                continue
            forecasts.append(forecast)
            if forecast.severity in (SeverityLevel.S0, SeverityLevel.S1):
                findings.append(
                    f"{resource.value} 容量不足: 预测峰值 {forecast.forecastPeakBytes}B, "
                    f"需求(2x) {forecast.requiredCapacityBytes}B, 当前 {forecast.currentCapacityBytes}B"
                )
        return CapacityPreflightResultV1(
            forecasts=tuple(forecasts),
            passed=not findings,
            blockingFindings=tuple(findings),
        )


class CandidateFreezeV1(StrictModel):
    """M2A 候选版本冻结清单（对齐 RunManifestV1 身份字段）。"""

    codeVersion: str = PascalAlias("CodeVersion", min_length=1)
    dependencyLockHash: str = PascalAlias("DependencyLockHash", min_length=64, max_length=64)
    eventSchemaRegistryHash: str = PascalAlias("EventSchemaRegistryHash", min_length=64, max_length=64)
    strategySandboxPolicyVersion: str = PascalAlias("StrategySandboxPolicyVersion", min_length=1)
    strategyDslSchemaVersion: str = PascalAlias("StrategyDslSchemaVersion", min_length=1)
    investmentPlanSchemaVersion: str = PascalAlias("InvestmentPlanSchemaVersion", min_length=1)
    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion", min_length=1)
    riskPolicyVersion: str = PascalAlias("RiskPolicyVersion", min_length=1)
    reliabilityPolicyVersion: str = PascalAlias("ReliabilityPolicyVersion", min_length=1)
    frozenAt: datetime = PascalAlias("FrozenAt")
    frozenBy: str = PascalAlias("FrozenBy", min_length=1)
    capacityPreflightPassed: bool = PascalAlias("CapacityPreflightPassed")
    preflightSeverity: str = PascalAlias("PreflightSeverity", min_length=1)

    @classmethod
    def create(cls, **values: Any) -> "CandidateFreezeV1":
        """从 Python 内部字段名构造；按 alias 转换，兼容 mypy 与 StrictModel。"""
        wireValues = {
            fieldInfo.validation_alias: values[fieldName]
            for fieldName, fieldInfo in cls.model_fields.items()
        }
        return cls.model_validate(wireValues)

    def freezeHash(self) -> str:
        """冻结清单身份哈希：任何字段变化都改变哈希。"""
        payload = self.model_dump(mode="python", by_alias=True, exclude_none=False)
        return canonicalHash(payload)

    def containsNoS0S1(self) -> bool:
        return self.preflightSeverity not in ("S0", "S1")


@dataclass(frozen=True, slots=True)
class FreezeRecordV1:
    """冻结记录：清单 + 哈希 + 状态。"""

    freeze: CandidateFreezeV1
    freezeHash: str
    state: str  # FROZEN / SUPERSEDED


class CandidateFreezeStoreV1:
    """候选版本冻结存储：只允许追加新冻结，禁止修改历史。"""

    def __init__(self) -> None:
        self._records: list[FreezeRecordV1] = []

    def freeze(self, candidate: CandidateFreezeV1) -> FreezeRecordV1:
        """冻结候选版本；容量预演未通过（S0/S1）拒绝冻结。"""
        if not candidate.capacityPreflightPassed:
            raise ValueError("容量预演未通过，禁止冻结 M2A 候选版本")
        record = FreezeRecordV1(
            freeze=candidate,
            freezeHash=candidate.freezeHash(),
            state="FROZEN",
        )
        self._records.append(record)
        return record

    def current(self) -> FreezeRecordV1 | None:
        return self._records[-1] if self._records else None

    def all(self) -> tuple[FreezeRecordV1, ...]:
        return tuple(self._records)

    def hashOf(self, freezeHash: str) -> FreezeRecordV1 | None:
        for record in self._records:
            if record.freezeHash == freezeHash:
                return record
        return None


def buildCandidateFreeze(
    *,
    codeVersion: str,
    dependencyLockHash: str,
    eventSchemaRegistryHash: str,
    frozenBy: str,
    preflight: CapacityPreflightResultV1,
    strategySandboxPolicyVersion: str = "V1",
    strategyDslSchemaVersion: str = "V1",
    investmentPlanSchemaVersion: str = "V1",
    configSchemaVersion: str = "V1",
    riskPolicyVersion: str = "V1",
    reliabilityPolicyVersion: str = "V1",
) -> CandidateFreezeV1:
    """便捷构造：从容量预演结果生成冻结清单。"""
    severity = preflight.worstSeverity().value
    return CandidateFreezeV1.create(
        codeVersion=codeVersion,
        dependencyLockHash=dependencyLockHash,
        eventSchemaRegistryHash=eventSchemaRegistryHash,
        strategySandboxPolicyVersion=strategySandboxPolicyVersion,
        strategyDslSchemaVersion=strategyDslSchemaVersion,
        investmentPlanSchemaVersion=investmentPlanSchemaVersion,
        configSchemaVersion=configSchemaVersion,
        riskPolicyVersion=riskPolicyVersion,
        reliabilityPolicyVersion=reliabilityPolicyVersion,
        frozenAt=_utcNowMillisecond(),
        frozenBy=frozenBy,
        capacityPreflightPassed=preflight.passed,
        preflightSeverity=severity,
    )
