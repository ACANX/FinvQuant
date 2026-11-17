"""P5-021 每日 Go/No-Go 审核。

对齐 TechSpec 13 阶段 5 与 ISSUE #217 验收标准：
- 每日有指标快照（资金/持仓/订单/成交/限额利用率）、风险状态（告警/硬限制/对账）、
  审批人和唯一决策（GO / NO_GO）；
- 任一硬限制失败立即退回仿真（决策为 NO_GO 并给出退回原因）；
- 决策证据保留：每日记录生成时间、审批人和决策哈希，可审计追溯。

- `DailyMetricSnapshotV1`：每日指标快照（金额/订单/限额利用率，Decimal 字符串语义）；
- `RiskStateV1`：风险状态（告警/硬限制违反/对账差异）；
- `GoNoGoDecision`：唯一决策（GO / NO_GO / 证据不足）；
- `DailyGoNoGoRecordV1`：每日审核记录（快照 + 风险 + 审批人 + 决策 + 哈希）；
- `DailyGoNoGoServiceV1`：每日审核编排（硬限制失败自动退回仿真）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import date, datetime, timezone
from enum import StrEnum
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class GoNoGoDecision(StrEnum):
    Go = "GO"
    NoGo = "NO_GO"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class DailyMetricSnapshotV1:
    """每日指标快照。"""

    tradingDay: date
    accountId: str
    netAssetValue: Decimal  # 净资产（Decimal 字符串语义）
    openPositions: int  # 持仓数
    ordersToday: int  # 当日订单数
    filledToday: int  # 当日成交数
    initialFundCap: Decimal  # 批准的初始资金上限
    orderCap: Decimal  # 批准的订单上限
    navUtilizationPct: Decimal  # 资金利用率 %（nav/initialFundCap*100）
    orderUtilizationPct: Decimal  # 订单利用率 %

    def __post_init__(self) -> None:
        if self.netAssetValue < 0 or self.openPositions < 0 or self.ordersToday < 0 or self.filledToday < 0:
            raise ValueError("指标快照值不得为负")
        if self.initialFundCap <= 0 or self.orderCap <= 0:
            raise ValueError("批准上限必须为正")

    def hardLimitBreached(self) -> bool:
        """任一硬限制失败：资金/订单利用率超过 100%。"""
        return self.navUtilizationPct > Decimal("100") or self.orderUtilizationPct > Decimal("100")


@dataclass(frozen=True, slots=True)
class RiskStateV1:
    """每日风险状态。"""

    openS0S1Alerts: int  # 开放 S0/S1 告警
    hardLimitViolations: int  # 硬限制违反数
    unreconciledDifferences: int  # 未解释对账差异
    riskControlActive: bool  # 活动风险控制（保护状态）是否激活

    def __post_init__(self) -> None:
        if self.openS0S1Alerts < 0 or self.hardLimitViolations < 0 or self.unreconciledDifferences < 0:
            raise ValueError("风险计数不得为负")


@dataclass(frozen=True, slots=True)
class DailyGoNoGoRecordV1:
    """每日 Go/No-Go 审核记录。"""

    tradingDay: date
    accountId: str
    snapshot: DailyMetricSnapshotV1
    risk: RiskStateV1
    approvedBy: str  # 审批人
    decision: GoNoGoDecision
    rollbackReason: str = ""  # NO_GO 时退回仿真原因
    recordedAt: datetime = field(default_factory=_utcNowMillisecond)
    recordHash: str = ""

    def verify(self) -> bool:
        return self.computeHash() == self.recordHash

    def computeHash(self) -> str:
        payload = {
            "trading_day": self.tradingDay.isoformat(),
            "account_id": self.accountId,
            "nav": str(self.snapshot.netAssetValue),
            "open_positions": self.snapshot.openPositions,
            "orders_today": self.snapshot.ordersToday,
            "nav_utilization_pct": str(self.snapshot.navUtilizationPct),
            "order_utilization_pct": str(self.snapshot.orderUtilizationPct),
            "open_s0s1": self.risk.openS0S1Alerts,
            "hard_limit_violations": self.risk.hardLimitViolations,
            "unreconciled_differences": self.risk.unreconciledDifferences,
            "risk_control_active": self.risk.riskControlActive,
            "approved_by": self.approvedBy,
            "decision": self.decision.value,
            "rollback_reason": self.rollbackReason,
            "recorded_at": self.recordedAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()


class DailyGoNoGoServiceV1:
    """每日 Go/No-Go 编排：硬限制失败自动退回仿真。"""

    def __init__(self) -> None:
        self._records: dict[str, DailyGoNoGoRecordV1] = {}
        self._counter = 0

    def evaluate(
        self,
        *,
        snapshot: DailyMetricSnapshotV1,
        risk: RiskStateV1,
        approvedBy: str,
    ) -> DailyGoNoGoRecordV1:
        """每日审核：唯一决策 + 自动退回仿真逻辑。"""
        if not approvedBy:
            raise ValueError("每日审核必须指定审批人")

        if snapshot.hardLimitBreached() or risk.hardLimitViolations > 0:
            decision = GoNoGoDecision.NoGo
            rollbackReason = "硬限制失败：自动退回仿真"
        elif risk.openS0S1Alerts > 0 or risk.unreconciledDifferences > 0 or risk.riskControlActive:
            decision = GoNoGoDecision.NoGo
            rollbackReason = "存在开放 S0/S1 告警、未解释对账差异或活动保护控制"
        elif risk.openS0S1Alerts == 0 and risk.unreconciledDifferences == 0 and not risk.riskControlActive:
            decision = GoNoGoDecision.Go
            rollbackReason = ""
        else:  # pragma: no cover - 防御分支
            decision = GoNoGoDecision.InsufficientEvidence
            rollbackReason = "证据不足"

        self._counter += 1
        recordId = f"gono-{self._counter:04d}"
        draft = DailyGoNoGoRecordV1(
            tradingDay=snapshot.tradingDay,
            accountId=snapshot.accountId,
            snapshot=snapshot,
            risk=risk,
            approvedBy=approvedBy,
            decision=decision,
            rollbackReason=rollbackReason,
        )
        record = DailyGoNoGoRecordV1(
            tradingDay=draft.tradingDay,
            accountId=draft.accountId,
            snapshot=snapshot,
            risk=risk,
            approvedBy=approvedBy,
            decision=decision,
            rollbackReason=rollbackReason,
            recordedAt=draft.recordedAt,
            recordHash=draft.computeHash(),
        )
        self._records[recordId] = record
        return record

    def get(self, recordId: str) -> DailyGoNoGoRecordV1 | None:
        return self._records.get(recordId)

    def all(self) -> tuple[DailyGoNoGoRecordV1, ...]:
        return tuple(self._records.values())

    def verifyIntegrity(self, record: DailyGoNoGoRecordV1) -> bool:
        return record.verify()

    def countByDecision(self, decision: GoNoGoDecision) -> int:
        return sum(1 for r in self._records.values() if r.decision is decision)

    def latest(self) -> DailyGoNoGoRecordV1 | None:
        return self._records[max(self._records.keys())] if self._records else None


def buildDailySnapshot(
    *,
    tradingDay: date,
    accountId: str,
    netAssetValue: Decimal,
    openPositions: int,
    ordersToday: int,
    filledToday: int,
    initialFundCap: Decimal,
    orderCap: Decimal,
) -> DailyMetricSnapshotV1:
    """便捷构造：自动计算利用率。"""
    navUtilization = netAssetValue / initialFundCap * Decimal("100")
    orderUtilization = Decimal(ordersToday) / orderCap * Decimal("100")
    return DailyMetricSnapshotV1(
        tradingDay=tradingDay,
        accountId=accountId,
        netAssetValue=netAssetValue,
        openPositions=openPositions,
        ordersToday=ordersToday,
        filledToday=filledToday,
        initialFundCap=initialFundCap,
        orderCap=orderCap,
        navUtilizationPct=navUtilization,
        orderUtilizationPct=orderUtilization,
    )
