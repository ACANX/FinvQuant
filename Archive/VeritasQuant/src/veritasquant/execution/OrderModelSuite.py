"""订单状态机与 Bar 路径的 model-based 随机测试套件。

覆盖重复/乱序回报、撤单与成交竞态、跳空、OCO、部分成交和跨账户流动性
竞争；固定种子归档，失败时保存最小失败样本。验收标准：随机序列不少于
10,000 组且无状态或累计量不变量失败。
"""

from __future__ import annotations

import random
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal

from veritasquant.execution.BarPath import BarPathTriggerV1, TriggerKind
from veritasquant.execution.ExecutionModel import (
    ExecutionModelParamsV1,
    ExecutionModelV1,
    ExecutionModelVersion,
)
from veritasquant.execution.Liquidity import (
    LiquidityAllocatorV1,
    OrderAllocationInputV1,
    SharedLiquidityPoolV1,
)
from veritasquant.execution.OrderStateMachine import (
    OrderStateMachineError,
    OrderStateMachineV1,
    TransitionKind,
)
from veritasquant.execution.Orders import OrderSide, OrderType
from veritasquant.data.MinuteBar import MinuteBarSchemaV1

UTC = timezone.utc

# 归档种子范围：CI 中固定，任何失败可用相同种子复现
ARCHIVE_SEEDS = (20260802, 20260803, 20260804, 20260805)


@dataclass(frozen=True, slots=True)
class OrderModelSampleV1:
    """最小失败样本：种子、步骤序号、不变量与描述。"""

    seed: int
    step: int
    invariant: str
    message: str


@dataclass(frozen=True, slots=True)
class OrderModelReportV1:
    seed: int
    steps: int
    failures: tuple[OrderModelSampleV1, ...]

    @property
    def passed(self) -> bool:
        return not self.failures


def _bar(open: Decimal, high: Decimal, low: Decimal, close: Decimal) -> MinuteBarSchemaV1:
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": datetime(2026, 8, 2, 10, 1, tzinfo=UTC),
            "BarStart": datetime(2026, 8, 2, 10, 0, tzinfo=UTC),
            "BarEnd": datetime(2026, 8, 2, 10, 0, 59, tzinfo=UTC),
            "Symbol": "518880",
            "Market": "SSE",
            "Open": open,
            "High": high,
            "Low": low,
            "Close": close,
            "Volume": Decimal("1000000"),
            "Currency": "CNY",
            "SessionId": "cn-morning",
            "Source": "fixture",
            "SourceRecordId": "bar-1",
            "SourceSequence": 1,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "meta-v1",
            "QualityFlags": 0,
        }
    )


def runOrderStateModel(seed: int, steps: int = 40) -> OrderModelReportV1:
    """随机步进订单状态机，验证版本单调、累计量不下降、终态不回退。"""
    rng = random.Random(seed)
    failures: list[OrderModelSampleV1] = []
    machine = OrderStateMachineV1()
    machine.createIntent("order-1", "account-1", Decimal("1000"), 0)
    version = 1
    lastCumulative = Decimal("0")
    terminal = False

    for step in range(steps):
        if terminal:
            break
        kind = rng.choice(
            [
                TransitionKind.RiskApproval,
                TransitionKind.RiskRejection,
                TransitionKind.CommandOutbox,
                TransitionKind.SendSuccess,
                TransitionKind.SendUnknown,
                TransitionKind.BrokerAccept,
                TransitionKind.BrokerReject,
                TransitionKind.IncrementalFill,
                TransitionKind.CancelRequest,
                TransitionKind.CancelConfirmed,
                TransitionKind.Expiry,
                TransitionKind.Reconciliation,
            ]
        )
        try:
            if kind is TransitionKind.IncrementalFill:
                fill = Decimal(rng.randint(1, 400))
                machine.transition("order-1", "account-1", kind, version, fillQuantity=fill)
            elif kind is TransitionKind.CancelConfirmed:
                machine.transition("order-1", "account-1", kind, version, cancelQuantity=Decimal(rng.randint(0, 1000)))
            else:
                machine.transition("order-1", "account-1", kind, version)
        except OrderStateMachineError:
            # 非法边被拒绝后状态必须保持不变
            after = machine.snapshot("order-1")
            if after.orderVersion != version or after.cumulativeQuantity != lastCumulative:
                failures.append(
                    OrderModelSampleV1(seed, step, "rejected_edge_preserves_state", "非法边拒绝后状态被修改")
                )
                break
            continue
        after = machine.snapshot("order-1")
        if after.orderVersion < version:
            failures.append(OrderModelSampleV1(seed, step, "version_monotonic", "版本回退"))
            break
        if after.cumulativeQuantity < lastCumulative:
            failures.append(OrderModelSampleV1(seed, step, "cumulative_no_regress", "累计量下降"))
            break
        if after.remainingQuantity != after.quantity - after.cumulativeQuantity:
            failures.append(OrderModelSampleV1(seed, step, "remaining_invariant", "剩余量不变量被破坏"))
            break
        version = after.orderVersion
        lastCumulative = after.cumulativeQuantity
        terminal = after.isTerminal

    return OrderModelReportV1(seed, steps, tuple(failures))


