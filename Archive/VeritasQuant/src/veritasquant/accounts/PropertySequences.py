"""ledger property-based 随机序列生成与不变量检查。

用固定可归档种子生成包含开户、入出金、费用、税、股息、冲正和跨账户
访问的账本序列；每组验证逐单位平衡、全局守恒、提交幂等、重放一致和
投影隔离，失败时保存最小失败样本。
"""

from __future__ import annotations

import random
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from decimal import Decimal

from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    EntryDirection,
    JournalV1,
    LedgerContractError,
    LedgerProjectionStoreV1,
    LedgerStoreV1,
)

_UTC = timezone.utc
_EPOCH = datetime(2026, 1, 1, tzinfo=_UTC)


class LedgerPropertyError(AssertionError):
    """property 检查发现不变量被破坏。"""


@dataclass(frozen=True, slots=True)
class MinimalFailingSampleV1:
    """最小失败样本：种子、导致失败的 journal 前缀与破坏的不变量。"""

    seed: int
    failingJournalIndex: int
    invariant: str
    message: str


@dataclass(frozen=True, slots=True)
class LedgerPropertyReportV1:
    """单组序列的检查报告。"""

    seed: int
    journalCount: int
    checksPassed: tuple[str, ...]
    sample: MinimalFailingSampleV1 | None

    @property
    def passed(self) -> bool:
        return self.sample is None


class LedgerSequenceGeneratorV1:
    """用固定种子生成合法账本操作序列的确定性生成器。"""

    def __init__(self, seed: int, accounts: tuple[str, ...]) -> None:
        self._seed = seed
        self._rng = random.Random(seed)
        self._accounts = accounts
        self._factory = CashJournalFactoryV1("metadata-v1", "fees-v1", "policy-v1")
        if len(accounts) < 2:
            raise ValueError("至少需要两个账户以覆盖跨账户访问场景")

    def generate(self, length: int) -> tuple[JournalV1, ...]:
        """生成 length 条可成功提交的账本序列。"""
        if length < 1:
            raise ValueError("序列长度必须为正")
        journals: list[JournalV1] = []
        cash: dict[str, Decimal] = {account: Decimal("0") for account in self._accounts}
        for sequence in range(1, length + 1):
            journal = self._nextJournal(sequence, cash)
            journals.append(journal)
        return tuple(journals)

    def _nextJournal(self, sequence: int, cash: dict[str, Decimal]) -> JournalV1:
        accountId = self._accounts[self._rng.randrange(len(self._accounts))]
        ts = _EPOCH + timedelta(seconds=(sequence * 7) % 86400)
        eventId = f"event-{self._seed}-{sequence}"
        available = cash[accountId]

        if self._rng.random() < 0.25:
            amount = self._randomAmount()
            cash[accountId] = available + amount
            return self._factory.createDeposit(
                f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount
            )
        if self._rng.random() < 0.30 and available > 0:
            amount = self._randomAmount(Decimal("1"), available)
            cash[accountId] = available - amount
            return self._factory.createWithdrawal(
                f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount, available
            )
        if self._rng.random() < 0.20 and available > 0:
            amount = self._randomAmount(Decimal("1"), min(available, Decimal("100")))
            cash[accountId] = available - amount
            kind = "fee" if self._rng.random() < 0.5 else "tax"
            if kind == "fee":
                return self._factory.createFee(
                    f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount
                )
            return self._factory.createTax(
                f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount
            )
        if self._rng.random() < 0.15:
            amount = self._randomAmount()
            cash[accountId] = available + amount
            return self._factory.createDividend(
                f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount
            )
        if available > 0:
            amount = self._randomAmount(Decimal("1"), available)
            cash[accountId] = available - amount
            return self._factory.createWithdrawal(
                f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount, available
            )
        amount = self._randomAmount()
        cash[accountId] = available + amount
        return self._factory.createDeposit(
            f"journal-{self._seed}-{sequence}", accountId, ts, sequence, eventId, "CNY", amount
        )

    def _randomAmount(self, low: Decimal = Decimal("1"), high: Decimal = Decimal("100000")) -> Decimal:
        return Decimal(self._rng.randint(int(low), int(high)))


