"""期货合约保证金、逐日盯市和到期边界的确定性计算。"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import date
from decimal import Decimal


class FuturesStateError(ValueError):
    """期货仓位、保证金或到期状态不满足契约。"""


@dataclass(frozen=True, slots=True)
class FuturesPositionV1:
    contractId: str
    quantity: Decimal
    contractMultiplier: Decimal
    initialMarginRate: Decimal
    settlementPrice: Decimal
    expiryDate: date
    cumulativeMarkToMarket: Decimal

    @property
    def requiredMargin(self) -> Decimal:
        return abs(self.quantity) * self.settlementPrice * self.contractMultiplier * self.initialMarginRate


@dataclass(frozen=True, slots=True)
class FuturesMarkResultV1:
    contractId: str
    cashVariation: Decimal
    requiredMargin: Decimal
    cumulativeMarkToMarket: Decimal


class FuturesMarginBookV1:
    """阶段 1 单合约仓位基线；到期前不得把未平仓合约视为可忽略。"""

    def __init__(self) -> None:
        self._positions: dict[str, FuturesPositionV1] = {}

    def openPosition(
        self, contractId: str, quantity: Decimal, price: Decimal, contractMultiplier: Decimal,
        initialMarginRate: Decimal, expiryDate: date,
    ) -> FuturesPositionV1:
        _positive(contractId, "合约 ID")
        _nonZero(quantity, "合约数量")
        _positiveDecimal(price, "成交价格")
        _positiveDecimal(contractMultiplier, "合约乘数")
        if not isinstance(initialMarginRate, Decimal) or initialMarginRate <= 0 or initialMarginRate > 1:
            raise FuturesStateError("初始保证金率必须在零到一之间")
        if contractId in self._positions:
            raise FuturesStateError("合约仓位已存在，必须通过成交状态机变更")
        position = FuturesPositionV1(contractId, quantity, contractMultiplier, initialMarginRate, price, expiryDate, Decimal("0"))
        self._positions[contractId] = position
        return position

    def markToMarket(self, contractId: str, settlementPrice: Decimal) -> FuturesMarkResultV1:
        _positiveDecimal(settlementPrice, "结算价格")
        position = self._requirePosition(contractId)
        variation = (settlementPrice - position.settlementPrice) * position.quantity * position.contractMultiplier
        updated = FuturesPositionV1(
            position.contractId, position.quantity, position.contractMultiplier, position.initialMarginRate,
            settlementPrice, position.expiryDate, position.cumulativeMarkToMarket + variation,
        )
        self._positions[contractId] = updated
        return FuturesMarkResultV1(contractId, variation, updated.requiredMargin, updated.cumulativeMarkToMarket)

    def requiresMarginCall(self, contractId: str, availableMarginCash: Decimal) -> bool:
        if not isinstance(availableMarginCash, Decimal) or availableMarginCash < 0:
            raise FuturesStateError("可用保证金资金必须为非负 Decimal")
        return availableMarginCash < self._requirePosition(contractId).requiredMargin

    def requireDeliveryOrClose(self, contractId: str, tradingDate: date) -> bool:
        position = self._requirePosition(contractId)
        if tradingDate < position.expiryDate:
            raise FuturesStateError("合约尚未到期，不得执行交割边界处理")
        return position.quantity != 0

    def positionFor(self, contractId: str) -> FuturesPositionV1:
        return self._requirePosition(contractId)

    def _requirePosition(self, contractId: str) -> FuturesPositionV1:
        position = self._positions.get(contractId)
        if position is None:
            raise FuturesStateError("未知期货合约仓位")
        return position


def _positive(value: str, label: str) -> None:
    if not value:
        raise FuturesStateError(f"{label}不能为空")


def _positiveDecimal(value: Decimal, label: str) -> None:
    if not isinstance(value, Decimal) or value <= 0:
        raise FuturesStateError(f"{label}必须为正 Decimal")


def _nonZero(value: Decimal, label: str) -> None:
    if not isinstance(value, Decimal) or value == 0:
        raise FuturesStateError(f"{label}必须为非零 Decimal")
