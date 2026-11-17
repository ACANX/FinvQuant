"""历史分钟行情行模型（领域层，无基础设施依赖）。

与 `finv_quote_secu_kline_min` 表结构对齐（V4 迁移），
供导入、存储与回放读取共用。
"""

from __future__ import annotations

from decimal import Decimal

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel


class UpsertMode:
    """覆盖更新模式。"""

    Field = "FIELD"  # 字段级：非 NULL 覆盖，NULL 保留旧值
    Row = "ROW"      # 整行覆盖：NULL 也覆盖


class QuoteRowV1(StrictModel):
    """与 `finv_quote_secu_kline_min` 表对齐的一行历史行情。"""

    market_code: int = PascalAlias("MarketCode")
    secu_code: str = PascalAlias("SecuCode")
    ts: int = PascalAlias("Ts")
    date: int | None = PascalAlias("Date", default=None)
    time: int | None = PascalAlias("Time", default=None)
    prev_close: Decimal | None = PascalAlias("PrevClose", default=None)
    open: Decimal | None = PascalAlias("Open", default=None)
    high: Decimal | None = PascalAlias("High", default=None)
    low: Decimal | None = PascalAlias("Low", default=None)
    close: Decimal | None = PascalAlias("Close", default=None)
    paocd: Decimal | None = PascalAlias("Paocd", default=None)
    volume: int | None = PascalAlias("Volume", default=None)
    turnover: Decimal | None = PascalAlias("Turnover", default=None)
    ext_field: str | None = PascalAlias("ExtField", default=None)
    remark: str | None = PascalAlias("Remark", default=None)

    @field_validator("market_code")
    @classmethod
    def validateMarketCode(cls, value: int) -> int:
        if value < 0 or value > 99_999_999:
            raise ValueError("market_code 必须在 0..99999999")
        return value

    @field_validator("ts")
    @classmethod
    def validateTs(cls, value: int) -> int:
        if value < 0:
            raise ValueError("ts 必须非负")
        return value

    @field_validator("secu_code")
    @classmethod
    def validateSecuCode(cls, value: str) -> str:
        if not value:
            raise ValueError("secu_code 不能为空")
        return value

    @field_validator("date")
    @classmethod
    def validateDate(cls, value: int | None) -> int | None:
        if value is not None and not (19_500_101 <= value <= 21_001_231):
            raise ValueError("date 必须是 yyyymmdd（19500101..21001231）")
        return value

    @field_validator("time")
    @classmethod
    def validateTime(cls, value: int | None) -> int | None:
        if value is not None and not (0 <= value <= 235_959):
            raise ValueError("time 必须是 hhmmss（0..235959）")
        return value

    @field_validator("volume")
    @classmethod
    def validateVolume(cls, value: int | None) -> int | None:
        if value is not None and value < 0:
            raise ValueError("volume 必须非负")
        return value

    @field_validator("turnover")
    @classmethod
    def validateTurnover(cls, value: Decimal | None) -> Decimal | None:
        if value is not None and value < 0:
            raise ValueError("turnover 必须非负")
        return value

    @model_validator(mode="after")
    def validateOhlc(self) -> "QuoteRowV1":
        """OHLC 关系校验（与 MinuteBarSchemaV1 契约一致）。"""
        for price in (self.open, self.high, self.low, self.close, self.prev_close, self.paocd):
            if price is not None and price <= 0:
                raise ValueError("价格字段必须为正数")
        if self.open is not None and self.high is not None and self.low is not None:
            if not (self.low <= self.open <= self.high):
                raise ValueError("OHLC 必须满足 low <= open <= high")
        if self.close is not None and self.high is not None and self.low is not None:
            if not (self.low <= self.close <= self.high):
                raise ValueError("OHLC 必须满足 low <= close <= high")
        return self
