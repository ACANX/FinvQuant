"""P2-002 inbox/outbox/租约数据库集成测试（CI postgres service 运行）。

验收标准映射：
- 双写者测试中旧 token 写入被拒绝（fencing token 门禁）；
- 重投无重复副作用（inbox 幂等、outbox 幂等发布）。
"""

from __future__ import annotations

import psycopg
import pytest

from test_db_helpers import applyMigrations, openConnection, resetSchema

from veritasquant.infrastructure.persistence.InboxStore import InboxError, InboxStoreV1
from veritasquant.infrastructure.persistence.LeaseStore import LeaseError, LeaseStoreV1
from veritasquant.infrastructure.persistence.OutboxStore import OutboxStoreV1

_GROUP = "ag-fencing-test"
_HOLDER_A = "worker-a"
_HOLDER_B = "worker-b"
_RUN = "run-fencing-test"
_PARTITION = _GROUP


@pytest.fixture(scope="module")
def database() -> bool:
    try:
        openConnection().close()
    except psycopg.OperationalError:
        pytest.skip("PostgreSQL 测试实例不可用，跳过 inbox/outbox/租约集成测试")
    resetSchema()
    versions = applyMigrations()
    assert versions, "首版迁移应至少应用一个版本"
    return True


@pytest.fixture()
def seededRun(database) -> str:  # noqa: ANN001
    runId = _RUN
    with openConnection() as connection:
        # 先清引用表再重建 run_manifests（不可变事实表禁止 DELETE，使用 TRUNCATE）
        connection.execute(
            "TRUNCATE inbox_records, inbox_conflicts, outbox_records, "
            "partition_leases, fact_events, run_manifests CASCADE"
        )
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
            (runId,),
        )
    yield runId
    with openConnection() as connection:
        # 先清引用表（不可变事实表禁止 DELETE）再删 run_manifests，避免外键残留
        connection.execute(
            "TRUNCATE inbox_records, inbox_conflicts, outbox_records, "
            "partition_leases, fact_events CASCADE"
        )
        connection.execute("DELETE FROM run_manifests WHERE run_id = %s", (runId,))


@pytest.fixture()
def stores(seededRun: str):
    """清理租约/收件/外发残留并返回 store 三件套。

    不可变事实表（inbox_records/inbox_conflicts）禁止 DELETE，使用 TRUNCATE；
    run_manifests 由 seededRun 提供，不在本清理范围。
    """
    with openConnection() as connection:
        connection.execute(
            "TRUNCATE inbox_records, inbox_conflicts, outbox_records, partition_leases"
        )
    connection = openConnection()
    leaseStore = LeaseStoreV1(connection)
    inboxStore = InboxStoreV1(connection, leaseStore)
    outboxStore = OutboxStoreV1(connection, leaseStore)
    yield leaseStore, inboxStore, outboxStore
    connection.close()


class TestLeaseFencing:
    def test_acquire_renew_release_lifecycle(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, _ = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A)
        assert lease.fencingToken == 1
        assert leaseStore.renew(_GROUP, _HOLDER_A, lease.fencingToken)
        assert leaseStore.release(_GROUP, _HOLDER_A, lease.fencingToken)

    def test_second_writer_rejected_while_lease_active(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, _ = stores
        leaseStore.acquire(_GROUP, _HOLDER_A)
        with pytest.raises(LeaseError):
            leaseStore.acquire(_GROUP, _HOLDER_B)

    def test_expired_lease_can_be_taken_over_with_incremented_token(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, _ = stores
        leaseA = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=1)
        assert leaseA.fencingToken == 1
        # 模拟 A 过期：直接修改 expires_at
        with openConnection() as connection:
            connection.execute(
                "UPDATE partition_leases SET lease_expires_at = now() - interval '1 second' "
                "WHERE account_group_id = %s",
                (_GROUP,),
            )
        leaseB = leaseStore.acquire(_GROUP, _HOLDER_B, ttlSeconds=10)
        assert leaseB.fencingToken == 2
        # 旧 token 的续租必须失败
        assert not leaseStore.renew(_GROUP, _HOLDER_A, leaseA.fencingToken)

    def test_old_token_write_rejected_by_guard(self, stores) -> None:  # noqa: ANN001
        """双写者验收：A 丢失租约后，旧 token 的所有写入被持久层拒绝。"""
        leaseStore, inboxStore, _ = stores
        leaseA = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=1)
        with openConnection() as connection:
            connection.execute(
                "UPDATE partition_leases SET lease_expires_at = now() - interval '1 second' "
                "WHERE account_group_id = %s",
                (_GROUP,),
            )
        leaseStore.acquire(_GROUP, _HOLDER_B, ttlSeconds=10)
        with pytest.raises(LeaseError):
            inboxStore.accept(
                "key-stale", "a" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, leaseA.fencingToken
            )


