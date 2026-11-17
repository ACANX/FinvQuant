"""P2-004 账户组 worker 单元测试。

验收标准映射：
- 组内串行、组间并行；
- 单组失败不污染其他组；
- LIVE 与非 LIVE 不能混组。
"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.application.AccountGroupTopology import (
    AccountGroupError,
    AccountGroupTopologyV1,
    ExecutionModeV1,
    validateGroupPartitioning,
)
from veritasquant.application.AccountGroupWorker import (
    AccountGroupWorkerV1,
    GroupState,
    GroupWorkerPoolV1,
)
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias


class MarkerPayloadV1(EventPayloadV1):
    marker: str = PascalAlias("Marker")


def _makeEvent() -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId="evt-group-1",
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId="run-1",
        ts=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        source="fixture",
        producer="group-test",
        producerVersion="1.0",
        correlationId="corr-1",
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=10,
        priority=0,
        sourceRank=0,
        sourceSequence=1,
        payload=MarkerPayloadV1.model_validate({"Marker": "m"}),
    )


class TestAccountGroupTopology:
    def test_account_rank_unique_enforced(self) -> None:
        with pytest.raises(AccountGroupError):
            AccountGroupTopologyV1(
                "ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1), ("a2", 1))
            )

    def test_account_not_duplicated_within_group(self) -> None:
        with pytest.raises(AccountGroupError):
            AccountGroupTopologyV1(
                "ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1), ("a1", 2))
            )

    def test_empty_group_rejected(self) -> None:
        with pytest.raises(AccountGroupError):
            AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, ())

    def test_accounts_by_rank_orders_ascending(self) -> None:
        group = AccountGroupTopologyV1(
            "ag-1", ExecutionModeV1.PaperTrading, 0, (("a3", 3), ("a1", 1), ("a2", 2))
        )
        assert group.accountsByRank() == ("a1", "a2", "a3")
        assert group.accountRankFor("a2") == 2
        with pytest.raises(AccountGroupError):
            group.accountRankFor("unknown")

    def test_live_and_non_live_groups_rejected(self) -> None:
        live = AccountGroupTopologyV1("ag-live", ExecutionModeV1.ControlledLive, 0, (("a1", 1),))
        paper = AccountGroupTopologyV1("ag-paper", ExecutionModeV1.PaperTrading, 1, (("a2", 1),))
        with pytest.raises(AccountGroupError):
            validateGroupPartitioning((live, paper))

    def test_account_across_groups_rejected(self) -> None:
        g1 = AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        g2 = AccountGroupTopologyV1("ag-2", ExecutionModeV1.PaperTrading, 1, (("a1", 1),))
        with pytest.raises(AccountGroupError):
            validateGroupPartitioning((g1, g2))

    def test_duplicate_partition_rank_rejected(self) -> None:
        g1 = AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        g2 = AccountGroupTopologyV1("ag-2", ExecutionModeV1.PaperTrading, 0, (("a2", 1),))
        with pytest.raises(AccountGroupError):
            validateGroupPartitioning((g1, g2))

    def test_all_paper_groups_accepted(self) -> None:
        g1 = AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        g2 = AccountGroupTopologyV1("ag-2", ExecutionModeV1.PaperTrading, 1, (("a2", 1),))
        validateGroupPartitioning((g1, g2))


class TestAccountGroupWorker:
    def test_serial_processing_by_rank(self) -> None:
        order: list[str] = []
        group = AccountGroupTopologyV1(
            "ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1), ("a2", 2))
        )
        worker = AccountGroupWorkerV1(
            group,
            {
                "a1": lambda _event: order.append("a1"),
                "a2": lambda _event: order.append("a2"),
            },
        )
        worker.processEvent(_makeEvent())
        assert order == ["a1", "a2"]

    def test_group_failure_isolates_only_its_partition(self) -> None:
        calls: dict[str, int] = {}
        groupA = AccountGroupTopologyV1("ag-a", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        groupB = AccountGroupTopologyV1("ag-b", ExecutionModeV1.PaperTrading, 1, (("b1", 1),))

        def failingHandler(_event: EventEnvelopeV1) -> None:
            calls["a1"] = calls.get("a1", 0) + 1
            raise RuntimeError("account handler failure")

        workerA = AccountGroupWorkerV1(groupA, {"a1": failingHandler})
        workerB = AccountGroupWorkerV1(groupB, {"b1": lambda _event: calls.__setitem__("b1", calls.get("b1", 0) + 1)})
        pool = GroupWorkerPoolV1((workerA, workerB), maxWorkers=2)
        results = pool.fanOut(_makeEvent())
        # 组 B 正常处理完成；组 A 隔离
        assert results["ag-b"].state is GroupState.Active
        assert results["ag-a"].state is GroupState.Isolated
        assert calls["b1"] == 1
        assert pool.isolatedGroups == ("ag-a",)

    def test_missing_handler_rejected(self) -> None:
        group = AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        with pytest.raises(Exception):
            AccountGroupWorkerV1(group, {})

    def test_stopped_worker_rejected(self) -> None:
        group = AccountGroupTopologyV1("ag-1", ExecutionModeV1.PaperTrading, 0, (("a1", 1),))
        worker = AccountGroupWorkerV1(group, {"a1": lambda _event: None})
        worker.stop()
        with pytest.raises(Exception):
            worker.processEvent(_makeEvent())

    def test_parallel_fanout_processes_all_groups(self) -> None:
        processed: set[str] = set()
        groups = [
            AccountGroupWorkerV1(
                AccountGroupTopologyV1(f"ag-{index}", ExecutionModeV1.PaperTrading, index, ((f"a{index}", 1),)),
                {f"a{index}": lambda _event, name=f"ag-{index}": processed.add(name)},
            )
            for index in range(1, 6)
        ]
        pool = GroupWorkerPoolV1(groups, maxWorkers=3)
        results = pool.fanOut(_makeEvent())
        assert len(results) == 5
        assert processed == {f"ag-{index}" for index in range(1, 6)}
