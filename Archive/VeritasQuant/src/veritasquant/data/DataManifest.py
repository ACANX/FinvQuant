"""P1-020 不可变 DataManifestV1、数据版本哈希与修订链。

``data_version_id`` 只由规范化身份字段的规范 JSON 计算：Schema、精度、逻辑路径、
文件 SHA-256/行数/主键范围、原始对象哈希、映射/元数据/日历/复权/质量规则版本、
隔离记录与转换依赖版本。存储 URI、下载时间、本机绝对路径、签名和备注不参与身份。
"""

from __future__ import annotations

from datetime import datetime
from decimal import Decimal
from typing import Any

from pydantic import field_validator, model_validator

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class DataManifestError(ValueError):
    """manifest 身份字段、修订链或哈希不满足不可变契约。"""


class DataManifestFileV1(StrictModel):
    """规范化数据集内单个文件的不可变指纹。"""

    logicalPath: str = PascalAlias("LogicalPath", min_length=1)
    sha256: str = PascalAlias("Sha256", min_length=64, max_length=64)
    rowCount: int = PascalAlias("RowCount", ge=0)
    minPrimaryKey: str = PascalAlias("MinPrimaryKey", min_length=1)
    maxPrimaryKey: str = PascalAlias("MaxPrimaryKey", min_length=1)

    @model_validator(mode="after")
    def validatePrimaryKeyRange(self) -> "DataManifestFileV1":
        if self.minPrimaryKey > self.maxPrimaryKey:
            raise DataManifestError("MinPrimaryKey 不得大于 MaxPrimaryKey")
        return self


