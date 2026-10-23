"""MVSV-1 外部记录到标准化 MinuteBar 的受控映射。"""

from __future__ import annotations

from datetime import timedelta
from decimal import Decimal
from enum import StrEnum

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.data.Mvsv import MvsvHeaderV1, MvsvRecordV1
from veritasquant.instruments.Registry import InstrumentV1


class MvsvNormalizationError(ValueError):
    """来源记录不能安全映射为标准化行情。"""


class BarLabelMeaning(StrEnum):
    """来源 `ts` 所代表的 Bar 边界。"""

    Start = "Start"
    End = "End"


TURNOVER_SCALE_UNKNOWN = 1


class MvsvImportPolicyV1(StrictModel):
    """由数据来源契约显式提供的映射参数。"""

    source: str = PascalAlias("Source", min_length=1)
    barLabelMeaning: BarLabelMeaning = PascalAlias("BarLabelMeaning")
    barIntervalSeconds: int = PascalAlias("BarIntervalSeconds", gt=0)
    availabilityDelaySeconds: int = PascalAlias("AvailabilityDelaySeconds", ge=0)
    turnoverScale: Decimal | None = PascalAlias("TurnoverScale", default=None, gt=Decimal("0"))
    changeTolerance: Decimal = PascalAlias("ChangeTolerance", ge=Decimal("0"))
    changeRateTolerance: Decimal = PascalAlias("ChangeRateTolerance", ge=Decimal("0"))

    @field_validator("barLabelMeaning", mode="before")
    @classmethod
    def parseBarLabelMeaning(cls, value: object) -> BarLabelMeaning:
        if isinstance(value, BarLabelMeaning):
            return value
        if not isinstance(value, str):
            raise MvsvNormalizationError("BarLabelMeaning 必须是 Start 或 End")
        try:
            return BarLabelMeaning(value)
        except ValueError as error:
            raise MvsvNormalizationError("BarLabelMeaning 必须是 Start 或 End") from error

    @model_validator(mode="after")
    def rejectImplicitPolicy(self) -> "MvsvImportPolicyV1":
        if self.barIntervalSeconds != 60:
            raise MvsvNormalizationError("阶段 1 MVSV-1 仅接受 60 秒 BarInterval")
        return self


class MvsvNormalizerV1:
    """验证来源语义后生成可用时间正确的 MinuteBar。"""

    def __init__(self, policy: MvsvImportPolicyV1, instrument: InstrumentV1, tsPrecision: TsPrecision) -> None:
        self._policy = policy
        self._instrument = instrument
        self._tsPrecision = tsPrecision

    def normalize(
        self,
        header: MvsvHeaderV1,
        record: MvsvRecordV1,
        sourceObjectHash: str,
        sourceRelativePath: str,
    ) -> MinuteBarSchemaV1:
        """映射单行来源记录，并保留对象哈希、路径和行号。"""
        if len(sourceObjectHash) != 64:
            raise MvsvNormalizationError("来源对象哈希必须为 SHA-256")
        if not sourceRelativePath or sourceRelativePath.startswith(("/", "\\")):
            raise MvsvNormalizationError("来源路径必须是非空相对路径")
        if header.values["Code"] != self._instrument.symbol:
            raise MvsvNormalizationError("来源 Code 必须匹配标的 symbol")
        self._validateSemantics(record)
        interval = timedelta(seconds=self._policy.barIntervalSeconds)
        if self._policy.barLabelMeaning is BarLabelMeaning.Start:
            barStart, barEnd = record.sourceTs, record.sourceTs + interval
        else:
            barStart, barEnd = record.sourceTs - interval, record.sourceTs
        amount = None if self._policy.turnoverScale is None else record.turnover / self._policy.turnoverScale
        qualityFlags = TURNOVER_SCALE_UNKNOWN if amount is None else 0
        bar = MinuteBarSchemaV1.model_validate({
            "Ts": barEnd + timedelta(seconds=self._policy.availabilityDelaySeconds),
            "BarStart": barStart,
            "BarEnd": barEnd,
            "Symbol": self._instrument.symbol,
            "Market": self._instrument.market,
            "Open": record.open,
            "High": record.high,
            "Low": record.low,
            "Close": record.close,
            "Volume": record.volume,
            "Amount": amount,
            "TradeCount": None,
            "Currency": self._instrument.currency,
            "SessionId": "source-unverified",
            "Source": self._policy.source,
            "SourceRecordId": f"{sourceObjectHash}:{sourceRelativePath}:{record.sourceLine}",
            "SourceSequence": record.sourceSequence,
            "IsAdjusted": False,
            "AdjustmentVersion": None,
            "InstrumentMetadataVersion": self._instrument.metadataVersion,
            "QualityFlags": qualityFlags,
        })
        return bar.validateAgainstInstrument(self._instrument, self._tsPrecision)

    def _validateSemantics(self, record: MvsvRecordV1) -> None:
        if record.previousClose <= 0:
            raise MvsvNormalizationError("来源前收盘价必须大于零")
        expectedChange = record.close - record.previousClose
        expectedRate = expectedChange / record.close * Decimal("100")
        if abs(record.change - expectedChange) > self._policy.changeTolerance:
            raise MvsvNormalizationError("来源 cp 与 c - p 不一致")
        if abs(record.changeRate - expectedRate) > self._policy.changeRateTolerance:
            raise MvsvNormalizationError("来源 cr 与 (c - p) / c * 100 不一致")
