"""订单资源预占、部分成交消耗与剩余释放契约。"""

from __future__ import annotations

from dataclasses import dataclass, replace
from decimal import Decimal
from enum import StrEnum


class ReservationError(ValueError):
    """预占、成交或释放不符合账户资源边界。"""


class ReservationKind(StrEnum):
    Cash = "CASH"
    Security = "SECURITY"
    Margin = "MARGIN"


class ReservationStatus(StrEnum):
    Active = "ACTIVE"
    Closed = "CLOSED"


@dataclass(frozen=True, slots=True)
class ReservationV1:
    reservationId: str
    accountId: str
    orderId: str
    kind: ReservationKind
    unitId: str
    reservedAmount: Decimal
    consumedAmount: Decimal
    releasedAmount: Decimal
    status: ReservationStatus

    @property
    def remainingAmount(self) -> Decimal:
        return self.reservedAmount - self.consumedAmount - self.releasedAmount


class ReservationBookV1:
    """按账户和订单隔离资源；重复成交或重复释放不得重复消耗资源。"""

    def __init__(self) -> None:
        self._reservations: dict[str, ReservationV1] = {}
        self._executionAmounts: dict[tuple[str, str], Decimal] = {}

    def reserve(
        self,
        reservationId: str,
        accountId: str,
        orderId: str,
        kind: ReservationKind,
        unitId: str,
        amount: Decimal,
        availableAmount: Decimal,
    ) -> ReservationV1:
        """在风险批准时冻结资源，任何不足均拒绝而不产生部分预占。"""
        _validateReservationInput(reservationId, accountId, orderId, unitId, amount)
        _validateAmount(availableAmount, "可用资源")
        if reservationId in self._reservations:
            raise ReservationError("预占 ID 已存在")
        if amount > availableAmount:
            raise ReservationError("可用资源不足，禁止预占")
        reservation = ReservationV1(
            reservationId, accountId, orderId, kind, unitId, amount, Decimal("0"), Decimal("0"), ReservationStatus.Active
        )
        self._reservations[reservationId] = reservation
        return reservation

    def applyFill(self, reservationId: str, accountId: str, executionId: str, amount: Decimal) -> ReservationV1:
        """按 executionId 幂等消耗预占，部分成交只扣减真实成交数量。"""
        _validateAmount(amount, "成交数量")
        reservation = self._requireOwned(reservationId, accountId)
        executionKey = (reservationId, executionId)
        priorAmount = self._executionAmounts.get(executionKey)
        if priorAmount is not None:
            if priorAmount != amount:
                raise ReservationError("同一 executionId 的预占消耗数量冲突")
            return reservation
        if reservation.status is ReservationStatus.Closed or amount > reservation.remainingAmount:
            raise ReservationError("成交数量超过可用预占")
        updated = replace(reservation, consumedAmount=reservation.consumedAmount + amount)
        if updated.remainingAmount == 0:
            updated = replace(updated, status=ReservationStatus.Closed)
        self._reservations[reservationId] = updated
        self._executionAmounts[executionKey] = amount
        return updated

    def releaseRemaining(self, reservationId: str, accountId: str) -> ReservationV1:
        """拒单、撤单或终态时仅释放未成交部分；重复调用保持幂等。"""
        reservation = self._requireOwned(reservationId, accountId)
        if reservation.status is ReservationStatus.Closed:
            return reservation
        updated = replace(
            reservation,
            releasedAmount=reservation.releasedAmount + reservation.remainingAmount,
            status=ReservationStatus.Closed,
        )
        self._reservations[reservationId] = updated
        return updated

    def get(self, reservationId: str, accountId: str) -> ReservationV1:
        return self._requireOwned(reservationId, accountId)

    def _requireOwned(self, reservationId: str, accountId: str) -> ReservationV1:
        reservation = self._reservations.get(reservationId)
        if reservation is None:
            raise ReservationError("未知预占 ID")
        if reservation.accountId != accountId:
            raise ReservationError("预占不得跨账户访问")
        return reservation


def _validateReservationInput(reservationId: str, accountId: str, orderId: str, unitId: str, amount: Decimal) -> None:
    if not all((reservationId, accountId, orderId, unitId)):
        raise ReservationError("预占必须包含 ID、账户、订单和计量单位")
    _validateAmount(amount, "预占数量")


def _validateAmount(amount: Decimal, label: str) -> None:
    if not isinstance(amount, Decimal) or amount <= 0:
        raise ReservationError(f"{label}必须为正 Decimal")
