"""P5-019 上线前安全、可靠性和操作准备评审测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.GoLiveReview import (
    GoLiveDecision,
    GoLiveReviewReportV1,
    GoLiveReviewServiceV1,
    ReviewCategory,
    ReviewCheckStatus,
    ReviewCheckV1,
    buildStandardChecks,
)


def _check(category: ReviewCategory = ReviewCategory.Security, status: ReviewCheckStatus = ReviewCheckStatus.Pass, checkId: str = "c1") -> ReviewCheckV1:
    return ReviewCheckV1(checkId=checkId, category=category, description="检查项", status=status)


def _full_checks(all_pass: bool = True) -> tuple[ReviewCheckV1, ...]:
    status = ReviewCheckStatus.Pass if all_pass else ReviewCheckStatus.Fail
    return (
        _check(ReviewCategory.Security, status, "SEC-001"),
        _check(ReviewCategory.Reliability, status, "REL-001"),
        _check(ReviewCategory.OperationalReadiness, status, "OPS-001"),
    )


class TestReviewCheck:
    def test_check_valid(self) -> None:
        check = _check()
        assert not check.blocking()

    def test_check_fail_blocking(self) -> None:
        check = _check(status=ReviewCheckStatus.Fail)
        assert check.blocking()

    def test_check_not_executed_blocking(self) -> None:
        check = _check(status=ReviewCheckStatus.NotExecuted)
        assert check.blocking()

    def test_check_requires_fields(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            ReviewCheckV1(checkId="", category=ReviewCategory.Security, description="x", status=ReviewCheckStatus.Pass)


class TestGoLiveReviewReport:
    def test_unique_conclusion_pass(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.uniqueConclusion() is GoLiveDecision.Pass

    def test_unique_conclusion_fail_on_check(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(all_pass=False), openS0S1=0,
            unreconciledDifferences=0, overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.uniqueConclusion() is GoLiveDecision.Fail

    def test_unique_conclusion_fail_on_open_s0s1(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=1,
            unreconciledDifferences=0, overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.uniqueConclusion() is GoLiveDecision.Fail

    def test_unique_conclusion_fail_on_differences(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=0,
            unreconciledDifferences=2, overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.uniqueConclusion() is GoLiveDecision.Fail

    def test_unique_conclusion_fail_on_overdue(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=0,
            unreconciledDifferences=0, overdueHighRiskActions=1, reviewedBy="reviewer",
        )
        assert report.uniqueConclusion() is GoLiveDecision.Fail

    def test_unique_conclusion_insufficient_without_reviewer(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=0,
            unreconciledDifferences=0, overdueHighRiskActions=0, reviewedBy=None,
        )
        assert report.uniqueConclusion() is GoLiveDecision.InsufficientEvidence

    def test_hash_verify(self) -> None:
        report = GoLiveReviewReportV1(
            reportId="r1", checks=_full_checks(), openS0S1=0,
            unreconciledDifferences=0, overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.computeHash() == report.computeHash()


class TestGoLiveReviewService:
    def test_review_success(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(
            checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="independent-qa",
        )
        assert report.decision is GoLiveDecision.Pass
        assert report.verify()
        assert service.verifyIntegrity(report)
        assert service.latest() is not None

    def test_review_requires_all_categories(self) -> None:
        service = GoLiveReviewServiceV1()
        checks = (_check(ReviewCategory.Security), _check(ReviewCategory.Reliability))
        with pytest.raises(ValueError, match="全部类别"):
            service.review(checks=checks, openS0S1=0, unreconciledDifferences=0,
                           overdueHighRiskActions=0, reviewedBy="reviewer")

    def test_review_fail_on_open_s0s1(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(
            checks=_full_checks(), openS0S1=3, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        assert report.decision is GoLiveDecision.Fail

    def test_review_insufficient_without_reviewer(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(
            checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy=None,
        )
        assert report.decision is GoLiveDecision.InsufficientEvidence

    def test_review_duplicate_rejected(self) -> None:
        service = GoLiveReviewServiceV1()
        service.review(checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
                       overdueHighRiskActions=0, reviewedBy="reviewer", reportId="r1")
        with pytest.raises(ValueError, match="已存在"):
            service.review(checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
                           overdueHighRiskActions=0, reviewedBy="reviewer", reportId="r1")

    def test_review_rejects_negative_counts(self) -> None:
        service = GoLiveReviewServiceV1()
        with pytest.raises(ValueError, match="不得为负"):
            service.review(checks=_full_checks(), openS0S1=-1, unreconciledDifferences=0,
                           overdueHighRiskActions=0, reviewedBy="reviewer")

    def test_tamper_detected(self) -> None:
        service = GoLiveReviewServiceV1()
        report = service.review(
            checks=_full_checks(), openS0S1=0, unreconciledDifferences=0,
            overdueHighRiskActions=0, reviewedBy="reviewer",
        )
        tampered = GoLiveReviewReportV1(
            reportId=report.reportId, checks=_full_checks(), openS0S1=0,
            unreconciledDifferences=5, overdueHighRiskActions=0, reviewedBy=report.reviewedBy,
            reviewedAt=report.reviewedAt, decision=report.decision, reportHash=report.reportHash,
        )
        assert not service.verifyIntegrity(tampered)


class TestBuildStandardChecks:
    def test_build_covers_three_categories(self) -> None:
        checks = buildStandardChecks()
        assert {c.category for c in checks} == set(ReviewCategory)
        assert len(checks) == 8

    def test_build_with_failures(self) -> None:
        checks = buildStandardChecks(secretsRotated=False, drillsPassed=False)
        failed = [c for c in checks if c.status is ReviewCheckStatus.Fail]
        assert len(failed) == 2
        assert {c.checkId for c in failed} == {"SEC-001", "OPS-002"}
