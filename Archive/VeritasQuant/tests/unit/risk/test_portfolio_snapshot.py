"""P2-005 组合快照屏障单元测试。

验收标准映射：
- 不齐或不同 barrier 的快照不能拼接；
- 缺失时保持更严格控制（tryAssemble 返回 None）。
"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.risk.PortfolioSnapshot import (
    AccountRiskSnapshotV1,
    PortfolioSnapshotError,
    PortfolioSnapshotRegistryV1,
    PortfolioSnapshotSetV1,
)


def _snapshot(
    accountId: str,
    barrier: str,
    *,
    group: str = "ag-1",
    ledger: int = 1,
    order: int = 1,
    control: int = 1,
) -> AccountRiskSnapshotV1:
    return AccountRiskSnapshotV1(
        accountGroupId=group,
        accountId=accountId,
        barrierEventId=barrier,
        logicalTs=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        ledgerSequence=ledger,
        orderVersion=order,
        controlVersion=control,
    )


class TestSnapshotContract:
    def test_content_hash_changes_with_barrier_or_versions(self) -> None:
        base = _snapshot("a1", "barrier-1")
        otherBarrier = _snapshot("a1", "barrier-2")
        otherLedger = _snapshot("a1", "barrier-1", ledger=99)
        assert base.contentHash != otherBarrier.contentHash
        assert base.contentHash != otherLedger.contentHash
        assert len(base.contentHash) == 64

    def test_matches_barrier(self) -> None:
        assert _snapshot("a1", "b1").matchesBarrier("b1")
        assert not _snapshot("a1", "b1").matchesBarrier("b2")


class TestPortfolioSnapshotSet:
    def test_mixed_barriers_rejected(self) -> None:
        with pytest.raises(PortfolioSnapshotError):
            PortfolioSnapshotSetV1("b1", (_snapshot("a1", "b1"), _snapshot("a2", "b2")))

    def test_duplicate_account_rejected(self) -> None:
        with pytest.raises(PortfolioSnapshotError):
            PortfolioSnapshotSetV1("b1", (_snapshot("a1", "b1"), _snapshot("a1", "b1")))

    def test_empty_set_rejected(self) -> None:
        with pytest.raises(PortfolioSnapshotError):
            PortfolioSnapshotSetV1("b1", ())


class TestRegistryBarrier:
    def test_assemble_requires_all_accounts_at_same_barrier(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        registry.register(_snapshot("a1", "b1"))
        registry.register(_snapshot("a2", "b1"))
        assembled = registry.tryAssemble(("a1", "a2"), "b1")
        assert assembled is not None
        assert assembled.barrierEventId == "b1"
        assert assembled.snapshotFor("a1").accountId == "a1"

    def test_missing_account_returns_none_keeps_tight_control(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        registry.register(_snapshot("a1", "b1"))
        # a2 缺失：不得用新旧快照拼接，维持更严格控制
        assert registry.tryAssemble(("a1", "a2"), "b1") is None

    def test_stale_barrier_returns_none(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        registry.register(_snapshot("a1", "b1"))
        registry.register(_snapshot("a2", "b1"))
        # a1 已推进到 b2，a2 仍在 b1：不同屏障不得拼接
        registry.register(_snapshot("a1", "b2"))
        assert registry.tryAssemble(("a1", "a2"), "b1") is None
        assert registry.tryAssemble(("a1", "a2"), "b2") is None  # a2 尚未到达 b2

    def test_same_barrier_same_account_conflict_rejected(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        registry.register(_snapshot("a1", "b1", ledger=1))
        with pytest.raises(PortfolioSnapshotError):
            registry.register(_snapshot("a1", "b1", ledger=2))

    def test_same_barrier_identical_re_registration_accepted(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        first = _snapshot("a1", "b1")
        registry.register(first)
        registry.register(_snapshot("a1", "b1"))  # 幂等
        assert registry.snapshotFor("a1").contentHash == first.contentHash

    def test_barrier_progression_requires_full_set(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        registry.register(_snapshot("a1", "b1"))
        registry.register(_snapshot("a2", "b1"))
        assert registry.tryAssemble(("a1", "a2"), "b1") is not None
        registry.register(_snapshot("a1", "b2"))
        registry.register(_snapshot("a2", "b2"))
        assembled = registry.tryAssemble(("a1", "a2"), "b2")
        assert assembled is not None
        assert assembled.barrierEventId == "b2"

    def test_empty_target_rejected(self) -> None:
        registry = PortfolioSnapshotRegistryV1()
        with pytest.raises(PortfolioSnapshotError):
            registry.tryAssemble((), "b1")
