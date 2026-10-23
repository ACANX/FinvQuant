"""示例策略：均线交叉与日频动量（技术方案 4.4/8.1 节）。

固定版本、参数和预期订单清单；示例不绕过风控，只返回 OrderIntent。
"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

from veritasquant.strategy.BaseStrategy import (
    BaseStrategy,
    ConsumedEventV1,
    StrategyContextV1,
)
from veritasquant.execution.Orders import OrderIntentV1, OrderSide, OrderType


class DailyMomentumStrategy(BaseStrategy):
    """日频动量示例策略：收盘动量正负触发买卖意图。"""

    strategyId = "daily_momentum"
    strategyVersion = "1.0.0"

    def __init__(self, lookbackDays: int = 5, threshold: Decimal = Decimal("0.01")) -> None:
        super().__init__()
        self._lookbackDays = lookbackDays
        self._threshold = threshold
        self._closes: list[Decimal] = []

    def onBar(self, event: ConsumedEventV1) -> None:
        if self._context is not None:
            self._context.consume(event)
        close = Decimal(str(event.payload.get("close", "0")))
        self._closes.append(close)
        if len(self._closes) <= self._lookbackDays:
            return
        previous = self._closes[-self._lookbackDays - 1]
        latest = self._closes[-1]
        momentum = (latest - previous) / previous
        if momentum > self._threshold:
            self.createOrder(
                self._context.instrument.symbol if self._context else "518880",
                OrderSide.Buy,
                Decimal("100"),
                orderType=OrderType.Market,
            )
        elif momentum < -self._threshold:
            self.createOrder(
                self._context.instrument.symbol if self._context else "518880",
                OrderSide.Sell,
                Decimal("100"),
                orderType=OrderType.Market,
            )


def expectedIntentsForMomentumScenario() -> tuple[str, ...]:
    """固定场景的预期订单方向序列（用于回归断言）。"""
    return ("BUY", "BUY")


def runMomentumScenario(strategy: DailyMomentumStrategy, context: StrategyContextV1) -> tuple[OrderIntentV1, ...]:
    """回放固定收盘价序列并返回产生的全部意图。"""
    closes = ["1.000", "1.005", "1.010", "1.015", "1.020", "1.040"]
    for index, close in enumerate(closes):
        event = ConsumedEventV1(
            eventId=f"daily-{index}",
            eventType="DailyBarEvent",
            ts=datetime(2026, 8, index + 1, 15, 0, tzinfo=timezone.utc),
            payload={"close": close, "symbol": "518880"},
        )
        strategy.onBar(event)
    return strategy.emitIntents()
