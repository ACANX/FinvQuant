from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.execution.OrderStateMachine import OrderStateMachineV1, TransitionKind
from veritasquant.execution.Orders import (
    BrokerState,
    ExecutionReportEventV1,
    ExecutionType,
)
from veritasquant.execution.ReportProcessor import (
    ReportDisposition,
    ReportProcessingError,
    ReportProcessorV1,
)

UTC = timezone.utc


def _utc(hour: int = 0) -> datetime:
    return datetime(2026, 8, 2, hour, tzinfo=UTC)


def _machine() -> OrderStateMachineV1:
    machine = OrderStateMachineV1()
    machine.createIntent("client-1", "account-1", Decimal("100"), 0)
    machine.transition("client-1", "account-1", TransitionKind.RiskApproval, 1)
    machine.transition("client-1", "account-1", TransitionKind.CommandOutbox, 2)
    machine.transition("client-1", "account-1", TransitionKind.SendSuccess, 3)
    machine.transition("client-1", "account-1", TransitionKind.BrokerAccept, 4)
    return machine


def _report(sequence: int, **overrides: object) -> ExecutionReportEventV1:
    values: dict[str, object] = {
        "BrokerReportId": f"report-{sequence}",
        "ClientOrderId": "client-1",
        "BrokerOrderId": "broker-1",
        "ReportSequence": sequence,
        "ExecutionType": ExecutionType.PartialFill,
        "ExecutionId": f"exec-{sequence}",
        "LastQuantity": Decimal("40"),
        "LastPrice": Decimal("1.200"),
        "CumulativeQuantity": Decimal("40"),
        "RemainingQuantity": Decimal("60"),
        "BrokerState": BrokerState.Partial,
        "DiagnosticTs": _utc(),
        "AccountId": "account-1",
        "Ts": _utc(),
    }
    values.update(overrides)
    return ExecutionReportEventV1.model_validate(values)


def _processor() -> ReportProcessorV1:
    return ReportProcessorV1(_machine())


def test_duplicate_report_returns_committed_result() -> None:
    processor = _processor()
    first = processor.process(_report(1))
    assert first.disposition is ReportDisposition.Applied
    duplicate = processor.process(_report(1))
    assert duplicate.disposition is ReportDisposition.Duplicate
    assert processor.cumulativeFor("client-1") == Decimal("40")


def test_same_report_id_different_hash_is_conflict() -> None:
    processor = _processor()
    processor.process(_report(1))
    conflict = processor.process(_report(1, LastQuantity=Decimal("50"), CumulativeQuantity=Decimal("50")))
    assert conflict.disposition is ReportDisposition.Conflict


def test_duplicate_execution_id_is_deduplicated() -> None:
    processor = _processor()
    processor.process(_report(1))
    # 同 executionId 的非成交回报（撤单）仍按重复处置
    # 同 executionId 的重复回报（不同 reportId）：按重复处置
    sameExecution = _report(
        2,
        ExecutionId="exec-1",
        LastQuantity=Decimal("40"),
        CumulativeQuantity=Decimal("40"),
        RemainingQuantity=Decimal("60"),
    )
    assert processor.process(sameExecution).disposition is ReportDisposition.Duplicate


def test_stale_sequence_does_not_regress_state() -> None:
    processor = _processor()
    processor.process(_report(1))
    processor.process(_report(2, ExecutionId="exec-2", LastQuantity=Decimal("60"), CumulativeQuantity=Decimal("100"), RemainingQuantity=Decimal("0")))
    # 新 reportId/executionId 但旧序号：只保留审计，不回退状态
    stale = processor.process(
        _report(
            1,
            BrokerReportId="report-99",
            ExecutionId="exec-99",
            LastQuantity=Decimal("40"),
            CumulativeQuantity=Decimal("40"),
            RemainingQuantity=Decimal("60"),
        )
    )
    assert stale.disposition is ReportDisposition.StaleSequence
    assert processor.cumulativeFor("client-1") == Decimal("100")


