"""BaseStrategy、只读 StrategyContext 和回调协议（技术方案 4.4 节）。

策略只能访问自身订阅的已发生事件、指标窗口、只读标的元数据及当前虚拟
分账户；只能返回 OrderIntent；不得获得未来游标、可写账户、数据库或券商
接口。策略禁止直接调用券商接口、修改账户、读取未来数据或绕过 RiskEngine。
"""

from __future__ import annotations

from copy import deepcopy
from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal
from enum import StrEnum
from typing import Any

from veritasquant.execution.Orders import (
    OrderIntentV1,
    OrderSide,
    OrderType,
    PositionEffect,
    TimeInForce,
)


class StrategyContractError(ValueError):
    """策略违反只读上下文或回调协议时抛出。"""


class StrategyState(StrEnum):
    Initialized = "INITIALIZED"
    Running = "RUNNING"
    Suspended = "SUSPENDED"
    Stopped = "STOPPED"


@dataclass(frozen=True, slots=True)
class StrategySnapshotV1:
    """策略可读的不可变账户快照（本账户已固化状态）。"""

    accountId: str
    subaccountId: str
    cashAvailable: Decimal
    positions: dict[str, Decimal]
    snapshotVersion: int


@dataclass(frozen=True, slots=True)
class StrategyInstrumentV1:
    """只读版本化标的元数据。"""

    symbol: str
    metadataVersion: str
    tickSize: Decimal
    lotSize: Decimal
    currency: str


@dataclass(frozen=True, slots=True)
class ConsumedEventV1:
    """策略已消费的事件（只读，不可写）。"""

    eventId: str
    eventType: str
    ts: datetime
    payload: dict[str, Any]


class StrategyContextV1:
    """只读上下文：不暴露未来索引、可写账户或券商接口。"""

    def __init__(
        self,
        *,
        strategyId: str,
        strategyVersion: str,
        runId: str,
        accountId: str,
        subaccountId: str,
        snapshot: StrategySnapshotV1,
        instrument: StrategyInstrumentV1,
    ) -> None:
        self._strategyId = strategyId
        self._strategyVersion = strategyVersion
        self._runId = runId
        self._accountId = accountId
        self._subaccountId = subaccountId
        self._snapshot = snapshot
        self._instrument = instrument
        self._consumedEvents: list[ConsumedEventV1] = []
        self._clock: datetime | None = None

    @property
    def strategyId(self) -> str:
        return self._strategyId

    @property
    def accountId(self) -> str:
        return self._accountId

    @property
    def subaccountId(self) -> str:
        return self._subaccountId

    @property
    def currentSnapshot(self) -> StrategySnapshotV1:
        """只读快照；返回深拷贝防止策略篡改内部状态。"""
        return deepcopy(self._snapshot)

    @property
    def instrument(self) -> StrategyInstrumentV1:
        return self._instrument

    @property
    def clock(self) -> datetime | None:
        """当前逻辑时间；未推进时为 None（禁止用系统时间）。"""
        return self._clock

    def consume(self, event: ConsumedEventV1) -> None:
        """登记一个已消费事件；时间必须单调前进。"""
        if self._clock is not None and event.ts < self._clock:
            raise StrategyContractError("策略不得消费时间回退的事件")
        self._consumedEvents.append(event)
        self._clock = event.ts

    def consumedEvents(self) -> tuple[ConsumedEventV1, ...]:
        """已消费事件窗口（只读副本）。"""
        return tuple(self._consumedEvents)

    def consumedCount(self) -> int:
        return len(self._consumedEvents)

    def latestEvent(self) -> ConsumedEventV1 | None:
        """最近消费的事件。"""
        return self._consumedEvents[-1] if self._consumedEvents else None


