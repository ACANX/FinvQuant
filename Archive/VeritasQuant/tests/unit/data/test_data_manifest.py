"""P1-020 DataManifestV1 数据版本哈希与修订链验证。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest
from pydantic import ValidationError

from veritasquant.data.DataManifest import (
    DataManifestFileV1,
    DataManifestV1,
    SignedDataManifestV1,
    signManifest,
)

ZERO_HASH = "0" * 64


def _file(path: str = "MinuteBarV1/SSE/518880/Year=2026/Month=08/abc.parquet") -> DataManifestFileV1:
    return DataManifestFileV1.model_validate({
        "LogicalPath": path,
        "Sha256": "a" * 64,
        "RowCount": 15000,
        "MinPrimaryKey": "SSE|518880|2026-08-03T01:00:00Z|2026-08-03T01:01:00Z|fixture",
        "MaxPrimaryKey": "SSE|518880|2026-08-03T14:59:00Z|2026-08-03T15:00:00Z|fixture",
    })


def _manifest(**overrides: object) -> DataManifestV1:
    base: dict[str, object] = {
        "SchemaId": "MinuteBarSchemaV1",
        "SchemaHash": "b" * 64,
        "TsPrecision": "Millisecond",
        "Files": (_file(),),
        "RawObjectHashes": ("c" * 64,),
        "InstrumentMappingVersion": "mapping-v1",
        "InstrumentMetadataVersion": "meta-v1",
        "CalendarVersion": "calendar-v1",
        "AdjustmentPolicyVersion": "adjust-v1",
        "QualityRuleVersion": "quality-v1",
        "IsolationRecordHash": ZERO_HASH,
        "IsolationRecordCount": 0,
        "ConversionVersion": "vq-parquet-v1",
        "DependencyVersions": ("vq-parquet-v1",),
        "SupersedesDataVersionId": None,
        "RevisionReason": None,
    }
    base.update(overrides)
    return DataManifestV1.model_validate(base)


def test_identity_is_stable_across_equivalent_manifests() -> None:
    first = _manifest().computeDataVersionId()
    second = _manifest().computeDataVersionId()
    assert first == second == _manifest().computeDataVersionId()
    assert len(first) == 64


def test_storage_uri_and_download_time_do_not_change_identity() -> None:
    plain = _manifest().computeDataVersionId()
    signed = signManifest(
        _manifest(),
        "sig-1",
        storageUri="file:///mnt/data/raw",
        downloadedAt=datetime(2026, 8, 3, 0, 0, 0, tzinfo=timezone.utc),
        localAbsolutePath="/home/user/data/raw",
        note="本机路径与备注不得影响身份",
    )
    assert signed.dataVersionId == plain


def test_content_change_must_change_id() -> None:
    original = _manifest()
    changed = _manifest(**{"Files": (_file(path="MinuteBarV1/SSE/518880/Year=2026/Month=08/def.parquet"),)})
    assert changed.computeDataVersionId() != original.computeDataVersionId()


def test_schema_and_calendar_change_must_change_id() -> None:
    original = _manifest()
    assert _manifest(SchemaId="MinuteBarSchemaV2").computeDataVersionId() != original.computeDataVersionId()
    assert (
        _manifest(CalendarVersion="calendar-v2").computeDataVersionId() != original.computeDataVersionId()
    )
    assert (
        _manifest(QualityRuleVersion="quality-v2").computeDataVersionId() != original.computeDataVersionId()
    )


def test_tampered_manifest_is_rejected_on_validation() -> None:
    manifest = _manifest()
    signManifest(manifest, "sig-1")
    # 直接改 dataVersionId 应被拒绝（模型校验器包装为 ValidationError）
    with pytest.raises(ValidationError, match="篡改"):
        SignedDataManifestV1.model_validate({
            "DataVersionId": "f" * 64,
            "Manifest": manifest,
            "Signature": "sig-1",
        })


def test_sign_manifest_explicit_verify_raises_domain_error() -> None:
    manifest = _manifest()
    signed = signManifest(manifest, "sig-1")
    assert signed.dataVersionId == manifest.computeDataVersionId()
    # 篡改 schema 后重新校验应被拒绝（模型校验器包装为 ValidationError）
    with pytest.raises(ValidationError, match="篡改"):
        SignedDataManifestV1.model_validate({
            "DataVersionId": signed.dataVersionId,
            "Manifest": manifest.model_copy(update={"schemaId": "MinuteBarSchemaV2"}),
            "Signature": "sig-1",
        })


def test_revision_chain_requires_reason_and_valid_hash() -> None:
    with pytest.raises(ValidationError, match="修订原因"):
        _manifest(SupersedesDataVersionId="d" * 64, RevisionReason=None)
    with pytest.raises(ValidationError, match="SHA-256"):
        _manifest(SupersedesDataVersionId="short", RevisionReason="供应商更正")
    chain = _manifest(SupersedesDataVersionId="d" * 64, RevisionReason="供应商更正")
    assert chain.computeDataVersionId() != _manifest().computeDataVersionId()


def test_isolation_hash_consistency_rule() -> None:
    with pytest.raises(ValidationError, match="零哈希"):
        _manifest(IsolationRecordHash=ZERO_HASH, IsolationRecordCount=3)
    with pytest.raises(ValidationError, match="零哈希"):
        _manifest(IsolationRecordHash="e" * 64, IsolationRecordCount=0)


def test_primary_key_range_validation() -> None:
    with pytest.raises(ValidationError, match="MinPrimaryKey"):
        DataManifestFileV1.model_validate({
            "LogicalPath": "x.parquet",
            "Sha256": "a" * 64,
            "RowCount": 1,
            "MinPrimaryKey": "z",
            "MaxPrimaryKey": "a",
        })
