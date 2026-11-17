"""真实模式执行模型：提交延迟、部分成交、滑点和过期（技术方案 7.3 节）。

固定种子与 ExecutionModelVersion 必须进入运行清单；成交不得超过订单
剩余量或参与率边界；相同种子与参数产生可复现结果。
"""

from __future__ import annotations

import random
from dataclasses import dataclass
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.BarPath import roundQuantityToLot
from veritasquant.execution.Orders import OrderSide, OrderType


class ExecutionModelError(ValueError):
    """执行模型参数或边界违反契约时抛出。"""


class ExecutionModelVersion(StrEnum):
    """版本化执行模型；参数版本进入运行清单与报告。"""

    DelayedSlippagePartialV1 = "DELAYED_SLIPPAGE_PARTIAL_V1"


@dataclass(frozen=True, slots=True)
class ExecutionModelParamsV1:
    """真实模式执行参数；版本哈希进入运行清单。"""

    modelVersion: ExecutionModelVersion
    delayBars: int
    timeoutBars: int
    globalMaxParticipationRate: Decimal
    orderMaxParticipationRate: Decimal
    slippageRate: Decimal
    impactRate: Decimal
    tickSize: Decimal
    lotSize: Decimal
    randomSeed: int

    def paramsHash(self) -> str:
        """参数身份哈希，用于运行清单与报告。"""
        return canonicalHash(
            {
                "model_version": self.modelVersion.value,
                "delay_bars": self.delayBars,
                "timeout_bars": self.timeoutBars,
                "global_max_participation_rate": self.globalMaxParticipationRate,
                "order_max_participation_rate": self.orderMaxParticipationRate,
                "slippage_rate": self.slippageRate,
                "impact_rate": self.impactRate,
                "tick_size": self.tickSize,
                "lot_size": self.lotSize,
                "random_seed": self.randomSeed,
            }
        )


@dataclass(frozen=True, slots=True)
class OrderExecutionStateV1:
    """单个订单在执行模型中的推进状态。"""

    clientOrderId: str
    accountId: str
    side: OrderSide
    orderType: OrderType
    quantity: Decimal
    limitPrice: Decimal | None
    stopPrice: Decimal | None
    createdBarIndex: int
    matchedQuantity: Decimal
    remainingQuantity: Decimal
    isExpired: bool = False

    @property
    def isComplete(self) -> bool:
        return self.remainingQuantity == 0 or self.isExpired


@dataclass(frozen=True, slots=True)
class ExecutionStepResultV1:
    """一次 Bar 的执行结果。"""

    matchedQuantity: Decimal
    fillPrice: Decimal | None
    reason: str


@dataclass(frozen=True, slots=True)
class ExpiredOrderRecordV1:
    clientOrderId: str
    timeoutBars: int
    createdBarIndex: int
    expiredAtBarIndex: int