class BaseStrategy:
    """策略基类：回调只能通过 createOrder 返回意图，禁止触碰账户/券商。"""

    strategyId: str = "base"
    strategyVersion: str = "1.0.0"

    def __init__(self) -> None:
        self._context: StrategyContextV1 | None = None
        self._state = StrategyState.Initialized
        self._intents: list[OrderIntentV1] = []

    @property
    def state(self) -> StrategyState:
        return self._state

    def bind(self, context: StrategyContextV1) -> None:
        """宿主在运行前绑定只读上下文。"""
        self._context = context
        self._state = StrategyState.Running

    def onBar(self, event: ConsumedEventV1) -> None:
        """行情 Bar 回调；子类覆写。"""

    def onEvent(self, event: ConsumedEventV1) -> None:
        """通用事件回调；子类覆写。"""

    def onFill(self, event: ConsumedEventV1) -> None:
        """成交回调；子类覆写。"""

    def onPartialFill(self, event: ConsumedEventV1) -> None:
        """部分成交回调；子类覆写。"""

    def onOrderCancelled(self, event: ConsumedEventV1) -> None:
        """撤单回调；子类覆写。"""

    def createOrder(
        self,
        symbol: str,
        side: OrderSide,
        quantity: Decimal,
        orderType: OrderType = OrderType.Limit,
        price: Decimal | None = None,
        positionEffect: PositionEffect = PositionEffect.Open,
        timeInForce: TimeInForce = TimeInForce.Day,
    ) -> OrderIntentV1:
        """只返回 OrderIntent；不直接发单、不改账户、不碰券商。"""
        if self._context is None or self._state is not StrategyState.Running:
            raise StrategyContractError("策略未绑定上下文或未运行")
        context = self._context
        if symbol != context.instrument.symbol:
            raise StrategyContractError("策略只能交易订阅的标的")
        if not isinstance(quantity, Decimal) or quantity <= 0:
            raise StrategyContractError("数量必须为正 Decimal")
        if orderType in (OrderType.Limit, OrderType.StopLimit) and price is None:
            raise StrategyContractError("限价类订单必须提供价格")
        if orderType is OrderType.Market and price is not None:
            raise StrategyContractError("市价单不得携带价格")
        if quantity % context.instrument.lotSize != 0:
            raise StrategyContractError("数量必须符合最小手数")
        latest = context.latestEvent()
        if latest is None:
            raise StrategyContractError("策略不得在没有已消费事件时下单")
        intentId = f"{self.strategyId}-{context.consumedCount()}-{len(self._intents) + 1}"
        intent = OrderIntentV1.model_validate(
            {
                "IntentId": intentId,
                "RunId": context._runId if hasattr(context, "_runId") else "run-1",
                "AccountId": context.accountId,
                "SubaccountId": context.subaccountId,
                "StrategyId": self.strategyId,
                "StrategyVersion": self.strategyVersion,
                "Symbol": symbol,
                "InstrumentMetadataVersion": context.instrument.metadataVersion,
                "Side": side,
                "PositionEffect": positionEffect,
                "OrderType": orderType,
                "Quantity": quantity,
                "TimeInForce": timeInForce,
                "Ts": latest.ts,
                "CreatedFromEventId": latest.eventId,
                "ExpectedAccountVersion": context.currentSnapshot.snapshotVersion,
                "LimitPrice": price if orderType in (OrderType.Limit, OrderType.StopLimit) else None,
                "StopPrice": None,
            }
        )
        self._intents.append(intent)
        return intent

    def emitIntents(self) -> tuple[OrderIntentV1, ...]:
        """宿主取走本回调产生的全部意图并清空。"""
        intents = tuple(self._intents)
        self._intents.clear()
        return intents

    def suspend(self) -> None:
        """宿主因超时/资源超限把策略置为 SUSPENDED。"""
        self._state = StrategyState.Suspended

    def stop(self) -> None:
        self._state = StrategyState.Stopped


class MovingAverageCrossStrategy(BaseStrategy):
    """示例策略：均线交叉（供 P1-064 使用的基础实现）。"""

    strategyId = "moving_average_cross"
    strategyVersion = "1.0.0"

    def __init__(self, fastPeriod: int = 5, slowPeriod: int = 20) -> None:
        super().__init__()
        self._fastPeriod = fastPeriod
        self._slowPeriod = slowPeriod
        self._closes: list[Decimal] = []

    def onBar(self, event: ConsumedEventV1) -> None:
        if self._context is not None:
            self._context.consume(event)
        close = Decimal(str(event.payload.get("close", "0")))
        self._closes.append(close)
        if len(self._closes) < self._slowPeriod + 1:
            return
        fastNow = self._average(self._closes[-self._fastPeriod :])
        slowNow = self._average(self._closes[-self._slowPeriod :])
        fastPrev = self._average(self._closes[-self._fastPeriod - 1 : -1])
        slowPrev = self._average(self._closes[-self._slowPeriod - 1 : -1])
        if fastPrev <= slowPrev and fastNow > slowNow:
            # 金叉：买入
            self.createOrder(
                self._context.instrument.symbol if self._context else "518880",
                OrderSide.Buy,
                Decimal("100"),
                orderType=OrderType.Market,
            )
        elif fastPrev >= slowPrev and fastNow < slowNow:
            self.createOrder(
                self._context.instrument.symbol if self._context else "518880",
                OrderSide.Sell,
                Decimal("100"),
                orderType=OrderType.Market,
            )

    def _average(self, values: list[Decimal]) -> Decimal:
        return sum(values, Decimal("0")) / Decimal(len(values))
