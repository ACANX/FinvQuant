"""订单、撤单、改单和执行回报的强类型契约（技术方案 4.6 节）。

所有数量、价格和金额使用 Decimal；方向、订单类型、有效期和状态使用
受控枚举；模型拒绝未知字段、隐式类型和无约束载荷。
"""

from __future__ import annotations

from datetime import datetime
from decimal import Decimal
from enum import StrEnum
from typing import Any, TypeVar

from pydantic import field_validator, model_validator

from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class OrderContractError(ValueError):
    """订单模型不满足强类型契约时抛出。"""


class OrderSide(StrEnum):
    """订单方向。"""

    Buy = "BUY"
    Sell = "SELL"


class PositionEffect(StrEnum):
    """开平仓效果。"""

    Open = "OPEN"
    Close = "CLOSE"
    ReduceOnly = "REDUCE_ONLY"


class OrderType(StrEnum):
    """订单类型；价格字段按类型强制。"""

    Market = "MARKET"
    Limit = "LIMIT"
    Stop = "STOP"
    StopLimit = "STOP_LIMIT"


class TimeInForce(StrEnum):
    """订单有效期。"""

    Day = "DAY"
    GoodTillCancelled = "GTC"
    ImmediateOrCancel = "IOC"
    FillOrKill = "FOK"
    GoodTillDate = "GTD"


class OrderState(StrEnum):
    """技术方案 4.6 节固定的订单全生命周期状态。"""

    New = "NEW"
    PendingRisk = "PENDING_RISK"
    Approved = "APPROVED"
    PendingSubmit = "PENDING_SUBMIT"
    Submitted = "SUBMITTED"
    Accepted = "ACCEPTED"
    PartiallyFilled = "PARTIALLY_FILLED"
    PendingCancel = "PENDING_CANCEL"
    Cancelled = "CANCELLED"
    Filled = "FILLED"
    Rejected = "REJECTED"
    Expired = "EXPIRED"
    ReconciliationRequired = "RECONCILIATION_REQUIRED"


class ExecutionType(StrEnum):
    """执行回报类别；成交类回报必须携带 execution_id。"""

    New = "NEW"
    PartialFill = "PARTIAL_FILL"
    Fill = "FILL"
    Cancelled = "CANCELLED"
    Rejected = "REJECTED"
    Expired = "EXPIRED"
    Corrected = "CORRECTED"


class BrokerState(StrEnum):
    """券商侧订单状态（受控枚举，禁止自由字符串）。"""

    Accepted = "ACCEPTED"
    Working = "WORKING"
    Partial = "PARTIAL"
    Filled = "FILLED"
    Cancelled = "CANCELLED"
    Rejected = "REJECTED"
    Expired = "EXPIRED"
    Unknown = "UNKNOWN"


EnumT = TypeVar("EnumT", bound=StrEnum)


def _parseEnum(value: Any, enumType: type[EnumT], label: str) -> EnumT:
    """仅接受模型枚举或协议中完全匹配的字符串。"""
    if isinstance(value, enumType):
        return value
    if not isinstance(value, str):
        raise OrderContractError(f"{label}必须是受控字符串")
    try:
        return enumType(value)
    except ValueError as error:
        raise OrderContractError(f"未知{label}: {value}") from error


def _validateUtc(value: object) -> Any:
    if value is None:
        return None
    if not isinstance(value, datetime):
        raise OrderContractError("时间必须是 datetime")
    validateUtcTimestamp(value, TsPrecision.Millisecond)
    return value


