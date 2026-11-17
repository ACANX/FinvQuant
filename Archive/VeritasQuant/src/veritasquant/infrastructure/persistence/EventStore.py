"""P2-003 共享事件持久化存储。

共享市场事件只创建一次并持久化到 `fact_events`；确定性扇出器按
`partition_rank` 升序将同一事件写入每个账户组分区。同一共享事件在
所有分区保留完全相同的信封、排序键和内容哈希；分区投递序号
（`delivery_sequence`）是信封外元数据，不参与事件内容哈希。

写入由单活租约串行化：同一分区同一时刻只有一个写入者，因此
`MAX(delivery_sequence) + 1` 在租约保护下是安全的。
"""

from __future__ import annotations

from psycopg import Connection
from psycopg.types.json import Jsonb

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.infrastructure.persistence.LeaseStore import LeaseStoreV1

_INSERT_SQL = """
INSERT INTO fact_events (
    event_id, event_type, schema_version, run_id, ts, occurred_at, published_at,
    ingested_at, source, producer, producer_version, correlation_id, causation_id,
    account_id, subaccount_id, event_ordering_version, phase, priority, source_rank,
    source_sequence, payload, content_hash, account_group_id, partition_rank,
    delivery_sequence
) VALUES (
    %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s,
    %s, %s::jsonb, %s, %s, %s, %s
)
ON CONFLICT (event_id, account_group_id) DO NOTHING
"""

_FETCH_BY_ID_SQL = """
SELECT event_id, content_hash, account_group_id, partition_rank, delivery_sequence
FROM fact_events WHERE event_id = %s AND account_group_id = %s
"""

_LATEST_DELIVERY_SQL = """
SELECT COALESCE(MAX(delivery_sequence), 0)
FROM fact_events WHERE run_id = %s AND account_group_id = %s
"""


class EventStoreError(RuntimeError):
    """事件持久化不合法。"""


class EventStoreV1:
    """PostgreSQL 事件事实存储。"""

    def __init__(self, connection: Connection, leaseStore: LeaseStoreV1) -> None:
        self._connection = connection
        self._leaseStore = leaseStore

    def append(
        self,
        event: EventEnvelopeV1,
        accountGroupId: str,
        partitionRank: int,
        deliverySequence: int,
        holder: str,
        fencingToken: int,
    ) -> int:
        """持久化事件；同 event_id 幂等（返回既有 delivery_sequence）。"""
        if partitionRank < 0 or deliverySequence < 1:
            raise EventStoreError("partition_rank 非负且 delivery_sequence 必须为正")
        with self._connection.transaction():
            self._leaseStore.guard(accountGroupId, holder, fencingToken)
            self._connection.execute(
                _INSERT_SQL,
                (
                    event.eventId, event.eventType, event.schemaVersion, event.runId,
                    event.ts, event.occurredAt, event.publishedAt, event.ingestedAt,
                    event.source, event.producer, event.producerVersion,
                    event.correlationId, event.causationId, event.accountId,
                    event.subaccountId, event.eventOrderingVersion, event.phase,
                    event.priority, event.sourceRank, event.sourceSequence,
                    Jsonb(event.payload.model_dump(mode="json", by_alias=True)),
                    event.contentHash, accountGroupId, partitionRank, deliverySequence,
                ),
            )
            row = self._connection.execute(_FETCH_BY_ID_SQL, (event.eventId, accountGroupId)).fetchone()
            assert row is not None, "事件写入后记录必须存在"
            return int(row[4])
    def latestDeliverySequence(self, runId: str, accountGroupId: str) -> int:
        """返回分区当前最大投递序号（0 表示尚无事件）。"""
        row = self._connection.execute(
            _LATEST_DELIVERY_SQL, (runId, accountGroupId)
        ).fetchone()
        assert row is not None
        return int(row[0])

    def countByPartition(self, runId: str, accountGroupId: str) -> int:
        """返回分区内已持久化事件数。"""
        row = self._connection.execute(
            "SELECT count(*) FROM fact_events WHERE run_id = %s AND account_group_id = %s",
            (runId, accountGroupId),
        ).fetchone()
        assert row is not None
        return int(row[0])
