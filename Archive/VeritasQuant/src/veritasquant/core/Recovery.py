"""崩溃注入后的事务恢复不变量验证。"""

from __future__ import annotations

from dataclasses import dataclass

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Checkpoint import EventProcessingCheckpointV1, ProjectionStoreV1
from veritasquant.core.CrashInjection import CrashInjectedError, CrashInjectorV1, CrashPoint
from veritasquant.core.Transaction import TransactionStoreV1


@dataclass(frozen=True, slots=True)
class RecoveryVerificationReportV1:
    point: CrashPoint
    randomSeed: int
    replayCount: int
    committedFactCount: int
    committedOutboxCount: int
    factsHash: str
    projectionHash: str | None
    controlHash: str | None


_RECOVERY_SEED = 20260801
_INPUT_HASH = "a" * 64
_OUTBOX_HASH = "b" * 64
_CONTROL_HASH = "c" * 64
_FACTS: tuple[tuple[CrashPoint, CrashPoint, str, str], ...] = (
    (CrashPoint.BeforeInbox, CrashPoint.AfterInbox, "Inbox", _INPUT_HASH),
    (CrashPoint.BeforeLedger, CrashPoint.AfterLedger, "Ledger", _INPUT_HASH),
    (CrashPoint.BeforeOrder, CrashPoint.AfterOrder, "Order", _INPUT_HASH),
    (CrashPoint.BeforeControl, CrashPoint.AfterControl, "Control", _CONTROL_HASH),
    (CrashPoint.BeforeCheckpoint, CrashPoint.AfterCheckpoint, "Checkpoint", _INPUT_HASH),
)


def verifyRecoveryInvariant(point: CrashPoint) -> RecoveryVerificationReportV1:
    """对单一注入点重启并重放固定输入，验证最终状态完全一致。"""
    store = TransactionStoreV1()
    replayCount = 0
    try:
        _applyWork(store, CrashInjectorV1(point))
    except CrashInjectedError:
        # 提交前崩溃未留下事实；提交后崩溃已有完整原子结果，均可安全恢复。
        if not store.facts:
            replayCount = 1
            _applyWork(store, CrashInjectorV1())
    checkpoint = EventProcessingCheckpointV1("run-1", "partition-1", store.facts[-1].sequence, "tx-1")
    projectionStore = ProjectionStoreV1(store)
    projectionHash = projectionStore.rebuild(checkpoint).projectionHash
    projectionStore.discardProjection()
    assert projectionStore.rebuild(checkpoint).projectionHash == projectionHash
    factsHash = canonicalHash([{"sequence": item.sequence, "type": item.factType, "hash": item.payloadHash} for item in store.facts])
    controlHash = next((item.payloadHash for item in store.facts if item.factType == "Control"), None)
    return RecoveryVerificationReportV1(point, _RECOVERY_SEED, replayCount, len(store.facts), len(store.outbox), factsHash, projectionHash, controlHash)


def _applyWork(store: TransactionStoreV1, injector: CrashInjectorV1) -> None:
    transaction = store.begin()
    for before, after, factType, payloadHash in _FACTS:
        injector.hit(before)
        transaction.appendFact(factType, payloadHash)
        injector.hit(after)
    injector.hit(CrashPoint.BeforeOutbox)
    transaction.enqueue("command-1", "events", _OUTBOX_HASH)
    transaction.commit()
    injector.hit(CrashPoint.AfterOutbox)
