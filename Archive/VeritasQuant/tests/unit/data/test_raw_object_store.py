from __future__ import annotations

import hashlib
from pathlib import Path

import pytest

from veritasquant.data.RawObjectStore import RawObjectStoreError, RawObjectStoreV1


def test_content_addressed_store_deduplicates_and_preserves_source_reference(tmp_path: Path) -> None:
    store = RawObjectStoreV1(tmp_path / "raw")
    first = store.storeBytes(b"market-data", "Vendor/Nvda.mvsv")
    second = store.storeBytes(b"market-data", "Vendor/Nvda-revision.mvsv")
    assert first.contentHash == second.contentHash == hashlib.sha256(b"market-data").hexdigest()
    assert first.sourceRelativePath != second.sourceRelativePath
    assert store.readBytes(first.contentHash) == b"market-data"
    assert len(list((tmp_path / "raw" / "sha256").rglob("*"))) == 2


def test_content_addressed_store_rejects_traversal_and_corrupted_existing_object(tmp_path: Path) -> None:
    store = RawObjectStoreV1(tmp_path / "raw")
    with pytest.raises(RawObjectStoreError, match="上级目录"):
        store.storeBytes(b"x", "../outside.mvsv")
    reference = store.storeBytes(b"stable", "source.mvsv")
    objectPath = tmp_path / "raw" / "sha256" / reference.contentHash[:2] / reference.contentHash
    objectPath.chmod(0o666)
    objectPath.write_bytes(b"tampered")
    with pytest.raises(RawObjectStoreError, match="篡改"):
        store.storeBytes(b"stable", "source.mvsv")
