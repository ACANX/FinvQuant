from __future__ import annotations

import pytest

from veritasquant.core.Inbox import InboxDisposition, InboxError, InboxV1


def test_inbox_processes_same_key_and_hash_once() -> None:
    inbox = InboxV1()
    first = inbox.accept("run-1:group-1:event-1", "a" * 64)
    repeated = inbox.accept("run-1:group-1:event-1", "a" * 64)
    assert first.disposition is InboxDisposition.Applied
    assert repeated.disposition is InboxDisposition.Duplicate
    assert repeated.receiptSequence == first.receiptSequence


def test_inbox_quarantines_same_key_with_different_hash() -> None:
    inbox = InboxV1()
    inbox.accept("account-1:execution-1", "a" * 64)
    with pytest.raises(InboxError, match="隔离"):
        inbox.accept("account-1:execution-1", "b" * 64)
    assert inbox.conflicts[0].existingContentHash == "a" * 64
    assert inbox.conflicts[0].conflictingContentHash == "b" * 64
