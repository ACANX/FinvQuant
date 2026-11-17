from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal


from veritasquant.execution.Orders import (
    OrderIntentV1,
    OrderSide,
    OrderType,
    PositionEffect,
    TimeInForce,
)
from veritasquant.risk.BasicRules import (
    CashRuleConfigV1,
    MarginRuleConfigV1,
    QuantityRuleConfigV1,
    RiskRuleEngineV1,
    StalenessRuleConfigV1,
)

UTC = timezone.utc


def _intent(quantity: Decimal = Decimal("100")) -> OrderIntentV1:
    return OrderIntentV1.model_validate(
        {
            "IntentId": "intent-1",
            "RunId": "run-1",
            "AccountId": "account-1",
            "SubaccountId": "strategy-1",
            "StrategyId": "strategy-1",
            "StrategyVersion": "1.0.0",
            "Symbol": "518880",
            "InstrumentMetadataVersion": "meta-v1",
            "Side": OrderSide.Buy,
            "PositionEffect": PositionEffect.Open,
            "OrderType": OrderType.Market,
            "Quantity": quantity,
            "TimeInForce": TimeInForce.Day,
            "Ts": datetime(2026, 8, 2, tzinfo=UTC),
            "CreatedFromEventId": "event-100",
            "ExpectedAccountVersion": 5,
        }
    )


def _engine() -> RiskRuleEngineV1:
    return RiskRuleEngineV1()


def test_cash_rule_blocks_insufficient_funds() -> None:
    result = _engine().checkCash(_intent(Decimal("5000")), Decimal("100"), "snapshot-1")
    assert result.blocked
    assert result.reasonCode == "INSUFFICIENT_CASH"
    assert result.ruleId == "rule.cash_sufficient"
    assert result.snapshotReference == "snapshot-1"


def test_quantity_rule_blocks_over_limit_and_non_lot() -> None:
    engine = _engine()
    over = engine.checkQuantity(_intent(Decimal("2000000")), "snapshot-1")
    assert over.blocked
    assert over.reasonCode == "QUANTITY_LIMIT"
    nonLot = engine.checkQuantity(_intent(Decimal("150")), "snapshot-1")
    assert nonLot.blocked
    assert nonLot.reasonCode == "LOT_SIZE"


def test_concentration_rule_blocks_over_exposure() -> None:
    result = _engine().checkConcentration(_intent(Decimal("10000")), Decimal("9000"), Decimal("20000"), "snapshot-1")
    # (9000 + 10000) / 20000 = 0.95 > 0.25
    assert result.blocked
    assert result.reasonCode == "CONCENTRATION_LIMIT"


def test_margin_rule_blocks_high_utilization() -> None:
    result = _engine().checkMargin(Decimal("19000"), Decimal("20000"), "snapshot-1")
    assert result.blocked
    assert result.reasonCode == "MARGIN_LIMIT"


def test_staleness_rule_blocks_stale_data() -> None:
    now = datetime(2026, 8, 2, 12, 0, tzinfo=UTC)
    stale = datetime(2026, 8, 2, 11, 0, tzinfo=UTC)
    result = _engine().checkStaleness(now, stale, "snapshot-1")
    assert result.blocked
    assert result.reasonCode == "STALE_DATA"


def test_all_rules_pass_for_healthy_intent() -> None:
    results = _engine().checkAll(
        _intent(Decimal("100")),
        cashAvailable=Decimal("10000"),
        symbolExposure=Decimal("1000"),
        equity=Decimal("20000"),
        marginUsed=Decimal("1000"),
        now=datetime(2026, 8, 2, 12, 0, tzinfo=UTC),
        lastBarAt=datetime(2026, 8, 2, 11, 59, tzinfo=UTC),
        snapshotRef="snapshot-1",
    )
    assert all(result.passed for result in results)


def test_hard_limit_cannot_be_loosened_by_config() -> None:
    # 数量硬限制：即使显式配置也不能超过默认上限（配置即硬限制）
    config = QuantityRuleConfigV1(maxOrderQuantity=Decimal("500"))
    engine = RiskRuleEngineV1(quantityConfig=config)
    blocked = engine.checkQuantity(_intent(Decimal("600")), "snapshot-1")
    assert blocked.blocked
    # 手数校验仍生效
    assert engine.checkQuantity(_intent(Decimal("250")), "snapshot-1").reasonCode == "LOT_SIZE"


def test_config_hashes_are_stable_and_versioned() -> None:
    assert CashRuleConfigV1().configHash() == CashRuleConfigV1().configHash()
    assert CashRuleConfigV1().configHash() != CashRuleConfigV1(minCashReserve=Decimal("100")).configHash()
    assert len(MarginRuleConfigV1().configHash()) == 64
    assert len(StalenessRuleConfigV1().configHash()) == 64


def test_zero_equity_fails_safely() -> None:
    engine = _engine()
    assert engine.checkConcentration(_intent(), Decimal("0"), Decimal("0"), "snapshot-1").blocked
    assert engine.checkMargin(Decimal("0"), Decimal("0"), "snapshot-1").blocked
