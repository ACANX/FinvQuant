"""P2-040 运行期观测接线测试。"""

from __future__ import annotations

from datetime import datetime, timezone


from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    LedgerStoreV1,
)
from veritasquant.application.AccountGroupTopology import (
    AccountGroupTopologyV1,
    ExecutionModeV1,
)
from veritasquant.application.AccountGroupWorker import AccountGroupWorkerV1
from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.monitoring.SloCalculator import (
    ExecutionMode,
    SliName,
)
from veritasquant.reliability.ObservabilityWiring import (
    ObservabilityWiringV1,
)

UTC = timezone.utc


class MarkerPayloadV1(EventPayloadV1):
    marker: str = PascalAlias("Marker")


def _event(eventId: str) -> EventEnvelopeV1:
    now = datetime.now(UTC).replace(microsecond=0)
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="TestEvent",
        schemaVersion="1.0",
        runId="run-wiring",
        ts=now,
        occurredAt=None,
        publishedAt=None,
        ingestedAt=now,
        source="fixture",
        producer="wiring-test",
        producerVersion="1.0",
        correlationId="corr-" + eventId,
        causationId=None,
        accountId=None,
        subaccountId=None,
        eventOrderingVersion="V1",
        phase=10,
        priority=0,
        sourceRank=0,
        sourceSequence=1,
        payload=MarkerPayloadV1.model_validate({"Marker": eventId}),
    )


class _FakeReadiness:
    def ready(self) -> bool:
        return True

    def state(self) -> str:
        return "TRADING_READY"

    def failedChecks(self) -> tuple[object, ...]:
        return ()


class TestObservabilityWiring:
    def test_wiring_exposes_collector_and_calculator(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        assert wiring.registry is not None
        assert wiring.collector is not None
        assert wiring.calculator is not None
        assert wiring.runId == "run-1"

    def test_snapshot_contains_metrics_and_slo(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        wiring.collector.observeOrderTransition("FILLED", "g1")
        snapshot = wiring.snapshot()
        assert "vq_order_state_transitions_total" in snapshot.metricsText
        assert len(snapshot.sloSummaries) == 1
        assert snapshot.sloSummaries[0].mode is ExecutionMode.Paper


class TestInstrumentedLedger:
    def test_commit_records_ledger_metric(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        inner = LedgerStoreV1()
        ledger = wiring.ledger(inner, "g1")
        journal = CashJournalFactoryV1(
            "metadata-v1", "fees-v1", "policy-v1"
        ).createOpeningBalance(
            "open-1", "acc-1", datetime(2026, 1, 1, tzinfo=UTC), 1,
            "wiring-open", "CNY", __import__("decimal").Decimal("100000"),
        )
        ledger.commitJournal(journal)
        text = wiring.registry.render()
        assert 'vq_ledger_commits_total{account_group="g1"} 1.0' in text
        assert 'vq_ledger_commit_latency_seconds_count{account_group="g1"} 1.0' in text
        assert len(inner.journals) == 1  # 内层真实提交


class TestInstrumentedWorker:
    def test_process_records_event_latency(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        topology = AccountGroupTopologyV1(
            accountGroupId="g1",
            executionMode=ExecutionModeV1.PaperTrading,
            partitionRank=0,
            accountRanks=(("acc-1", 0),),
        )
        received: list[str] = []

        def handler(event: EventEnvelopeV1) -> None:
            received.append(event.eventId)

        worker = AccountGroupWorkerV1(topology, {"acc-1": handler})
        instrumented = wiring.worker(worker, "g1")
        instrumented.processEvent(_event("evt-1"))
        assert received == ["evt-1"]
        text = wiring.registry.render()
        assert 'vq_event_ingest_latency_seconds_count{account_group="g1"} 1.0' in text


class TestSloObservationRecorder:
    def test_record_and_evaluate(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        recorder = wiring.recorder("g1")
        recorder.record(SliName.LedgerCommitLatencyP99, 0.2, "2026-08-01")
        summary = recorder.evaluate()
        led = next(r for r in summary.results if r.sli is SliName.LedgerCommitLatencyP99)
        assert led.observedDays == 1
        assert led.status.value == "WITHIN_BUDGET"

    def test_record_propagates_to_calculator(self) -> None:
        wiring = ObservabilityWiringV1(mode=ExecutionMode.Paper, runId="run-1")
        recorder = wiring.recorder("g1")
        recorder.record(SliName.OutboxMaxAge, 5.0, "2026-08-01")
        assert len(wiring.calculator.observations()) == 1
        assert wiring.calculator.observations()[0].accountGroup == "g1"
        assert wiring.calculator.observations()[0].runId == "run-1"
