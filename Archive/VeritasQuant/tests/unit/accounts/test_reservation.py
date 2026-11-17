from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.accounts.Reservation import ReservationBookV1, ReservationError, ReservationKind, ReservationStatus


def test_partial_fill_consumes_only_its_amount_and_cancel_releases_remainder() -> None:
    book = ReservationBookV1()
    reserved = book.reserve("reservation-1", "account-1", "order-1", ReservationKind.Cash, "CNY", Decimal("100"), Decimal("120"))
    partial = book.applyFill(reserved.reservationId, "account-1", "execution-1", Decimal("40"))
    released = book.releaseRemaining(reserved.reservationId, "account-1")
    assert partial.remainingAmount == Decimal("60")
    assert released.consumedAmount == Decimal("40")
    assert released.releasedAmount == Decimal("60")
    assert released.status is ReservationStatus.Closed


def test_duplicate_fill_and_release_are_idempotent_but_conflicts_fail() -> None:
    book = ReservationBookV1()
    book.reserve("reservation-1", "account-1", "order-1", ReservationKind.Security, "INSTRUMENT:518880", Decimal("10"), Decimal("10"))
    first = book.applyFill("reservation-1", "account-1", "execution-1", Decimal("4"))
    repeated = book.applyFill("reservation-1", "account-1", "execution-1", Decimal("4"))
    assert repeated == first
    with pytest.raises(ReservationError, match="冲突"):
        book.applyFill("reservation-1", "account-1", "execution-1", Decimal("5"))
    assert book.releaseRemaining("reservation-1", "account-1") == book.releaseRemaining("reservation-1", "account-1")


def test_reservation_rejects_overcommit_overspend_and_cross_account_access() -> None:
    book = ReservationBookV1()
    with pytest.raises(ReservationError, match="不足"):
        book.reserve("reservation-1", "account-1", "order-1", ReservationKind.Margin, "CNY", Decimal("101"), Decimal("100"))
    book.reserve("reservation-1", "account-1", "order-1", ReservationKind.Margin, "CNY", Decimal("100"), Decimal("100"))
    with pytest.raises(ReservationError, match="超过"):
        book.applyFill("reservation-1", "account-1", "execution-1", Decimal("101"))
    with pytest.raises(ReservationError, match="跨账户"):
        book.get("reservation-1", "account-2")
