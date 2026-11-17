from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.accounts.Futures import FuturesMarginBookV1, FuturesStateError


def test_futures_margin_and_daily_mark_to_market_use_contract_multiplier() -> None:
    book = FuturesMarginBookV1()
    position = book.openPosition("AU-202612", Decimal("2"), Decimal("500"), Decimal("1000"), Decimal("0.1"), date(2026, 12, 15))
    marked = book.markToMarket("AU-202612", Decimal("510"))
    assert position.requiredMargin == Decimal("100000.0")
    assert marked.cashVariation == Decimal("20000")
    assert marked.cumulativeMarkToMarket == Decimal("20000")
    assert marked.requiredMargin == Decimal("102000.0")


def test_futures_margin_pressure_and_expiry_boundary_are_explicit() -> None:
    book = FuturesMarginBookV1()
    book.openPosition("AU-202612", Decimal("1"), Decimal("500"), Decimal("1000"), Decimal("0.1"), date(2026, 12, 15))
    assert book.requiresMarginCall("AU-202612", Decimal("49999"))
    with pytest.raises(FuturesStateError, match="尚未到期"):
        book.requireDeliveryOrClose("AU-202612", date(2026, 12, 14))
    assert book.requireDeliveryOrClose("AU-202612", date(2026, 12, 15))