class TestInboxIdempotency:
    def test_accept_applies_once_and_duplicate_returns_original(self, stores, seededRun: str) -> None:  # noqa: ANN001
        leaseStore, inboxStore, _ = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=30)
        first = inboxStore.accept("key-1", "b" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        assert first.disposition.value == "APPLIED"
        second = inboxStore.accept("key-1", "b" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        assert second.disposition.value == "DUPLICATE"
        assert second.receiptSequence == first.receiptSequence
        with openConnection() as connection:
            count = connection.execute(
                "SELECT count(*) FROM inbox_records WHERE idempotency_key = 'key-1'"
            ).fetchone()[0]
        assert count == 1  # 重投无重复副作用

    def test_same_key_different_hash_isolated_conflict(self, stores) -> None:  # noqa: ANN001
        leaseStore, inboxStore, _ = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=30)
        inboxStore.accept("key-2", "c" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        with pytest.raises(InboxError):
            inboxStore.accept("key-2", "d" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        with openConnection() as connection:
            conflicts = connection.execute(
                "SELECT existing_content_hash, conflicting_content_hash "
                "FROM inbox_conflicts WHERE idempotency_key = 'key-2'"
            ).fetchall()
        assert conflicts == [("c" * 64, "d" * 64)]


class TestOutboxAtLeastOnce:
    def test_publish_pending_in_sequence_and_idempotent(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, outboxStore = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=30)
        delivered: list[str] = []
        outboxStore.enqueue("msg-1", "orders", "e" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        outboxStore.enqueue("msg-2", "orders", "f" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        published = outboxStore.publishPending(
            lambda message: delivered.append(message.messageId), _RUN, _PARTITION
        )
        assert published == 2
        assert delivered == ["msg-1", "msg-2"]  # 按提交序号升序
        # 再次发布：无 PENDING，无重复副作用
        assert outboxStore.publishPending(lambda message: delivered.append(message.messageId), _RUN, _PARTITION) == 0
        assert delivered == ["msg-1", "msg-2"]

    def test_enqueue_same_message_id_idempotent(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, outboxStore = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=30)
        first = outboxStore.enqueue("msg-dup", "orders", "a" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        second = outboxStore.enqueue("msg-dup", "orders", "a" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)
        assert first.sequence == second.sequence
        with openConnection() as connection:
            count = connection.execute(
                "SELECT count(*) FROM outbox_records WHERE message_id = 'msg-dup'"
            ).fetchone()[0]
        assert count == 1

    def test_failed_publisher_keeps_message_pending(self, stores) -> None:  # noqa: ANN001
        leaseStore, _, outboxStore = stores
        lease = leaseStore.acquire(_GROUP, _HOLDER_A, ttlSeconds=30)
        outboxStore.enqueue("msg-fail", "orders", "b" * 64, _RUN, _PARTITION, _GROUP, _HOLDER_A, lease.fencingToken)

        def failingPublisher(_message) -> None:  # noqa: ANN001
            raise RuntimeError("broker unreachable")

        with pytest.raises(RuntimeError):
            outboxStore.publishPending(failingPublisher, _RUN, _PARTITION)
        assert outboxStore.pendingCount(_RUN, _PARTITION) == 1  # 失败保留，下次重试