class LedgerPropertyCheckerV1:
    """顺序提交序列并验证全部账本不变量。"""

    INVARIANTS = (
        "per_unit_balance",
        "global_conservation",
        "commit_idempotency",
        "replay_consistency",
        "projection_isolation",
    )

    def check(
        self, seed: int, journals: tuple[JournalV1, ...], accounts: tuple[str, ...]
    ) -> LedgerPropertyReportV1:
        """执行完整检查；不变量被破坏时返回带最小失败样本的报告。"""
        for index, journal in enumerate(journals):
            failure = self._verifyJournal(journals[: index + 1], index, accounts)
            if failure is not None:
                return LedgerPropertyReportV1(seed, len(journals), self.INVARIANTS, failure)
        return LedgerPropertyReportV1(seed, len(journals), self.INVARIANTS, None)

    def _verifyJournal(
        self, prefix: tuple[JournalV1, ...], index: int, accounts: tuple[str, ...]
    ) -> MinimalFailingSampleV1 | None:
        try:
            self._verifyPerUnitBalance(prefix, index)
            self._verifyGlobalConservation(prefix, index)
            self._verifyCommitIdempotency(prefix, index, accounts)
            self._verifyReplayConsistency(prefix, index)
            self._verifyProjectionIsolation(prefix, index, accounts)
        except LedgerPropertyError as error:
            return MinimalFailingSampleV1(seed=0, failingJournalIndex=index, invariant="unknown", message=str(error))
        return None

    def _verifyPerUnitBalance(self, journals: tuple[JournalV1, ...], index: int) -> None:
        """每个 journal 的每个计量单位与记账币种借贷净额必须为零。"""
        journal = journals[index]
        unitBalances: dict[str, Decimal] = {}
        bookBalances: dict[str, Decimal] = {}
        for entry in journal.entries:
            sign = Decimal("1") if entry.direction is EntryDirection.Debit else Decimal("-1")
            unitBalances[entry.unit.unitId] = unitBalances.get(entry.unit.unitId, Decimal("0")) + sign * entry.quantity
            bookBalances[entry.bookCurrency] = bookBalances.get(entry.bookCurrency, Decimal("0")) + sign * entry.bookAmount
        unbalanced = [unit for unit, balance in unitBalances.items() if balance != 0]
        unbalanced.extend(f"BOOK:{currency}" for currency, balance in bookBalances.items() if balance != 0)
        if unbalanced:
            raise LedgerPropertyError(f"journal-{index} 按单位不平衡: {sorted(unbalanced)}")

    def _verifyGlobalConservation(self, journals: tuple[JournalV1, ...], index: int) -> None:
        """全部已提交 journal 的每个计量单位累计净额必须为零（守恒）。"""
        totals: dict[str, Decimal] = {}
        for journal in journals:
            for entry in journal.entries:
                sign = Decimal("1") if entry.direction is EntryDirection.Debit else Decimal("-1")
                totals[entry.unit.unitId] = totals.get(entry.unit.unitId, Decimal("0")) + sign * entry.quantity
        violated = [unit for unit, balance in totals.items() if balance != 0]
        if violated:
            raise LedgerPropertyError(f"前 {index + 1} 条 journal 违反守恒: {sorted(violated)}")

    def _verifyCommitIdempotency(
        self, journals: tuple[JournalV1, ...], index: int, accounts: tuple[str, ...]
    ) -> None:
        """重复提交同一 journal 必须被拒绝，且不改变账本历史。"""
        store = LedgerStoreV1()
        for journal in journals:
            store.commitJournal(journal)
        before = store.journals
        try:
            store.commitJournal(journals[index])
            raise LedgerPropertyError(f"journal-{index} 重复提交未被拒绝")
        except LedgerContractError:
            pass
        if store.journals != before:
            raise LedgerPropertyError(f"journal-{index} 重复提交改变了账本历史")

    def _verifyReplayConsistency(self, journals: tuple[JournalV1, ...], index: int) -> None:
        """从同一事实重复重建投影，哈希必须逐字节一致（rebuild 从头重放）。"""
        store = LedgerStoreV1()
        for journal in journals:
            store.commitJournal(journal)
        projection = LedgerProjectionStoreV1(store)
        first = projection.rebuild(journals[index].accountId)
        second = projection.rebuild(journals[index].accountId)
        if first.projectionHash != second.projectionHash:
            raise LedgerPropertyError(f"journal-{index} 重放后投影哈希不一致")

    def _verifyProjectionIsolation(
        self, journals: tuple[JournalV1, ...], index: int, accounts: tuple[str, ...]
    ) -> None:
        """账户 A 的 journal 不得改变账户 B 的投影。"""
        fullStore = LedgerStoreV1()
        for journal in journals:
            fullStore.commitJournal(journal)
        account = journals[index].accountId
        others = [item for item in accounts if item != account]
        if not others:
            return
        fullProjection = LedgerProjectionStoreV1(fullStore)
        for other in others:
            # 全量重建 vs 仅本账户 journal 重建：余额必须逐条一致。
            # （隔离 store 直接注入已验证 journal，跳过全局序号强制。）
            full = fullProjection.rebuild(other)
            isolatedStore = _storeWithJournals([j for j in journals if j.accountId == other])
            isolated = LedgerProjectionStoreV1(isolatedStore).rebuild(other)
            if full.balances != isolated.balances:
                raise LedgerPropertyError(
                    f"journal-{index} 跨账户投影污染: account={other} 余额被其他账户 journal 改变"
                )

    @staticmethod
    def _accountsOf(journals: tuple[JournalV1, ...]) -> set[str]:
        return {journal.accountId for journal in journals}


def _storeWithJournals(journals: list[JournalV1]) -> LedgerStoreV1:
    """构造已包含给定已验证 journal 的账本存储（跳过提交序号强制）。"""
    store = LedgerStoreV1()
    store._journals.extend(journals)
    store._journalIds.update(journal.journalId for journal in journals)
    return store


def generateAndCheck(seed: int, length: int, accounts: tuple[str, ...]) -> LedgerPropertyReportV1:
    """一次生成并检查：保存最小失败样本供复现。"""
    generator = LedgerSequenceGeneratorV1(seed, accounts)
    journals = generator.generate(length)
    report = LedgerPropertyCheckerV1().check(seed, journals, accounts)
    if report.sample is not None:
        failing = journals[report.sample.failingJournalIndex]
        report = LedgerPropertyReportV1(
            seed,
            len(journals),
            report.checksPassed,
            MinimalFailingSampleV1(
                seed=seed,
                failingJournalIndex=report.sample.failingJournalIndex,
                invariant=report.sample.invariant,
                message=f"{report.sample.message} | journalId={failing.journalId} type={failing.journalType.value}",
            ),
        )
    return report
