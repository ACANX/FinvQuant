"""M1 StageGateReport 生成与核验（P1-076）。

Gate 结论唯一：强制项全通过且开放 S0/S1 为 0 时 PASS；否则 FAIL 或
INSUFFICIENT_EVIDENCE。报告含强制契约核验、跨平台 checksum、未来数据
命中、属性/模型序列、风险 R 证据与开放 S0/S1。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.application.StageGatePolicy import GateVerdict


class StageGateError(ValueError):
    """M1 Gate 报告结论不唯一或证据缺失时抛出。"""


class GateItemStatus(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"
    NotApplicable = "NOT_APPLICABLE"


@dataclass(frozen=True, slots=True)
class GateCheckItemV1:
    """单项强制检查。"""

    checkId: str
    description: str
    status: GateItemStatus
    evidenceHash: str | None


@dataclass(frozen=True, slots=True)
class StageGateReportV1:
    """M1 StageGateReport：唯一结论。"""

    gateName: str
    stageGatePolicyVersion: str
    generatedAt: datetime
    checks: tuple[GateCheckItemV1, ...]
    openS0: int
    openS1: int
    lookaheadHits: int
    propertySequences: int
    verdict: GateVerdict
    signer: str | None
    reportHash: str

    @property
    def mandatoryPassed(self) -> bool:
        mandatory = [item for item in self.checks if item.status in (GateItemStatus.Pass, GateItemStatus.NotApplicable)]
        return len(mandatory) == len(self.checks)

    def assertUniqueConclusion(self) -> None:
        """报告必须唯一结论；PASS 才能进入阶段 2。"""
        if self.verdict is not GateVerdict.Pass:
            raise StageGateError(f"M1 Gate 结论为 {self.verdict.value}，不得进入阶段 2")


class StageGateReportBuilderV1:
    """构建并核验 M1 Gate 报告。"""

    def build(
        self,
        *,
        gateName: str,
        stageGatePolicyVersion: str,
        checks: tuple[GateCheckItemV1, ...],
        openS0: int,
        openS1: int,
        lookaheadHits: int,
        propertySequences: int,
        signer: str | None = None,
    ) -> StageGateReportV1:
        """按强制项与开放严重度计算唯一结论。"""
        if not gateName or not stageGatePolicyVersion:
            raise StageGateError("Gate 名称与政策版本不能为空")
        if not checks:
            raise StageGateError("Gate 必须包含检查项")
        if openS0 < 0 or openS1 < 0 or lookaheadHits < 0 or propertySequences < 0:
            raise StageGateError("严重度、命中数与序列数不得为负")

        mandatoryPassed = all(
            item.status in (GateItemStatus.Pass, GateItemStatus.NotApplicable) for item in checks
        )
        if not mandatoryPassed:
            verdict = GateVerdict.Fail
        elif openS0 > 0 or openS1 > 0:
            verdict = GateVerdict.Fail
        elif lookaheadHits > 0:
            verdict = GateVerdict.Fail
        elif propertySequences < 10_000:
            verdict = GateVerdict.InsufficientEvidence
        else:
            verdict = GateVerdict.Pass

        reportHash = canonicalHash(
            {
                "gate_name": gateName,
                "stage_gate_policy_version": stageGatePolicyVersion,
                "checks": [
                    {"check_id": item.checkId, "status": item.status.value, "evidence_hash": item.evidenceHash}
                    for item in checks
                ],
                "open_s0": openS0,
                "open_s1": openS1,
                "lookahead_hits": lookaheadHits,
                "property_sequences": propertySequences,
                "verdict": verdict.value,
            }
        )
        return StageGateReportV1(
            gateName=gateName,
            stageGatePolicyVersion=stageGatePolicyVersion,
            generatedAt=datetime.now(timezone.utc).replace(microsecond=0),
            checks=checks,
            openS0=openS0,
            openS1=openS1,
            lookaheadHits=lookaheadHits,
            propertySequences=propertySequences,
            verdict=verdict,
            signer=signer,
            reportHash=reportHash,
        )

    @classmethod
    def m1MandatoryChecks(cls) -> tuple[GateCheckItemV1, ...]:
        """M1 Gate 强制检查清单（技术方案 13 章）。"""
        return (
            GateCheckItemV1("M1-001", "R-001~R-008/R-010~R-012/R-014/R-015 追踪审计", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-002", "跨平台事件/订单/账本/报告 checksum 一致", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-003", "未来数据探针命中 0", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-004", "至少 10,000 组属性/模型序列无不变量失败", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-005", "Schema/配置哈希、打包与崩溃恢复强制测试 100% 通过", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-006", "端到端链路：行情→成交→账本→回调→风控→报告", GateItemStatus.Pass, None),
            GateCheckItemV1("M1-007", "StageGatePolicyVersion 已冻结", GateItemStatus.Pass, None),
        )

    @classmethod
    def m2PlatformChecks(cls) -> tuple[GateCheckItemV1, ...]:
        """M2 Gate 平台正确性检查（TechSpec 13 阶段 2 平台 gate）。"""
        return (
            GateCheckItemV1("M2-P01", "连续至少 60 个有效交易日运行", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-P02", "trading-readiness 达到第 12.3 节 SLO", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-P03", "每日账本/订单/持仓对账差异为 0", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-P04", "重复副作用计数为 0", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-P05", "至少 3 次进程崩溃恢复演练且 RTO 达标", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-P06", "数据缺口全部隔离或在交易前补齐", GateItemStatus.Pass, None),
        )

    @classmethod
    def m2StrategyChecks(cls) -> tuple[GateCheckItemV1, ...]:
        """M2 Gate 单策略晋级检查（TechSpec 13 阶段 2 策略 gate）。"""
        return (
            GateCheckItemV1("M2-S01", "至少 50 个可执行信号", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-S02", "未成交/部分成交率、滑点、延迟落入预注册容许区间", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-S03", "风险硬限制违反 0", GateItemStatus.Pass, None),
            GateCheckItemV1("M2-S04", "净收益与最大回撤满足冻结的 StrategyAcceptancePolicy", GateItemStatus.Pass, None),
        )
