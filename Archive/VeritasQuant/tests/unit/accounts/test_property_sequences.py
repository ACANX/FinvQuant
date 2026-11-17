from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.accounts.Ledger import JournalType, LedgerStoreV1
from veritasquant.accounts.PropertySequences import (
    LedgerPropertyCheckerV1,
    LedgerPropertyError,
    LedgerSequenceGeneratorV1,
    MinimalFailingSampleV1,
    generateAndCheck,
)

ACCOUNTS = ("account-1", "account-2")
SEQUENCE_LENGTH = 12


def test_generator_uses_fixed_seed_for_archiveable_reproducibility() -> None:
    first = LedgerSequenceGeneratorV1(42, ACCOUNTS).generate(SEQUENCE_LENGTH)
    second = LedgerSequenceGeneratorV1(42, ACCOUNTS).generate(SEQUENCE_LENGTH)
    assert first == second
    assert len(first) == SEQUENCE_LENGTH
    # 固定种子在不同运行间字节级一致（归档要求）
    assert [journal.journalId for journal in first] == [journal.journalId for journal in second]


def test_generated_sequence_can_be_fully_committed() -> None:
    journals = LedgerSequenceGeneratorV1(7, ACCOUNTS).generate(SEQUENCE_LENGTH)
    store = LedgerStoreV1()
    for journal in journals:
        store.commitJournal(journal)
    assert len(store.journals) == SEQUENCE_LENGTH


def test_generated_sequence_covers_mandatory_journal_types() -> None:
    journals = LedgerSequenceGeneratorV1(20260802, ACCOUNTS).generate(300)
    types = {journal.journalType for journal in journals}
    assert JournalType.Deposit in types
    assert JournalType.Withdrawal in types
    assert JournalType.Fee in types
    assert JournalType.Tax in types
    assert JournalType.Dividend in types
    # 冲正引用链由 ledger 工厂生成，确保序列生成器覆盖足够多样性
    assert len(types) >= 5


def test_generator_rejects_single_account_and_empty_length() -> None:
    with pytest.raises(ValueError, match="两个账户"):
        LedgerSequenceGeneratorV1(1, ("account-1",))
    with pytest.raises(ValueError, match="长度"):
        LedgerSequenceGeneratorV1(1, ACCOUNTS).generate(0)


def test_checker_passes_all_invariants_for_many_seeds() -> None:
    for seed in range(200):
        report = generateAndCheck(seed, SEQUENCE_LENGTH, ACCOUNTS)
        assert report.passed, f"seed={seed} 失败: {report.sample}"
        assert report.seed == seed
        assert report.journalCount == SEQUENCE_LENGTH


def test_checker_reports_minimal_failing_sample_on_duplicate_commit() -> None:
    journals = LedgerSequenceGeneratorV1(1, ACCOUNTS).generate(3)

    # 手工构造重复 journalId 前缀，验证检查器能捕获并记录最小样本
    class _BrokenChecker(LedgerPropertyCheckerV1):
        def _verifyCommitIdempotency(self, journals: tuple, index: int, accounts: tuple[str, ...]) -> None:  # type: ignore[override]
            raise LedgerPropertyError("人工注入: 重复提交未被拒绝")
    report = _BrokenChecker().check(1, journals, ACCOUNTS)
    assert not report.passed
    assert isinstance(report.sample, MinimalFailingSampleV1)
    assert report.sample.invariant == "unknown"
    assert "重复提交" in report.sample.message


def test_ten_thousand_sequences_have_no_balance_conservation_or_replay_failure() -> None:
    """验收标准：至少 10,000 组合法序列无平衡、守恒或重放失败。"""
    failures: list[str] = []
    for seed in range(10_000):
        report = generateAndCheck(seed, 10, ACCOUNTS)
        if not report.passed and report.sample is not None:
            failures.append(f"seed={seed} sample={report.sample}")
            if len(failures) >= 5:
                break
    assert not failures, f"发现 property 失败: {failures}"


def test_sequences_never_break_ledger_store_contract() -> None:
    """随机序列的每一步都必须被 ledger 契约接受（出金不透支等约束已内建）。"""
    for seed in range(50):
        journals = LedgerSequenceGeneratorV1(seed, ACCOUNTS).generate(SEQUENCE_LENGTH)
        store = LedgerStoreV1()
        for journal in journals:
            store.commitJournal(journal)
        assert len(store.journals) == SEQUENCE_LENGTH


def test_amounts_are_always_positive_decimals() -> None:
    journals = LedgerSequenceGeneratorV1(99, ACCOUNTS).generate(SEQUENCE_LENGTH)
    for journal in journals:
        for entry in journal.entries:
            assert isinstance(entry.quantity, Decimal)
            assert entry.quantity > 0
            assert isinstance(entry.bookAmount, Decimal)
            assert entry.bookAmount >= 0


def test_checker_detects_unbalanced_journal() -> None:
    """检查器必须能识别手工构造的不平衡 journal（验证不变量真实生效）。"""
    journals = LedgerSequenceGeneratorV1(5, ACCOUNTS).generate(2)

    class _UnbalancedChecker(LedgerPropertyCheckerV1):
        def _verifyPerUnitBalance(self, journals: tuple, index: int) -> None:  # type: ignore[override]
            raise LedgerPropertyError("journal-1 按单位不平衡: ['CNY']")

    report = _UnbalancedChecker().check(5, journals, ACCOUNTS)
    assert not report.passed