def test_gap_pauses_advance_until_verified() -> None:
    processor = _processor()
    result = processor.process(_report(3))
    assert result.disposition is ReportDisposition.GapPaused
    assert processor.isGapPaused("account-1")
    assert processor.pendingGapCount("account-1") == 1
    # 权威快照核验后补齐缺口
    processor.applyVerifiedSnapshot("account-1", 3)
    assert not processor.isGapPaused("account-1")
    assert processor.cumulativeFor("client-1") == Decimal("40")


def test_gap_filled_by_intervening_sequence() -> None:
    processor = _processor()
    # 先到高序号：进入缺口缓冲并暂停
    assert processor.process(_report(2)).disposition is ReportDisposition.GapPaused
    assert processor.isGapPaused("account-1")
    # 后到低序号：应用并尝试消费连续缓冲
    assert processor.process(_report(1)).disposition is ReportDisposition.Applied
    assert processor.cumulativeFor("client-1") == Decimal("40")


def test_unknown_order_goes_to_isolation() -> None:
    machine = _machine()
    processor = ReportProcessorV1(machine)
    result = processor.process(_report(1, ClientOrderId="ghost-order"))
    assert result.disposition is ReportDisposition.UnknownOrder
    records = processor.unknownOrders()
    assert len(records) == 1
    assert records[0].clientOrderId == "ghost-order"


def test_cumulative_decrease_is_quarantined() -> None:
    processor = _processor()
    processor.process(_report(1))
    # 序列 2 累计量 30 < 已应用 40：必须拒绝且不改变状态
    result = processor.process(_report(2, ExecutionId="exec-2", LastQuantity=Decimal("30"), CumulativeQuantity=Decimal("30")))
    assert result.disposition is ReportDisposition.Quarantined
    assert processor.cumulativeFor("client-1") == Decimal("40")


def test_cumulative_above_order_quantity_is_quarantined() -> None:
    processor = _processor()
    result = processor.process(_report(1, LastQuantity=Decimal("101"), CumulativeQuantity=Decimal("101")))
    assert result.disposition is ReportDisposition.Quarantined


def test_late_fill_after_cancel_chain_is_recorded() -> None:
    machine = _machine()
    processor = ReportProcessorV1(machine)
    processor.process(_report(1))
    processor.process(_report(2, ExecutionId="exec-2", LastQuantity=Decimal("60"), CumulativeQuantity=Decimal("100"), RemainingQuantity=Decimal("0")))
    # 撤单确认回报（全成后仅审计）
    result = processor.process(_report(3, ExecutionType=ExecutionType.Cancelled, ExecutionId=None, LastQuantity=Decimal("0"), LastPrice=None, CumulativeQuantity=Decimal("100"), RemainingQuantity=Decimal("0"), BrokerState=BrokerState.Cancelled))
    assert result.disposition is ReportDisposition.Applied
    assert processor.cumulativeFor("client-1") == Decimal("100")


def test_audit_trail_records_every_disposition() -> None:
    processor = _processor()
    processor.process(_report(1))
    processor.process(_report(1))
    processor.process(_report(3))
    dispositions = [item.disposition for item in processor.audit]
    assert dispositions == [
        ReportDisposition.Applied,
        ReportDisposition.Duplicate,
        ReportDisposition.GapPaused,
    ]


def test_verified_snapshot_below_applied_sequence_rejected() -> None:
    processor = _processor()
    processor.process(_report(1))
    processor.process(
        _report(
            2,
            ExecutionId="exec-2",
            LastQuantity=Decimal("60"),
            CumulativeQuantity=Decimal("100"),
            RemainingQuantity=Decimal("0"),
        )
    )
    with pytest.raises(ReportProcessingError, match="不得低于"):
        processor.applyVerifiedSnapshot("account-1", 1)