class DataManifestV1(StrictModel):
    """定义单一数据版本的不可变身份与可追溯性。"""

    schemaId: str = PascalAlias("SchemaId", min_length=1)
    schemaHash: str = PascalAlias("SchemaHash", min_length=64, max_length=64)
    tsPrecision: TsPrecision = PascalAlias("TsPrecision")
    files: tuple[DataManifestFileV1, ...] = PascalAlias("Files", min_length=1)
    rawObjectHashes: tuple[str, ...] = PascalAlias("RawObjectHashes", min_length=1)
    instrumentMappingVersion: str = PascalAlias("InstrumentMappingVersion", min_length=1)
    instrumentMetadataVersion: str = PascalAlias("InstrumentMetadataVersion", min_length=1)
    calendarVersion: str = PascalAlias("CalendarVersion", min_length=1)
    adjustmentPolicyVersion: str = PascalAlias("AdjustmentPolicyVersion", min_length=1)
    qualityRuleVersion: str = PascalAlias("QualityRuleVersion", min_length=1)
    isolationRecordHash: str = PascalAlias("IsolationRecordHash", min_length=64, max_length=64)
    isolationRecordCount: int = PascalAlias("IsolationRecordCount", ge=0)
    conversionVersion: str = PascalAlias("ConversionVersion", min_length=1)
    dependencyVersions: tuple[str, ...] = PascalAlias("DependencyVersions", default=())
    supersedesDataVersionId: str | None = PascalAlias("SupersedesDataVersionId", default=None)
    revisionReason: str | None = PascalAlias("RevisionReason", default=None)

    @field_validator("tsPrecision", mode="before")
    @classmethod
    def parseTsPrecision(cls, value: object) -> TsPrecision:
        if isinstance(value, TsPrecision):
            return value
        if not isinstance(value, str):
            raise DataManifestError("TsPrecision 必须是受控字符串")
        try:
            return TsPrecision(value)
        except ValueError as error:
            raise DataManifestError(f"未知 TsPrecision: {value}") from error

    @field_validator("rawObjectHashes")
    @classmethod
    def validateRawObjectHashes(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        if any(len(item) != 64 for item in value):
            raise DataManifestError("原始对象哈希必须是 SHA-256")
        return value

    @field_validator("dependencyVersions")
    @classmethod
    def validateDependencyVersions(cls, value: tuple[str, ...]) -> tuple[str, ...]:
        if any(not item.strip() for item in value):
            raise DataManifestError("依赖版本不得为空")
        return value

    @model_validator(mode="after")
    def validateRevisionChain(self) -> "DataManifestV1":
        if self.supersedesDataVersionId is None:
            if self.revisionReason is not None:
                raise DataManifestError("无 SupersedesDataVersionId 时不得填写修订原因")
        else:
            if len(self.supersedesDataVersionId) != 64:
                raise DataManifestError("SupersedesDataVersionId 必须是 SHA-256")
            if self.revisionReason is None or not self.revisionReason.strip():
                raise DataManifestError("修订链必须携带结构化修订原因")
        if self.isolationRecordCount == 0 and self.isolationRecordHash != "0" * 64:
            raise DataManifestError("无隔离记录时隔离哈希必须为零哈希")
        if self.isolationRecordCount > 0 and self.isolationRecordHash == "0" * 64:
            raise DataManifestError("存在隔离记录时不得使用零哈希")
        return self

    def identityFields(self) -> dict[str, Any]:
        """仅返回参与身份哈希的规范化字段（按协议固定顺序）。"""
        return {
            "schema_id": self.schemaId,
            "schema_hash": self.schemaHash,
            "ts_precision": self.tsPrecision.value,
            "files": [
                {
                    "logical_path": file.logicalPath,
                    "sha256": file.sha256,
                    "row_count": file.rowCount,
                    "min_primary_key": file.minPrimaryKey,
                    "max_primary_key": file.maxPrimaryKey,
                }
                for file in self.files
            ],
            "raw_object_hashes": list(self.rawObjectHashes),
            "instrument_mapping_version": self.instrumentMappingVersion,
            "instrument_metadata_version": self.instrumentMetadataVersion,
            "calendar_version": self.calendarVersion,
            "adjustment_policy_version": self.adjustmentPolicyVersion,
            "quality_rule_version": self.qualityRuleVersion,
            "isolation_record_hash": self.isolationRecordHash,
            "isolation_record_count": self.isolationRecordCount,
            "conversion_version": self.conversionVersion,
            "dependency_versions": list(self.dependencyVersions),
            "supersedes_data_version_id": self.supersedesDataVersionId,
            "revision_reason": self.revisionReason,
        }

    def computeDataVersionId(self) -> str:
        """计算规范 JSON SHA-256；存储 URI/下载时间/本机路径/签名/备注不参与。"""
        return canonicalHash(self.identityFields(), self.tsPrecision)


class SignedDataManifestV1(StrictModel):
    """携带签名与审计字段的持久化 manifest，签名不参与身份。"""

    dataVersionId: str = PascalAlias("DataVersionId", min_length=64, max_length=64)
    manifest: DataManifestV1 = PascalAlias("Manifest")
    signature: str = PascalAlias("Signature", min_length=1)
    storageUri: str | None = PascalAlias("StorageUri", default=None)
    downloadedAt: datetime | None = PascalAlias("DownloadedAt", default=None)
    localAbsolutePath: str | None = PascalAlias("LocalAbsolutePath", default=None)
    note: str | None = PascalAlias("Note", default=None)

    @field_validator("downloadedAt")
    @classmethod
    def validateDownloadedAt(cls, value: datetime | None) -> datetime | None:
        if value is None:
            return None
        return validateUtcTimestamp(value, TsPrecision.Millisecond)

    @model_validator(mode="after")
    def verifyIdentity(self) -> "SignedDataManifestV1":
        expected = self.manifest.computeDataVersionId()
        if self.dataVersionId != expected:
            raise DataManifestError("data_version_id 与身份字段不匹配，manifest 已篡改或被改写")
        return self


def signManifest(
    manifest: DataManifestV1,
    signature: str,
    *,
    storageUri: str | None = None,
    downloadedAt: datetime | None = None,
    localAbsolutePath: str | None = None,
    note: str | None = None,
) -> SignedDataManifestV1:
    """签名并校验身份，返回持久化 manifest。"""
    signed = SignedDataManifestV1.model_validate({
        "DataVersionId": manifest.computeDataVersionId(),
        "Manifest": manifest,
        "Signature": signature,
        "StorageUri": storageUri,
        "DownloadedAt": downloadedAt,
        "LocalAbsolutePath": localAbsolutePath,
        "Note": note,
    })
    # 身份校验由模型校验器在构造时强制执行，篡改无法绕过
    return signed


def primaryKeyOf(bar: Any) -> str:
    """构造规范化主键字符串（market + symbol + bar_start + bar_end + source）。"""
    from veritasquant.core.Time import serializeUtcTimestamp
    from veritasquant.data.MinuteBar import MinuteBarSchemaV1

    if not isinstance(bar, MinuteBarSchemaV1):
        raise DataManifestError("主键仅支持 MinuteBarSchemaV1")
    return "|".join((
        bar.market.value,
        bar.symbol,
        serializeUtcTimestamp(bar.barStart, TsPrecision.Millisecond),
        serializeUtcTimestamp(bar.barEnd, TsPrecision.Millisecond),
        bar.source,
    ))


def canonicalDecimalForManifest(value: Decimal) -> str:
    """与 CanonicalJson 一致的 Decimal 规范字符串（供外部校验工具复用）。"""
    if not value.is_finite():
        raise DataManifestError("Decimal 必须有限")
    return value.normalize().to_eng_string() if False else format(value, "f").rstrip("0").rstrip(".")
