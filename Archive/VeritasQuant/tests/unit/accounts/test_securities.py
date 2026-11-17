from __future__ import annotations

from datetime import date
from decimal import Decimal

import pytest

from veritasquant.accounts.Securities import SecuritiesSettlementBookV1, SecuritiesStateError


def test_security_bought_today_cannot_sell_until_next_trading_day_settlement() -> None:
    book = SecuritiesSettlementBookV1()
    book.recordBuy("trade-1", "518880", date(2026, 8, 3), Decimal("100"))
    with pytest.raises(SecuritiesStateError, match=r"T\+1"):
        book.recordSell("518880", Decimal("1"))
    book.settleThrough(date(2026, 8, 3))
    with pytest.raises(SecuritiesStateError, match=r"T\+1"):
        book.recordSell("518880", Decimal("1"))
    book.settleThrough(date(2026, 8, 4))
    assert book.recordSell("518880", Decimal("40")).settledQuantity == Decimal("60")


def test_split_adjusts_settled_and_unsettled_quantities_once() -> None:
    book = SecuritiesSettlementBookV1()
    book.recordBuy("trade-1", "518880", date(2026, 8, 3), Decimal("100"))
    book.settleThrough(date(2026, 8, 4))
    book.recordBuy("trade-2", "518880", date(2026, 8, 4), Decimal("10"))
    adjusted = book.applySplit("action-1", "518880", Decimal("2"))
    assert adjusted.settledQuantity == Decimal("200")
    assert adjusted.unsettledQuantity == Decimal("20")
    with pytest.raises(SecuritiesStateError, match="重复"):
        book.applySplit("action-1", "518880", Decimal("2"))
