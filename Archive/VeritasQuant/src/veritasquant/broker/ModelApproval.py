"""P4-009 候选模型 A/B 回测与批准流程。

对齐 TechSpec 7.4/13 阶段 4：
- 候选先跑固定历史样本；未审批不成为默认；
- 避免同样本训练验收（训练/验证/留出/滚动样本划分）。

- `CandidateModelV1`：候选模型（参数集 + 固定历史样本集引用）；
- `AbTestRunV1`：一次 A/B 回测运行（训练/验证/留出划分）；
- `AbTestEvaluatorV1`：对比基准与候选（固定样本，避免同样本验收）；
- `ModelApprovalWorkflowV1`：批准流程（未审批不成为默认；
  审批记录含评估证据与审批人）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum

from veritasquant.broker.Calibration import CalibrationError, CandidateParameterSetV1


class ApprovalStatus(StrEnum):
    Pending = "PENDING"
    Approved = "APPROVED"
    Rejected = "REJECTED"


class SampleSplit(StrEnum):
    Train = "TRAIN"
    Validation = "VALIDATION"
    Holdout = "HOLDOUT"


@dataclass(frozen=True, slots=True)
class CandidateModelV1:
    """候选模型：参数集 + 固定历史样本集。"""

    modelId: str
    parameterSet: CandidateParameterSetV1
    sampleSetId: str  # 固定历史样本集引用
    approvalStatus: ApprovalStatus = ApprovalStatus.Pending
    approvedBy: str | None = None
    approvedAt: datetime | None = None

    def __post_init__(self) -> None:
        if not self.modelId or not self.sampleSetId:
            raise CalibrationError("候选模型标识字段不能为空")
        if self.approvalStatus is ApprovalStatus.Approved:
            if self.approvedBy is None or self.approvedAt is None:
                raise CalibrationError("已批准模型必须记录审批人与时间")


@dataclass(frozen=True, slots=True)
class AbTestRunV1:
    """一次 A/B 回测运行：训练/验证/留出划分。"""

    runId: str
    modelId: str
    baselineModelId: str
    split: SampleSplit
    metricScore: float  # 统一指标（如调整后净收益/风险调整收益）
    sampleCount: int

    def __post_init__(self) -> None:
        if not self.runId or not self.modelId or not self.baselineModelId:
            raise CalibrationError("A/B 运行标识字段不能为空")


@dataclass(frozen=True, slots=True)
class AbTestSummaryV1:
    """A/B 对比摘要。"""

    modelId: str
    baselineModelId: str
    trainScore: float | None
    validationScore: float | None
    holdoutScore: float | None
    improvementRatio: float | None  # 留出集相对基准改善

    @property
    def beatsBaseline(self) -> bool:
        return self.improvementRatio is not None and self.improvementRatio > 1.0


@dataclass(frozen=True, slots=True)
class ApprovalDecisionV1:
    """批准决策记录。"""

    modelId: str
    decision: ApprovalStatus
    approvedBy: str
    decidedAt: datetime
    evidenceSummary: str
    reason: str = ""


class AbTestEvaluatorV1:
    """固定历史样本 A/B 评估：避免同样本训练验收。"""

    def __init__(self) -> None:
        self._runs: list[AbTestRunV1] = []

    def evaluate(
        self,
        *,
        runId: str,
        modelId: str,
        baselineModelId: str,
        trainScore: float,
        validationScore: float,
        holdoutScore: float,
        baselineHoldoutScore: float,
        trainSampleCount: int,
        validationSampleCount: int,
        holdoutSampleCount: int,
    ) -> AbTestSummaryV1:
        """跑固定历史样本的三段划分；训练集成绩不作为验收依据。"""
        if min(trainSampleCount, validationSampleCount, holdoutSampleCount) <= 0:
            raise CalibrationError("样本划分必须为正")
        if baselineHoldoutScore <= 0:
            raise CalibrationError("基准留出集成绩必须为正")
        self._runs.append(
            AbTestRunV1(
                runId=runId,
                modelId=modelId,
                baselineModelId=baselineModelId,
                split=SampleSplit.Train,
                metricScore=trainScore,
                sampleCount=trainSampleCount,
            )
        )
        self._runs.append(
            AbTestRunV1(
                runId=runId,
                modelId=modelId,
                baselineModelId=baselineModelId,
                split=SampleSplit.Validation,
                metricScore=validationScore,
                sampleCount=validationSampleCount,
            )
        )
        self._runs.append(
            AbTestRunV1(
                runId=runId,
                modelId=modelId,
                baselineModelId=baselineModelId,
                split=SampleSplit.Holdout,
                metricScore=holdoutScore,
                sampleCount=holdoutSampleCount,
            )
        )
        return AbTestSummaryV1(
            modelId=modelId,
            baselineModelId=baselineModelId,
            trainScore=trainScore,
            validationScore=validationScore,
            holdoutScore=holdoutScore,
            improvementRatio=holdoutScore / baselineHoldoutScore,
        )

    def runs(self) -> tuple[AbTestRunV1, ...]:
        return tuple(self._runs)


class ModelApprovalWorkflowV1:
    """批准流程：未审批不成为默认；审批需评估证据。"""

    def __init__(self, evaluator: AbTestEvaluatorV1 | None = None) -> None:
        self._evaluator = evaluator or AbTestEvaluatorV1()
        self._models: dict[str, CandidateModelV1] = {}
        self._decisions: list[ApprovalDecisionV1] = []
        self._defaultModelId: str | None = None

    def register(self, model: CandidateModelV1) -> None:
        if model.modelId in self._models:
            raise CalibrationError(f"模型已注册: {model.modelId}")
        self._models[model.modelId] = model

    def get(self, modelId: str) -> CandidateModelV1 | None:
        return self._models.get(modelId)

    def decide(
        self,
        *,
        modelId: str,
        decision: ApprovalStatus,
        approvedBy: str,
        summary: AbTestSummaryV1 | None = None,
        reason: str = "",
    ) -> ApprovalDecisionV1:
        """批准/拒绝候选；APPROVED 需要评估证据（summary）。"""
        model = self._models.get(modelId)
        if model is None:
            raise CalibrationError(f"模型未注册: {modelId}")
        if decision is ApprovalStatus.Approved and summary is None:
            raise CalibrationError("批准必须提供 A/B 评估证据")
        if decision is ApprovalStatus.Approved and summary is not None and not summary.beatsBaseline:
            raise CalibrationError("留出集未跑赢基准，不得批准")
        approvedModel = CandidateModelV1(
            modelId=model.modelId,
            parameterSet=model.parameterSet,
            sampleSetId=model.sampleSetId,
            approvalStatus=decision,
            approvedBy=approvedBy if decision is ApprovalStatus.Approved else None,
            approvedAt=datetime.now(timezone.utc) if decision is ApprovalStatus.Approved else None,
        )
        self._models[modelId] = approvedModel
        decisionRecord = ApprovalDecisionV1(
            modelId=modelId,
            decision=decision,
            approvedBy=approvedBy,
            decidedAt=datetime.now(timezone.utc),
            evidenceSummary=(
                f"holdout={summary.holdoutScore:.4f} ratio={summary.improvementRatio:.4f}"
                if summary is not None
                else "no-evidence"
            ),
            reason=reason,
        )
        self._decisions.append(decisionRecord)
        return decisionRecord

    def setDefault(self, modelId: str) -> None:
        """设为默认执行模型；未审批不得成为默认。"""
        model = self._models.get(modelId)
        if model is None:
            raise CalibrationError(f"模型未注册: {modelId}")
        if model.approvalStatus is not ApprovalStatus.Approved:
            raise CalibrationError(f"未审批模型不得成为默认: {modelId}")
        self._defaultModelId = modelId

    @property
    def defaultModelId(self) -> str | None:
        return self._defaultModelId

    def decisions(self) -> tuple[ApprovalDecisionV1, ...]:
        return tuple(self._decisions)
