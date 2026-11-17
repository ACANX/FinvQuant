from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.BarPath import (
    BarPathError,
    BarPathModelVersion,
    BarPathTriggerV1,
    TriggerKind,
    directionalPathV1,
    roundQuantityToLot,
    roundToTick,
)
from veritasquant.execution.Orders import OrderSide, OrderType

UTC = timezone.utc


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


TICK = Decimal("0.001")


def _trigger(**kwargs: object) -> object:
    trigger = BarPathTriggerV1()
    return trigger.evaluate(**kwargs)


def test_directional_path_up_bar() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    assert directionalPathV1(bar) == (Decimal("1.200"), Decimal("1.190"), Decimal("1.220"), Decimal("1.210"))


def test_directional_path_down_bar() -> None:
    bar = _bar(Decimal("1.210"), Decimal("1.220"), Decimal("1.190"), Decimal("1.200"))
    assert directionalPathV1(bar) == (Decimal("1.210"), Decimal("1.220"), Decimal("1.190"), Decimal("1.200"))


def test_market_order_triggers_at_open() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy, orderType=OrderType.Market, bar=bar, limitPrice=None, stopPrice=None, tickSize=TICK
    )
    assert result.triggered
    assert result.kind is TriggerKind.MarketAtOpen
    assert result.fillPrice == Decimal("1.200")


def test_buy_limit_triggers_on_low_with_price_protection() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy, orderType=OrderType.Limit, bar=bar, limitPrice=Decimal("1.195"), stopPrice=None, tickSize=TICK
    )
    assert result.triggered
    assert result.kind is TriggerKind.LimitTouched
    assert result.fillPrice == Decimal("1.190")  # 路径价优于限价，成交价不高于限价


def test_buy_limit_improves_on_open_gap() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy, orderType=OrderType.Limit, bar=bar, limitPrice=Decimal("1.210"), stopPrice=None, tickSize=TICK
    )
    assert result.triggered
    assert result.fillPrice == Decimal("1.200")  # 开盘触发按开盘价改善


def test_sell_limit_protects_minimum_price() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Sell, orderType=OrderType.Limit, bar=bar, limitPrice=Decimal("1.215"), stopPrice=None, tickSize=TICK
    )
    assert result.triggered
    assert result.fillPrice == Decimal("1.220")  # 路径价优于限价，成交价不低于限价


def test_buy_stop_activates_and_fills_at_path_price() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy, orderType=OrderType.Stop, bar=bar, limitPrice=None, stopPrice=Decimal("1.205"), tickSize=TICK
    )
    assert result.triggered
    assert result.kind is TriggerKind.StopActivated
    # 止损转市价：不能假设按止损价成交
    assert result.fillPrice == Decimal("1.220")


def test_sell_stop_activates_on_down_path() -> None:
    bar = _bar(Decimal("1.210"), Decimal("1.220"), Decimal("1.190"), Decimal("1.200"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Sell, orderType=OrderType.Stop, bar=bar, limitPrice=None, stopPrice=Decimal("1.205"), tickSize=TICK
    )
    assert result.triggered
    assert result.fillPrice == Decimal("1.190")


def test_stop_limit_activation_requires_limit_protection() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    # 开盘越过 stop（1.200 >= 1.195）但开盘价 > limit（1.200 > 1.190）：不成交继续挂单
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy,
        orderType=OrderType.StopLimit,
        bar=bar,
        limitPrice=Decimal("1.190"),
        stopPrice=Decimal("1.195"),
        tickSize=TICK,
    )
    assert not result.triggered
    # 限价保护满足时成交
    ok = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy,
        orderType=OrderType.StopLimit,
        bar=bar,
        limitPrice=Decimal("1.205"),
        stopPrice=Decimal("1.195"),
        tickSize=TICK,
    )
    assert ok.triggered
    assert ok.kind is TriggerKind.StopLimitActivated


def test_not_triggered_when_price_out_of_range() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    result = BarPathTriggerV1().evaluate(
        side=OrderSide.Buy, orderType=OrderType.Limit, bar=bar, limitPrice=Decimal("1.185"), stopPrice=None, tickSize=TICK
    )
    assert not result.triggered
    assert result.kind is TriggerKind.NotTriggered


def test_oco_first_trigger_wins_and_ambiguous_flagged() -> None:
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    trigger = BarPathTriggerV1()
    # limit=1.185 在路径上永不触发（open/low 均高于），stop 在 high=1.220 触发
    oco = trigger.evaluateOco(side=OrderSide.Buy, stopPrice=Decimal("1.205"), limitPrice=Decimal("1.185"), bar=bar, tickSize=TICK)
    assert oco.triggered
    assert oco.kind is TriggerKind.StopActivated
    # 同点双触发标记 AMBIGUOUS：stop 在 open=1.200 触发，limit 1.210 也在 open 触发
    ambiguous = trigger.evaluateOco(side=OrderSide.Buy, stopPrice=Decimal("1.195"), limitPrice=Decimal("1.210"), bar=bar, tickSize=TICK)
    assert ambiguous.triggered
    assert ambiguous.kind is TriggerKind.AmbiguousTrigger
    assert ambiguous.fillPrice is None


def test_round_to_tick_buy_down_sell_up() -> None:
    assert roundToTick(Decimal("1.2345"), Decimal("0.01"), OrderSide.Buy) == Decimal("1.23")
    assert roundToTick(Decimal("1.2345"), Decimal("0.01"), OrderSide.Sell) == Decimal("1.24")


def test_round_quantity_to_lot() -> None:
    assert roundQuantityToLot(Decimal("123"), Decimal("100")) == Decimal("100")
    assert roundQuantityToLot(Decimal("99"), Decimal("100")) == Decimal("0")


def test_rejects_unknown_path_version_and_bad_inputs() -> None:
    with pytest.raises(BarPathError, match="仅支持"):
        BarPathTriggerV1(pathVersion=BarPathModelVersion.TickReplayV1)
    bar = _bar(Decimal("1.200"), Decimal("1.220"), Decimal("1.190"), Decimal("1.210"))
    with pytest.raises(BarPathError, match="limitPrice"):
        BarPathTriggerV1().evaluate(
            side=OrderSide.Buy, orderType=OrderType.Limit, bar=bar, limitPrice=None, stopPrice=None, tickSize=TICK
        )
    with pytest.raises(BarPathError, match="stopPrice"):
        BarPathTriggerV1().evaluate(
            side=OrderSide.Buy, orderType=OrderType.Stop, bar=bar, limitPrice=None, stopPrice=None, tickSize=TICK
        )
    with pytest.raises(BarPathError, match="tick"):
        BarPathTriggerV1().evaluate(
            side=OrderSide.Buy, orderType=OrderType.Market, bar=bar, limitPrice=None, stopPrice=None, tickSize=Decimal("0")
        )
