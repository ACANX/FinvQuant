"""P2-007 模拟盘每日对账：差异检测、分类与恢复门禁。

每日对账对比权威事实（账本/订单/持仓）与运行状态，任何未解释差异：

- 按类别分类（账本/订单/持仓/投递）与差异类型（缺失/多余/金额不符/状态不符）；
- 未解释差异阻止恢复（recovery_blocked），只有差异清零后才允许恢复交易。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from enum import StrEnum


class ReconciliationCategory(StrEnum):
    Ledger = "LEDGER"
    Order = "ORDER"
    Position = "POSITION"
    Outbox = "OUTBOX"


class DifferenceClass(StrEnum):
    Missing = "MISSING"  # 权威有、运行无
    Extra = "EXTRA"  # 运行有、权威无
    AmountMismatch = "AMOUNT_MISMATCH"
    StateMismatch = "STATE_MISMATCH"


@dataclass(frozen=True, slots=True)
class ReconciliationDifferenceV1:
    """一条已分类的对账差异。"""

    category: ReconciliationCategory
    accountId: str
    differenceClass: DifferenceClass
    description: str

    @property
    def blocksRecovery(self) -> bool:
        """所有差异类型都阻止恢复，直到解释并修复。"""
        return True


@dataclass(frozen=True, slots=True)
class DailyReconciliationReportV1:
    """一次每日对账的结果。"""

    runId: str
    differences: tuple[ReconciliationDifferenceV1, ...]

    @property
    def recoveryBlocked(self) -> bool:
        """存在任何未解释差异时禁止恢复交易。"""
        return len(self.differences) > 0

    @property
    def categories(self) -> tuple[ReconciliationCategory, ...]:
        return tuple(sorted({difference.category for difference in self.differences}))


class DailyReconciliationV1:
    """纯函数对账器：比较权威快照与运行状态。"""

    def reconcile(
        self,
        runId: str,
        *,
        authoritativeLedger: dict[tuple[str, str], Decimal],
        actualLedger: dict[tuple[str, str], Decimal],
        authoritativeOrders: dict[str, str],
        actualOrders: dict[str, str],
        authoritativePositions: dict[tuple[str, str], Decimal],
        actualPositions: dict[tuple[str, str], Decimal],
    ) -> DailyReconciliationReportV1:
        """执行每日对账并返回已分类差异集合。"""
        differences: list[ReconciliationDifferenceV1] = []
        differences.extend(self._checkLedger(authoritativeLedger, actualLedger))
        differences.extend(self._checkOrders(authoritativeOrders, actualOrders))
        differences.extend(self._checkPositions(authoritativePositions, actualPositions))
        return DailyReconciliationReportV1(runId, tuple(differences))

    def _checkLedger(
        self,
        authoritative: dict[tuple[str, str], Decimal],
        actual: dict[tuple[str, str], Decimal],
    ) -> list[ReconciliationDifferenceV1]:
        differences: list[ReconciliationDifferenceV1] = []
        for key, expected in authoritative.items():
            accountId, unitId = key
            observed = actual.get(key)
            if observed is None:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Ledger, accountId, DifferenceClass.Missing,
                        f"账本缺失 {unitId} 应 {expected}",
                    )
                )
            elif observed != expected:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Ledger, accountId, DifferenceClass.AmountMismatch,
                        f"账本不符 {unitId} 应 {expected} 实 {observed}",
                    )
                )
        for key in actual:
            if key not in authoritative:
                accountId, unitId = key
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Ledger, accountId, DifferenceClass.Extra,
                        f"账本多余 {unitId} = {actual[key]}",
                    )
                )
        return differences

    def _checkOrders(
        self,
        authoritative: dict[str, str],
        actual: dict[str, str],
    ) -> list[ReconciliationDifferenceV1]:
        differences: list[ReconciliationDifferenceV1] = []
        for orderId, expectedState in authoritative.items():
            accountId = orderId.split(":", 1)[0]
            observed = actual.get(orderId)
            if observed is None:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Order, accountId, DifferenceClass.Missing,
                        f"订单缺失 {orderId} 应 {expectedState}",
                    )
                )
            elif observed != expectedState:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Order, accountId, DifferenceClass.StateMismatch,
                        f"订单状态不符 {orderId} 应 {expectedState} 实 {observed}",
                    )
                )
        for orderId in actual:
            if orderId not in authoritative:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Order, orderId.split(":", 1)[0],
                        DifferenceClass.Extra, f"订单多余 {orderId}",
                    )
                )
        return differences

    def _checkPositions(
        self,
        authoritative: dict[tuple[str, str], Decimal],
        actual: dict[tuple[str, str], Decimal],
    ) -> list[ReconciliationDifferenceV1]:
        differences: list[ReconciliationDifferenceV1] = []
        for key, expected in authoritative.items():
            accountId, symbol = key
            observed = actual.get(key)
            if observed is None:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Position, accountId, DifferenceClass.Missing,
                        f"持仓缺失 {symbol} 应 {expected}",
                    )
                )
            elif observed != expected:
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Position, accountId, DifferenceClass.AmountMismatch,
                        f"持仓不符 {symbol} 应 {expected} 实 {observed}",
                    )
                )
        for key in actual:
            if key not in authoritative:
                accountId, symbol = key
                differences.append(
                    ReconciliationDifferenceV1(
                        ReconciliationCategory.Position, accountId, DifferenceClass.Extra,
                        f"持仓多余 {symbol} = {actual[key]}",
                    )
                )
        return differences
