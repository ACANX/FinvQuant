from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.reporting.Artifacts import (
    ArtifactError,
    ArtifactType,
    RepeatableExporterV1,
    RunArtifactIndexerV1,
    sha256Bytes,
)


def test_sha256_is_stable() -> None:
    assert sha256Bytes(b"hello") == sha256Bytes(b"hello")
    assert len(sha256Bytes(b"hello")) == 64
    assert sha256Bytes(b"hello") != sha256Bytes(b"world")


def test_index_records_byte_hash_and_content_hash() -> None:
    indexer = RunArtifactIndexerV1()
    index = indexer.index(
        runId="run-1",
        artifacts={
            "events": (ArtifactType.Events, "run-1/events.bin", b"event-data"),
        },
    )
    entry = index.entryFor("events")
    assert entry.byteSha256 == sha256Bytes(b"event-data")
    assert entry.byteSize == 10
    assert len(entry.contentHash) == 64
    assert len(index.indexHash) == 64


def test_same_inputs_produce_same_index() -> None:
    indexer = RunArtifactIndexerV1()
    first = indexer.index(
        runId="run-1",
        artifacts={"events": (ArtifactType.Events, "e", b"data")},
    )
    second = indexer.index(
        runId="run-1",
        artifacts={"events": (ArtifactType.Events, "e", b"data")},
    )
    assert first.indexHash == second.indexHash


def test_verify_detects_tampering() -> None:
    indexer = RunArtifactIndexerV1()
    index = indexer.index(
        runId="run-1",
        artifacts={"events": (ArtifactType.Events, "e", b"data")},
    )
    assert indexer.verify(index, {"events": b"data"})
    assert not indexer.verify(index, {"events": b"tampered"})
    assert not indexer.verify(index, {})


def test_artifact_type_hash_is_deterministic() -> None:
    indexer = RunArtifactIndexerV1()
    index = indexer.index(
        runId="run-1",
        artifacts={
            "events-a": (ArtifactType.Events, "a", b"x"),
            "events-b": (ArtifactType.Events, "b", b"y"),
            "orders": (ArtifactType.Orders, "o", b"z"),
        },
    )
    eventsHash = index.artifactHash(ArtifactType.Events)
    assert len(eventsHash) == 64
    assert eventsHash != index.artifactHash(ArtifactType.Orders)


def test_exporter_same_input_same_checksum() -> None:
    exporter = RepeatableExporterV1()
    first = exporter.export(runId="run-1", events=({"ts": 1},), orders=({"o": 1},), metrics={"return": Decimal("0.1")})
    second = exporter.export(runId="run-1", events=({"ts": 1},), orders=({"o": 1},), metrics={"return": Decimal("0.1")})
    assert first.indexHash == second.indexHash
    assert first.entryFor("events").byteSha256 == second.entryFor("events").byteSha256


def test_exporter_different_input_different_checksum() -> None:
    exporter = RepeatableExporterV1()
    first = exporter.export(runId="run-1", events=({"ts": 1},), orders=(), metrics={})
    second = exporter.export(runId="run-1", events=({"ts": 2},), orders=(), metrics={})
    assert first.indexHash != second.indexHash


def test_unknown_artifact_id_rejected() -> None:
    indexer = RunArtifactIndexerV1()
    index = indexer.index(runId="run-1", artifacts={})
    with pytest.raises(ArtifactError, match="未知工件"):
        index.entryFor("ghost")
