"""StageGatePolicyVersion 冻结与策略验收政策（P1-075）。

在任何晋级观察窗口前冻结：样本、阈值、统计方法、随机种子、数据集、
观察窗口中断规则、签署人及 PASS/FAIL/INSUFFICIENT_EVIDENCE 判定。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash


class StageGatePolicyError(ValueError):
    """策略验收政策违反冻结契约时抛出。"""


class GateVerdict(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class StrategyAcceptancePolicyV1:
    """冻结的策略验收政策。"""

    policyVersion: str
    sampleMonths: int
    minimumClosedTrades: int
    bootstrapSeed: int
    bootstrapPercentile: Decimal
    netReturnLowerBound: Decimal
    feeSlippageStressMultiple: Decimal
    maxDrawdownLimit: Decimal
    datasetRef: str
    statisticalMethod: str
    windowInterruptionRule: str
    signer: str

    def policyHash(self) -> str:
        """冻结政策哈希；任何参数变动改变身份。"""
        return canonicalHash(
            {
                "policy_version": self.policyVersion,
                "sample_months": self.sampleMonths,
                "minimum_closed_trades": self.minimumClosedTrades,
                "bootstrap_seed": self.bootstrapSeed,
                "bootstrap_percentile": self.bootstrapPercentile,
                "net_return_lower_bound": self.netReturnLowerBound,
                "fee_slippage_stress_multiple": self.feeSlippageStressMultiple,
                "max_drawdown_limit": self.maxDrawdownLimit,
                "dataset_ref": self.datasetRef,
                "statistical_method": self.statisticalMethod,
                "window_interruption_rule": self.windowInterruptionRule,
                "signer": self.signer,
            }
        )


@dataclass(frozen=True, slots=True)
class FrozenPolicyRecordV1:
    """冻结记录：版本、哈希、冻结时间与签署人。"""

    policyVersion: str
    policyHash: str
    frozenAt: datetime
    signer: str


class StageGatePolicyStoreV1:
    """冻结 StageGatePolicyVersion；冻结后禁止修改。"""

    def __init__(self) -> None:
        self._frozen: dict[str, FrozenPolicyRecordV1] = {}
        self._policies: dict[str, StrategyAcceptancePolicyV1] = {}

    def freeze(self, policy: StrategyAcceptancePolicyV1) -> FrozenPolicyRecordV1:
        """冻结政策；同版本重复冻结要求哈希一致。"""
        if not policy.policyVersion or not policy.signer:
            raise StageGatePolicyError("政策版本与签署人不能为空")
        self._validatePolicy(policy)
        policyHash = policy.policyHash()
        existing = self._frozen.get(policy.policyVersion)
        if existing is not None:
            if existing.policyHash != policyHash:
                raise StageGatePolicyError("已冻结政策不得修改参数")
            return existing
        record = FrozenPolicyRecordV1(
            policyVersion=policy.policyVersion,
            policyHash=policyHash,
            frozenAt=datetime.now(timezone.utc).replace(microsecond=0),
            signer=policy.signer,
        )
        self._frozen[policy.policyVersion] = record
        self._policies[policy.policyVersion] = policy
        return record

    def frozen(self, policyVersion: str) -> FrozenPolicyRecordV1:
        """查询冻结记录。"""
        record = self._frozen.get(policyVersion)
        if record is None:
            raise StageGatePolicyError("政策尚未冻结")
        return record

    @property
    def frozenVersions(self) -> tuple[str, ...]:
        return tuple(sorted(self._frozen))

    def evaluate(
        self,
        *,
        policyVersion: str,
        netReturn: Decimal,
        maxDrawdown: Decimal,
        closedTrades: int,
    ) -> GateVerdict:
        """按冻结政策给出 PASS/FAIL/INSUFFICIENT_EVIDENCE 判定。"""
        record = self.frozen(policyVersion)
        policy = self._policies[policyVersion]
        # 哈希核验：冻结记录与当前政策参数必须一致
        if policy.policyHash() != record.policyHash:
            raise StageGatePolicyError("冻结政策参数与记录不一致")
        if closedTrades < policy.minimumClosedTrades:
            return GateVerdict.InsufficientEvidence
        if netReturn < policy.netReturnLowerBound:
            return GateVerdict.Fail
        if maxDrawdown > policy.maxDrawdownLimit:
            return GateVerdict.Fail
        return GateVerdict.Pass

    def _validatePolicy(self, policy: StrategyAcceptancePolicyV1) -> None:
        if policy.sampleMonths < 1 or policy.minimumClosedTrades < 1:
            raise StageGatePolicyError("样本月数与最少平仓笔数必须为正")
        if not Decimal("0") < policy.bootstrapPercentile < Decimal("1"):
            raise StageGatePolicyError("bootstrap 分位数必须在 0..1 之间")
        if not policy.datasetRef or not policy.statisticalMethod or not policy.windowInterruptionRule:
            raise StageGatePolicyError("数据集、统计方法与窗口中断规则必须明确")


_DEFAULT_POLICY = StrategyAcceptancePolicyV1(
    policyVersion="StageGatePolicyVersion-1",
    sampleMonths=24,
    minimumClosedTrades=100,
    bootstrapSeed=20260802,
    bootstrapPercentile=Decimal("0.95"),
    netReturnLowerBound=Decimal("0"),
    feeSlippageStressMultiple=Decimal("2"),
    maxDrawdownLimit=Decimal("0.30"),
    datasetRef="Docs/VeritasQuantDevelopmentPlan.md 13 章样本",
    statisticalMethod="固定种子 bootstrap 95% 下界",
    windowInterruptionRule="窗口中断则重置观察窗口并重新计算",
    signer="ACANX",
)
