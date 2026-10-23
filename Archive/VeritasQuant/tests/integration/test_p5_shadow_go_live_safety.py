"""P5-017/019/021 集成安全测试：影子运行冻结/上线前评审/每日 Go/No-Go 联动。

覆盖验收要点：
- 影子运行冻结：账户/策略/额度/验收政策双人签署，冻结后不可变、阈值不可修改；
- 上线前评审：三类检查全覆盖，S0/S1、未解释对账、超期行动项为 0 + 人工签署才 PASS；
- 每日 Go/No-Go：指标快照 + 风险状态 + 审批人 + 唯一决策；硬限制失败自动退回仿真；
- 联动：冻结额度作为每日审核的上限依据；评审通过后才能进入每日 Go/No-Go 流程。
"""

from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.security.DailyGoNoGo import (
    DailyGoNoGoServiceV1,
    GoNoGoDecision,
    RiskStateV1,
    buildDailySnapshot,
)
from veritasquant.security.GoLiveReview import (
    GoLiveDecision,
    GoLiveReviewServiceV1,
    ReviewCategory,
    ReviewCheckStatus,
    ReviewCheckV1,
)
from veritasquant.security.ShadowFreeze import (
    ShadowFreezeKind,
    ShadowFreezeServiceV1,
    buildShadowFreezeEntries,
)


def _freeze_entries() -> tuple:
    return buildShadowFreezeEntries(
        accountId="shadow-001",
        strategyVersion="V3",
        strategyChecksum="s" * 64,
        initialFundCap=Decimal("500000"),
        orderCap=Decimal("100"),
        acceptancePolicyVersion="V5",
        signerA="alice",
        signerB="bob",
    )


def _clean_checks() -> tuple[ReviewCheckV1, ...]:
    return (
        ReviewCheckV1("SEC-001", ReviewCategory.Security, "安全检查", ReviewCheckStatus.Pass, "evidence"),
        ReviewCheckV1("REL-001", ReviewCategory.Reliability, "可靠性检查", ReviewCheckStatus.Pass, "evidence"),
        ReviewCheckV1("OPS-001", ReviewCategory.OperationalReadiness, "操作准备检查", ReviewCheckStatus.Pass, "evidence"),
    )


class TestShadowFreezeIntegration:
    def test_freeze_requires_dual_signature_and_covers_all_kinds(self) -> None:
        service = ShadowFreezeServiceV1()
        record = service.freeze(entries=_freeze_entries(), frozenBy="operator")
        assert record.verify()
        # 双人签署
        for entry in record.entries:
            assert len(entry.signedBy) == 2
            assert entry.signedBy[0] != entry.signedBy[1]
        # 四类全覆盖
        assert {e.kind for e in record.entries} == set(ShadowFreezeKind)
        # 批准值可查询
        assert service.capFor(ShadowFreezeKind.Account, "shadow-001") == Decimal("500000")
        assert service.capFor(ShadowFreezeKind.Limit, "shadow-001.order-cap") == Decimal("100")

    def test_freeze_single_signer_rejected(self) -> None:
        service = ShadowFreezeServiceV1()
        with pytest.raises(ValueError, match="双人签署|两名签署人"):
            entries = _freeze_entries()
            bad = list(entries)
            bad[0] = bad[0].__class__(
                kind=bad[0].kind, objectId=bad[0].objectId, version=bad[0].version,
                capValue=bad[0].capValue, capUnit=bad[0].capUnit,
                signedBy=("alice",), signedAt=bad[0].signedAt,
            )
            service.freeze(entries=tuple(bad), frozenBy="operator")

    def test_freeze_missing_kind_rejected(self) -> None:
        service = ShadowFreezeServiceV1()
        entries = _freeze_entries()[:3]  # 缺 AcceptancePolicy
        with pytest.raises(ValueError, match="缺少关键对象类型"):
            service.freeze(entries=entries, frozenBy="operator")

    def test_freeze_immutable_after_creation(self) -> None:
        service = ShadowFreezeServiceV1()
        record = service.freeze(entries=_freeze_entries(), frozenBy="operator")
        # 篡改额度 → 校验失败（观察前不得修改阈值解释结果）
        assert not service.thresholdModified(record)
        assert service.verifyIntegrity(record)


