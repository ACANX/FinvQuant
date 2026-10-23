from __future__ import annotations

import pytest

from veritasquant.core.Transaction import OutboxStatus, TransactionStoreV1


def test_transaction_commits_facts_and_outbox_atomically() -> None:
    store = TransactionStoreV1()
    transaction = store.begin()
    transaction.appendFact("OrderApproved", "a" * 64)
    transaction.enqueue("cmd-1", "orders", "b" * 64)
    transaction.commit()
    assert len(store.facts) == len(store.outbox) == 1
    assert store.outbox[0].status is OutboxStatus.Pending


def test_failed_or_rolled_back_transaction_exposes_no_partial_write_and_retries_publish() -> None:
    store = TransactionStoreV1()
    transaction = store.begin()
    transaction.appendFact("OrderApproved", "a" * 64)
    transaction.enqueue("cmd-1", "orders", "b" * 64)
    transaction.rollback()
    assert not store.facts and not store.outbox
    transaction = store.begin()
    transaction.appendFact("OrderApproved", "a" * 64)
    transaction.enqueue("cmd-1", "orders", "b" * 64)
    transaction.commit()
    with pytest.raises(RuntimeError):
        store.publishPending(lambda _: (_ for _ in ()).throw(RuntimeError("offline")))
    assert store.outbox[0].status is OutboxStatus.Pending
    delivered: list[str] = []
    assert store.publishPending(lambda message: delivered.append(message.messageId)) == 1
    assert delivered == ["cmd-1"]
    assert store.outbox[0].status is OutboxStatus.Published
