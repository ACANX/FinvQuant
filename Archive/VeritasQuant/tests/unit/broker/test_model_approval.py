"""P4-009 候选模型 A/B 回测与批准流程测试。"""

from __future__ import annotations

import pytest

from veritasquant.broker.Calibration import CalibrationError, CandidateParameterSetV1
from veritasquant.broker.ModelApproval import (
    AbTestEvaluatorV1,
    AbTestSummaryV1,
    ApprovalDecisionV1,
    ApprovalStatus,
    CandidateModelV1,
    ModelApprovalWorkflowV1,
    SampleSplit,
)


def _params(candidateId: str = "cand-001") -> CandidateParameterSetV1:
    return CandidateParameterSetV1(
        candidateId=candidateId,
        version="V1.1",
        modelType="IDEAL",
        slippageBps="2",
        fillRateMultiplier="1",
        latencyBudgetSeconds=0.5,
    )


def _model(modelId: str = "model-001") -> CandidateModelV1:
    return CandidateModelV1(
        modelId=modelId,
        parameterSet=_params(),
        sampleSetId="sampleset-hist-2026",
    )


class TestCandidateModel:
    def test_valid(self) -> None:
        model = _model()
        assert model.approvalStatus is ApprovalStatus.Pending
        assert model.approvedBy is None

    def test_approved_requires_approver(self) -> None:
        with pytest.raises(CalibrationError):
            CandidateModelV1(
                modelId="model-001",
                parameterSet=_params(),
                sampleSetId="s-1",
                approvalStatus=ApprovalStatus.Approved,
                approvedBy=None,
                approvedAt=None,
            )


class TestAbTestEvaluator:
    def test_evaluate_three_splits(self) -> None:
        evaluator = AbTestEvaluatorV1()
        summary = evaluator.evaluate(
            runId="ab-001",
            modelId="model-001",
            baselineModelId="baseline-001",
            trainScore=0.05,
            validationScore=0.06,
            holdoutScore=0.07,
            baselineHoldoutScore=0.05,
            trainSampleCount=100,
            validationSampleCount=50,
            holdoutSampleCount=50,
        )
        assert isinstance(summary, AbTestSummaryV1)
        assert summary.trainScore == 0.05
        assert summary.holdoutScore == 0.07
        assert len(evaluator.runs()) == 3
        splits = {r.split for r in evaluator.runs()}
        assert splits == {SampleSplit.Train, SampleSplit.Validation, SampleSplit.Holdout}

    def test_zero_sample_rejected(self) -> None:
        evaluator = AbTestEvaluatorV1()
        with pytest.raises(CalibrationError, match="必须为正"):
            evaluator.evaluate(
                runId="ab-001",
                modelId="model-001",
                baselineModelId="baseline-001",
                trainScore=0.05,
                validationScore=0.06,
                holdoutScore=0.07,
                baselineHoldoutScore=0.05,
                trainSampleCount=0,
                validationSampleCount=50,
                holdoutSampleCount=50,
            )


class TestModelApprovalWorkflow:
    def _approve_flow(self) -> tuple[ModelApprovalWorkflowV1, AbTestSummaryV1]:
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        summary = workflow._evaluator.evaluate(  # type: ignore[attr-defined]
            runId="ab-001",
            modelId="model-001",
            baselineModelId="baseline-001",
            trainScore=0.05,
            validationScore=0.06,
            holdoutScore=0.07,
            baselineHoldoutScore=0.05,
            trainSampleCount=100,
            validationSampleCount=50,
            holdoutSampleCount=50,
        )
        return workflow, summary

    def test_approve_with_evidence(self) -> None:
        workflow, summary = self._approve_flow()
        decision = workflow.decide(
            modelId="model-001",
            decision=ApprovalStatus.Approved,
            approvedBy="qa-owner",
            summary=summary,
        )
        assert decision.decision is ApprovalStatus.Approved
        assert isinstance(decision, ApprovalDecisionV1)
        assert workflow.get("model-001").approvalStatus is ApprovalStatus.Approved  # type: ignore[union-attr]
        assert workflow.get("model-001").approvedBy == "qa-owner"  # type: ignore[union-attr]

    def test_approve_without_evidence_rejected(self) -> None:
        """未审批不成为默认；批准必须有评估证据。"""
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        with pytest.raises(CalibrationError, match="证据"):
            workflow.decide(
                modelId="model-001",
                decision=ApprovalStatus.Approved,
                approvedBy="qa-owner",
                summary=None,
            )

    def test_approve_not_beating_baseline_rejected(self) -> None:
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        losing = AbTestSummaryV1(
            modelId="model-001",
            baselineModelId="baseline-001",
            trainScore=0.05,
            validationScore=0.03,
            holdoutScore=0.02,
            improvementRatio=0.5,
        )
        with pytest.raises(CalibrationError, match="未跑赢基准"):
            workflow.decide(
                modelId="model-001",
                decision=ApprovalStatus.Approved,
                approvedBy="qa-owner",
                summary=losing,
            )

    def test_set_default_requires_approval(self) -> None:
        """未审批不成为默认。"""
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        with pytest.raises(CalibrationError, match="未审批"):
            workflow.setDefault("model-001")

    def test_set_default_after_approval(self) -> None:
        workflow, summary = self._approve_flow()
        workflow.decide(
            modelId="model-001",
            decision=ApprovalStatus.Approved,
            approvedBy="qa-owner",
            summary=summary,
        )
        workflow.setDefault("model-001")
        assert workflow.defaultModelId == "model-001"

    def test_reject(self) -> None:
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        decision = workflow.decide(
            modelId="model-001",
            decision=ApprovalStatus.Rejected,
            approvedBy="qa-owner",
            reason="留出集回撤超限",
        )
        assert decision.decision is ApprovalStatus.Rejected
        assert len(workflow.decisions()) == 1

    def test_unknown_model_rejected(self) -> None:
        workflow = ModelApprovalWorkflowV1()
        with pytest.raises(CalibrationError, match="未注册"):
            workflow.decide(
                modelId="model-unknown",
                decision=ApprovalStatus.Rejected,
                approvedBy="qa",
            )

    def test_duplicate_registration_rejected(self) -> None:
        workflow = ModelApprovalWorkflowV1()
        workflow.register(_model())
        with pytest.raises(CalibrationError, match="已注册"):
            workflow.register(_model())
