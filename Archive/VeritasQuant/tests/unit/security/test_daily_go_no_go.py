"""P5-021 每日 Go/No-Go 审核测试。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.security.DailyGoNoGo import (
    DailyGoNoGoRecordV1,
    DailyGoNoGoServiceV1,
    DailyMetricSnapshotV1,
    GoNoGoDecision,
    RiskStateV1,
    buildDailySnapshot,
)


def _snapshot(
    nav: Decimal = Decimal("300000"),
    orders: int = 40,
    initialCap: Decimal = Decimal("500000"),
    orderCap: Decimal = Decimal("100"),
    day: date = date(2026, 8, 3),
    account: str = "shadow-001",
) -> DailyMetricSnapshotV1:
    return buildDailySnapshot(
        tradingDay=day, accountId=account, netAssetValue=nav, openPositions=5,
        ordersToday=orders, filledToday=orders, initialFundCap=initialCap, orderCap=orderCap,
    )


def _risk(
    alerts: int = 0,
    violations: int = 0,
    diffs: int = 0,
    control: bool = False,
) -> RiskStateV1:
    return RiskStateV1(
        openS0S1Alerts=alerts, hardLimitViolations=violations,
        unreconciledDifferences=diffs, riskControlActive=control,
    )


class TestDailyMetricSnapshot:
    def test_snapshot_utilization_calculation(self) -> None:
        snap = _snapshot()
        assert snap.navUtilizationPct == Decimal("60")
        assert snap.orderUtilizationPct == Decimal("40")

    def test_snapshot_rejects_negative(self) -> None:
        with pytest.raises(ValueError, match="不得为负"):
            DailyMetricSnapshotV1(
                tradingDay=date(2026, 8, 3), accountId="a", netAssetValue=Decimal("-1"),
                openPositions=0, ordersToday=0, filledToday=0,
                initialFundCap=Decimal("100"), orderCap=Decimal("100"),
                navUtilizationPct=Decimal("0"), orderUtilizationPct=Decimal("0"),
            )

    def test_snapshot_requires_positive_caps(self) -> None:
        with pytest.raises(ValueError, match="必须为正"):
            DailyMetricSnapshotV1(
                tradingDay=date(2026, 8, 3), accountId="a", netAssetValue=Decimal("1"),
                openPositions=0, ordersToday=0, filledToday=0,
                initialFundCap=Decimal("0"), orderCap=Decimal("100"),
                navUtilizationPct=Decimal("0"), orderUtilizationPct=Decimal("0"),
            )

    def test_hard_limit_breach(self) -> None:
        snap = _snapshot(nav=Decimal("600000"))  # 120% 利用率
        assert snap.hardLimitBreached()
        assert not _snapshot().hardLimitBreached()


class TestRiskState:
    def test_risk_rejects_negative(self) -> None:
        with pytest.raises(ValueError, match="不得为负"):
            RiskStateV1(openS0S1Alerts=-1, hardLimitViolations=0, unreconciledDifferences=0, riskControlActive=False)


class TestDailyGoNoGoService:
    def test_go_when_clean(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(snapshot=_snapshot(), risk=_risk(), approvedBy="daily-reviewer")
        assert record.decision is GoNoGoDecision.Go
        assert record.rollbackReason == ""
        assert record.verify()
        assert service.verifyIntegrity(record)
        assert service.countByDecision(GoNoGoDecision.Go) == 1

    def test_nogo_on_hard_limit_violation(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(
            snapshot=_snapshot(nav=Decimal("700000")), risk=_risk(), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.NoGo
        assert "退回仿真" in record.rollbackReason

    def test_nogo_on_risk_violation_count(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(
            snapshot=_snapshot(), risk=_risk(violations=1), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.NoGo

    def test_nogo_on_open_s0s1(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(
            snapshot=_snapshot(), risk=_risk(alerts=2), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.NoGo
        assert "S0/S1" in record.rollbackReason

    def test_nogo_on_unreconciled_differences(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(
            snapshot=_snapshot(), risk=_risk(diffs=1), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.NoGo

    def test_nogo_on_active_control(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(
            snapshot=_snapshot(), risk=_risk(control=True), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.NoGo

    def test_requires_approver(self) -> None:
        service = DailyGoNoGoServiceV1()
        with pytest.raises(ValueError, match="审批人"):
            service.evaluate(snapshot=_snapshot(), risk=_risk(), approvedBy="")

    def test_record_hash_and_tamper_detection(self) -> None:
        service = DailyGoNoGoServiceV1()
        record = service.evaluate(snapshot=_snapshot(), risk=_risk(), approvedBy="reviewer")
        tampered = DailyGoNoGoRecordV1(
            tradingDay=record.tradingDay, accountId=record.accountId, snapshot=record.snapshot,
            risk=RiskStateV1(openS0S1Alerts=0, hardLimitViolations=1, unreconciledDifferences=0, riskControlActive=False),
            approvedBy=record.approvedBy, decision=record.decision,
            rollbackReason=record.rollbackReason, recordedAt=record.recordedAt,
            recordHash=record.recordHash,
        )
        assert not service.verifyIntegrity(tampered)

    def test_latest_and_all(self) -> None:
        service = DailyGoNoGoServiceV1()
        service.evaluate(snapshot=_snapshot(), risk=_risk(), approvedBy="a")
        service.evaluate(snapshot=_snapshot(), risk=_risk(), approvedBy="b")
        assert len(service.all()) == 2
        assert service.latest().approvedBy == "b"

    def test_sequential_days_decision_history(self) -> None:
        """多日决策历史：指标快照/风险状态/审批人/唯一决策齐全。"""
        service = DailyGoNoGoServiceV1()
        days = [date(2026, 8, 1), date(2026, 8, 2), date(2026, 8, 3)]
        decisions = []
        for i, day in enumerate(days):
            risk = _risk(alerts=i)  # 第 2、3 天有告警
            record = service.evaluate(snapshot=_snapshot(day=day), risk=risk, approvedBy=f"reviewer-{i}")
            decisions.append(record.decision)
        assert decisions[0] is GoNoGoDecision.Go
        assert decisions[1] is GoNoGoDecision.NoGo
        assert decisions[2] is GoNoGoDecision.NoGo
        # 每条记录都有指标快照、风险状态、审批人和唯一决策
        for record in service.all():
            assert record.snapshot is not None
            assert record.risk is not None
            assert record.approvedBy
            assert record.decision in (GoNoGoDecision.Go, GoNoGoDecision.NoGo)
            assert record.verify()