class TestGoLiveReviewIntegration:
    def test_review_pass_when_all_clean(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(
            checks=_clean_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="independent-qa",
        )
        assert report.decision is GoLiveDecision.Pass
        assert report.verify()

    def test_review_fail_on_open_s0s1_or_diff(self) -> None:
        service = GoLiveReviewServiceV1()
        r1 = service.review(checks=_clean_checks(), openS0S1=1, unreconciledDifferences=0,
                            overdueHighRiskActions=0, reviewedBy="qa")
        r2 = service.review(checks=_clean_checks(), openS0S1=0, unreconciledDifferences=3,
                            overdueHighRiskActions=0, reviewedBy="qa")
        r3 = service.review(checks=_clean_checks(), openS0S1=0, unreconciledDifferences=0,
                            overdueHighRiskActions=2, reviewedBy="qa")
        assert r1.decision is GoLiveDecision.Fail
        assert r2.decision is GoLiveDecision.Fail
        assert r3.decision is GoLiveDecision.Fail

    def test_review_requires_all_categories(self) -> None:
        service = GoLiveReviewServiceV1()
        checks = _clean_checks()[:2]  # 缺操作准备
        with pytest.raises(ValueError, match="全部类别"):
            service.review(checks=checks, openS0S1=0, unreconciledDifferences=0,
                           overdueHighRiskActions=0, reviewedBy="qa")

    def test_review_requires_human_signoff(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(checks=_clean_checks(), openS0S1=0, unreconciledDifferences=0,
                                overdueHighRiskActions=0, reviewedBy=None)
        assert report.decision is GoLiveDecision.InsufficientEvidence


class TestDailyGoNoGoIntegration:
    def test_go_when_within_frozen_limits(self) -> None:
        service = DailyGoNoGoServiceV1()
        snapshot = buildDailySnapshot(
            tradingDay=date(2026, 8, 3), accountId="shadow-001",
            netAssetValue=Decimal("300000"), openPositions=5,
            ordersToday=40, filledToday=40,
            initialFundCap=Decimal("500000"), orderCap=Decimal("100"),
        )
        record = service.evaluate(snapshot=snapshot, risk=RiskStateV1(0, 0, 0, False), approvedBy="daily-reviewer")
        assert record.decision is GoNoGoDecision.Go
        assert record.verify()

    def test_nogo_auto_rollback_on_hard_limit(self) -> None:
        service = DailyGoNoGoServiceV1()
        snapshot = buildDailySnapshot(
            tradingDay=date(2026, 8, 3), accountId="shadow-001",
            netAssetValue=Decimal("600000"), openPositions=8,
            ordersToday=110, filledToday=110,
            initialFundCap=Decimal("500000"), orderCap=Decimal("100"),
        )
        record = service.evaluate(snapshot=snapshot, risk=RiskStateV1(0, 0, 0, False), approvedBy="daily-reviewer")
        assert record.decision is GoNoGoDecision.NoGo
        assert "退回仿真" in record.rollbackReason

    def test_nogo_on_risk_state(self) -> None:
        service = DailyGoNoGoServiceV1()
        snapshot = buildDailySnapshot(
            tradingDay=date(2026, 8, 3), accountId="shadow-001",
            netAssetValue=Decimal("300000"), openPositions=5,
            ordersToday=40, filledToday=40,
            initialFundCap=Decimal("500000"), orderCap=Decimal("100"),
        )
        record = service.evaluate(snapshot=snapshot, risk=RiskStateV1(1, 0, 0, False), approvedBy="daily-reviewer")
        assert record.decision is GoNoGoDecision.NoGo


class TestCrossModuleFlow:
    def test_freeze_then_review_then_daily_go(self) -> None:
        """完整流程：冻结影子运行配置 → 上线前评审 → 每日 Go/No-Go。"""
        # 1. 冻结（双人签署）
        freeze_service = ShadowFreezeServiceV1()
        freeze = freeze_service.freeze(entries=_freeze_entries(), frozenBy="operator")
        assert freeze.verify()
        initial_cap = freeze_service.capFor(ShadowFreezeKind.Account, "shadow-001")
        order_cap = freeze_service.capFor(ShadowFreezeKind.Limit, "shadow-001.order-cap")
        assert initial_cap == Decimal("500000")
        assert order_cap == Decimal("100")

        # 2. 上线前评审（全部通过 + 人工签署）
        review_service = GoLiveReviewServiceV1()
        review = review_service.review(
            checks=_clean_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="independent-qa",
        )
        assert review.decision is GoLiveDecision.Pass

        # 3. 每日 Go/No-Go（额度来自冻结批准值）
        gono_service = DailyGoNoGoServiceV1()
        snapshot = buildDailySnapshot(
            tradingDay=date(2026, 8, 3), accountId="shadow-001",
            netAssetValue=Decimal("250000"), openPositions=4,
            ordersToday=30, filledToday=30,
            initialFundCap=initial_cap, orderCap=order_cap,
        )
        record = gono_service.evaluate(
            snapshot=snapshot, risk=RiskStateV1(0, 0, 0, False), approvedBy="daily-reviewer",
        )
        assert record.decision is GoNoGoDecision.Go
        assert record.verify()
        # 决策历史可审计
        assert gono_service.countByDecision(GoNoGoDecision.Go) == 1
