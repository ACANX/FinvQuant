from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.execution.Liquidity import (
    LiquidityAllocatorV1,
    LiquidityError,
    OrderAllocationInputV1,
    SharedLiquidityPoolV1,
    UnallocatedReason,
)
from veritasquant.execution.Orders import OrderSide, OrderType


def _order(
    orderId: str,
    side: OrderSide = OrderSide.Buy,
    orderType: OrderType = OrderType.Market,
    remaining: Decimal = Decimal("1000"),
    limitPrice: Decimal | None = None,
    pricePriority: Decimal | None = None,
    groupRank: int = 0,
    accountRank: int = 0,
) -> OrderAllocationInputV1:
    return OrderAllocationInputV1(
        clientOrderId=orderId,
        accountGroupRank=groupRank,
        accountRank=accountRank,
        accountId=f"account-{accountRank}",
        side=side,
        orderType=orderType,
        remainingQuantity=remaining,
        limitPrice=limitPrice,
        effectiveOrderingKey=f"key-{pricePriority.value if pricePriority is not None else 0}",
        orderMaxParticipationRate=Decimal("0.5"),
    )


def test_pool_caps_allocations_to_shared_quantity() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    assert pool.poolQuantity == Decimal("1000")
    plan = LiquidityAllocatorV1().allocate(
        planId="plan-1",
        marketEventId="event-1",
        symbol="518880",
        pool=pool,
        orders=(_order("o1", remaining=Decimal("800")), _order("o2", remaining=Decimal("800"))),
        barOpen=Decimal("1.200"),
    )
    # 两订单各受单订单参与率上限（50% = 400），总分配 800 不超过共享池 1000
    assert plan.totalAllocated == Decimal("800")
    assert plan.poolQuantity == Decimal("1000")
    assert pool.remaining == Decimal("200")


def test_market_orders_priority_over_limits() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    limit = _order("limit-1", orderType=OrderType.Limit, remaining=Decimal("600"), limitPrice=Decimal("1.205"))
    market = _order("market-1", remaining=Decimal("600"))
    plan = LiquidityAllocatorV1().allocate(
        planId="plan-1",
        marketEventId="event-1",
        symbol="518880",
        pool=pool,
        orders=(limit, market),
        barOpen=Decimal("1.200"),
    )
    byId = {item.clientOrderId: item for item in plan.allocations}
    # 市价单优先：参与率上限 50% = 300，先分配 300
    assert byId["market-1"].allocatedQuantity == Decimal("300")
    # 池剩余 700 给限价单（限价 1.205 >= open 1.200 触发，参与率上限 300）
    assert byId["limit-1"].allocatedQuantity == Decimal("300")


def test_limit_price_priority_within_limits() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("100000"), Decimal("0.10"))
    higher = _order("high", orderType=OrderType.Limit, remaining=Decimal("500"), limitPrice=Decimal("1.205"))
    lower = _order("low", orderType=OrderType.Limit, remaining=Decimal("500"), limitPrice=Decimal("1.200"))
    plan = LiquidityAllocatorV1().allocate(
        planId="plan-1",
        marketEventId="event-1",
        symbol="518880",
        pool=pool,
        orders=(lower, higher),
        barOpen=Decimal("1.200"),
    )
    byId = {item.clientOrderId: item for item in plan.allocations}
    # 买入限价价格高者优先（买价 1.205 > 1.200），各受参与率上限 50% = 250
    assert byId["high"].allocatedQuantity == Decimal("250")
    assert byId["low"].allocatedQuantity == Decimal("250")


def test_buy_limit_not_met_when_open_above_limit() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    order = _order("o1", orderType=OrderType.Limit, remaining=Decimal("1000"), limitPrice=Decimal("1.150"))
    plan = LiquidityAllocatorV1().allocate(
        planId="plan-1",
        marketEventId="event-1",
        symbol="518880",
        pool=pool,
        orders=(order,),
        barOpen=Decimal("1.200"),
    )
    item = plan.allocations[0]
    assert item.allocatedQuantity == Decimal("0")
    assert item.reason is UnallocatedReason.LimitNotMet


def test_pool_exhausted_reason_recorded() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("1000"), Decimal("0.10"))
    orders = tuple(_order(f"o{i}", remaining=Decimal("100")) for i in range(5))
    plan = LiquidityAllocatorV1().allocate(
        planId="plan-1",
        marketEventId="event-1",
        symbol="518880",
        pool=pool,
        orders=orders,
        barOpen=Decimal("1.200"),
    )
    reasons = [item.reason for item in plan.allocations]
    assert UnallocatedReason.PoolExhausted in reasons
    assert plan.totalAllocated <= Decimal("100")


def test_deterministic_plan_hash_regardless_of_input_order() -> None:
    def build(reversedOrders: bool) -> str:
        pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
        orders = (_order("a", remaining=Decimal("300")), _order("b", remaining=Decimal("300")))
        if reversedOrders:
            orders = (orders[1], orders[0])
        plan = LiquidityAllocatorV1().allocate(
            planId="plan-1",
            marketEventId="event-1",
            symbol="518880",
            pool=pool,
            orders=orders,
            barOpen=Decimal("1.200"),
        )
        return plan.planHash()

    assert build(False) == build(True)


def test_allocation_does_not_depend_on_container_order() -> None:
    poolA = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    poolB = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    ordersA = (_order("a", remaining=Decimal("300")), _order("b", remaining=Decimal("300")))
    ordersB = (ordersA[1], ordersA[0])
    planA = LiquidityAllocatorV1().allocate(
        planId="plan-1", marketEventId="event-1", symbol="518880", pool=poolA, orders=ordersA, barOpen=Decimal("1.200")
    )
    planB = LiquidityAllocatorV1().allocate(
        planId="plan-1", marketEventId="event-1", symbol="518880", pool=poolB, orders=ordersB, barOpen=Decimal("1.200")
    )
    assert planA.allocations == planB.allocations


def test_rejects_mismatched_pool_and_plan_identity() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("10000"), Decimal("0.10"))
    with pytest.raises(LiquidityError, match="标识不一致"):
        LiquidityAllocatorV1().allocate(
            planId="plan-1",
            marketEventId="event-other",
            symbol="518880",
            pool=pool,
            orders=(_order("o1"),),
            barOpen=Decimal("1.200"),
        )


def test_rejects_invalid_pool_inputs() -> None:
    with pytest.raises(LiquidityError, match="marketEventId"):
        SharedLiquidityPoolV1("", "518880", Decimal("1000"), Decimal("0.1"))
    with pytest.raises(LiquidityError, match="成交量"):
        SharedLiquidityPoolV1("event-1", "518880", Decimal("-1"), Decimal("0.1"))
    with pytest.raises(LiquidityError, match="参与率"):
        SharedLiquidityPoolV1("event-1", "518880", Decimal("1000"), Decimal("1.5"))


def test_pool_consume_enforces_shared_boundary() -> None:
    pool = SharedLiquidityPoolV1("event-1", "518880", Decimal("1000"), Decimal("0.10"))
    pool.consume(Decimal("100"))
    with pytest.raises(LiquidityError, match="超过共享池"):
        pool.consume(Decimal("901"))
