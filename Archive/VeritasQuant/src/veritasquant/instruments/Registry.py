"""阶段 1 标的注册表及资产能力模型。"""

from __future__ import annotations

from datetime import date
from decimal import Decimal
from enum import StrEnum

from pydantic import field_validator, model_validator

from veritasquant.core.Models import PascalAlias, StrictModel


class InstrumentContractError(ValueError):
    """标的元数据、日历或能力清单不满足确定性契约。"""


class AssetClass(StrEnum):
    """阶段 1 支持的资产类别。"""

    EquityEtf = "EQUITY_ETF"
    Futures = "FUTURES"


class Market(StrEnum):
    """受控市场代码。"""

    Sse = "SSE"
    Shfe = "SHFE"


class ExecutionMode(StrEnum):
    """资产能力可分别启用的执行模式。"""

    Backtest = "BACKTEST"
    Simulation = "SIMULATION"
    Live = "LIVE"


class SettlementRule(StrEnum):
    """阶段 1 证券和期货的结算口径。"""

    SecurityTPlusOne = "SECURITY_T_PLUS_1"
    FuturesDailyMarkToMarket = "FUTURES_DAILY_MARK_TO_MARKET"


class TradingSessionRuleV1(StrictModel):
    """交易日内的版本化本地会话规则。"""

    sessionId: str = PascalAlias("SessionId", min_length=1)
    openLocalTime: str = PascalAlias("OpenLocalTime", pattern=r"^\d{2}:\d{2}$")
    closeLocalTime: str = PascalAlias("CloseLocalTime", pattern=r"^\d{2}:\d{2}$")
    spansMidnight: bool = PascalAlias("SpansMidnight")
    tradingWeekdays: tuple[int, ...] = PascalAlias("TradingWeekdays", min_length=1)

    @model_validator(mode="after")
    def validateSession(self) -> "TradingSessionRuleV1":
        if len(set(self.tradingWeekdays)) != len(self.tradingWeekdays):
            raise InstrumentContractError("交易会话工作日不得重复")
        if any(day < 0 or day > 6 for day in self.tradingWeekdays):
            raise InstrumentContractError("交易会话工作日必须在 0 至 6 之间")
        if self.openLocalTime == self.closeLocalTime:
            raise InstrumentContractError("交易会话开闭时间不得相同")
        if self.spansMidnight != (self.closeLocalTime < self.openLocalTime):
            raise InstrumentContractError("跨日标记必须与会话开闭时间一致")
        return self


class TradingCalendarV1(StrictModel):
    """市场日历及交易会话的不可变版本。"""

    calendarId: str = PascalAlias("CalendarId", min_length=1)
    version: str = PascalAlias("Version", min_length=1)
    market: Market = PascalAlias("Market")
    timeZone: str = PascalAlias("TimeZone", min_length=1)
    sessions: tuple[TradingSessionRuleV1, ...] = PascalAlias("Sessions", min_length=1)
    holidays: tuple[date, ...] = PascalAlias("Holidays", default=())

    @field_validator("market", mode="before")
    @classmethod
    def parseMarket(cls, value: object) -> Market:
        return _parseEnum(value, Market, "市场")

    @model_validator(mode="after")
    def validateCalendar(self) -> "TradingCalendarV1":
        sessionIds = [session.sessionId for session in self.sessions]
        if len(set(sessionIds)) != len(sessionIds):
            raise InstrumentContractError("交易日历不得包含重复 sessionId")
        if len(set(self.holidays)) != len(self.holidays):
            raise InstrumentContractError("交易日历不得包含重复节假日")
        return self

    def isTradingDate(self, value: date) -> bool:
        """按工作日和节假日判断日期能否交易。"""
        return value.weekday() in {day for session in self.sessions for day in session.tradingWeekdays} and value not in self.holidays


class FeeScheduleV1(StrictModel):
    """不含价格推断的版本化费率表。"""

    feeScheduleId: str = PascalAlias("FeeScheduleId", min_length=1)
    version: str = PascalAlias("Version", min_length=1)
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    brokerFeeRate: Decimal = PascalAlias("BrokerFeeRate", ge=Decimal("0"))
    exchangeFeeRate: Decimal = PascalAlias("ExchangeFeeRate", ge=Decimal("0"))
    taxRate: Decimal = PascalAlias("TaxRate", ge=Decimal("0"))
    minimumFee: Decimal = PascalAlias("MinimumFee", ge=Decimal("0"))
    effectiveFrom: date = PascalAlias("EffectiveFrom")
    effectiveTo: date | None = PascalAlias("EffectiveTo", default=None)

    @model_validator(mode="after")
    def validateEffectiveRange(self) -> "FeeScheduleV1":
        if self.effectiveTo is not None and self.effectiveTo < self.effectiveFrom:
            raise InstrumentContractError("费率结束日期不得早于生效日期")
        return self