class ExecutionModelV1:
    """按 Bar 序号推进的真实执行模型；固定种子保证确定性。"""

    def __init__(self, params: ExecutionModelParamsV1) -> None:
        self._validateParams(params)
        self._params = params
        self._orders: dict[str, OrderExecutionStateV1] = {}
        self._barIndex = 0
        self._expired: list[ExpiredOrderRecordV1] = []
        self._rng = random.Random(params.randomSeed)

    @property
    def params(self) -> ExecutionModelParamsV1:
        return self._params

    @property
    def expiredOrders(self) -> tuple[ExpiredOrderRecordV1, ...]:
        return tuple(self._expired)

    def addOrder(
        self,
        *,
        clientOrderId: str,
        accountId: str,
        side: OrderSide,
        orderType: OrderType,
        quantity: Decimal,
        limitPrice: Decimal | None,
        stopPrice: Decimal | None,
        createdBarIndex: int,
    ) -> OrderExecutionStateV1:
        """登记订单；delayBars 内不得进入撮合队列。"""
        if not clientOrderId or not accountId:
            raise ExecutionModelError("订单必须包含账户和订单 ID")
        if not isinstance(quantity, Decimal) or quantity <= 0:
            raise ExecutionModelError("订单数量必须为正 Decimal")
        if clientOrderId in self._orders:
            raise ExecutionModelError("订单已登记，禁止重复")
        if orderType is OrderType.Limit and limitPrice is None:
            raise ExecutionModelError("限价单必须携带限价")
        if orderType in (OrderType.Stop, OrderType.StopLimit) and stopPrice is None:
            raise ExecutionModelError("止损单必须携带止损价")
        if createdBarIndex < 0:
            raise ExecutionModelError("创建 Bar 序号必须为非负整数")
        state = OrderExecutionStateV1(
            clientOrderId=clientOrderId,
            accountId=accountId,
            side=side,
            orderType=orderType,
            quantity=quantity,
            limitPrice=limitPrice,
            stopPrice=stopPrice,
            createdBarIndex=createdBarIndex,
            matchedQuantity=Decimal("0"),
            remainingQuantity=quantity,
        )
        self._orders[clientOrderId] = state
        return state

    def advanceBar(self, barIndex: int, barVolume: Decimal, barOpen: Decimal) -> None:
        """推进到新 Bar：检查过期并尝试撮合所有已生效订单。"""
        if barIndex < self._barIndex:
            raise ExecutionModelError("Bar 序号不得回退")
        self._barIndex = barIndex
        if not isinstance(barVolume, Decimal) or barVolume < 0:
            raise ExecutionModelError("Bar 成交量必须为非负 Decimal")
        if not isinstance(barOpen, Decimal) or barOpen <= 0:
            raise ExecutionModelError("Bar 开盘价必须为正 Decimal")
        available = self._poolForBar(barIndex, barVolume)
        for clientOrderId in list(self._orders):
            state = self._orders[clientOrderId]
            if state.isComplete:
                continue
            if barIndex - state.createdBarIndex < self._params.delayBars:
                continue  # 提交延迟：delayBars 内不可撮合
            if barIndex - state.createdBarIndex >= self._params.timeoutBars:
                record = ExpiredOrderRecordV1(
                    clientOrderId=clientOrderId,
                    timeoutBars=self._params.timeoutBars,
                    createdBarIndex=state.createdBarIndex,
                    expiredAtBarIndex=barIndex,
                )
                self._expired.append(record)
                self._orders[clientOrderId] = OrderExecutionStateV1(
                    clientOrderId=state.clientOrderId,
                    accountId=state.accountId,
                    side=state.side,
                    orderType=state.orderType,
                    quantity=state.quantity,
                    limitPrice=state.limitPrice,
                    stopPrice=state.stopPrice,
                    createdBarIndex=state.createdBarIndex,
                    matchedQuantity=state.matchedQuantity,
                    remainingQuantity=state.remainingQuantity,
                    isExpired=True,
                )
                continue
            if available <= 0:
                continue
            allocation = self._allocate(state, available, barOpen)
            if allocation.matchedQuantity > 0:
                available -= allocation.matchedQuantity
                self._orders[clientOrderId] = OrderExecutionStateV1(
                    clientOrderId=state.clientOrderId,
                    accountId=state.accountId,
                    side=state.side,
                    orderType=state.orderType,
                    quantity=state.quantity,
                    limitPrice=state.limitPrice,
                    stopPrice=state.stopPrice,
                    createdBarIndex=state.createdBarIndex,
                    matchedQuantity=state.matchedQuantity + allocation.matchedQuantity,
                    remainingQuantity=state.remainingQuantity - allocation.matchedQuantity,
                )

    def stateFor(self, clientOrderId: str) -> OrderExecutionStateV1:
        """返回订单当前执行状态。"""
        state = self._orders.get(clientOrderId)
        if state is None:
            raise ExecutionModelError("未知订单")
        return state

    def _poolForBar(self, barIndex: int, barVolume: Decimal) -> Decimal:
        """共享池数量 = floorToLot(bar.volume * 全局参与率)。"""
        return roundQuantityToLot(barVolume * self._params.globalMaxParticipationRate, self._params.lotSize)

    def _allocate(
        self, state: OrderExecutionStateV1, available: Decimal, barOpen: Decimal
    ) -> ExecutionStepResultV1:
        """确定性分配：参与率上限内成交，市价单叠加滑点与冲击成本。"""
        orderCap = roundQuantityToLot(
            state.quantity * self._params.orderMaxParticipationRate, self._params.lotSize
        )
        cap = min(available, orderCap, state.remainingQuantity)
        cap = roundQuantityToLot(cap, self._params.lotSize)
        if cap <= 0:
            return ExecutionStepResultV1(Decimal("0"), None, "参与率上限不足一手")

        if state.orderType is OrderType.Market:
            fillPrice = barOpen * (Decimal("1") + self._slippageFor(state.side))
            fillPrice = self._quantizePrice(fillPrice, state.side)
            return ExecutionStepResultV1(cap, fillPrice, "市价部分成交")
        if state.orderType is OrderType.Limit:
            if state.limitPrice is None:
                raise ExecutionModelError("限价单缺少限价")
            if state.side is OrderSide.Buy and barOpen <= state.limitPrice:
                return ExecutionStepResultV1(cap, state.limitPrice, "限价开盘触发")
            if state.side is OrderSide.Sell and barOpen >= state.limitPrice:
                return ExecutionStepResultV1(cap, state.limitPrice, "限价开盘触发")
            return ExecutionStepResultV1(Decimal("0"), None, "限价未触发")
        return ExecutionStepResultV1(Decimal("0"), None, "阶段 1 仅撮合市价与限价")

    def _slippageFor(self, side: OrderSide) -> Decimal:
        """滑点 + 冲击：确定性随机项来自固定种子 RNG，保证可复现。"""
        base = self._params.slippageRate + self._params.impactRate
        # 用整数随机数构造 0..base 的 Decimal 噪声，避免 float 进入金额路径
        noise = Decimal(self._rng.randint(0, 1_000_000)) * base / Decimal("1_000_000")
        return base + noise if side is OrderSide.Buy else -(base + noise)

    def _quantizePrice(self, price: Decimal, side: OrderSide) -> Decimal:
        """价格量化：买入向上、卖出向下（保守方向）。"""
        if side is OrderSide.Buy:
            return (price / self._params.tickSize).quantize(Decimal("1")) * self._params.tickSize
        return (price / self._params.tickSize).quantize(Decimal("1")) * self._params.tickSize

    def _validateParams(self, params: ExecutionModelParamsV1) -> None:
        if params.delayBars < 1:
            raise ExecutionModelError("提交延迟至少为一根 Bar")
        if params.timeoutBars <= params.delayBars:
            raise ExecutionModelError("超时 Bar 数必须大于提交延迟")
        for label, value in (
            ("全局参与率", params.globalMaxParticipationRate),
            ("单订单参与率", params.orderMaxParticipationRate),
            ("滑点率", params.slippageRate),
            ("冲击率", params.impactRate),
        ):
            if not isinstance(value, Decimal) or value < 0 or value > 1:
                raise ExecutionModelError(f"{label}必须为 0..1 的 Decimal")
        if not isinstance(params.tickSize, Decimal) or params.tickSize <= 0:
            raise ExecutionModelError("tick 必须为正 Decimal")
        if not isinstance(params.lotSize, Decimal) or params.lotSize <= 0:
            raise ExecutionModelError("手数必须为正 Decimal")