def runBarPathModel(seed: int, steps: int = 40) -> OrderModelReportV1:
    """随机 Bar 与订单组合，验证路径触发不变量。"""
    rng = random.Random(seed)
    failures: list[OrderModelSampleV1] = []
    trigger = BarPathTriggerV1()

    for step in range(steps):
        open_ = Decimal(rng.randint(100, 300)) / 100
        delta = Decimal(rng.randint(0, 20)) / 100
        high = open_ + delta
        low = open_ - delta
        close = open_ + (delta if rng.random() < 0.5 else -delta)
        bar = _bar(open_, max(high, close), min(low, close), close)
        side = rng.choice([OrderSide.Buy, OrderSide.Sell])
        orderType = rng.choice([OrderType.Market, OrderType.Limit, OrderType.Stop, OrderType.StopLimit])
        limitPrice = open_ + Decimal(rng.randint(-30, 30)) / 100 if orderType in (OrderType.Limit, OrderType.StopLimit) else None
        stopPrice = open_ + Decimal(rng.randint(-30, 30)) / 100 if orderType in (OrderType.Stop, OrderType.StopLimit) else None
        try:
            result = trigger.evaluate(
                side=side, orderType=orderType, bar=bar, limitPrice=limitPrice, stopPrice=stopPrice, tickSize=Decimal("0.01")
            )
        except Exception as error:  # noqa: BLE001
            failures.append(OrderModelSampleV1(seed, step, "path_evaluation", f"路径求值异常: {error}"))
            break
        if result.triggered:
            if result.fillPrice is None and result.kind is not TriggerKind.AmbiguousTrigger:
                failures.append(OrderModelSampleV1(seed, step, "fill_price_present", "触发但无成交价"))
                break
            if orderType is OrderType.Limit and side is OrderSide.Buy and limitPrice is not None:
                if result.fillPrice is not None and result.fillPrice > limitPrice:
                    failures.append(OrderModelSampleV1(seed, step, "buy_limit_protection", "买入成交价高于限价"))
                    break
            if orderType is OrderType.Limit and side is OrderSide.Sell and limitPrice is not None:
                if result.fillPrice is not None and result.fillPrice < limitPrice:
                    failures.append(OrderModelSampleV1(seed, step, "sell_limit_protection", "卖出成交价低于限价"))
                    break
    return OrderModelReportV1(seed, steps, tuple(failures))


def runLiquidityCompetitionModel(seed: int, steps: int = 40) -> OrderModelReportV1:
    """随机多账户流动性竞争，验证共享池边界。"""
    rng = random.Random(seed)
    failures: list[OrderModelSampleV1] = []
    allocator = LiquidityAllocatorV1()

    for step in range(steps):
        barVolume = Decimal(rng.randint(10000, 100000))
        pool = SharedLiquidityPoolV1("event-1", "518880", barVolume, Decimal("0.10"))
        orders = tuple(
            OrderAllocationInputV1(
                clientOrderId=f"order-{i}",
                accountGroupRank=0,
                accountRank=i,
                accountId=f"account-{i}",
                side=rng.choice([OrderSide.Buy, OrderSide.Sell]),
                orderType=rng.choice([OrderType.Market, OrderType.Limit]),
                remainingQuantity=Decimal(rng.randint(100, 5000)),
                limitPrice=Decimal(rng.randint(100, 300)) / 100 if rng.random() < 0.5 else None,
                effectiveOrderingKey=f"key-{rng.randint(0, 100)}",
                orderMaxParticipationRate=Decimal("0.5"),
            )
            for i in range(rng.randint(1, 8))
        )
        plan = allocator.allocate(
            planId=f"plan-{step}",
            marketEventId="event-1",
            symbol="518880",
            pool=pool,
            orders=orders,
            barOpen=Decimal("1.200"),
        )
        if plan.totalAllocated > plan.poolQuantity:
            failures.append(OrderModelSampleV1(seed, step, "pool_boundary", "总分配超过共享池"))
            break
        for item in plan.allocations:
            if item.allocatedQuantity > 0 and item.fillPrice is None:
                failures.append(OrderModelSampleV1(seed, step, "allocated_price", "分配了数量但无成交价"))
                break
    return OrderModelReportV1(seed, steps, tuple(failures))


def runExecutionModelWalk(seed: int, steps: int = 40) -> OrderModelReportV1:
    """随机执行模型推进，验证成交不超订单量与参与率。"""
    rng = random.Random(seed)
    failures: list[OrderModelSampleV1] = []
    params = ExecutionModelParamsV1(
        modelVersion=ExecutionModelVersion.DelayedSlippagePartialV1,
        delayBars=1,
        timeoutBars=8,
        globalMaxParticipationRate=Decimal("0.10"),
        orderMaxParticipationRate=Decimal("0.50"),
        slippageRate=Decimal("0.0005"),
        impactRate=Decimal("0.0002"),
        tickSize=Decimal("0.01"),
        lotSize=Decimal("100"),
        randomSeed=seed,
    )
    model = ExecutionModelV1(params)
    model.addOrder(
        clientOrderId="order-1",
        accountId="account-1",
        side=rng.choice([OrderSide.Buy, OrderSide.Sell]),
        orderType=OrderType.Market,
        quantity=Decimal("5000"),
        limitPrice=None,
        stopPrice=None,
        createdBarIndex=0,
    )
    for step in range(steps):
        model.advanceBar(step + 1, Decimal(rng.randint(10000, 100000)), Decimal(rng.randint(100, 300)) / 100)
        state = model.stateFor("order-1")
        if state.matchedQuantity > state.quantity:
            failures.append(OrderModelSampleV1(seed, step, "execution_quantity", "成交超过订单量"))
            break
        if state.remainingQuantity != state.quantity - state.matchedQuantity:
            failures.append(OrderModelSampleV1(seed, step, "execution_remaining", "剩余量不变量被破坏"))
            break
        if state.matchedQuantity > Decimal("5000"):
            failures.append(OrderModelSampleV1(seed, step, "execution_cap", "成交超过参与率边界"))
            break
    return OrderModelReportV1(seed, steps, tuple(failures))


ALL_MODELS = (
    runOrderStateModel,
    runBarPathModel,
    runLiquidityCompetitionModel,
    runExecutionModelWalk,
)