class InstrumentV1(StrictModel):
    """单一可交易标的的完整、版本化元数据。"""

    instrumentId: str = PascalAlias("InstrumentId", min_length=1)
    symbol: str = PascalAlias("Symbol", min_length=1)
    market: Market = PascalAlias("Market")
    assetClass: AssetClass = PascalAlias("AssetClass")
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    metadataVersion: str = PascalAlias("MetadataVersion", min_length=1)
    calendarId: str = PascalAlias("CalendarId", min_length=1)
    feeScheduleId: str = PascalAlias("FeeScheduleId", min_length=1)
    tickSize: Decimal = PascalAlias("TickSize", gt=Decimal("0"))
    lotSize: Decimal = PascalAlias("LotSize", gt=Decimal("0"))
    settlementRule: SettlementRule = PascalAlias("SettlementRule")
    contractMultiplier: Decimal | None = PascalAlias("ContractMultiplier", default=None, gt=Decimal("0"))
    initialMarginRate: Decimal | None = PascalAlias(
        "InitialMarginRate", default=None, gt=Decimal("0"), le=Decimal("1")
    )
    expiryDate: date | None = PascalAlias("ExpiryDate", default=None)

    @field_validator("market", mode="before")
    @classmethod
    def parseMarket(cls, value: object) -> Market:
        return _parseEnum(value, Market, "市场")

    @field_validator("assetClass", mode="before")
    @classmethod
    def parseAssetClass(cls, value: object) -> AssetClass:
        return _parseEnum(value, AssetClass, "资产类别")

    @field_validator("settlementRule", mode="before")
    @classmethod
    def parseSettlementRule(cls, value: object) -> SettlementRule:
        return _parseEnum(value, SettlementRule, "结算规则")

    @model_validator(mode="after")
    def validateAssetRules(self) -> "InstrumentV1":
        if self.assetClass is AssetClass.EquityEtf:
            if self.market is not Market.Sse or self.settlementRule is not SettlementRule.SecurityTPlusOne:
                raise InstrumentContractError("阶段 1 ETF 必须使用 SSE 与证券 T+1 结算")
            if any(value is not None for value in (self.contractMultiplier, self.initialMarginRate, self.expiryDate)):
                raise InstrumentContractError("ETF 不得声明期货乘数、保证金或到期日")
        if self.assetClass is AssetClass.Futures:
            if self.market is not Market.Shfe or self.settlementRule is not SettlementRule.FuturesDailyMarkToMarket:
                raise InstrumentContractError("阶段 1期货必须使用 SHFE 与逐日盯市结算")
            if any(value is None for value in (self.contractMultiplier, self.initialMarginRate, self.expiryDate)):
                raise InstrumentContractError("期货必须声明合约乘数、初始保证金率和到期日")
        return self


class AssetCapabilityManifestV1(StrictModel):
    """显式列出某标的在特定执行模式下可用的能力。"""

    capabilityVersion: str = PascalAlias("CapabilityVersion", min_length=1)
    instrumentId: str = PascalAlias("InstrumentId", min_length=1)
    assetClass: AssetClass = PascalAlias("AssetClass")
    market: Market = PascalAlias("Market")
    allowedExecutionModes: tuple[ExecutionMode, ...] = PascalAlias("AllowedExecutionModes", min_length=1)
    minuteBarSchemaId: str = PascalAlias("MinuteBarSchemaId", min_length=1)
    calendarVersion: str = PascalAlias("CalendarVersion", min_length=1)
    instrumentMetadataVersion: str = PascalAlias("InstrumentMetadataVersion", min_length=1)
    feeScheduleVersion: str = PascalAlias("FeeScheduleVersion", min_length=1)
    executionAdapterId: str = PascalAlias("ExecutionAdapterId", min_length=1)
    contractTestHashes: tuple[str, ...] = PascalAlias("ContractTestHashes", min_length=1)

    @field_validator("assetClass", mode="before")
    @classmethod
    def parseAssetClass(cls, value: object) -> AssetClass:
        return _parseEnum(value, AssetClass, "资产类别")

    @field_validator("market", mode="before")
    @classmethod
    def parseMarket(cls, value: object) -> Market:
        return _parseEnum(value, Market, "市场")

    @field_validator("allowedExecutionModes", mode="before")
    @classmethod
    def parseExecutionModes(cls, value: object) -> tuple[ExecutionMode, ...]:
        if not isinstance(value, tuple):
            raise InstrumentContractError("允许执行模式必须是元组")
        return tuple(_parseEnum(item, ExecutionMode, "执行模式") for item in value)

    @model_validator(mode="after")
    def validateManifest(self) -> "AssetCapabilityManifestV1":
        if len(set(self.allowedExecutionModes)) != len(self.allowedExecutionModes):
            raise InstrumentContractError("允许执行模式不得重复")
        if len(set(self.contractTestHashes)) != len(self.contractTestHashes):
            raise InstrumentContractError("能力测试哈希不得重复")
        if any(len(value) != 64 for value in self.contractTestHashes):
            raise InstrumentContractError("能力测试哈希必须为 SHA-256 十六进制摘要")
        if ExecutionMode.Live in self.allowedExecutionModes:
            raise InstrumentContractError("阶段 1 能力清单不得启用 LIVE")
        return self

    def supports(self, executionMode: ExecutionMode) -> bool:
        """未显式列入清单的模式一律禁用。"""
        return executionMode in self.allowedExecutionModes


