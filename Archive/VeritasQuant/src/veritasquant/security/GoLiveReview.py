"""P5-019 上线前安全、可靠性和操作准备评审。

对齐 TechSpec 13 阶段 5 与 ISSUE #215 验收标准：
- 开放 S0/S1、未解释对账、超期高风险和阻断行动项均为 0 才允许上线；
- 覆盖安全（密钥/权限/隔离）、可靠性（备份恢复/门禁/对账）和操作准备（Runbook/演练/监控）三类检查；
- 评审结论唯一：全部强制项通过 + 非作者评审人工签署才 PASS。

- `ReviewCategory`：评审类别（安全/可靠性/操作准备）；
- `ReviewCheckV1`：单项检查（类别 + 描述 + 通过/失败/未执行）；
- `GoLiveReviewReportV1`：评审报告（唯一结论 + 报告哈希）；
- `GoLiveReviewServiceV1`：评审编排（阻断项校验 + 人工签署）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class ReviewCategory(StrEnum):
    Security = "SECURITY"  # 安全：密钥/权限/隔离
    Reliability = "RELIABILITY"  # 可靠性：备份恢复/门禁/对账
    OperationalReadiness = "OPERATIONAL_READINESS"  # 操作准备：Runbook/演练/监控


class ReviewCheckStatus(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    NotExecuted = "NOT_EXECUTED"  # 未执行 = 不通过（不静默放行）


class GoLiveDecision(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class ReviewCheckV1:
    """单项上线前检查。"""

    checkId: str
    category: ReviewCategory
    description: str
    status: ReviewCheckStatus
    evidence: str = ""  # 证据引用（测试/文档/报告）

    def __post_init__(self) -> None:
        if not self.checkId or not self.description:
            raise ValueError("检查 ID 与描述不能为空")

    def blocking(self) -> bool:
        """失败/未执行即阻断。"""
        return self.status is not ReviewCheckStatus.Pass


@dataclass(frozen=True, slots=True)
class GoLiveReviewReportV1:
    """上线前评审报告。"""

    reportId: str
    checks: tuple[ReviewCheckV1, ...]
    openS0S1: int  # 开放 S0/S1 告警数
    unreconciledDifferences: int  # 未解释对账差异数
    overdueHighRiskActions: int  # 超期高风险行动项数
    reviewedBy: str | None  # 非作者评审人工签署
    reviewedAt: datetime = field(default_factory=_utcNowMillisecond)
    decision: GoLiveDecision = GoLiveDecision.InsufficientEvidence
    reportHash: str = ""

    def uniqueConclusion(self) -> GoLiveDecision:
        """唯一结论：全部检查通过 + 阻断项为 0 + 人工签署才 PASS。"""
        if any(c.blocking() for c in self.checks):
            return GoLiveDecision.Fail
        if self.openS0S1 > 0:
            return GoLiveDecision.Fail
        if self.unreconciledDifferences > 0:
            return GoLiveDecision.Fail
        if self.overdueHighRiskActions > 0:
            return GoLiveDecision.Fail
        if self.reviewedBy is None:
            return GoLiveDecision.InsufficientEvidence
        return GoLiveDecision.Pass

    def verify(self) -> bool:
        return self.computeHash() == self.reportHash

    def computeHash(self) -> str:
        payload = {
            "report_id": self.reportId,
            "checks": [
                {
                    "check_id": c.checkId,
                    "category": c.category.value,
                    "status": c.status.value,
                }
                for c in self.checks
            ],
            "open_s0_s1": self.openS0S1,
            "unreconciled_differences": self.unreconciledDifferences,
            "overdue_high_risk_actions": self.overdueHighRiskActions,
            "reviewed_by": self.reviewedBy,
            "reviewed_at": self.reviewedAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()


class GoLiveReviewServiceV1:
    """上线前评审编排。"""

    def __init__(self) -> None:
        self._reports: dict[str, GoLiveReviewReportV1] = {}
        self._counter = 0

    def review(
        self,
        *,
        checks: tuple[ReviewCheckV1, ...],
        openS0S1: int,
        unreconciledDifferences: int,
        overdueHighRiskActions: int,
        reviewedBy: str | None,
        reportId: str | None = None,
    ) -> GoLiveReviewReportV1:
        """执行评审：三类类别必须全部覆盖；生成唯一结论与哈希。"""
        if openS0S1 < 0 or unreconciledDifferences < 0 or overdueHighRiskActions < 0:
            raise ValueError("阻断项计数不得为负")
        categories = {c.category for c in checks}
        missing = set(ReviewCategory) - categories
        if missing:
            raise ValueError(f"评审必须覆盖全部类别，缺少: {sorted(m.value for m in missing)}")
        if reportId is None:
            self._counter += 1
            reportId = f"review-{self._counter:04d}"
        if reportId in self._reports:
            raise ValueError(f"评审报告已存在: {reportId}")
        draft = GoLiveReviewReportV1(
            reportId=reportId,
            checks=checks,
            openS0S1=openS0S1,
            unreconciledDifferences=unreconciledDifferences,
            overdueHighRiskActions=overdueHighRiskActions,
            reviewedBy=reviewedBy,
        )
        decision = draft.uniqueConclusion()
        report = GoLiveReviewReportV1(
            reportId=reportId,
            checks=checks,
            openS0S1=openS0S1,
            unreconciledDifferences=unreconciledDifferences,
            overdueHighRiskActions=overdueHighRiskActions,
            reviewedBy=reviewedBy,
            reviewedAt=draft.reviewedAt,
            decision=decision,
        )
        report = GoLiveReviewReportV1(
            reportId=reportId,
            checks=checks,
            openS0S1=openS0S1,
            unreconciledDifferences=unreconciledDifferences,
            overdueHighRiskActions=overdueHighRiskActions,
            reviewedBy=reviewedBy,
            reviewedAt=draft.reviewedAt,
            decision=decision,
            reportHash=report.computeHash(),
        )
        self._reports[reportId] = report
        return report

    def get(self, reportId: str) -> GoLiveReviewReportV1 | None:
        return self._reports.get(reportId)

    def all(self) -> tuple[GoLiveReviewReportV1, ...]:
        return tuple(self._reports.values())

    def verifyIntegrity(self, report: GoLiveReviewReportV1) -> bool:
        return report.verify()

    def latest(self) -> GoLiveReviewReportV1 | None:
        return self._reports[max(self._reports.keys())] if self._reports else None


def buildStandardChecks(
    *,
    secretsRotated: bool = True,
    isolationVerified: bool = True,
    backupVerified: bool = True,
    readinessGatePassed: bool = True,
    reconciliationClean: bool = True,
    runbooksComplete: bool = True,
    drillsPassed: bool = True,
    monitoringActive: bool = True,
) -> tuple[ReviewCheckV1, ...]:
    """构建标准上线前检查清单（三类全覆盖）。"""
    return (
        # 安全
        ReviewCheckV1("SEC-001", ReviewCategory.Security, "密钥已轮换且无泄露", _status(secretsRotated), "SecretService 轮换记录"),
        ReviewCheckV1("SEC-002", ReviewCategory.Security, "环境/账户组隔离验证通过", _status(isolationVerified), "EnvironmentIsolation 测试"),
        # 可靠性
        ReviewCheckV1("REL-001", ReviewCategory.Reliability, "备份可读性自动验证通过", _status(backupVerified), "BackupReadabilityVerifier"),
        ReviewCheckV1("REL-002", ReviewCategory.Reliability, "trading-readiness 门禁全绿", _status(readinessGatePassed), "TradingReadinessGate"),
        ReviewCheckV1("REL-003", ReviewCategory.Reliability, "券商对账差异为 0", _status(reconciliationClean), "Reconciliation 报告"),
        # 操作准备
        ReviewCheckV1("OPS-001", ReviewCategory.OperationalReadiness, "六类 Runbook 齐全", _status(runbooksComplete), "RunbookRegistry"),
        ReviewCheckV1("OPS-002", ReviewCategory.OperationalReadiness, "紧急停止/断连演练通过", _status(drillsPassed), "演练报告"),
        ReviewCheckV1("OPS-003", ReviewCategory.OperationalReadiness, "监控与分页告警生效", _status(monitoringActive), "PagingService"),
    )


def _status(ok: bool) -> ReviewCheckStatus:
    return ReviewCheckStatus.Pass if ok else ReviewCheckStatus.Fail
