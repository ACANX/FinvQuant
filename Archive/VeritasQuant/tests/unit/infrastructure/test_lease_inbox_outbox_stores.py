"""P2-002 租约/inbox/outbox 存储的本地可验证单元测试。

不依赖真实数据库：验证 SQL 守卫语义（fencing token 条件）、
输入校验与幂等键/冲突 ID 生成规则。
"""

from __future__ import annotations

import pytest

from veritasquant.infrastructure.persistence.InboxStore import InboxError, _conflictId, _validateInput
from veritasquant.infrastructure.persistence import LeaseStore as LeaseStoreModule
from veritasquant.infrastructure.persistence.LeaseStore import LeaseError, LeaseStoreV1
from veritasquant.infrastructure.persistence.OutboxStore import OutboxError, _validateInput as _validateOutboxInput


class TestLeaseStoreSqlSemantics:
    def test_acquire_inserts_fencing_token_starting_at_one(self) -> None:
        assert "fencing_token" in LeaseStoreModule._ACQUIRE_SQL  # noqa: SLF001
        assert "VALUES (%s, %s, 1," in LeaseStoreModule._ACQUIRE_SQL  # noqa: SLF001
        assert "ON CONFLICT (account_group_id) DO NOTHING" in LeaseStoreModule._ACQUIRE_SQL  # noqa: SLF001

    def test_take_over_increments_token_and_requires_expiry_or_same_holder(self) -> None:
        sql = LeaseStoreModule._TAKE_OVER_SQL  # noqa: SLF001
        assert "fencing_token = fencing_token + 1" in sql
        assert "lease_expires_at <= now()" in sql
        assert "lease_holder = %s" in sql
        assert "RETURNING fencing_token" in sql

    def test_renew_guards_holder_token_and_expiry(self) -> None:
        sql = LeaseStoreModule._RENEW_SQL  # noqa: SLF001
        assert "lease_holder = %s" in sql
        assert "fencing_token = %s" in sql
        assert "lease_expires_at > now()" in sql

    def test_guard_requires_exact_token_match(self) -> None:
        sql = LeaseStoreModule._GUARD_SQL  # noqa: SLF001
        assert "lease_holder = %s" in sql
        assert "fencing_token = %s" in sql
        assert "lease_expires_at > now()" in sql

    def test_default_ttl_and_renew_interval(self) -> None:
        assert LeaseStoreV1.defaultTtl() == 10
        assert LeaseStoreV1.renewIntervalSeconds() == 3

    def test_acquire_validates_inputs(self) -> None:
        store = LeaseStoreV1(None)  # type: ignore[arg-type]
        with pytest.raises(LeaseError):
            store.acquire("", "holder")
        with pytest.raises(LeaseError):
            store.acquire("ag", "")
        with pytest.raises(LeaseError):
            store.acquire("ag", "holder", ttlSeconds=0)


class TestInboxInputValidation:
    def test_validate_input_rejects_empty_key(self) -> None:
        with pytest.raises(InboxError):
            _validateInput("", "0" * 64)

    def test_validate_input_rejects_bad_hash(self) -> None:
        with pytest.raises(InboxError):
            _validateInput("key-1", "not-a-hash")
        with pytest.raises(InboxError):
            _validateInput("key-1", "A" * 64)

    def test_validate_input_accepts_lowercase_sha256(self) -> None:
        _validateInput("key-1", "a" * 64)

    def test_conflict_id_deterministic(self) -> None:
        assert _conflictId("key-1") == "conflict:key-1"


class TestOutboxInputValidation:
    def test_validate_input_rejects_empty_message(self) -> None:
        with pytest.raises(OutboxError):
            _validateOutboxInput("", "0" * 64)

    def test_validate_input_rejects_bad_hash(self) -> None:
        with pytest.raises(OutboxError):
            _validateOutboxInput("msg-1", "short")

    def test_enqueue_sql_is_idempotent_on_message_id(self) -> None:
        from veritasquant.infrastructure.persistence.OutboxStore import _ENQUEUE_SQL

        assert "ON CONFLICT (run_id, partition_id, message_id) DO NOTHING" in _ENQUEUE_SQL

    def test_publish_pending_orders_by_sequence(self) -> None:
        from veritasquant.infrastructure.persistence.OutboxStore import _PENDING_SQL

        assert "ORDER BY sequence ASC" in _PENDING_SQL
        assert "status = 'PENDING'" in _PENDING_SQL

    def test_mark_published_guards_status(self) -> None:
        from veritasquant.infrastructure.persistence.OutboxStore import _MARK_PUBLISHED_SQL

        assert "status = 'PUBLISHED'" in _MARK_PUBLISHED_SQL
        assert "status = 'PENDING'" in _MARK_PUBLISHED_SQL
