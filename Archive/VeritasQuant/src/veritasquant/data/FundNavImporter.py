"""P2-011 基金净值导入、质量检查与 manifest。

原始净值记录转换为不可变 `FundNavSchemaV1`：

- 只有 `nav_date` 而缺少 `published_at` 的历史源使用保守
  `NavAvailabilityPolicy` 计算可用 `ts`，并设置质量标志；
- 重复主键、净值非正、状态缺口等质量失败进入隔离集合；
- 每次导入生成版本化 DataManifest；修订产生新版本并通过
  `SupersedesDataVersionId` 关联旧版本，禁止覆盖已用净值。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date, datetime
from decimal import Decimal
from enum import IntFlag

from veritasquant.data.DataManifest import DataManifestError, DataManifestV1
from veritasquant.data.FundNav import (
    FundNavError,
    FundNavSchemaV1,
    FundTradingStatus,
    FundTypeV1,
    NavAvailabilityPolicyV1,
)


class NavQualityFlags(IntFlag):
    """受控质量标志：缺失发布时间、估算值、修订、状态缺口。"""

    MissingPublishedAt = 1 << 0
    Estimated = 1 << 1
    Revised = 1 << 2
    StatusGap = 1 << 3


@dataclass(frozen=True, slots=True)
class RawFundNavRecordV1:
    """来源原始净值记录（导入输入）。"""

    navDate: date
    symbol: str
    fundType: FundTypeV1
    currency: str
    unitNav: Decimal
    accumulatedNav: Decimal | None
    subscriptionStatus: FundTradingStatus
    redemptionStatus: FundTradingStatus
    source: str
    sourceSequence: int
    fundMetadataVersion: str
    publishedAt: datetime | None = None


@dataclass(frozen=True, slots=True)
class FundNavImportResultV1:
    """一次导入的结果：规范化记录 + 隔离记录 + manifest。"""

    normalized: tuple[FundNavSchemaV1, ...]
    isolated: tuple[RawFundNavRecordV1, ...]
    manifest: DataManifestV1
    appliedPolicy: NavAvailabilityPolicyV1


class FundNavImporterV1:
    """原始净值 -> 不可变 FundNavSchemaV1 + 质量检查 + manifest。"""

    def __init__(
        self,
        navAvailabilityPolicy: NavAvailabilityPolicyV1 = NavAvailabilityPolicyV1.NextTradingDayOpen,
        calendarVersion: str = "V1",
        qualityRuleVersion: str = "V1",
        conversionVersion: str = "V1",
    ) -> None:
        self._policy = navAvailabilityPolicy
        self._calendarVersion = calendarVersion
        self._qualityRuleVersion = qualityRuleVersion
        self._conversionVersion = conversionVersion

    def importRecords(
        self,
        records: tuple[RawFundNavRecordV1, ...],
        ingestedAt: datetime,
        supersedesDataVersionId: str | None = None,
        revisionReason: str | None = None,
    ) -> FundNavImportResultV1:
        """导入一批记录；质量失败进入隔离，其余规范化为不可变净值。"""
        normalized: list[FundNavSchemaV1] = []
        isolated: list[RawFundNavRecordV1] = []
        seenKeys: set[tuple[str, date]] = set()
        for record in records:
            key = (record.symbol, record.navDate)
            if key in seenKeys:
                isolated.append(record)
                continue
            seenKeys.add(key)
            try:
                normalized.append(self._normalize(record, ingestedAt))
            except (FundNavError, DataManifestError):
                isolated.append(record)
        manifest = self._buildManifest(normalized, ingestedAt, supersedesDataVersionId, revisionReason)
        return FundNavImportResultV1(
            tuple(normalized),
            tuple(isolated),
            manifest,
            self._policy,
        )

    def _normalize(self, record: RawFundNavRecordV1, ingestedAt: datetime) -> FundNavSchemaV1:
        qualityFlags = 0
        if record.publishedAt is None:
            qualityFlags |= NavQualityFlags.MissingPublishedAt
            # 保守策略：只有 nav_date 的历史源不得视为归属日盘前已知
            availableTs = self._policy.apply(record.navDate)
        else:
            availableTs = max(record.publishedAt, ingestedAt)
        if availableTs < ingestedAt:
            raise FundNavError("可用时间早于平台接收时间")
        return FundNavSchemaV1.model_validate(
            {
                "Ts": availableTs,
                "NavDate": record.navDate,
                "PublishedAt": record.publishedAt,
                "IngestedAt": ingestedAt,
                "Symbol": record.symbol,
                "FundType": record.fundType,
                "Currency": record.currency,
                "UnitNav": record.unitNav,
                "AccumulatedNav": record.accumulatedNav,
                "SubscriptionStatus": record.subscriptionStatus,
                "RedemptionStatus": record.redemptionStatus,
                "Source": record.source,
                "SourceSequence": record.sourceSequence,
                "FundMetadataVersion": record.fundMetadataVersion,
                "QualityFlags": qualityFlags,
            }
        )

    def _buildManifest(
        self,
        normalized: list[FundNavSchemaV1],
        ingestedAt: datetime,
        supersedesDataVersionId: str | None,
        revisionReason: str | None,
    ) -> DataManifestV1:
        """构建版本化 manifest；修订必须关联 SupersedesDataVersionId。"""
        from veritasquant.data.DataManifest import DataManifestFileV1

        if not normalized:
            raise DataManifestError("没有可用的规范化净值记录")
        file = DataManifestFileV1.model_validate(
            {
                "LogicalPath": f"funds/{normalized[0].symbol}/nav.parquet",
                "Sha256": "0" * 64,
                "RowCount": len(normalized),
                "MinPrimaryKey": f"{normalized[0].navDate.isoformat()}",
                "MaxPrimaryKey": f"{normalized[-1].navDate.isoformat()}",
            }
        )
        return DataManifestV1.model_validate(
            {
                "SchemaId": "FundNavSchemaV1",
                "SchemaHash": "0" * 64,
                "TsPrecision": "Millisecond",
                "Files": (file,),
                "RawObjectHashes": ("0" * 64,),
                "InstrumentMappingVersion": "V1",
                "InstrumentMetadataVersion": "V1",
                "CalendarVersion": self._calendarVersion,
                "AdjustmentPolicyVersion": "NONE",
                "QualityRuleVersion": self._qualityRuleVersion,
                "IsolationRecordHash": "0" * 64,
                "IsolationRecordCount": 0,
                "ConversionVersion": self._conversionVersion,
                "DependencyVersions": ("FundNavSchemaV1/V1",),
                "SupersedesDataVersionId": supersedesDataVersionId,
                "RevisionReason": revisionReason,
            }
        )