class InstrumentRegistryV1(StrictModel):
    """关联版本化元数据并执行交易能力门禁。"""

    registryVersion: str = PascalAlias("RegistryVersion", min_length=1)
    instruments: tuple[InstrumentV1, ...] = PascalAlias("Instruments", min_length=1)
    calendars: tuple[TradingCalendarV1, ...] = PascalAlias("Calendars", min_length=1)
    feeSchedules: tuple[FeeScheduleV1, ...] = PascalAlias("FeeSchedules", min_length=1)
    capabilities: tuple[AssetCapabilityManifestV1, ...] = PascalAlias("Capabilities", min_length=1)

    @model_validator(mode="after")
    def validateReferences(self) -> "InstrumentRegistryV1":
        calendars = _uniqueById(self.calendars, "calendarId", "交易日历")
        fees = _uniqueById(self.feeSchedules, "feeScheduleId", "费率表")
        instruments = _uniqueById(self.instruments, "instrumentId", "标的")
        seenSymbols: set[tuple[Market, str]] = set()
        for instrument in instruments.values():
            symbolKey = (instrument.market, instrument.symbol)
            if symbolKey in seenSymbols:
                raise InstrumentContractError("同一市场不得注册重复 symbol")
            seenSymbols.add(symbolKey)
            calendar = calendars.get(instrument.calendarId)
            fee = fees.get(instrument.feeScheduleId)
            if calendar is None or calendar.market is not instrument.market:
                raise InstrumentContractError("标的必须引用同市场交易日历")
            if fee is None or fee.currency != instrument.currency:
                raise InstrumentContractError("标的必须引用同币种费率表")
        capabilityIds = _uniqueById(self.capabilities, "instrumentId", "标的能力")
        for instrumentId, capability in capabilityIds.items():
            referencedInstrument = instruments.get(instrumentId)
            if referencedInstrument is None:
                raise InstrumentContractError("能力清单引用未知标的")
            if (
                capability.assetClass is not referencedInstrument.assetClass
                or capability.market is not referencedInstrument.market
            ):
                raise InstrumentContractError("能力清单的资产类别和市场必须与标的一致")
            if capability.instrumentMetadataVersion != referencedInstrument.metadataVersion:
                raise InstrumentContractError("能力清单必须引用当前标的元数据版本")
            if capability.calendarVersion != calendars[referencedInstrument.calendarId].version:
                raise InstrumentContractError("能力清单必须引用当前日历版本")
            if capability.feeScheduleVersion != fees[referencedInstrument.feeScheduleId].version:
                raise InstrumentContractError("能力清单必须引用当前费率版本")
        if set(instruments) != set(capabilityIds):
            raise InstrumentContractError("每个注册标的必须有唯一能力清单")
        return self

    def requireTradable(self, symbol: str, market: Market, executionMode: ExecutionMode) -> InstrumentV1:
        """返回已显式授权模式的标的，任何缺失元数据都不允许继续。"""
        instrument = next(
            (item for item in self.instruments if item.symbol == symbol and item.market is market), None
        )
        if instrument is None:
            raise InstrumentContractError("标的不在版本化注册表中")
        capability = next(item for item in self.capabilities if item.instrumentId == instrument.instrumentId)
        if not capability.supports(executionMode):
            raise InstrumentContractError("标的未获当前执行模式能力授权")
        return instrument


def _uniqueById[T: StrictModel](items: tuple[T, ...], attribute: str, label: str) -> dict[str, T]:
    """按稳定 ID 建立索引并拒绝重复。"""
    result: dict[str, T] = {}
    for item in items:
        itemId = getattr(item, attribute)
        if itemId in result:
            raise InstrumentContractError(f"{label} ID 不得重复: {itemId}")
        result[itemId] = item
    return result


def _parseEnum[T: StrEnum](value: object, enumType: type[T], label: str) -> T:
    """只接受协议列出的字符串值或已解析枚举，不作宽松猜测。"""
    if isinstance(value, enumType):
        return value
    if not isinstance(value, str):
        raise InstrumentContractError(f"{label}必须是受控字符串")
    try:
        return enumType(value)
    except ValueError as error:
        raise InstrumentContractError(f"未知{label}: {value}") from error
