"""P4-006 开盘/盘中/收盘及重连对账。

对齐 TechSpec 7.4/13 阶段 4：
- 本地与券商订单、成交、持仓、现金逐项核对；
- 未解释差异阻止交易（进入 RECONCILIATION_REQUIRED）。

- `ReconciliationSession`：对账时点（开盘/盘中/收盘/重连）；
- `AccountPositionV1`：持仓/现金快照（字符串金额，禁止 float）；
- `ReconciliationItemV1`：单项核对结果（订单/成交/持仓/现金）；
- `BrokerReconcilerV1`：逐项核对 + 差异门禁（未解释差异阻止交易）。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum
from typing import Protocol


class ReconciliationError(ValueError):
    """对账不满足契约时抛出。"""


class ReconciliationSession(StrEnum):
    Open = "OPEN"
    Intraday = "INTRADAY"
    Close = "CLOSE"
    Reconnect = "RECONNECT"


@dataclass(frozen=True, slots=True)
class AccountPositionV1:
    """账户持仓/现金快照（Decimal 字符串）。"""

    symbol: str
    quantity: str
    cash: str

    def __post_init__(self) -> None:
        if not self.symbol:
            raise ReconciliationError("持仓标的不能为空")


@dataclass(frozen=True, slots=True)
class LocalOrderStateV1:
    """本地订单状态（用于与券商侧核对）。"""

    clientOrderId: str
    brokerOrderId: str | None
    state: str
    cumulativeQuantity: str = "0"


@dataclass(frozen=True, slots=True)
class ReconciliationItemV1:
    """单项核对结果。"""

    itemType: str  # ORDER / FILL / POSITION / CASH
    referenceId: str
    matched: bool
    localValue: str
    brokerValue: str
    detail: str = ""


@dataclass(frozen=True, slots=True)
class ReconciliationReportV1:
    """对账报告：差异门禁。"""

    session: ReconciliationSession
    accountId: str
    items: tuple[ReconciliationItemV1, ...]
    unexplainedDifferences: int
    blocking: bool  # True = 未解释差异阻止交易

    @property
    def clean(self) -> bool:
        return self.unexplainedDifferences == 0 and not self.blocking


class BrokerStateProvider(Protocol):
    """券商侧状态查询端口（由 BrokerPort 实现）。"""

    def queryOrderState(self, clientOrderId: str, brokerOrderId: str) -> str: ...

    def queryPosition(self, accountId: str, symbol: str) -> str: ...

    def queryCash(self, accountId: str) -> str: ...


class BrokerReconcilerV1:
    """开盘/盘中/收盘及重连对账：逐项核对 + 差异门禁。"""

    def __init__(self, brokerProvider: BrokerStateProvider) -> None:
        if brokerProvider is None:
            raise ReconciliationError("券商状态提供者不能为空")
        self._broker = brokerProvider
        self._reports: list[ReconciliationReportV1] = []

    def reconcileOrders(
        self,
        *,
        session: ReconciliationSession,
        accountId: str,
        localOrders: list[LocalOrderStateV1],
        brokerStateOverride: dict[str, str] | None = None,
    ) -> ReconciliationReportV1:
        """订单对账：本地状态 vs 券商状态逐项核对。"""
        items: list[ReconciliationItemV1] = []
        unexplained = 0
        for order in localOrders:
            if order.brokerOrderId is None:
                items.append(
                    ReconciliationItemV1(
                        itemType="ORDER",
                        referenceId=order.clientOrderId,
                        matched=False,
                        localValue=order.state,
                        brokerValue="UNKNOWN",
                        detail="本地订单无 brokerOrderId，无法核对",
                    )
                )
                unexplained += 1
                continue
            if brokerStateOverride is not None and order.clientOrderId in brokerStateOverride:
                brokerState = brokerStateOverride[order.clientOrderId]
            else:
                brokerState = self._broker.queryOrderState(
                    order.clientOrderId, order.brokerOrderId
                )
            matched = brokerState == order.state
            if not matched:
                unexplained += 1
            items.append(
                ReconciliationItemV1(
                    itemType="ORDER",
                    referenceId=order.clientOrderId,
                    matched=matched,
                    localValue=order.state,
                    brokerValue=brokerState,
                    detail="" if matched else "本地与券商订单状态不一致",
                )
            )
        report = ReconciliationReportV1(
            session=session,
            accountId=accountId,
            items=tuple(items),
            unexplainedDifferences=unexplained,
            blocking=unexplained > 0,
        )
        self._reports.append(report)
        return report

    def reconcilePositions(
        self,
        *,
        session: ReconciliationSession,
        accountId: str,
        localPositions: list[AccountPositionV1],
        brokerPositionOverride: dict[str, str] | None = None,
        brokerCashOverride: dict[str, str] | None = None,
    ) -> ReconciliationReportV1:
        """持仓与现金对账。"""
        items: list[ReconciliationItemV1] = []
        unexplained = 0
        for position in localPositions:
            if brokerPositionOverride is not None and position.symbol in brokerPositionOverride:
                brokerQty = brokerPositionOverride[position.symbol]
            else:
                brokerQty = self._broker.queryPosition(accountId, position.symbol)
            matched = brokerQty == position.quantity
            if not matched:
                unexplained += 1
            items.append(
                ReconciliationItemV1(
                    itemType="POSITION",
                    referenceId=position.symbol,
                    matched=matched,
                    localValue=position.quantity,
                    brokerValue=brokerQty,
                    detail="" if matched else "持仓数量不一致",
                )
            )
        if localPositions:
            localCash = localPositions[0].cash
            if brokerCashOverride is not None and accountId in brokerCashOverride:
                brokerCash = brokerCashOverride[accountId]
            else:
                brokerCash = self._broker.queryCash(accountId)
            cashMatched = brokerCash == localCash
            if not cashMatched:
                unexplained += 1
            items.append(
                ReconciliationItemV1(
                    itemType="CASH",
                    referenceId=accountId,
                    matched=cashMatched,
                    localValue=localCash,
                    brokerValue=brokerCash,
                    detail="" if cashMatched else "现金余额不一致",
                )
            )
        report = ReconciliationReportV1(
            session=session,
            accountId=accountId,
            items=tuple(items),
            unexplainedDifferences=unexplained,
            blocking=unexplained > 0,
        )
        self._reports.append(report)
        return report

    def reports(self) -> tuple[ReconciliationReportV1, ...]:
        return tuple(self._reports)


class InMemoryBrokerStateProviderV1:
    """测试/演示用券商状态提供者。"""

    def __init__(self) -> None:
        self._orderStates: dict[str, str] = {}
        self._positions: dict[tuple[str, str], str] = {}
        self._cash: dict[str, str] = {}

    def setOrderState(self, clientOrderId: str, brokerOrderId: str, state: str) -> None:
        self._orderStates[clientOrderId] = state

    def setPosition(self, accountId: str, symbol: str, quantity: str) -> None:
        self._positions[(accountId, symbol)] = quantity

    def setCash(self, accountId: str, cash: str) -> None:
        self._cash[accountId] = cash

    def queryOrderState(self, clientOrderId: str, brokerOrderId: str) -> str:
        return self._orderStates.get(clientOrderId, "UNKNOWN")

    def queryPosition(self, accountId: str, symbol: str) -> str:
        return self._positions.get((accountId, symbol), "UNKNOWN")

    def queryCash(self, accountId: str) -> str:
        return self._cash.get(accountId, "UNKNOWN")
