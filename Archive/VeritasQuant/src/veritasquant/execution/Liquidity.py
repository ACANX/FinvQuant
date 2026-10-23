"""共享流动性池与确定性多账户分配（技术方案 7.3.2 节）。

每个 market_event_id + symbol 建立唯一 SharedLiquidityPool；可分配数量为
floorToLot(bar.volume * 全局参与率)。所有订单分配之和必须小于等于共享池，
不能按完整 Bar 成交量分别成交。纯函数分配器按全局键排序：市价优先于可
成交限价；限价按价格优先；同价格按有效排序键、账户组、账户、订单 ID 升序。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.BarPath import roundQuantityToLot
from veritasquant.execution.Orders import OrderSide, OrderType


class LiquidityError(ValueError):
    """流动性池或分配计划违反共享池边界时抛出。"""


class UnallocatedReason(StrEnum):
    """订单未分配或未完全分配的原因。"""

    PoolExhausted = "POOL_EXHAUSTED"
    PriceNotTouched = "PRICE_NOT_TOUCHED"
    ParticipationCap = "PARTICIPATION_CAP"
    InsufficientLot = "INSUFFICIENT_LOT"
    LimitNotMet = "LIMIT_NOT_MET"


@dataclass(frozen=True, slots=True)
class OrderAllocationInputV1:
    """分配器输入的订单快照。"""

    clientOrderId: str
    accountGroupRank: int
    accountRank: int
    accountId: str
    side: OrderSide
    orderType: OrderType
    remainingQuantity: Decimal
    limitPrice: Decimal | None
    effectiveOrderingKey: str
    orderMaxParticipationRate: Decimal

    def allocationKey(self) -> tuple[Decimal, int, int, str, str]:
        """全局键：市价优先、限价价格优先、同价按 key/账户组/账户/ID。"""
        pricePriority = Decimal("0") if self.orderType is OrderType.Market else (self.limitPrice or Decimal("0"))
        return (
            pricePriority,
            self.accountGroupRank,
            self.accountRank,
            self.effectiveOrderingKey,
            self.clientOrderId,
        )


@dataclass(frozen=True, slots=True)
class OrderAllocationV1:
    """单订单分配结果。"""

    clientOrderId: str
    accountId: str
    allocatedQuantity: Decimal
    fillPrice: Decimal | None
    reason: UnallocatedReason | None


@dataclass(frozen=True, slots=True)
class LiquidityAllocationPlanV1:
    """不可变分配计划：输入快照哈希、池数量、每单分配与版本。"""

    planId: str
    symbol: str
    marketEventId: str
    inputSnapshotHash: str
    poolQuantity: Decimal
    allocations: tuple[OrderAllocationV1, ...]
    allocationVersion: str

    def planHash(self) -> str:
        """计划身份哈希，供分区原子记账前核对一致性。"""
        return canonicalHash(
            {
                "plan_id": self.planId,
                "symbol": self.symbol,
                "market_event_id": self.marketEventId,
                "input_snapshot_hash": self.inputSnapshotHash,
                "pool_quantity": self.poolQuantity,
                "allocations": [
                    {
                        "client_order_id": item.clientOrderId,
                        "account_id": item.accountId,
                        "allocated_quantity": item.allocatedQuantity,
                        "fill_price": item.fillPrice,
                    }
                    for item in self.allocations
                ],
                "allocation_version": self.allocationVersion,
            }
        )

    @property
    def totalAllocated(self) -> Decimal:
        """总分配量，必须小于等于共享池。"""
        return sum((item.allocatedQuantity for item in self.allocations), Decimal("0"))


class SharedLiquidityPoolV1:
    """单一 market_event_id + symbol 的共享成交量池。"""

    def __init__(self, marketEventId: str, symbol: str, barVolume: Decimal, globalParticipationRate: Decimal) -> None:
        if not marketEventId or not symbol:
            raise LiquidityError("池必须包含 marketEventId 与 symbol")
        if not isinstance(barVolume, Decimal) or barVolume < 0:
            raise LiquidityError("Bar 成交量必须为非负 Decimal")
        if not isinstance(globalParticipationRate, Decimal) or globalParticipationRate < 0 or globalParticipationRate > 1:
            raise LiquidityError("全局参与率必须为 0..1 的 Decimal")
        self._marketEventId = marketEventId
        self._symbol = symbol
        self._poolQuantity = roundQuantityToLot(barVolume * globalParticipationRate, Decimal("1"))
        self._consumed = Decimal("0")

    @property
    def marketEventId(self) -> str:
        return self._marketEventId

    @property
    def symbol(self) -> str:
        return self._symbol

    @property
    def poolQuantity(self) -> Decimal:
        return self._poolQuantity

    @property
    def remaining(self) -> Decimal:
        return self._poolQuantity - self._consumed

    def consume(self, quantity: Decimal) -> None:
        """消费池内数量；超过剩余量即违反共享池边界。"""
        if not isinstance(quantity, Decimal) or quantity < 0:
            raise LiquidityError("消费数量必须为非负 Decimal")
        if quantity > self.remaining:
            raise LiquidityError("分配之和超过共享池剩余量")
        self._consumed += quantity


class LiquidityAllocatorV1:
    """纯函数确定性分配：读取订单快照，按全局键排序逐单分配。"""

    def __init__(self, allocationVersion: str = "V1") -> None:
        if not allocationVersion:
            raise LiquidityError("分配版本不能为空")
        self._allocationVersion = allocationVersion

    def allocate(
        self,
        *,
        planId: str,
        marketEventId: str,
        symbol: str,
        pool: SharedLiquidityPoolV1,
        orders: tuple[OrderAllocationInputV1, ...],
        barOpen: Decimal,
    ) -> LiquidityAllocationPlanV1:
        """生成不可变分配计划；总分配不超过共享池。"""
        if not planId:
            raise LiquidityError("计划 ID 不能为空")
        if not isinstance(barOpen, Decimal) or barOpen <= 0:
            raise LiquidityError("Bar 开盘价必须为正 Decimal")
        if pool.symbol != symbol or pool.marketEventId != marketEventId:
            raise LiquidityError("池与分配请求的标识不一致")
        orderedOrders = sorted(orders, key=lambda item: item.allocationKey())
        inputHash = canonicalHash(
            [
                {
                    "client_order_id": order.clientOrderId,
                    "account_group_rank": order.accountGroupRank,
                    "account_rank": order.accountRank,
                    "account_id": order.accountId,
                    "side": order.side.value,
                    "order_type": order.orderType.value,
                    "remaining_quantity": order.remainingQuantity,
                    "limit_price": order.limitPrice,
                    "effective_ordering_key": order.effectiveOrderingKey,
                    "order_max_participation_rate": order.orderMaxParticipationRate,
                }
                for order in orderedOrders
            ]
        )

        allocations: list[OrderAllocationV1] = []
        for order in orderedOrders:
            if order.remainingQuantity <= 0:
                continue
            if pool.remaining <= 0:
                allocations.append(
                    OrderAllocationV1(order.clientOrderId, order.accountId, Decimal("0"), None, UnallocatedReason.PoolExhausted)
                )
                continue
            cap = roundQuantityToLot(
                order.remainingQuantity * order.orderMaxParticipationRate, Decimal("1")
            )
            if cap <= 0:
                allocations.append(
                    OrderAllocationV1(
                        order.clientOrderId, order.accountId, Decimal("0"), None, UnallocatedReason.ParticipationCap
                    )
                )
                continue

            fillPrice: Decimal | None = None
            if order.orderType is OrderType.Market:
                fillPrice = barOpen
            elif order.limitPrice is not None:
                if order.side is OrderSide.Buy and barOpen <= order.limitPrice:
                    fillPrice = order.limitPrice
                elif order.side is OrderSide.Sell and barOpen >= order.limitPrice:
                    fillPrice = order.limitPrice
                else:
                    allocations.append(
                        OrderAllocationV1(
                            order.clientOrderId, order.accountId, Decimal("0"), None, UnallocatedReason.LimitNotMet
                        )
                    )
                    continue
            else:
                allocations.append(
                    OrderAllocationV1(
                        order.clientOrderId, order.accountId, Decimal("0"), None, UnallocatedReason.PriceNotTouched
                    )
                )
                continue

            allocateQuantity = min(cap, order.remainingQuantity, pool.remaining)
            if allocateQuantity <= 0:
                allocations.append(
                    OrderAllocationV1(
                        order.clientOrderId, order.accountId, Decimal("0"), None, UnallocatedReason.InsufficientLot
                    )
                )
                continue
            pool.consume(allocateQuantity)
            allocations.append(
                OrderAllocationV1(order.clientOrderId, order.accountId, allocateQuantity, fillPrice, None)
            )

        return LiquidityAllocationPlanV1(
            planId=planId,
            symbol=symbol,
            marketEventId=marketEventId,
            inputSnapshotHash=inputHash,
            poolQuantity=pool.poolQuantity,
            allocations=tuple(allocations),
            allocationVersion=self._allocationVersion,
        )
