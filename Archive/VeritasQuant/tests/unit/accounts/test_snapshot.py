from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    LedgerContractError,
    LedgerProjectionStoreV1,
    LedgerStoreV1,
)
from veritasquant.accounts.Snapshot import (
    AccountSnapshotBuilderV1,
    AccountSnapshotError,
    AccountSnapshotStoreV1,
    AccountSnapshotV1,
    PortfolioSummaryServiceV1,
)
from veritasquant.core.CanonicalJson import canonicalHash

UTC = timezone.utc
VERSIONS = ("metadata-v1", "fees-v1", "policy-v1")


def _utc(year: int, month: int, day: int) -> datetime:
    return datetime(year, month, day, tzinfo=UTC)


def _ledgerWithAccount(accountId: str, opening: Decimal, deposits: tuple[Decimal, ...] = ()) -> LedgerStoreV1:
    store = LedgerStoreV1()
    factory = CashJournalFactoryV1(*VERSIONS)
    store.commitJournal(
        factory.createOpeningBalance(
            f"{accountId}:open", accountId, _utc(2026, 1, 1), 1, f"{accountId}:open-event", "CNY", opening
        )
    )
    for index, amount in enumerate(deposits, start=2):
        store.commitJournal(
            factory.createDeposit(
                f"{accountId}:dep{index}", accountId, _utc(2026, 1, index), index, f"{accountId}:dep{index}-event", "CNY", amount
            )
        )
    return store


def _projection(store: LedgerStoreV1, accountId: str) -> LedgerProjectionStoreV1:
    return LedgerProjectionStoreV1(store)


def _firstSnapshot(accountId: str = "account-1") -> AccountSnapshotV1:
    store = _ledgerWithAccount(accountId, Decimal("1000"))
    builder = AccountSnapshotBuilderV1(_projection(store, accountId))
    return builder.build(accountId, 1, _utc(2026, 1, 1))


def test_snapshot_carries_ledger_upper_bound_and_hash() -> None:
    snapshot = _firstSnapshot()
    assert snapshot.lastLedgerSequence == 1
    assert snapshot.contentHash == _snapshotHashOf(snapshot)
    assert snapshot.balanceFor  # 只读访问器存在


def test_snapshot_hash_changes_with_version() -> None:
    store = _ledgerWithAccount("account-1", Decimal("1000"))
    builder = AccountSnapshotBuilderV1(_projection(store, "account-1"))
    first = builder.build("account-1", 1, _utc(2026, 1, 1))
    second = builder.build("account-1", 2, _utc(2026, 1, 2))
    assert first.contentHash != second.contentHash


def test_stale_version_write_rejected() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    first = _firstSnapshot()
    snapshotStore.saveSnapshot(first)
    newer = AccountSnapshotV1(
        accountId=first.accountId,
        snapshotVersion=2,
        lastLedgerSequence=first.lastLedgerSequence,
        snapshotTs=_utc(2026, 1, 2),
        balances=first.balances,
        contentHash=first.contentHash,
    )
    snapshotStore.saveSnapshot(newer)
    stale = AccountSnapshotV1(
        accountId=first.accountId,
        snapshotVersion=1,
        lastLedgerSequence=first.lastLedgerSequence,
        snapshotTs=first.snapshotTs,
        balances=first.balances,
        contentHash=first.contentHash,
    )
    with pytest.raises(AccountSnapshotError, match="旧版本"):
        snapshotStore.saveSnapshot(stale)


def test_same_version_conflicting_content_rejected() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    first = _firstSnapshot()
    snapshotStore.saveSnapshot(first)
    conflict = AccountSnapshotV1(
        accountId=first.accountId,
        snapshotVersion=first.snapshotVersion,
        lastLedgerSequence=first.lastLedgerSequence,
        snapshotTs=first.snapshotTs,
        balances=first.balances,
        contentHash="0" * 64,
    )
    with pytest.raises(AccountSnapshotError, match="冲突"):
        snapshotStore.saveSnapshot(conflict)


def test_idempotent_replay_of_identical_snapshot_accepted() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    first = _firstSnapshot()
    snapshotStore.saveSnapshot(first)
    replay = AccountSnapshotV1(
        accountId=first.accountId,
        snapshotVersion=first.snapshotVersion,
        lastLedgerSequence=first.lastLedgerSequence,
        snapshotTs=first.snapshotTs,
        balances=first.balances,
        contentHash=first.contentHash,
    )
    assert snapshotStore.saveSnapshot(replay) is first
    assert snapshotStore.currentVersion(first.accountId) == 1


