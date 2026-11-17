"""基于不可变事实的 checkpoint、快照和投影重建。"""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Transaction import DomainFactV1, TransactionStoreV1


class CheckpointError(ValueError):
    """checkpoint 或快照不满足已提交事实边界。"""


@dataclass(frozen=True, slots=True)
class EventProcessingCheckpointV1:
    runId: str
    partitionId: str
    lastCommittedSequence: int
    transactionId: str


@dataclass(frozen=True, slots=True)
class ProjectionSnapshotV1:
    snapshotId: str
    lastFactSequence: int
    checkpointSequence: int
    projectionHash: str


class ProjectionStoreV1:
    """投影可随时删除并由已提交事实序列确定性重建。"""

    def __init__(self, transactionStore: TransactionStoreV1) -> None:
        self._transactionStore = transactionStore
        self._projection: dict[str, str] = {}

    def rebuild(self, checkpoint: EventProcessingCheckpointV1) -> ProjectionSnapshotV1:
        if not checkpoint.runId or not checkpoint.partitionId or not checkpoint.transactionId:
            raise CheckpointError("checkpoint 必须包含运行、分区和事务 ID")
        facts = self._transactionStore.facts
        maximum = max((fact.sequence for fact in facts), default=0)
        if checkpoint.lastCommittedSequence > maximum:
            raise CheckpointError("checkpoint 不得超过已提交事实序列")
        self._projection = _reduce(fact for fact in facts if fact.sequence <= checkpoint.lastCommittedSequence)
        return ProjectionSnapshotV1(
            snapshotId=f"snapshot-{checkpoint.lastCommittedSequence}",
            lastFactSequence=checkpoint.lastCommittedSequence,
            checkpointSequence=checkpoint.lastCommittedSequence,
            projectionHash=canonicalHash(self._projection),
        )

    def discardProjection(self) -> None:
        self._projection.clear()


def _reduce(facts: Iterable[DomainFactV1]) -> dict[str, str]:
    projection: dict[str, str] = {}
    for fact in facts:
        projection[f"{fact.sequence}:{fact.factType}"] = fact.payloadHash
    return projection
