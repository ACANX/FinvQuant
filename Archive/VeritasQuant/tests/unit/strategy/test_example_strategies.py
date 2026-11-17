from __future__ import annotations

from decimal import Decimal

from veritasquant.execution.Orders import OrderSide
from veritasquant.strategy.BaseStrategy import (
    MovingAverageCrossStrategy,
    StrategyContextV1,
    StrategyInstrumentV1,
    StrategySnapshotV1,
)
from veritasquant.strategy.ExampleStrategies import (
    DailyMomentumStrategy,
    expectedIntentsForMomentumScenario,
    runMomentumScenario,
)


def _context() -> StrategyContextV1:
    return StrategyContextV1(
        strategyId="example",
        strategyVersion="1.0.0",
        runId="run-1",
        accountId="account-1",
        subaccountId="strategy-1",
        snapshot=StrategySnapshotV1(
            accountId="account-1",
            subaccountId="strategy-1",
            cashAvailable=Decimal("10000"),
            positions={"518880": Decimal("0")},
            snapshotVersion=5,
        ),
        instrument=StrategyInstrumentV1(
            symbol="518880",
            metadataVersion="meta-v1",
            tickSize=Decimal("0.001"),
            lotSize=Decimal("100"),
            currency="CNY",
        ),
    )


def test_daily_momentum_generates_expected_buy_sequence() -> None:
    """固定版本/参数/场景的预期订单清单（回归基准）。"""
    strategy = DailyMomentumStrategy(lookbackDays=4, threshold=Decimal("0.01"))
    strategy.bind(_context())
    intents = runMomentumScenario(strategy, _context())
    expected = expectedIntentsForMomentumScenario()
    assert len(intents) == len(expected)
    assert tuple(intent.side.value for intent in intents) == expected


def test_momentum_flat_market_generates_no_intents() -> None:
    strategy = DailyMomentumStrategy(lookbackDays=3, threshold=Decimal("0.05"))
    strategy.bind(_context())
    from datetime import datetime, timezone

    from veritasquant.strategy.BaseStrategy import ConsumedEventV1

    closes = ["1.000", "1.001", "1.000", "1.001", "1.000"]
    for index, close in enumerate(closes):
        strategy.onBar(
            ConsumedEventV1(
                eventId=f"daily-{index}",
                eventType="DailyBarEvent",
                ts=datetime(2026, 8, index + 1, 15, 0, tzinfo=timezone.utc),
                payload={"close": close},
            )
        )
    assert strategy.emitIntents() == ()


def test_momentum_sell_on_negative_momentum() -> None:
    strategy = DailyMomentumStrategy(lookbackDays=2, threshold=Decimal("0.01"))
    strategy.bind(_context())
    from datetime import datetime, timezone

    from veritasquant.strategy.BaseStrategy import ConsumedEventV1

    closes = ["1.100", "1.050", "1.000", "0.950"]
    for index, close in enumerate(closes):
        strategy.onBar(
            ConsumedEventV1(
                eventId=f"daily-{index}",
                eventType="DailyBarEvent",
                ts=datetime(2026, 8, index + 1, 15, 0, tzinfo=timezone.utc),
                payload={"close": close},
            )
        )
    intents = strategy.emitIntents()
    assert any(intent.side is OrderSide.Sell for intent in intents)


def test_moving_average_cross_fixed_version() -> None:
    assert MovingAverageCrossStrategy.strategyId == "moving_average_cross"
    assert MovingAverageCrossStrategy.strategyVersion == "1.0.0"
    assert DailyMomentumStrategy.strategyId == "daily_momentum"
    assert DailyMomentumStrategy.strategyVersion == "1.0.0"
