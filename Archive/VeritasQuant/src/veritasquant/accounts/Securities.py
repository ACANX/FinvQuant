"""证券 T+1 可卖数量与公司行为数量调整。"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import Decimal


class SecuritiesStateError(ValueError):
    """证券结算、可卖数量或公司行为状态不合法。"""


@dataclass(frozen=True, slots=True)
class SecurityPositionV1:
    instrumentId: str
    settledQuantity: Decimal
    unsettledQuantity: Decimal

    @property
    def totalQuantity(self) -> Decimal:
        return self.settledQuantity + self.unsettledQuantity


@dataclass(frozen=True, slots=True)
class PendingSecurityLotV1:
    tradeId: str
    instrumentId: str
    tradeDate: date
    quantity: Decimal


class SecuritiesSettlementBookV1:
    """证券买入必须在后续交易日结算后才可卖出。"""

    def __init__(self) -> None:
        self._positions: dict[str, SecurityPositionV1] = {}
        self._pendingLots: dict[str, PendingSecurityLotV1] = {}
        self._appliedActions: set[str] = set()

    def recordBuy(self, tradeId: str, instrumentId: str, tradeDate: date, quantity: Decimal) -> SecurityPositionV1:
        _validatePositive(tradeId, "成交 ID")
        _validatePositive(instrumentId, "标的 ID")
        _validateQuantity(quantity)
        if tradeId in self._pendingLots:
            raise SecuritiesStateError("证券买入成交 ID 不得重复")
        position = self._position(instrumentId)
        updated = SecurityPositionV1(instrumentId, position.settledQuantity, position.unsettledQuantity + quantity)
        self._positions[instrumentId] = updated
        self._pendingLots[tradeId] = PendingSecurityLotV1(tradeId, instrumentId, tradeDate, quantity)
        return updated

    def settleThrough(self, settlementDate: date) -> None:
        """只结算严格早于结算日的买入，贯彻证券 T+1。"""
        for tradeId, lot in tuple(self._pendingLots.items()):
            if lot.tradeDate >= settlementDate:
                continue
            position = self._position(lot.instrumentId)
            self._positions[lot.instrumentId] = SecurityPositionV1(
                lot.instrumentId, position.settledQuantity + lot.quantity, position.unsettledQuantity - lot.quantity
            )
            del self._pendingLots[tradeId]

    def recordSell(self, instrumentId: str, quantity: Decimal) -> SecurityPositionV1:
        _validatePositive(instrumentId, "标的 ID")
        _validateQuantity(quantity)
        position = self._position(instrumentId)
        if quantity > position.settledQuantity:
            raise SecuritiesStateError("证券 T+1 规则禁止卖出未结算持仓")
        updated = SecurityPositionV1(instrumentId, position.settledQuantity - quantity, position.unsettledQuantity)
        self._positions[instrumentId] = updated
        return updated

    def applySplit(self, actionId: str, instrumentId: str, ratio: Decimal) -> SecurityPositionV1:
        """拆并股按比例同步调整已结算和待结算数量，重复公司行为拒绝。"""
        _validatePositive(actionId, "公司行为 ID")
        _validatePositive(instrumentId, "标的 ID")
        _validateQuantity(ratio)
        if actionId in self._appliedActions:
            raise SecuritiesStateError("公司行为不得重复应用")
        position = self._position(instrumentId)
        updated = SecurityPositionV1(instrumentId, position.settledQuantity * ratio, position.unsettledQuantity * ratio)
        self._positions[instrumentId] = updated
        self._appliedActions.add(actionId)
        return updated

    def positionFor(self, instrumentId: str) -> SecurityPositionV1:
        return self._position(instrumentId)

    def _position(self, instrumentId: str) -> SecurityPositionV1:
        return self._positions.get(instrumentId, SecurityPositionV1(instrumentId, Decimal("0"), Decimal("0")))


def _validatePositive(value: str, label: str) -> None:
    if not value:
        raise SecuritiesStateError(f"{label}不能为空")


def _validateQuantity(value: Decimal) -> None:
    if not isinstance(value, Decimal) or value <= 0:
        raise SecuritiesStateError("数量必须为正 Decimal")
