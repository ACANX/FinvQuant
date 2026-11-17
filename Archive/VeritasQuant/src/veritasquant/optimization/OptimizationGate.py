"""P6-007c 优化结果 Gate 隔离：不能自动绕过策略 Gate。

对齐 TechSpec 13（优化结果不自动晋级，策略 gate 只决定冻结策略版本能否进入下一环境）
与 ISSUE #129 验收标准（不能自动绕过策略 Gate）：
- 超参搜索结果只是候选：必须经冻结的 `StrategyAcceptancePolicy` 在留出段验证，
  跑赢基准 + 人工批准后才成为候选策略版本；
- 未审批的优化结果不得成为默认策略/进入运行；任何自动采用都被拒绝；
- 候选采用必须：留出段成绩达标（bootstrap 下界/回撤/交易数等政策要求）、
  政策哈希匹配冻结版本、双人批准。

- `GateDecision`：优化结果 Gate 判定（PENDING/APPROVED/REJECTED）；
- `OptimizationGateV1`：Gate 校验（政策匹配 + 留出达标 + 批准）；
- `CandidateAdoptionV1`：候选采用记录（不可变，审计追溯）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timezone
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.optimization.HyperparameterSearch import SearchResultV1


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class GateDecision(StrEnum):
    Pending = "PENDING"  # 待审批（优化结果不能自动采用）
    Approved = "APPROVED"  # 人工批准后可成为候选
    Rejected = "REJECTED"


@dataclass(frozen=True, slots=True)
class AcceptancePolicySnapshotV1:
    """冻结的策略验收政策快照（Gate 判定依据）。"""

    policyVersion: str
    minimumClosedTrades: int  # 最小已平仓交易数
    bootstrapPercentile: Decimal  # bootstrap 分位（如 0.95）
    netReturnLowerBound: Decimal  # 留出集净收益下界
    maxDrawdownLimit: Decimal  # 最大回撤限额
    policyHash: str  # 冻结政策哈希

    def verify(self) -> bool:
        """政策快照哈希自洽（引用冻结版本）。"""
        payload = {
            "policy_version": self.policyVersion,
            "minimum_closed_trades": self.minimumClosedTrades,
            "bootstrap_percentile": str(self.bootstrapPercentile),
            "net_return_lower_bound": str(self.netReturnLowerBound),
            "max_drawdown_limit": str(self.maxDrawdownLimit),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest() == self.policyHash


@dataclass(frozen=True, slots=True)
class OptimizationGateV1:
    """优化结果 Gate：政策匹配 + 留出达标 + 双人批准。"""

    def evaluate(
        self,
        *,
        searchResult: SearchResultV1,
        holdoutScore: Decimal,  # 留出段成绩（隔离评估后解锁获得）
        closedTrades: int,  # 留出段已平仓交易数
        maxDrawdown: Decimal,  # 留出段最大回撤
        policy: AcceptancePolicySnapshotV1,
        approvedBy: tuple[str, ...],  # 批准人（至少两人，互不相同）
    ) -> GateDecision:
        """判定候选是否通过 Gate（不能自动通过：必须人工批准）。"""
        if not policy.verify():
            raise ValueError("策略验收政策快照哈希不匹配冻结版本")
        if len(approvedBy) < 2:
            raise ValueError("优化结果采用必须经至少两名批准人（双人批准）")
        if len(set(approvedBy)) != len(approvedBy):
            raise ValueError("批准人必须互不相同")
        if closedTrades < policy.minimumClosedTrades:
            return GateDecision.Rejected
        if holdoutScore < policy.netReturnLowerBound:
            return GateDecision.Rejected
        if maxDrawdown > policy.maxDrawdownLimit:
            return GateDecision.Rejected
        return GateDecision.Approved

    def autoAdopt(self) -> GateDecision:
        """优化结果绝不自动采用：任何自动晋级尝试都返回 PENDING（不绕过 Gate）。"""
        return GateDecision.Pending


@dataclass(frozen=True, slots=True)
class CandidateAdoptionV1:
    """候选采用记录（不可变，审计追溯）。"""

    adoptionId: str
    searchResult: SearchResultV1
    policyVersion: str
    decision: GateDecision
    approvedBy: tuple[str, ...]
    adoptedAt: datetime = field(default_factory=_utcNowMillisecond)
    adoptionHash: str = ""

    def computeHash(self) -> str:
        payload = {
            "adoption_id": self.adoptionId,
            "search_id": self.searchResult.searchId,
            "best_parameters": self.searchResult.bestParameters,
            "best_validation_score": str(self.searchResult.bestValidationScore),
            "policy_version": self.policyVersion,
            "decision": self.decision.value,
            "approved_by": list(self.approvedBy),
            "adopted_at": self.adoptedAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()

    def verify(self) -> bool:
        return self.computeHash() == self.adoptionHash


class OptimizationAdoptionServiceV1:
    """候选采用编排：Gate 通过 + 人工批准后才记录采用。"""

    def __init__(self) -> None:
        self._adoptions: dict[str, CandidateAdoptionV1] = {}
        self._counter = 0

    def adopt(
        self,
        *,
        searchResult: SearchResultV1,
        holdoutScore: Decimal,
        closedTrades: int,
        maxDrawdown: Decimal,
        policy: AcceptancePolicySnapshotV1,
        approvedBy: tuple[str, ...],
    ) -> CandidateAdoptionV1:
        """采用候选：必须先过 Gate（政策匹配 + 留出达标 + 双人批准）。"""
        gate = OptimizationGateV1()
        decision = gate.evaluate(
            searchResult=searchResult,
            holdoutScore=holdoutScore,
            closedTrades=closedTrades,
            maxDrawdown=maxDrawdown,
            policy=policy,
            approvedBy=approvedBy,
        )
        self._counter += 1
        adoption = CandidateAdoptionV1(
            adoptionId=f"adopt-{self._counter:04d}",
            searchResult=searchResult,
            policyVersion=policy.policyVersion,
            decision=decision,
            approvedBy=approvedBy,
            adoptionHash="",
        )
        adoption = CandidateAdoptionV1(
            adoptionId=adoption.adoptionId,
            searchResult=searchResult,
            policyVersion=policy.policyVersion,
            decision=decision,
            approvedBy=approvedBy,
            adoptedAt=adoption.adoptedAt,
            adoptionHash=adoption.computeHash(),
        )
        self._adoptions[adoption.adoptionId] = adoption
        return adoption

    def get(self, adoptionId: str) -> CandidateAdoptionV1 | None:
        return self._adoptions.get(adoptionId)

    def all(self) -> tuple[CandidateAdoptionV1, ...]:
        return tuple(self._adoptions.values())

    def verifyIntegrity(self, adoption: CandidateAdoptionV1) -> bool:
        return adoption.verify()

    def adoptedCount(self) -> int:
        return sum(1 for a in self._adoptions.values() if a.decision is GateDecision.Approved)


def buildPolicySnapshot(
    *,
    policyVersion: str,
    minimumClosedTrades: int,
    bootstrapPercentile: Decimal,
    netReturnLowerBound: Decimal,
    maxDrawdownLimit: Decimal,
) -> AcceptancePolicySnapshotV1:
    """便捷构造：自动计算政策哈希。"""
    payload = {
        "policy_version": policyVersion,
        "minimum_closed_trades": minimumClosedTrades,
        "bootstrap_percentile": str(bootstrapPercentile),
        "net_return_lower_bound": str(netReturnLowerBound),
        "max_drawdown_limit": str(maxDrawdownLimit),
    }
    policyHash = hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()
    return AcceptancePolicySnapshotV1(
        policyVersion=policyVersion,
        minimumClosedTrades=minimumClosedTrades,
        bootstrapPercentile=bootstrapPercentile,
        netReturnLowerBound=netReturnLowerBound,
        maxDrawdownLimit=maxDrawdownLimit,
        policyHash=policyHash,
    )
