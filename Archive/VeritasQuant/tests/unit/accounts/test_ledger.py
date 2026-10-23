from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.accounts.Ledger import (
    AccountingPolicyV1,
    AssetUnitV1,
    CashJournalFactoryV1,
    EntryDirection,
    JournalType,
    JournalV1,
    LedgerAccount,
    LedgerContractError,
    LedgerEntryV1,
    LedgerProjectionStoreV1,
    LedgerStoreV1,
)


def _cashUnit() -> AssetUnitV1:
    return AssetUnitV1(UnitId="CNY", AssetId="CNY", Currency="CNY")


def _entry(entryId: str, direction: EntryDirection, quantity: Decimal = Decimal("100")) -> LedgerEntryV1:
    return LedgerEntryV1(
        EntryId=entryId,
        LedgerAccount=LedgerAccount.CashAvailable,
        Direction=direction,
        Unit=_cashUnit(),
        Quantity=quantity,
        BookCurrency="CNY",
        BookAmount=quantity,
        CostAmount=Decimal("0"),
    )


def _journal(**overrides: object) -> JournalV1:
    values: dict[str, object] = {
        "JournalId": "journal-1",
        "JournalType": JournalType.OpeningBalance,
        "AccountId": "account-1",
        "Ts": datetime(2026, 8, 1, tzinfo=timezone.utc),
        "CommitSequence": 1,
        "SourceEventId": "event-1",
        "InstrumentMetadataVersion": "instrument-v1",
        "FeeScheduleVersion": "fee-v1",
        "AccountingPolicyVersion": "policy-v1",
        "Entries": (_entry("entry-1", EntryDirection.Debit), _entry("entry-2", EntryDirection.Credit)),
    }
    values.update(overrides)
    return JournalV1.model_validate(values)


def test_journal_requires_balanced_decimal_entries_per_unit() -> None:
    journal = _journal()
    assert journal.entries[0].quantity == Decimal("100")
    assert journal.model_dump(by_alias=True)["JournalId"] == "journal-1"


