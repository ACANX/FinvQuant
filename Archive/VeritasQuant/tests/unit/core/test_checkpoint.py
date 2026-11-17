from __future__ import annotations

import pytest

from veritasquant.core.Checkpoint import CheckpointError, EventProcessingCheckpointV1, ProjectionStoreV1
from veritasquant.core.Transaction import TransactionStoreV1


def test_discarded_projection_rebuilds_to_same_hash() -> None:
    transactions = TransactionStoreV1()
    transaction = transactions.begin()
    transaction.appendFact("Order", "a" * 64)
    transaction.appendFact("Ledger", "b" * 64)
    facts = transaction.commit()
    projections = ProjectionStoreV1(transactions)
    checkpoint = EventProcessingCheckpointV1("run-1", "group-1", facts[-1].sequence, "tx-1")
    first = projections.rebuild(checkpoint)
    projections.discardProjection()
    assert projections.rebuild(checkpoint).projectionHash == first.projectionHash


def test_checkpoint_cannot_advance_beyond_committed_facts() -> None:
    projections = ProjectionStoreV1(TransactionStoreV1())
    with pytest.raises(CheckpointError, match="超过"):
        projections.rebuild(EventProcessingCheckpointV1("run-1", "group-1", 1, "tx-1"))
