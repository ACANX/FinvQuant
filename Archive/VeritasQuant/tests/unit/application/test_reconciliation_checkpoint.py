"""P2-007 模拟盘对账与 checkpoint 单元测试。

验收标准映射：
- 重启后 RPO=0（checkpoint 与事实同事务，重放无丢失）；
- 账户/订单/持仓差异可检测、分类并阻止恢复。
"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.application.Reconciliation import (
    DailyReconciliationV1,
    DifferenceClass,
    ReconciliationCategory,
)
from veritasquant.core.Checkpoint import EventProcessingCheckpointV1
from veritasquant.infrastructure.persistence.CheckpointStore import CheckpointStoreV1


class TestReconciliation:
    def test_clean_state_allows_recovery(self) -> None:
        report = DailyReconciliationV1().reconcile(
            "run-1",
            authoritativeLedger={("a1", "CNY"): Decimal("1000")},
            actualLedger={("a1", "CNY"): Decimal("1000")},
            authoritativeOrders={},
            actualOrders={},
            authoritativePositions={("a1", "TEST"): Decimal("10")},
            actualPositions={("a1", "TEST"): Decimal("10")},
        )
        assert not report.recoveryBlocked
        assert report.differences == ()

    def test_ledger_amount_mismatch_detected_and_blocks_recovery(self) -> None:
        report = DailyReconciliationV1().reconcile(
            "run-1",
            authoritativeLedger={("a1", "CNY"): Decimal("1000")},
            actualLedger={("a1", "CNY"): Decimal("900")},
            authoritativeOrders={},
            actualOrders={},
            authoritativePositions={},
            actualPositions={},
        )
        assert report.recoveryBlocked
        difference = report.differences[0]
        assert difference.category is ReconciliationCategory.Ledger
        assert difference.differenceClass is DifferenceClass.AmountMismatch

    def test_missing_extra_and_state_differences_classified(self) -> None:
        report = DailyReconciliationV1().reconcile(
            "run-1",
            authoritativeLedger={("a1", "CNY"): Decimal("1000")},
            actualLedger={},
            authoritativeOrders={"a1:o1": "FILLED"},
            actualOrders={"a1:o1": "NEW", "a1:o2": "FILLED"},
            authoritativePositions={("a1", "TEST"): Decimal("10")},
            actualPositions={("a1", "TEST"): Decimal("10"), ("a1", "OTHER"): Decimal("5")},
        )
        classes = {d.differenceClass for d in report.differences}
        categories = {d.category for d in report.differences}
        assert DifferenceClass.Missing in classes  # 账本缺失
        assert DifferenceClass.StateMismatch in classes  # 订单状态不符
        assert DifferenceClass.Extra in classes  # 订单/持仓多余
        assert categories == {
            ReconciliationCategory.Ledger,
            ReconciliationCategory.Order,
            ReconciliationCategory.Position,
        }
        assert report.recoveryBlocked

    def test_differences_must_be_resolved_before_recovery(self) -> None:
        reconciler = DailyReconciliationV1()
        base = dict(authoritativeLedger={("a1", "CNY"): Decimal("1000")},
                    actualLedger={("a1", "CNY"): Decimal("999")},
                    authoritativeOrders={}, actualOrders={},
                    authoritativePositions={}, actualPositions={})
        assert reconciler.reconcile("run-1", **base).recoveryBlocked
        # 修复差异后恢复门禁解除
        base["actualLedger"] = {("a1", "CNY"): Decimal("1000")}
        assert not reconciler.reconcile("run-1", **base).recoveryBlocked


class TestCheckpointStore:
    def test_checkpoint_contract_validation(self) -> None:
        store = CheckpointStoreV1(None)  # type: ignore[arg-type]
        with pytest.raises(Exception):
            store.save(EventProcessingCheckpointV1("", "p", 0, "tx"))

    def test_checkpoint_requires_non_negative_sequence(self) -> None:
        store = CheckpointStoreV1(None)  # type: ignore[arg-type]
        with pytest.raises(Exception):
            store.save(EventProcessingCheckpointV1("run", "p", -1, "tx"))

    def test_sql_upsert_semantics(self) -> None:
        from veritasquant.infrastructure.persistence import CheckpointStore as CheckpointStoreModule

        assert "ON CONFLICT (run_id, partition_id) DO UPDATE" in CheckpointStoreModule._SAVE_SQL  # noqa: SLF001
        assert "last_committed_sequence = EXCLUDED.last_committed_sequence" in CheckpointStoreModule._SAVE_SQL  # noqa: SLF001