class OrderIntentV1(EventPayloadV1):
    """策略返回的只读意图；不携带券商凭据或最终订单状态。"""

    intentId: str = PascalAlias("IntentId", min_length=1)
    runId: str = PascalAlias("RunId", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    subaccountId: str | None = PascalAlias("SubaccountId", default=None, min_length=1)
    strategyId: str = PascalAlias("StrategyId", min_length=1)
    strategyVersion: str = PascalAlias("StrategyVersion", min_length=1)
    symbol: str = PascalAlias("Symbol", min_length=1)
    instrumentMetadataVersion: str = PascalAlias("InstrumentMetadataVersion", min_length=1)
    side: OrderSide = PascalAlias("Side")
    positionEffect: PositionEffect = PascalAlias("PositionEffect")
    orderType: OrderType = PascalAlias("OrderType")
    quantity: Decimal = PascalAlias("Quantity", gt=Decimal("0"))
    timeInForce: TimeInForce = PascalAlias("TimeInForce")
    ts: datetime = PascalAlias("Ts")
    createdFromEventId: str = PascalAlias("CreatedFromEventId", min_length=1)
    expectedAccountVersion: int = PascalAlias("ExpectedAccountVersion", ge=0)
    limitPrice: Decimal | None = PascalAlias("LimitPrice", default=None, gt=Decimal("0"))
    stopPrice: Decimal | None = PascalAlias("StopPrice", default=None, gt=Decimal("0"))

    @field_validator("side", "positionEffect", "orderType", "timeInForce", mode="before")
    @classmethod
    def parseEnums(cls, value: object, info: Any) -> Any:
        fieldName = info.field_name
        mapping: dict[str, Any] = {
            "side": OrderSide,
            "positionEffect": PositionEffect,
            "orderType": OrderType,
            "timeInForce": TimeInForce,
        }
        return _parseEnum(value, mapping[fieldName], fieldName)

    @field_validator("ts", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> Any:
        return _validateUtc(value)

    @model_validator(mode="after")
    def validateIntent(self) -> "OrderIntentV1":
        if self.orderType in (OrderType.Limit, OrderType.StopLimit) and self.limitPrice is None:
            raise OrderContractError("LIMIT/STOP_LIMIT 必须携带 limitPrice")
        if self.orderType in (OrderType.Stop, OrderType.StopLimit) and self.stopPrice is None:
            raise OrderContractError("STOP/STOP_LIMIT 必须携带 stopPrice")
        if self.orderType is OrderType.Market and (self.limitPrice is not None or self.stopPrice is not None):
            raise OrderContractError("MARKET 订单不得携带限价或止损价格")
        return self


class OrderEventV1(EventPayloadV1):
    """获批订单的状态事实；每次迁移严格增加 order_version。"""

    clientOrderId: str = PascalAlias("ClientOrderId", min_length=1)
    intentId: str = PascalAlias("IntentId", min_length=1)
    commandId: str = PascalAlias("CommandId", min_length=1)
    orderVersion: int = PascalAlias("OrderVersion", ge=1)
    state: OrderState = PascalAlias("State")
    approvedQuantity: Decimal = PascalAlias("ApprovedQuantity", gt=Decimal("0"))
    orderType: OrderType = PascalAlias("OrderType")
    side: OrderSide = PascalAlias("Side")
    quantity: Decimal = PascalAlias("Quantity", gt=Decimal("0"))
    limitPrice: Decimal | None = PascalAlias("LimitPrice", default=None, gt=Decimal("0"))
    stopPrice: Decimal | None = PascalAlias("StopPrice", default=None, gt=Decimal("0"))
    effectiveAfterEventId: str = PascalAlias("EffectiveAfterEventId", min_length=1)
    riskDecisionId: str = PascalAlias("RiskDecisionId", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    subaccountId: str | None = PascalAlias("SubaccountId", default=None, min_length=1)
    ts: datetime = PascalAlias("Ts")

    @field_validator("state", "orderType", "side", mode="before")
    @classmethod
    def parseEnums(cls, value: object, info: Any) -> Any:
        mapping: dict[str, Any] = {
            "state": OrderState,
            "orderType": OrderType,
            "side": OrderSide,
        }
        return _parseEnum(value, mapping[info.field_name], info.field_name)

    @field_validator("ts", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> Any:
        return _validateUtc(value)

    @model_validator(mode="after")
    def validateOrder(self) -> "OrderEventV1":
        if self.approvedQuantity > self.quantity:
            raise OrderContractError("获批数量不得超过订单数量")
        return self


class CancelOrderEventV1(EventPayloadV1):
    """撤单请求（不是已撤状态）；重复请求必须使用同一 ID。"""

    cancelRequestId: str = PascalAlias("CancelRequestId", min_length=1)
    clientOrderId: str = PascalAlias("ClientOrderId", min_length=1)
    brokerOrderId: str | None = PascalAlias("BrokerOrderId", default=None, min_length=1)
    expectedOrderVersion: int = PascalAlias("ExpectedOrderVersion", ge=1)
    reason: str = PascalAlias("Reason", min_length=1)
    requestedBy: str = PascalAlias("RequestedBy", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    ts: datetime = PascalAlias("Ts")

    @field_validator("ts", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> Any:
        return _validateUtc(value)


class ReplaceOrderEventV1(EventPayloadV1):
    """改单请求；首期对不支持原子改单的券商转换为撤旧建新。"""

    replaceRequestId: str = PascalAlias("ReplaceRequestId", min_length=1)
    clientOrderId: str = PascalAlias("ClientOrderId", min_length=1)
    expectedOrderVersion: int = PascalAlias("ExpectedOrderVersion", ge=1)
    newQuantity: Decimal | None = PascalAlias("NewQuantity", default=None, gt=Decimal("0"))
    newLimitPrice: Decimal | None = PascalAlias("NewLimitPrice", default=None, gt=Decimal("0"))
    newStopPrice: Decimal | None = PascalAlias("NewStopPrice", default=None, gt=Decimal("0"))
    newTimeInForce: TimeInForce | None = PascalAlias("NewTimeInForce", default=None)
    reason: str = PascalAlias("Reason", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    ts: datetime = PascalAlias("Ts")

    @field_validator("newTimeInForce", mode="before")
    @classmethod
    def parseTimeInForce(cls, value: object) -> Any:
        if value is None:
            return None
        return _parseEnum(value, TimeInForce, "newTimeInForce")

    @field_validator("ts", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> Any:
        return _validateUtc(value)

    @model_validator(mode="after")
    def validateReplace(self) -> "ReplaceOrderEventV1":
        if (
            self.newQuantity is None
            and self.newLimitPrice is None
            and self.newStopPrice is None
            and self.newTimeInForce is None
        ):
            raise OrderContractError("改单必须至少修改数量、价格或有效期之一")
        return self


class ExecutionReportEventV1(EventPayloadV1):
    """券商执行回报；成交必须有账户内唯一 execution_id。"""

    brokerReportId: str = PascalAlias("BrokerReportId", min_length=1)
    clientOrderId: str = PascalAlias("ClientOrderId", min_length=1)
    brokerOrderId: str | None = PascalAlias("BrokerOrderId", default=None, min_length=1)
    reportSequence: int = PascalAlias("ReportSequence", ge=1)
    executionType: ExecutionType = PascalAlias("ExecutionType")
    executionId: str | None = PascalAlias("ExecutionId", default=None, min_length=1)
    lastQuantity: Decimal = PascalAlias("LastQuantity", ge=Decimal("0"))
    lastPrice: Decimal | None = PascalAlias("LastPrice", default=None, gt=Decimal("0"))
    cumulativeQuantity: Decimal = PascalAlias("CumulativeQuantity", ge=Decimal("0"))
    remainingQuantity: Decimal = PascalAlias("RemainingQuantity", ge=Decimal("0"))
    brokerState: BrokerState = PascalAlias("BrokerState")
    reasonCode: str | None = PascalAlias("ReasonCode", default=None, min_length=1)
    diagnosticTs: datetime = PascalAlias("DiagnosticTs")
    accountId: str = PascalAlias("AccountId", min_length=1)
    ts: datetime = PascalAlias("Ts")

    @field_validator("executionType", "brokerState", mode="before")
    @classmethod
    def parseEnums(cls, value: object, info: Any) -> Any:
        mapping: dict[str, Any] = {
            "executionType": ExecutionType,
            "brokerState": BrokerState,
        }
        return _parseEnum(value, mapping[info.field_name], info.field_name)

    @field_validator("ts", "diagnosticTs", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> Any:
        return _validateUtc(value)

    @model_validator(mode="after")
    def validateReport(self) -> "ExecutionReportEventV1":
        if self.executionType in (ExecutionType.PartialFill, ExecutionType.Fill):
            if self.executionId is None:
                raise OrderContractError("成交类回报必须携带 execution_id")
            if self.lastQuantity <= 0:
                raise OrderContractError("成交类回报的 lastQuantity 必须为正")
            if self.lastPrice is None:
                raise OrderContractError("成交类回报必须携带成交价格")
        elif self.executionId is not None:
            raise OrderContractError("非成交类回报不得携带 execution_id")
        if self.cumulativeQuantity < self.lastQuantity:
            raise OrderContractError("累计成交量不得小于本报告增量")
        return self
