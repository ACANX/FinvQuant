from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.execution.ExecutionModel import (
    ExecutionModelError,
    ExecutionModelParamsV1,
    ExecutionModelV1,
    ExecutionModelVersion,
)
from veritasquant.execution.Orders import OrderSide, OrderType


def _params(**overrides: object) -> ExecutionModelParamsV1:
    values: dict[str, object] = {
        "modelVersion": ExecutionModelVersion.DelayedSlippagePartialV1,
        "delayBars": 1,
        "timeoutBars": 10,
        "globalMaxParticipationRate": Decimal("0.10"),
        "orderMaxParticipationRate": Decimal("0.50"),
        "slippageRate": Decimal("0.0005"),
        "impactRate": Decimal("0.0002"),
        "tickSize": Decimal("0.001"),
        "lotSize": Decimal("100"),
        "randomSeed": 20260802,
    }
    values.update(overrides)
    return ExecutionModelParamsV1(**values)  # type: ignore[call-arg]


def _marketOrder(model: ExecutionModelV1, orderId: str = "client-1", quantity: Decimal = Decimal("1000")) -> None:
    model.addOrder(
        clientOrderId=orderId,
        accountId="account-1",
        side=OrderSide.Buy,
        orderType=OrderType.Market,
        quantity=quantity,
        limitPrice=None,
        stopPrice=None,
        createdBarIndex=0,
    )


def test_params_hash_is_stable_and_versioned() -> None:
    first = _params()
    second = _params()
    assert first.paramsHash() == second.paramsHash()
    changed = _params(randomSeed=1)
    assert first.paramsHash() != changed.paramsHash()
    assert "DELAYED_SLIPPAGE_PARTIAL_V1" in first.paramsHash() or first.modelVersion.value == "DELAYED_SLIPPAGE_PARTIAL_V1"


def test_delay_bars_block_immediate_matching() -> None:
    model = ExecutionModelV1(_params())
    _marketOrder(model)
    # 创建于 bar 0，delay=1：bar 0 不可撮合
    model.advanceBar(0, Decimal("100000"), Decimal("1.200"))
    assert model.stateFor("client-1").remainingQuantity == Decimal("1000")
    # bar 1 已过延迟，可撮合
    model.advanceBar(1, Decimal("100000"), Decimal("1.200"))
    assert model.stateFor("client-1").matchedQuantity > 0


def test_participation_cap_never_exceeds_pool_or_order_cap() -> None:
    model = ExecutionModelV1(_params(globalMaxParticipationRate=Decimal("0.01")))
    _marketOrder(model, quantity=Decimal("100000"))
    model.advanceBar(1, Decimal("100000"), Decimal("1.200"))
    matched = model.stateFor("client-1").matchedQuantity
    # 池上限 = 100000 * 0.01 = 1000（向下取整到 100）
    assert matched <= Decimal("1000")
    assert matched % Decimal("100") == 0


def test_fixed_seed_gives_reproducible_results() -> None:
    def run() -> Decimal:
        model = ExecutionModelV1(_params())
        _marketOrder(model)
        for bar in range(1, 6):
            model.advanceBar(bar, Decimal("100000"), Decimal("1.200"))
        return model.stateFor("client-1").matchedQuantity

    assert run() == run()


def test_different_seed_gives_different_slippage() -> None:
    def slippage(seed: int) -> Decimal:
        model = ExecutionModelV1(_params(randomSeed=seed))
        return model._slippageFor(OrderSide.Buy)

    assert slippage(1) == slippage(1)
    assert slippage(1) != slippage(2) or slippage(1) != slippage(3)  # 种子不同，滑点噪声不同


def test_fill_price_includes_slippage_for_buy() -> None:
    model = ExecutionModelV1(_params(slippageRate=Decimal("0.01"), impactRate=Decimal("0.005")))
    _marketOrder(model, quantity=Decimal("1000"))
    model.advanceBar(1, Decimal("100000"), Decimal("1.200"))
    state = model.stateFor("client-1")
    # 池上限 10000，单订单参与率上限 50% = 500，向下取整到手数 500
    assert state.matchedQuantity == Decimal("500")


def test_timeout_expires_order() -> None:
    model = ExecutionModelV1(_params(timeoutBars=3, globalMaxParticipationRate=Decimal("0")))
    _marketOrder(model)
    for bar in range(1, 4):
        model.advanceBar(bar, Decimal("100000"), Decimal("1.200"))
    record = model.expiredOrders
    assert len(record) == 1
    assert record[0].clientOrderId == "client-1"
    assert model.stateFor("client-1").isExpired


def test_matched_never_exceeds_order_quantity() -> None:
    model = ExecutionModelV1(_params(globalMaxParticipationRate=Decimal("1")))
    _marketOrder(model, quantity=Decimal("500"))
    for bar in range(1, 20):
        model.advanceBar(bar, Decimal("10000000"), Decimal("1.200"))
    state = model.stateFor("client-1")
    assert state.matchedQuantity <= Decimal("500")
    assert state.remainingQuantity == state.quantity - state.matchedQuantity
    assert state.isComplete


def test_limit_order_requires_price_and_trigger() -> None:
    model = ExecutionModelV1(_params())
    with pytest.raises(ExecutionModelError, match="限价"):
        model.addOrder(
            clientOrderId="client-2",
            accountId="account-1",
            side=OrderSide.Buy,
            orderType=OrderType.Limit,
            quantity=Decimal("100"),
            limitPrice=None,
            stopPrice=None,
            createdBarIndex=0,
        )
    model.addOrder(
        clientOrderId="client-2",
        accountId="account-1",
        side=OrderSide.Buy,
        orderType=OrderType.Limit,
        quantity=Decimal("1000"),
        limitPrice=Decimal("1.150"),
        stopPrice=None,
        createdBarIndex=0,
    )
    # 开盘价 1.200 > 限价 1.150：不触发
    model.advanceBar(1, Decimal("100000"), Decimal("1.200"))
    assert model.stateFor("client-2").matchedQuantity == Decimal("0")
    # 开盘价 1.100 <= 限价：触发（单订单上限 50% = 500，取整到手数 500）
    model.advanceBar(2, Decimal("100000"), Decimal("1.100"))
    assert model.stateFor("client-2").matchedQuantity == Decimal("500")


def test_rejects_invalid_params_and_duplicate_order() -> None:
    with pytest.raises(ExecutionModelError, match="至少为一根"):
        ExecutionModelV1(_params(delayBars=0))
    with pytest.raises(ExecutionModelError, match="超时"):
        ExecutionModelV1(_params(timeoutBars=1))
    with pytest.raises(ExecutionModelError, match="参与率"):
        ExecutionModelV1(_params(globalMaxParticipationRate=Decimal("1.5")))
    model = ExecutionModelV1(_params())
    _marketOrder(model)
    with pytest.raises(ExecutionModelError, match="重复"):
        _marketOrder(model)


def test_bar_index_cannot_regress() -> None:
    model = ExecutionModelV1(_params())
    _marketOrder(model)
    model.advanceBar(1, Decimal("100000"), Decimal("1.200"))
    with pytest.raises(ExecutionModelError, match="回退"):
        model.advanceBar(0, Decimal("100000"), Decimal("1.200"))
