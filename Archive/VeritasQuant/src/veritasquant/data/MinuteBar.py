"""MinuteBarSchemaV1 的严格市场数据契约。"""

from __future__ import annotations

from datetime import datetime
from decimal import Decimal

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp
from veritasquant.instruments.Registry import InstrumentContractError, InstrumentV1, Market


class MinuteBarContractError(ValueError):
    """分钟 Bar 不满足时间、价格或标的元数据契约。"""


class MinuteBarSchemaV1(StrictModel):
    """仅表示已完成、可按 `ts` 进入回测的标准化分钟行情。"""

    ts: datetime = PascalAlias("Ts")
    barStart: datetime = PascalAlias("BarStart")
    barEnd: datetime = PascalAlias("BarEnd")
    symbol: str = PascalAlias("Symbol", min_length=1)
    market: Market = PascalAlias("Market")
    open: Decimal = PascalAlias("Open", gt=Decimal("0"))
    high: Decimal = PascalAlias("High", gt=Decimal("0"))
    low: Decimal = PascalAlias("Low", gt=Decimal("0"))
    close: Decimal = PascalAlias("Close", gt=Decimal("0"))
    volume: Decimal = PascalAlias("Volume", ge=Decimal("0"))
    amount: Decimal | None = PascalAlias("Amount", default=None, ge=Decimal("0"))
    tradeCount: int | None = PascalAlias("TradeCount", default=None, ge=0)
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    sessionId: str = PascalAlias("SessionId", min_length=1)
    source: str = PascalAlias("Source", min_length=1)
    sourceRecordId: str = PascalAlias("SourceRecordId", min_length=1)
    sourceSequence: int = PascalAlias("SourceSequence", ge=0)
    isAdjusted: bool = PascalAlias("IsAdjusted")
    adjustmentVersion: str | None = PascalAlias("AdjustmentVersion", default=None)
    instrumentMetadataVersion: str = PascalAlias("InstrumentMetadataVersion", min_length=1)
    qualityFlags: int = PascalAlias("QualityFlags", ge=0)

    @field_validator("market", mode="before")
    @classmethod
    def parseMarket(cls, value: object) -> Market:
        if isinstance(value, Market):
            return value
        if not isinstance(value, str):
            raise MinuteBarContractError("市场必须是受控字符串")
        try:
            return Market(value)
        except ValueError as error:
            raise MinuteBarContractError(f"未知市场: {value}") from error

    @field_validator("ts", "barStart", "barEnd")
    @classmethod
    def validateTimestamp(cls, value: datetime) -> datetime:
        return validateUtcTimestamp(value, TsPrecision.Millisecond)

    @model_validator(mode="after")
    def validateBar(self) -> "MinuteBarSchemaV1":
        if not self.barStart < self.barEnd <= self.ts:
            raise MinuteBarContractError("必须满足 barStart < barEnd <= ts")
        if not self.low <= self.open <= self.high or not self.low <= self.close <= self.high:
            raise MinuteBarContractError("OHLC 必须满足 low <= open/close <= high")
        if self.isAdjusted != (self.adjustmentVersion is not None):
            raise MinuteBarContractError("复权状态必须与 AdjustmentVersion 一致")
        return self

    def validateAgainstInstrument(
        self, instrument: InstrumentV1, tsPrecision: TsPrecision
    ) -> "MinuteBarSchemaV1":
        """校验 Bar 所属标的、版本、精度及 tick/数量单位。"""
        if self.symbol != instrument.symbol or self.market is not instrument.market:
            raise InstrumentContractError("行情标的必须匹配注册表标的")
        if self.currency != instrument.currency:
            raise InstrumentContractError("行情币种必须匹配标的元数据")
        if self.instrumentMetadataVersion != instrument.metadataVersion:
            raise InstrumentContractError("行情必须引用当前标的元数据版本")
        for timestamp in (self.ts, self.barStart, self.barEnd):
            validateUtcTimestamp(timestamp, tsPrecision)
        for price in (self.open, self.high, self.low, self.close):
            if price % instrument.tickSize != 0:
                raise MinuteBarContractError("OHLC 必须符合标的 tickSize")
        if self.volume % instrument.lotSize != 0:
            raise MinuteBarContractError("成交量必须符合标的 lotSize")
        return self
