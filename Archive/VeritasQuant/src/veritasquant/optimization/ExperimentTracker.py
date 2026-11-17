"""P6-007a 离线优化试验追踪：训练/验证/留出隔离与可复现。

对齐 TechSpec 13（优化仅离线可复现回测）+ ISSUE #129 验收标准：
- 训练/验证/留出三段划分，隔离观察：优化只能在训练/验证段选择参数，
  批准以留出段成绩为准（观察前不得查看留出结果）；
- 试验可复现：参数、数据版本、随机种子、实现版本全部进入试验身份哈希，
  同输入必然同输出；
- 记录所有试验：每次候选评估留下不可变试验记录，支持审计追溯。

- `DatasetSplit`：训练/验证/留出三段枚举；
- `TrialStatus`：试验状态（RUNNING/COMPLETED/FAILED/REJECTED）；
- `TrialV1`：单次试验记录（参数 + 分段指标 + 身份哈希）；
- `ExperimentTrackerV1`：试验追踪器（三段隔离 + 可复现校验 + 审计）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum
from decimal import Decimal
from typing import Any

from veritasquant.core.CanonicalJson import canonicalHash


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class DatasetSplit(StrEnum):
    Training = "TRAINING"  # 训练段：参数搜索使用
    Validation = "VALIDATION"  # 验证段：模型选择/早停使用
    Holdout = "HOLDOUT"  # 留出段：仅最终批准查看


class TrialStatus(StrEnum):
    Running = "RUNNING"
    Completed = "COMPLETED"
    Failed = "FAILED"
    Rejected = "REJECTED"  # 违反 Gate/隔离被拒


@dataclass(frozen=True, slots=True)
class TrialV1:
    """单次试验记录。"""

    trialId: str
    experimentId: str
    parameters: dict[str, Any]  # 策略超参（确定性输入）
    dataVersionId: str  # 数据版本（回放确定性）
    randomSeed: int  # 随机种子
    implementationVersion: str  # 实现版本（算法/策略版本）
    trainingScore: Decimal | None  # 训练段成绩
    validationScore: Decimal | None  # 验证段成绩
    holdoutScore: Decimal | None  # 留出段成绩（批准前不可见）
    status: TrialStatus
    trialHash: str
    createdAt: datetime = field(default_factory=_utcNowMillisecond)
    notes: str = ""

    def computeHash(self) -> str:
        """试验内容哈希：只含确定性输入/输出，不含创建时间与自增 ID
        （同输入同输出可复现；trialId 仅作存储键）。"""
        payload = {
            "experiment_id": self.experimentId,
            "parameters": self.parameters,
            "data_version_id": self.dataVersionId,
            "random_seed": self.randomSeed,
            "implementation_version": self.implementationVersion,
            "training_score": None if self.trainingScore is None else str(self.trainingScore),
            "validation_score": None if self.validationScore is None else str(self.validationScore),
            "holdout_score": None if self.holdoutScore is None else str(self.holdoutScore),
            "status": self.status.value,
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()

    def verify(self) -> bool:
        return self.computeHash() == self.trialHash

    def reproducibleWith(self, *, parameters: dict[str, Any], dataVersionId: str, randomSeed: int, implementationVersion: str) -> bool:
        """同输入必然同输出：身份字段一致即视为同一试验可复现。"""
        return (
            self.parameters == parameters
            and self.dataVersionId == dataVersionId
            and self.randomSeed == randomSeed
            and self.implementationVersion == implementationVersion
        )


class ExperimentTrackerV1:
    """试验追踪器：三段隔离 + 可复现 + 留出成绩批准前锁定。"""

    def __init__(self) -> None:
        self._trials: dict[str, TrialV1] = {}
        self._counter = 0
        self._holdoutUnlocked = False  # 留出段成绩默认锁定（隔离观察）

    def createTrial(
        self,
        *,
        experimentId: str,
        parameters: dict[str, Any],
        dataVersionId: str,
        randomSeed: int,
        implementationVersion: str,
        trialId: str | None = None,
    ) -> TrialV1:
        """创建试验（RUNNING）；参数/种子/版本进入身份。"""
        if trialId is None:
            self._counter += 1
            trialId = f"trial-{self._counter:05d}"
        if trialId in self._trials:
            raise ValueError(f"试验已存在: {trialId}")
        trial = TrialV1(
            trialId=trialId,
            experimentId=experimentId,
            parameters=dict(parameters),
            dataVersionId=dataVersionId,
            randomSeed=randomSeed,
            implementationVersion=implementationVersion,
            trainingScore=None,
            validationScore=None,
            holdoutScore=None,
            status=TrialStatus.Running,
            trialHash="",
        )
        trial = TrialV1(
            trialId=trialId,
            experimentId=experimentId,
            parameters=dict(parameters),
            dataVersionId=dataVersionId,
            randomSeed=randomSeed,
            implementationVersion=implementationVersion,
            trainingScore=None,
            validationScore=None,
            holdoutScore=None,
            status=TrialStatus.Running,
            trialHash=trial.computeHash(),
            createdAt=trial.createdAt,
        )
        self._trials[trialId] = trial
        return trial

    def completeTrial(
        self,
        *,
        trialId: str,
        trainingScore: Decimal,
        validationScore: Decimal,
        holdoutScore: Decimal | None = None,
    ) -> TrialV1:
        """完成试验：记录训练/验证成绩；留出成绩仅在解锁后允许记录。"""
        trial = self._trials.get(trialId)
        if trial is None:
            raise ValueError(f"试验不存在: {trialId}")
        if trial.status is not TrialStatus.Running:
            raise ValueError(f"试验 {trialId} 不在 RUNNING 状态")
        if holdoutScore is not None and not self._holdoutUnlocked:
            raise ValueError("留出段成绩在批准前锁定：禁止在优化期间查看留出结果")
        updated = TrialV1(
            trialId=trial.trialId,
            experimentId=trial.experimentId,
            parameters=trial.parameters,
            dataVersionId=trial.dataVersionId,
            randomSeed=trial.randomSeed,
            implementationVersion=trial.implementationVersion,
            trainingScore=trainingScore,
            validationScore=validationScore,
            holdoutScore=holdoutScore,
            status=TrialStatus.Completed,
            trialHash="",
            createdAt=trial.createdAt,
            notes=trial.notes,
        )
        updated = TrialV1(
            trialId=updated.trialId,
            experimentId=updated.experimentId,
            parameters=updated.parameters,
            dataVersionId=updated.dataVersionId,
            randomSeed=updated.randomSeed,
            implementationVersion=updated.implementationVersion,
            trainingScore=trainingScore,
            validationScore=validationScore,
            holdoutScore=holdoutScore,
            status=TrialStatus.Completed,
            trialHash=updated.computeHash(),
            createdAt=updated.createdAt,
            notes=updated.notes,
        )
        self._trials[trialId] = updated
        return updated

    def failTrial(self, trialId: str, reason: str) -> TrialV1:
        """标记试验失败（不可复现/沙箱违规等）。"""
        trial = self._trials.get(trialId)
        if trial is None:
            raise ValueError(f"试验不存在: {trialId}")
        updated = TrialV1(
            trialId=trial.trialId,
            experimentId=trial.experimentId,
            parameters=trial.parameters,
            dataVersionId=trial.dataVersionId,
            randomSeed=trial.randomSeed,
            implementationVersion=trial.implementationVersion,
            trainingScore=trial.trainingScore,
            validationScore=trial.validationScore,
            holdoutScore=trial.holdoutScore,
            status=TrialStatus.Failed,
            trialHash=trial.trialHash,
            createdAt=trial.createdAt,
            notes=reason,
        )
        self._trials[trialId] = updated
        return updated

    def unlockHoldout(self) -> None:
        """批准前解锁留出段（仅最终评估流程调用）。"""
        self._holdoutUnlocked = True

    def recordHoldout(self, trialId: str, holdoutScore: Decimal) -> TrialV1:
        """记录留出段成绩（仅解锁后；记录后重新锁定防止旁路）。"""
        if not self._holdoutUnlocked:
            raise ValueError("留出段成绩在批准前锁定")
        trial = self._trials.get(trialId)
        if trial is None:
            raise ValueError(f"试验不存在: {trialId}")
        if trial.status is not TrialStatus.Completed:
            raise ValueError(f"试验 {trialId} 未完成，不能记录留出成绩")
        updated = TrialV1(
            trialId=trial.trialId,
            experimentId=trial.experimentId,
            parameters=trial.parameters,
            dataVersionId=trial.dataVersionId,
            randomSeed=trial.randomSeed,
            implementationVersion=trial.implementationVersion,
            trainingScore=trial.trainingScore,
            validationScore=trial.validationScore,
            holdoutScore=holdoutScore,
            status=trial.status,
            trialHash="",
            createdAt=trial.createdAt,
            notes=trial.notes,
        )
        updated = TrialV1(
            trialId=updated.trialId,
            experimentId=updated.experimentId,
            parameters=updated.parameters,
            dataVersionId=updated.dataVersionId,
            randomSeed=updated.randomSeed,
            implementationVersion=updated.implementationVersion,
            trainingScore=updated.trainingScore,
            validationScore=updated.validationScore,
            holdoutScore=holdoutScore,
            status=updated.status,
            trialHash=updated.computeHash(),
            createdAt=updated.createdAt,
            notes=updated.notes,
        )
        self._trials[trialId] = updated
        # 记录后重新锁定（留出段只能一次性解锁评估）
        self._holdoutUnlocked = False
        return updated

    def get(self, trialId: str) -> TrialV1 | None:
        return self._trials.get(trialId)

    def all(self) -> tuple[TrialV1, ...]:
        return tuple(self._trials.values())

    def completed(self) -> tuple[TrialV1, ...]:
        return tuple(t for t in self._trials.values() if t.status is TrialStatus.Completed)

    def bestByValidation(self) -> TrialV1 | None:
        """按验证段成绩选优（优化期间唯一允许的选优口径）。"""
        completed = [t for t in self.completed() if t.validationScore is not None]
        if not completed:
            return None
        return max(completed, key=lambda t: t.validationScore or Decimal("0"))

    def verifyIntegrity(self, trial: TrialV1) -> bool:
        return trial.verify()

    def holdoutLocked(self) -> bool:
        return not self._holdoutUnlocked
