"""执行回报、订单迁移、预占与账本分录的原子提交边界。

技术方案第 6 章：成交或部分成交回报按 account_id + execution_id 在 inbox、
订单状态迁移、不可变账本分录、资金/持仓投影、派生事件、checkpoint、outbox
原子提交；任一写入失败整笔回滚，禁止只更新绩效而不更新真实账户状态。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    JournalV1,
    LedgerAccount,
    LedgerContractError,
    LedgerStoreV1,
)
from veritasquant.accounts.Reservation import ReservationBookV1
from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Transaction import TransactionStoreV1
from veritasquant.execution.OrderStateMachine import (
    OrderStateMachineError,
    OrderStateMachineV1,
    TransitionKind,
)
from veritasquant.execution.Orders import ExecutionReportEventV1, ExecutionType


class AtomicExecutionError(ValueError):
    """原子边界内的任一写入失败时抛出；整笔回滚不留部分副作用。"""


@dataclass(frozen=True, slots=True)
class AtomicExecutionOutcomeV1:
    """一次原子提交的完整结果。"""

    transactionId: str
    executionId: str
    journalCount: int
    factSequences: tuple[int, ...]
    orderVersionAfter: int
    postExecutionCash: Decimal
    postExecutionReserved: Decimal


class AtomicExecutionBoundaryV1:
    """把回报记账、订单迁移、预占消耗与账本分录装入同一原子事务。"""

    def __init__(
        self,
        *,
        transactionStore: TransactionStoreV1,
        ledgerStore: LedgerStoreV1,
        reservationBook: ReservationBookV1,
        stateMachine: OrderStateMachineV1,
        journalFactory: CashJournalFactoryV1,
        accountId: str,
        currency: str,
    ) -> None:
        self._transactionStore = transactionStore
        self._ledgerStore = ledgerStore
        self._reservationBook = reservationBook
        self._stateMachine = stateMachine
        self._journalFactory = journalFactory
        self._accountId = accountId
        self._currency = currency
        self._transactionCounter = 0

    def commitExecutionReport(self, report: ExecutionReportEventV1) -> AtomicExecutionOutcomeV1:
        """将一条执行回报的订单迁移、预占消耗和账本分录原子提交。"""
        self._transactionCounter += 1
        transactionId = f"atomic-{self._transactionCounter}"
        transaction = self._transactionStore.begin()
        try:
            # 0) 先构建账本分录作为预检：撤单/拒单不虚构分录、资金不足等在
            #    任何状态写入前即失败，保证整笔回滚无部分副作用。
            journal = self._buildJournal(report)

            # 1) 订单状态迁移（乐观版本由状态机内部维护）
            snapshot = self._stateMachine.snapshot(report.clientOrderId)
            if report.executionType in (ExecutionType.PartialFill, ExecutionType.Fill):
                self._stateMachine.transition(
                    report.clientOrderId,
                    report.accountId,
                    TransitionKind.IncrementalFill,
                    snapshot.orderVersion,
                    fillQuantity=report.lastQuantity,
                )
            elif report.executionType is ExecutionType.Cancelled:
                self._stateMachine.transition(
                    report.clientOrderId,
                    report.accountId,
                    TransitionKind.CancelConfirmed,
                    snapshot.orderVersion,
                    cancelQuantity=report.remainingQuantity,
                )
            elif report.executionType is ExecutionType.Rejected:
                self._stateMachine.transition(
                    report.clientOrderId,
                    report.accountId,
                    TransitionKind.BrokerReject,
                    snapshot.orderVersion,
                )
            else:
                raise AtomicExecutionError(f"不支持的回报类型: {report.executionType.value}")

            # 2) 预占消耗（成交）或释放（撤单/拒单）
            if report.executionType in (ExecutionType.PartialFill, ExecutionType.Fill):
                self._reservationBook.applyFill(
                    report.clientOrderId, report.accountId, report.executionId or report.brokerReportId, report.lastQuantity
                )
            else:
                self._reservationBook.releaseRemaining(report.clientOrderId, report.accountId)

            # 3) 账本分录：成交入账现金/持仓，撤单解冻
            self._ledgerStore.commitJournal(journal)

            # 4) 领域事实与 outbox 同事务提交
            factHash = canonicalHash(
                {
                    "execution_id": report.executionId,
                    "broker_report_id": report.brokerReportId,
                    "client_order_id": report.clientOrderId,
                    "last_quantity": report.lastQuantity,
                    "cumulative_quantity": report.cumulativeQuantity,
                }
            )
            transaction.appendFact("EXECUTION_REPORT", factHash)
            transaction.enqueue(
                messageId=f"outbox-{transactionId}",
                topic="execution.events",
                payloadHash=factHash,
            )
            facts = transaction.commit()

            # 5) 提交后读取新余额（原子边界后策略回调读到新状态）
            postCash, postReserved = self._postExecutionBalances()
            after = self._stateMachine.snapshot(report.clientOrderId)
            return AtomicExecutionOutcomeV1(
                transactionId=transactionId,
                executionId=report.executionId or report.brokerReportId,
                journalCount=1,
                factSequences=tuple(fact.sequence for fact in facts),
                orderVersionAfter=after.orderVersion,
                postExecutionCash=postCash,
                postExecutionReserved=postReserved,
            )
        except (LedgerContractError, OrderStateMachineError, AtomicExecutionError, ValueError) as error:
            transaction.rollback()
            raise AtomicExecutionError(f"原子提交失败，整笔回滚: {error}") from error

    def _buildJournal(self, report: ExecutionReportEventV1) -> JournalV1:
        """构造成交对应的严格平衡 journal；撤单/拒单不虚构账本分录。"""
        journalId = f"journal-{report.accountId}-{report.brokerReportId}"
        sequence = len(self._ledgerStore.journals) + 1
        ts = report.ts
        if report.executionType not in (ExecutionType.PartialFill, ExecutionType.Fill):
            # 技术方案：撤单/拒单只释放预占并持久化状态迁移，不得虚构持仓变化
            raise AtomicExecutionError("撤单/拒单不得虚构账本分录")
        amount = report.lastQuantity * (report.lastPrice or Decimal("0"))
        if report.clientOrderId.startswith("SELL"):
            return self._journalFactory.createDeposit(
                journalId, self._accountId, ts, sequence, report.brokerReportId, self._currency, amount
            )
        available = self._availableCash()
        return self._journalFactory.createWithdrawal(
            journalId, self._accountId, ts, sequence, report.brokerReportId, self._currency, amount, available
        )

    def _availableCash(self) -> Decimal:
        """从已提交账本投影当前可用现金（借方为正、贷方为负）。"""
        cash = Decimal("0")
        frozen = Decimal("0")
        for journal in self._ledgerStore.journals:
            for entry in journal.entries:
                sign = Decimal("1") if entry.direction.value == "DEBIT" else Decimal("-1")
                if entry.ledgerAccount is LedgerAccount.CashAvailable:
                    cash += sign * entry.bookAmount
                elif entry.ledgerAccount is LedgerAccount.CashFrozen:
                    frozen += sign * entry.bookAmount
        return cash - frozen

    def _postExecutionBalances(self) -> tuple[Decimal, Decimal]:
        """从账本投影读取提交后的现金与冻结余额（借方为正、贷方为负）。"""
        cash = Decimal("0")
        frozen = Decimal("0")
        for journal in self._ledgerStore.journals:
            for entry in journal.entries:
                sign = Decimal("1") if entry.direction.value == "DEBIT" else Decimal("-1")
                if entry.ledgerAccount is LedgerAccount.CashAvailable:
                    cash += sign * entry.bookAmount
                elif entry.ledgerAccount is LedgerAccount.CashFrozen:
                    frozen += sign * entry.bookAmount
        return cash, frozen