def test_version_increases_monotonically() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    first = _firstSnapshot("account-1")
    second = AccountSnapshotV1(
        accountId=first.accountId,
        snapshotVersion=2,
        lastLedgerSequence=first.lastLedgerSequence,
        snapshotTs=_utc(2026, 1, 2),
        balances=first.balances,
        contentHash=first.contentHash,
    )
    snapshotStore.saveSnapshot(first)
    snapshotStore.saveSnapshot(second)
    assert snapshotStore.currentVersion(first.accountId) == 2
    assert snapshotStore.snapshotFor(first.accountId).snapshotVersion == 2


def test_builder_rejects_invalid_inputs() -> None:
    store = _ledgerWithAccount("account-1", Decimal("1000"))
    builder = AccountSnapshotBuilderV1(_projection(store, "account-1"))
    with pytest.raises(AccountSnapshotError, match="版本"):
        builder.build("account-1", 0, _utc(2026, 1, 1))
    with pytest.raises(AccountSnapshotError, match="UTC"):
        builder.build("account-1", 1, datetime(2026, 1, 1))
    with pytest.raises(LedgerContractError, match="账户 ID"):
        builder.build("", 1, _utc(2026, 1, 1))


def test_summary_is_read_only_and_does_not_move_funds() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    first = _firstSnapshot("account-1")
    secondStore = _ledgerWithAccount("account-2", Decimal("500"), deposits=(Decimal("250"),))
    secondBuilder = AccountSnapshotBuilderV1(_projection(secondStore, "account-2"))
    second = secondBuilder.build("account-2", 1, _utc(2026, 1, 2))
    snapshotStore.saveSnapshot(first)
    snapshotStore.saveSnapshot(second)

    summary = PortfolioSummaryServiceV1(snapshotStore).summarize(
        "summary-1", ("account-1", "account-2"), _utc(2026, 1, 3)
    )
    # 汇总后原快照分文未动
    assert snapshotStore.snapshotFor("account-1").contentHash == first.contentHash
    assert snapshotStore.snapshotFor("account-2").contentHash == second.contentHash
    assert summary.rowFor("account-1", "CNY").cashNet == Decimal("1000")
    assert summary.rowFor("account-2", "CNY").cashNet == Decimal("750")
    assert summary.totalCashNet == Decimal("1750")
    assert summary.totalAssetValue == Decimal("0")


def test_summary_rejects_unknown_account_and_bad_input() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    snapshotStore.saveSnapshot(_firstSnapshot())
    service = PortfolioSummaryServiceV1(snapshotStore)
    with pytest.raises(AccountSnapshotError, match="尚无快照"):
        service.summarize("summary-1", ("ghost",), _utc(2026, 1, 3))
    with pytest.raises(AccountSnapshotError, match="账户列表"):
        service.summarize("summary-1", (), _utc(2026, 1, 3))
    with pytest.raises(AccountSnapshotError, match="UTC"):
        service.summarize("summary-1", ("account-1",), datetime(2026, 1, 3))


def test_summary_hash_is_deterministic_and_stable() -> None:
    snapshotStore = AccountSnapshotStoreV1()
    snapshotStore.saveSnapshot(_firstSnapshot("account-1"))
    service = PortfolioSummaryServiceV1(snapshotStore)
    first = service.summarize("summary-1", ("account-1",), _utc(2026, 1, 3))
    second = service.summarize("summary-1", ("account-1",), _utc(2026, 1, 3))
    assert first.summaryHash == second.summaryHash
    assert first.summaryHash == canonicalHash(
        [
            {
                "account_id": "account-1",
                "currency": "CNY",
                "cash_net": Decimal("1000"),
                "asset_value": Decimal("0"),
            }
        ]
    )


def _snapshotHashOf(snapshot: AccountSnapshotV1) -> str:
    return canonicalHash(
        {
            "account_id": snapshot.accountId,
            "snapshot_version": snapshot.snapshotVersion,
            "last_ledger_sequence": snapshot.lastLedgerSequence,
            "projection_hash": canonicalHash(
                [
                    {
                        "ledger_account": item.ledgerAccount.value,
                        "unit_id": item.unitId,
                        "book_currency": item.bookCurrency,
                        "quantity": item.quantity,
                        "book_amount": item.bookAmount,
                    }
                    for item in snapshot.balances
                ]
            ),
        }
    )
