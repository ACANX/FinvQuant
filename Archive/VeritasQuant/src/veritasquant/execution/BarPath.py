"""版本化 Bar 内路径、触价/跳空矩阵与价格保护（技术方案 7.3.1 节）。

阶段 1 默认 DIRECTIONAL_OHLC_V1 路径；订单按路径首次触及时处理，不能
按对策略最有利的顺序选择触发。支持市价、限价、止损、OCO 组和价格保护，
派生限价按买入向下、卖出向上舍入到 tick；止损按买入向上、卖出向下舍入。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal, ROUND_DOWN, ROUND_UP
from enum import StrEnum

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.Orders import OrderSide, OrderType


class BarPathError(ValueError):
    """Bar 路径模型或触价判定违反契约时抛出。"""


class BarPathModelVersion(StrEnum):
    """版本化路径模型；报告不得混合不同版本的结果。"""

    DirectionalOhlcV1 = "DIRECTIONAL_OHLC_V1"
    TickReplayV1 = "TICK_REPLAY_V1"


class TriggerKind(StrEnum):
    """订单在路径上的触发方式。"""

    MarketAtOpen = "MARKET_AT_OPEN"
    LimitTouched = "LIMIT_TOUCHED"
    StopActivated = "STOP_ACTIVATED"
    StopLimitActivated = "STOP_LIMIT_ACTIVATED"
    AmbiguousTrigger = "AMBIGUOUS_TRIGGER"
    NotTriggered = "NOT_TRIGGERED"


@dataclass(frozen=True, slots=True)
class TriggerResultV1:
    """订单在 Bar 路径上的确定性触发结果。"""

    triggered: bool
    kind: TriggerKind
    fillPrice: Decimal | None
    touchedAtPoint: int


def directionalPathV1(bar: MinuteBarSchemaV1) -> tuple[Decimal, ...]:
    """DIRECTIONAL_OHLC_V1：close>=open 走 open->low->high->close，否则 open->high->low->close。"""
    _validateBar(bar)
    if bar.close >= bar.open:
        return (bar.open, bar.low, bar.high, bar.close)
    return (bar.open, bar.high, bar.low, bar.close)


class BarPathTriggerV1:
    """按路径顺序判定订单首次触发，禁止为订单挑选有利顺序。"""

    def __init__(self, pathVersion: BarPathModelVersion = BarPathModelVersion.DirectionalOhlcV1) -> None:
        self._pathVersion = pathVersion
        if pathVersion is not BarPathModelVersion.DirectionalOhlcV1:
            raise BarPathError("阶段 1 仅支持 DIRECTIONAL_OHLC_V1 路径模型")

    @property
    def pathVersion(self) -> BarPathModelVersion:
        return self._pathVersion

    def evaluate(
        self,
        *,
        side: OrderSide,
        orderType: OrderType,
        bar: MinuteBarSchemaV1,
        limitPrice: Decimal | None,
        stopPrice: Decimal | None,
        tickSize: Decimal,
    ) -> TriggerResultV1:
        """对单个订单按路径首次触发判定。"""
        _validateBar(bar)
        if not isinstance(tickSize, Decimal) or tickSize <= 0:
            raise BarPathError("tick 必须为正 Decimal")
        path = directionalPathV1(bar)

        if orderType is OrderType.Market:
            # 市价单：下一允许 Bar 开盘即有资格，按开盘可用价处理
            return TriggerResultV1(True, TriggerKind.MarketAtOpen, path[0], 0)

        if orderType is OrderType.Limit:
            if limitPrice is None:
                raise BarPathError("限价单必须携带 limitPrice")
            return self._evaluateLimit(side, limitPrice, path)

        if orderType is OrderType.Stop:
            if stopPrice is None:
                raise BarPathError("止损单必须携带 stopPrice")
            return self._evaluateStop(side, stopPrice, path)

        if orderType is OrderType.StopLimit:
            if stopPrice is None or limitPrice is None:
                raise BarPathError("止损限价单必须携带 stopPrice 和 limitPrice")
            return self._evaluateStopLimit(side, stopPrice, limitPrice, path)

        raise BarPathError(f"未知订单类型: {orderType.value}")

    def evaluateOco(
        self,
        *,
        side: OrderSide,
        stopPrice: Decimal,
        limitPrice: Decimal | None,
        bar: MinuteBarSchemaV1,
        tickSize: Decimal,
    ) -> TriggerResultV1:
        """OCO 组：路径首次触发者成交，同组剩余订单立即取消。"""
        stopResult = self.evaluate(
            side=side, orderType=OrderType.Stop, bar=bar, limitPrice=None, stopPrice=stopPrice, tickSize=tickSize
        )
        if limitPrice is not None:
            limitResult = self.evaluate(
                side=side, orderType=OrderType.Limit, bar=bar, limitPrice=limitPrice, stopPrice=None, tickSize=tickSize
            )
            if stopResult.triggered and limitResult.triggered:
                if stopResult.touchedAtPoint == limitResult.touchedAtPoint:
                    return TriggerResultV1(True, TriggerKind.AmbiguousTrigger, None, stopResult.touchedAtPoint)
                if stopResult.touchedAtPoint < limitResult.touchedAtPoint:
                    return stopResult
                return limitResult
            if stopResult.triggered:
                return stopResult
            return limitResult
        return stopResult

    def _evaluateLimit(self, side: OrderSide, limitPrice: Decimal, path: tuple[Decimal, ...]) -> TriggerResultV1:
        for index, price in enumerate(path):
            if side is OrderSide.Buy and price <= limitPrice:
                return TriggerResultV1(True, TriggerKind.LimitTouched, min(price, limitPrice), index)
            if side is OrderSide.Sell and price >= limitPrice:
                return TriggerResultV1(True, TriggerKind.LimitTouched, max(price, limitPrice), index)
        return TriggerResultV1(False, TriggerKind.NotTriggered, None, -1)

    def _evaluateStop(self, side: OrderSide, stopPrice: Decimal, path: tuple[Decimal, ...]) -> TriggerResultV1:
        for index, price in enumerate(path):
            if side is OrderSide.Buy and price >= stopPrice:
                # 止损触发后转为市价：不能假设按止损价成交，按当前路径价成交
                return TriggerResultV1(True, TriggerKind.StopActivated, price, index)
            if side is OrderSide.Sell and price <= stopPrice:
                return TriggerResultV1(True, TriggerKind.StopActivated, price, index)
        return TriggerResultV1(False, TriggerKind.NotTriggered, None, -1)

    def _evaluateStopLimit(
        self, side: OrderSide, stopPrice: Decimal, limitPrice: Decimal, path: tuple[Decimal, ...]
    ) -> TriggerResultV1:
        for index, price in enumerate(path):
            if side is OrderSide.Buy and price >= stopPrice:
                if price <= limitPrice:
                    return TriggerResultV1(True, TriggerKind.StopLimitActivated, price, index)
                return TriggerResultV1(False, TriggerKind.NotTriggered, None, -1)
            if side is OrderSide.Sell and price <= stopPrice:
                if price >= limitPrice:
                    return TriggerResultV1(True, TriggerKind.StopLimitActivated, price, index)
                return TriggerResultV1(False, TriggerKind.NotTriggered, None, -1)
        return TriggerResultV1(False, TriggerKind.NotTriggered, None, -1)


def roundToTick(price: Decimal, tickSize: Decimal, side: OrderSide) -> Decimal:
    """派生价格量化：买入向下、卖出向上舍入到 tick。"""
    if not isinstance(price, Decimal) or not isinstance(tickSize, Decimal) or tickSize <= 0:
        raise BarPathError("价格与 tick 必须为正 Decimal")
    if side is OrderSide.Buy:
        return (price / tickSize).quantize(Decimal("1"), rounding=ROUND_DOWN) * tickSize
    return (price / tickSize).quantize(Decimal("1"), rounding=ROUND_UP) * tickSize


def roundQuantityToLot(quantity: Decimal, lotSize: Decimal) -> Decimal:
    """数量始终向下舍入到最小手数。"""
    if not isinstance(quantity, Decimal) or not isinstance(lotSize, Decimal) or lotSize <= 0:
        raise BarPathError("数量与手数必须为正 Decimal")
    return (quantity / lotSize).quantize(Decimal("1"), rounding=ROUND_DOWN) * lotSize


def _validateBar(bar: MinuteBarSchemaV1) -> None:
    if bar.high < bar.low:
        raise BarPathError("Bar high 不得低于 low")
