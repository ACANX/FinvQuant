"""P2-041 进程崩溃与恢复演练报告（模拟盘证据窗口 3 次）。

对齐 TechSpec 12.3/13 阶段 2 验收：
- 每次演练 RTO <= 15 分钟、RPO = 0、活动控制恢复 100%、未解释对账差异 0；
- 报告必须记录：注入点、检测时间、保护动作、RTO/RPO、事实/投影哈希、
  outbox 清空时间、人工审批和唯一的通过/不通过结论；
- 演练完成后必须满 3 次有效演练（DRILL_COUNT_REQUIRED = 3）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash

# 模拟盘 RTO 目标（TechSpec 12.3 恢复目标表）
PAPER_RTO_TARGET = timedelta(minutes=15)


class DrillOutcome(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


class DrillState(StrEnum):
    Planned = "PLANNED"
    Executed = "EXECUTED"
    Approved = "APPROVED"


@dataclass(frozen=True, slots=True)
class CrashDrillReportV1:
    """一次崩溃恢复演练的完整报告。"""

    drillId: str
    injectedAt: datetime
    detectedAt: datetime
    recoveredAt: datetime
    rtoSeconds: float
    rpoSeconds: float
    injectionPoint: str  # CrashPoint 名称
    protectiveAction: str  # 保护动作（进入保护状态/停止新开仓等）
    factHash: str  # 事实序列哈希
    projectionHash: str  # 投影哈希
    outboxDrainedAt: datetime | None  # outbox 清空时间
    controlRecoveryPercent: float  # 活动 P0/P1 控制恢复完整率（0~100）
    unreconciledDifferences: int  # 恢复交易前未解释对账差异
    approvedBy: str | None  # 人工审批人
    outcome: DrillOutcome
    reportHash: str

    @property
    def rtoWithinTarget(self) -> bool:
        return self.rtoSeconds <= PAPER_RTO_TARGET.total_seconds()

    @property
    def rpoZero(self) -> bool:
        return self.rpoSeconds == 0.0

    @property
    def controlsFullyRecovered(self) -> bool:
        return self.controlRecoveryPercent == 100.0

    @property
    def differencesZero(self) -> bool:
        return self.unreconciledDifferences == 0

    def uniqueConclusion(self) -> DrillOutcome:
        """唯一结论：全部强制条件满足且有人工审批才 PASS。"""
        if not self.rtoWithinTarget:
            return DrillOutcome.Fail
        if not self.rpoZero:
            return DrillOutcome.Fail
        if not self.controlsFullyRecovered:
            return DrillOutcome.Fail
        if not self.differencesZero:
            return DrillOutcome.Fail
        if self.approvedBy is None:
            return DrillOutcome.InsufficientEvidence
        return DrillOutcome.Pass

    def assertPass(self) -> None:
        """PASS 才能计入证据窗口；否则抛出。"""
        if self.outcome is not DrillOutcome.Pass:
            raise ValueError(f"演练 {self.drillId} 结论为 {self.outcome.value}，不得计入证据窗口")


def buildCrashDrillReport(
    *,
    drillId: str,
    injectedAt: datetime,
    detectedAt: datetime,
    recoveredAt: datetime,
    injectionPoint: str,
    protectiveAction: str,
    factHash: str,
    projectionHash: str,
    outboxDrainedAt: datetime | None,
    controlRecoveryPercent: float,
    unreconciledDifferences: int,
    approvedBy: str | None,
    rpoSeconds: float = 0.0,
) -> CrashDrillReportV1:
    """构建演练报告：RTO 由时间差计算，结论自动判定。"""
    rtoSeconds = max(0.0, (recoveredAt - detectedAt).total_seconds())
    if rpoSeconds < 0.0 or controlRecoveryPercent < 0.0 or controlRecoveryPercent > 100.0:
        raise ValueError("RPO 非负、控制恢复率在 0~100")
    report = CrashDrillReportV1(
        drillId=drillId,
        injectedAt=injectedAt,
        detectedAt=detectedAt,
        recoveredAt=recoveredAt,
        rtoSeconds=rtoSeconds,
        rpoSeconds=rpoSeconds,
        injectionPoint=injectionPoint,
        protectiveAction=protectiveAction,
        factHash=factHash,
        projectionHash=projectionHash,
        outboxDrainedAt=outboxDrainedAt,
        controlRecoveryPercent=controlRecoveryPercent,
        unreconciledDifferences=unreconciledDifferences,
        approvedBy=approvedBy,
        outcome=DrillOutcome.Pass,  # 由 uniqueConclusion 最终判定
        reportHash="",
    )
    outcome = report.uniqueConclusion()
    payload = {
        "drill_id": drillId,
        "injected_at": injectedAt.isoformat(),
        "detected_at": detectedAt.isoformat(),
        "recovered_at": recoveredAt.isoformat(),
        "rto_seconds": str(rtoSeconds),
        "rpo_seconds": str(rpoSeconds),
        "injection_point": injectionPoint,
        "fact_hash": factHash,
        "projection_hash": projectionHash,
        "control_recovery_percent": str(controlRecoveryPercent),
        "unreconciled_differences": unreconciledDifferences,
        "approved_by": approvedBy,
        "outcome": outcome.value,
    }
    reportHash = canonicalHash(payload)
    return CrashDrillReportV1(
        drillId=drillId,
        injectedAt=injectedAt,
        detectedAt=detectedAt,
        recoveredAt=recoveredAt,
        rtoSeconds=rtoSeconds,
        rpoSeconds=rpoSeconds,
        injectionPoint=injectionPoint,
        protectiveAction=protectiveAction,
        factHash=factHash,
        projectionHash=projectionHash,
        outboxDrainedAt=outboxDrainedAt,
        controlRecoveryPercent=controlRecoveryPercent,
        unreconciledDifferences=unreconciledDifferences,
        approvedBy=approvedBy,
        outcome=outcome,
        reportHash=reportHash,
    )


class CrashDrillEvidenceWindowV1:
    """证据窗口演练登记：必须累计 3 次 PASS 演练。"""

    DRILL_COUNT_REQUIRED = 3

    def __init__(self) -> None:
        self._drills: list[CrashDrillReportV1] = []

    def register(self, report: CrashDrillReportV1) -> None:
        """登记演练；非 PASS 不累计。"""
        report.assertPass()
        self._drills.append(report)

    def passedCount(self) -> int:
        return len(self._drills)

    def windowComplete(self) -> bool:
        return self.passedCount() >= self.DRILL_COUNT_REQUIRED

    def missingCount(self) -> int:
        return max(0, self.DRILL_COUNT_REQUIRED - self.passedCount())

    def drills(self) -> tuple[CrashDrillReportV1, ...]:
        return tuple(self._drills)
