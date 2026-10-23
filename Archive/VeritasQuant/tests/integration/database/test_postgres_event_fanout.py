"""P2-003 共享事件持久化与确定性分区扇出集成测试。

验收标准映射：
- 相同 event/hash 按固定 partition_rank 扇出（各分区内容哈希一致）；
- 分区快慢不改事件内容（delivery_sequence 独立，内容哈希不变）。
"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import psycopg
import pytest

from test_db_helpers import applyMigrations, openConnection, resetSchema

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Fanout import (
    DeterministicFanoutV1,
    FanoutTargetV1,
)
from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.infrastructure.persistence.EventStore import EventStoreV1
from veritasquant.infrastructure.persistence.LeaseStore import LeaseStoreV1

_GROUP_A = "ag-events-a"
_GROUP_B = "ag-events-b"
_HOLDER = "fanout-worker"
_RUN = "run-fanout-test"


class BarPayloadV1(EventPayloadV1):
    symbol: str = PascalAlias("Symbol")
    price: Decimal = PascalAlias("Price")


def _makeEvent(eventId: str) -> EventEnvelopeV1:
    return EventEnvelopeV1.create(
        eventId=eventId,
        eventType="MarketBarEvent",
        schemaVersion="1.0",
        runId=_RUN,
        ts=datetime(2026, 8, 2, 0, 0, 0, tzinfo=timezone.utc),
        occurredAt=None,
        publishedAt=None,
        ingestedAt=datetime(2026, 8, 2, 0, 0, 0, 100000, tzinfo=timezone.utc),
        source="fixture",
        producer="fanout-test",
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
        payload=BarPayloadV1.model_validate({"Symbol": "TEST", "Price": Decimal("123.45")}),
    )


@pytest.fixture(scope="module")
def database() -> bool:
    try:
        openConnection().close()
    except psycopg.OperationalError:
        pytest.skip("PostgreSQL 测试实例不可用，跳过事件扇出集成测试")
    resetSchema()
    versions = applyMigrations()
    assert versions
    return True


@pytest.fixture()
def stores(database):
    with openConnection() as connection:
        connection.execute("TRUNCATE fact_events, partition_leases, run_manifests CASCADE")
        connection.execute("DELETE FROM run_manifests WHERE run_id = %s", (_RUN,))
        connection.execute(
            "INSERT INTO run_manifests (run_id, code_version, event_schema_registry_hash, "
            "strategy_version, strategy_source_hash, dependency_lock_hash, interpreter_version, "
            "sandbox_image_digest, strategy_sandbox_policy_version, strategy_dsl_schema_version, "
            "investment_plan_schema_version, config_hash, config_schema_version, data_version_id, "
            "asset_capability_version, account_group_id, account_ranks, random_seed, ts_precision, "
            "event_ordering_version, execution_model_version, fund_execution_model_version, "
            "nav_availability_policy_version, bar_path_model_version, liquidity_allocation_version, "
            "risk_policy_version, reliability_policy_version, started_at) "
            "VALUES (%s, 'v', '0'*64, 'v', '0'*64, '0'*64, 'v', 'd', 'v', 'v', 'v', "
            "'0'*64, 'v', 'dv', 'v', 'ag', '{}', 1, 'MILLISECOND', 'V1', 'v', 'v', 'v', 'v', 'v', "
            "'v', 'v', now())",
            (_RUN,),
        )
    connection = openConnection()
    leaseStore = LeaseStoreV1(connection)
    eventStore = EventStoreV1(connection, leaseStore)
    leaseA = leaseStore.acquire(_GROUP_A, _HOLDER, ttlSeconds=30)
    leaseB = leaseStore.acquire(_GROUP_B, _HOLDER, ttlSeconds=30)
    yield leaseStore, eventStore, leaseA, leaseB
    connection.close()


class TestEventStoreAndFanout:
    def test_same_event_fans_out_with_identical_hash_and_rank_order(self, stores) -> None:  # noqa: ANN001
        _, eventStore, leaseA, leaseB = stores
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1(_GROUP_B, 2),
                FanoutTargetV1(_GROUP_A, 1),
            )
        )
        event = _makeEvent("evt-fanout-1")
        # 快分区 A 已消费 3 个事件，慢分区 B 为 0：分区快慢不改事件内容
        sequences = {_GROUP_A: 3, _GROUP_B: 0}
        deliveries = fanout.plan(event, sequences)
        tokens = {_GROUP_A: leaseA.fencingToken, _GROUP_B: leaseB.fencingToken}
        for delivery in deliveries:
            eventStore.append(
                delivery.event, delivery.accountGroupId, delivery.partitionRank,
                delivery.deliverySequence, _HOLDER, tokens[delivery.accountGroupId],
            )
        with openConnection() as connection:
            rows = connection.execute(
                "SELECT account_group_id, partition_rank, delivery_sequence, content_hash "
                "FROM fact_events WHERE event_id = 'evt-fanout-1' ORDER BY partition_rank"
            ).fetchall()
        assert [(r[0], r[1], r[2]) for r in rows] == [
            (_GROUP_A, 1, 4),  # 快分区：3 + 1
            (_GROUP_B, 2, 1),  # 慢分区：0 + 1
        ]
        hashes = {r[3] for r in rows}
        assert len(hashes) == 1, "同一共享事件在所有分区必须保持相同内容哈希"

    def test_partition_sequences_advance_independently(self, stores) -> None:  # noqa: ANN001
        _, eventStore, leaseA, leaseB = stores
        fanout = DeterministicFanoutV1(
            (
                FanoutTargetV1(_GROUP_A, 1),
                FanoutTargetV1(_GROUP_B, 2),
            )
        )
        tokens = {_GROUP_A: leaseA.fencingToken, _GROUP_B: leaseB.fencingToken}
        sequences: dict[str, int] = {}
        for index in range(1, 4):
            event = _makeEvent(f"evt-seq-{index}")
            for delivery in fanout.plan(event, sequences):
                eventStore.append(
                    delivery.event, delivery.accountGroupId, delivery.partitionRank,
                    delivery.deliverySequence, _HOLDER, tokens[delivery.accountGroupId],
                )
                sequences[delivery.accountGroupId] = delivery.deliverySequence
        assert eventStore.latestDeliverySequence(_RUN, _GROUP_A) == 3
        assert eventStore.latestDeliverySequence(_RUN, _GROUP_B) == 3
        assert eventStore.countByPartition(_RUN, _GROUP_A) == 3
        # 慢分区追赶：再投递一个，只影响该分区序号
        event = _makeEvent("evt-seq-4")
        tokens = {_GROUP_A: leaseA.fencingToken, _GROUP_B: leaseB.fencingToken}
        for delivery in fanout.plan(event, {_GROUP_A: 3, _GROUP_B: 3}):
            eventStore.append(
                delivery.event, delivery.accountGroupId, delivery.partitionRank,
                delivery.deliverySequence, _HOLDER, tokens[delivery.accountGroupId],
            )
        assert eventStore.latestDeliverySequence(_RUN, _GROUP_A) == 4
        assert eventStore.latestDeliverySequence(_RUN, _GROUP_B) == 4

    def test_append_same_event_id_idempotent(self, stores) -> None:  # noqa: ANN001
        _, eventStore, leaseA, leaseB = stores
        event = _makeEvent("evt-dup")
        first = eventStore.append(event, _GROUP_A, 1, 1, _HOLDER, leaseA.fencingToken)
        second = eventStore.append(event, _GROUP_A, 1, 99, _HOLDER, leaseA.fencingToken)
        assert first == 1
        assert second == 1  # 同 event_id 幂等返回原 delivery_sequence
        assert eventStore.countByPartition(_RUN, _GROUP_A) == 1

    def test_stale_token_write_rejected(self, stores) -> None:  # noqa: ANN001
        leaseStore, eventStore, leaseA, leaseB = stores
        with openConnection() as connection:
            connection.execute(
                "UPDATE partition_leases SET lease_expires_at = now() - interval '1 second' "
                "WHERE account_group_id = %s",
                (_GROUP_A,),
            )
        leaseStore.acquire(_GROUP_A, "new-holder", ttlSeconds=30)
        from veritasquant.infrastructure.persistence.LeaseStore import LeaseError

        with pytest.raises(LeaseError):
            eventStore.append(_makeEvent("evt-stale"), _GROUP_A, 1, 1, _HOLDER, leaseA.fencingToken)