def test_journal_rejects_unbalanced_or_duplicate_entries() -> None:
    with pytest.raises(ValidationError, match="不平衡"):
        _journal(Entries=(_entry("entry-1", EntryDirection.Debit), _entry("entry-2", EntryDirection.Credit, Decimal("99"))))
    with pytest.raises(ValidationError, match="entryId"):
        _journal(Entries=(_entry("entry-1", EntryDirection.Debit), _entry("entry-1", EntryDirection.Credit)))
    with pytest.raises(ValidationError, match="BOOK:CNY"):
        _journal(
            Entries=(
                _entry("entry-1", EntryDirection.Debit),
                LedgerEntryV1(
                    EntryId="entry-2",
                    LedgerAccount=LedgerAccount.CashAvailable,
                    Direction=EntryDirection.Credit,
                    Unit=_cashUnit(),
                    Quantity=Decimal("100"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("99"),
                    CostAmount=Decimal("0"),
                ),
            )
        )


def test_journal_requires_utc_and_valid_reversal_reference() -> None:
    with pytest.raises(ValidationError, match="UTC"):
        _journal(Ts=datetime(2026, 8, 1))
    with pytest.raises(ValidationError, match="必须引用"):
        _journal(JournalType=JournalType.Reversal)
    reversedJournal = _journal(JournalType=JournalType.Reversal, ReversalOfJournalId="journal-original")
    assert reversedJournal.reversalOfJournalId == "journal-original"


def test_ledger_rejects_float_and_invalid_measurement_unit() -> None:
    with pytest.raises(ValidationError):
        _entry("entry-1", EntryDirection.Debit, 100.0)  # type: ignore[arg-type]
    with pytest.raises(ValidationError, match="INSTRUMENT"):
        AssetUnitV1(UnitId="gold", AssetId="gold")


def test_accounting_policy_requires_fixed_rounding_and_hash() -> None:
    policy = AccountingPolicyV1(
        PolicyVersion="policy-v1",
        CostBasisMethod="MOVING_AVERAGE",
        MonetaryRounding="ROUND_HALF_EVEN",
        PolicyHash="a" * 64,
    )
    assert policy.policyVersion == "policy-v1"


def test_ledger_store_commits_only_complete_journals_in_monotonic_order() -> None:
    store = LedgerStoreV1()
    committed = store.commitJournal(_journal())
    assert committed.journalId == "journal-1"
    assert len(store.journals) == 1
    assert len(store.entries) == 2
    second = _journal(JournalId="journal-2", CommitSequence=2)
    assert store.commitJournal(second).commitSequence == 2


def test_ledger_store_rejects_invalid_or_duplicate_commit_without_mutation() -> None:
    store = LedgerStoreV1()
    valid = _journal()
    invalid = valid.model_copy(
        update={"entries": (_entry("entry-1", EntryDirection.Debit), _entry("entry-2", EntryDirection.Credit, Decimal("99")))}
    )
    with pytest.raises(ValidationError, match="不平衡"):
        store.commitJournal(invalid)
    assert store.journals == ()
    store.commitJournal(valid)
    with pytest.raises(LedgerContractError, match="重复记账"):
        store.commitJournal(valid)
    with pytest.raises(LedgerContractError, match="序号"):
        store.commitJournal(_journal(JournalId="journal-2", CommitSequence=3))
    assert tuple(item.journalId for item in store.journals) == ("journal-1",)


def test_ledger_projection_rebuilds_cash_and_frozen_balances() -> None:
    store = LedgerStoreV1()
    store.commitJournal(
        _journal(
            Entries=(
                _entry("entry-1", EntryDirection.Debit, Decimal("10000")),
                LedgerEntryV1(
                    EntryId="entry-2",
                    LedgerAccount=LedgerAccount.ExternalCapital,
                    Direction=EntryDirection.Credit,
                    Unit=_cashUnit(),
                    Quantity=Decimal("10000"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("10000"),
                    CostAmount=Decimal("0"),
                ),
            )
        )
    )
    store.commitJournal(
        _journal(
            JournalId="journal-2",
            JournalType=JournalType.OrderReservation,
            CommitSequence=2,
            Entries=(
                LedgerEntryV1(
                    EntryId="entry-3",
                    LedgerAccount=LedgerAccount.CashFrozen,
                    Direction=EntryDirection.Debit,
                    Unit=_cashUnit(),
                    Quantity=Decimal("1000"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("1000"),
                    CostAmount=Decimal("0"),
                ),
                _entry("entry-4", EntryDirection.Credit, Decimal("1000")),
            ),
        )
    )
    snapshot = LedgerProjectionStoreV1(store).rebuild("account-1")
    assert snapshot.lastLedgerSequence == 2
    assert snapshot.balanceFor(LedgerAccount.CashAvailable, "CNY", "CNY").quantity == Decimal("9000")
    assert snapshot.balanceFor(LedgerAccount.CashFrozen, "CNY", "CNY").bookAmount == Decimal("1000")


def test_ledger_projection_rebuild_hash_is_stable_after_projection_discard() -> None:
    store = LedgerStoreV1()
    store.commitJournal(_journal())
    first = LedgerProjectionStoreV1(store).rebuild("account-1")
    rebuilt = LedgerProjectionStoreV1(store).rebuild("account-1")
    assert rebuilt.balances == first.balances
    assert rebuilt.projectionHash == first.projectionHash


def test_ledger_projection_includes_position_cost_and_profit_loss_accounts() -> None:
    store = LedgerStoreV1()
    stockUnit = AssetUnitV1(UnitId="INSTRUMENT:518880", AssetId="518880")
    store.commitJournal(
        _journal(
            JournalType=JournalType.Trade,
            Entries=(
                LedgerEntryV1(
                    EntryId="entry-1",
                    LedgerAccount=LedgerAccount.SecuritiesAvailable,
                    Direction=EntryDirection.Debit,
                    Unit=stockUnit,
                    Quantity=Decimal("10"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("1000"),
                    CostAmount=Decimal("1000"),
                ),
                LedgerEntryV1(
                    EntryId="entry-2",
                    LedgerAccount=LedgerAccount.TradingClearing,
                    Direction=EntryDirection.Credit,
                    Unit=stockUnit,
                    Quantity=Decimal("10"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("1000"),
                    CostAmount=Decimal("1000"),
                ),
            ),
        )
    )
    store.commitJournal(
        _journal(
            JournalId="journal-2",
            JournalType=JournalType.Trade,
            CommitSequence=2,
            Entries=(
                LedgerEntryV1(
                    EntryId="entry-3",
                    LedgerAccount=LedgerAccount.TradingClearing,
                    Direction=EntryDirection.Debit,
                    Unit=_cashUnit(),
                    Quantity=Decimal("12"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("12"),
                    CostAmount=Decimal("0"),
                ),
                LedgerEntryV1(
                    EntryId="entry-4",
                    LedgerAccount=LedgerAccount.RealizedProfitLoss,
                    Direction=EntryDirection.Credit,
                    Unit=_cashUnit(),
                    Quantity=Decimal("12"),
                    BookCurrency="CNY",
                    BookAmount=Decimal("12"),
                    CostAmount=Decimal("0"),
                ),
            ),
        )
    )
    snapshot = LedgerProjectionStoreV1(store).rebuild("account-1")
    position = snapshot.balanceFor(LedgerAccount.SecuritiesAvailable, "INSTRUMENT:518880", "CNY")
    profitLoss = snapshot.balanceFor(LedgerAccount.RealizedProfitLoss, "CNY", "CNY")
    assert position.quantity == Decimal("10")
    assert position.bookAmount == Decimal("1000")
    assert profitLoss.bookAmount == Decimal("-12")


def test_cash_journal_factory_creates_funding_fee_tax_and_reversal() -> None:
    factory = CashJournalFactoryV1("instrument-v1", "fee-v1", "policy-v1")
    ts = datetime(2026, 8, 1, tzinfo=timezone.utc)
    opening = factory.createOpeningBalance("opening-1", "account-1", ts, 1, "event-1", "CNY", Decimal("100"))
    deposit = factory.createDeposit("deposit-1", "account-1", ts, 2, "event-2", "CNY", Decimal("5"))
    fee = factory.createFee("fee-1", "account-1", ts, 2, "event-2", "CNY", Decimal("3"))
    tax = factory.createTax("tax-1", "account-1", ts, 3, "event-3", "CNY", Decimal("2"))
    dividend = factory.createDividend("dividend-1", "account-1", ts, 4, "event-4", "CNY", Decimal("8"))
    reversal = factory.createReversal("reversal-1", "event-4", fee, 4)
    assert opening.journalType is JournalType.OpeningBalance
    assert deposit.journalType is JournalType.Deposit
    assert fee.entries[0].ledgerAccount is LedgerAccount.FeeExpense
    assert tax.entries[0].ledgerAccount is LedgerAccount.TaxExpense
    assert dividend.journalType is JournalType.Dividend
    assert reversal.reversalOfJournalId == fee.journalId
    assert reversal.entries[0].direction is EntryDirection.Credit


def test_cash_journal_factory_rejects_overdraft_withdrawal() -> None:
    factory = CashJournalFactoryV1("instrument-v1", "fee-v1", "policy-v1")
    with pytest.raises(LedgerContractError, match="不得超过"):
        factory.createWithdrawal(
            "withdrawal-1", "account-1", datetime(2026, 8, 1, tzinfo=timezone.utc), 1, "event-1", "CNY", Decimal("101"), Decimal("100")
        )
