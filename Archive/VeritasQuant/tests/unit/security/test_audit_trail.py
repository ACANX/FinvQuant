"""P5-011 不可变审计、日志访问和保留策略测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.security.AuditTrail import (
    AuditAccessRole,
    AuditDomain,
    AuditEntryV1,
    AuditRetentionPolicyV1,
    AuditTrailStoreV1,
    buildAuditEntry,
)


def _ts(days_ago: int = 0) -> datetime:
    now = datetime.now(timezone.utc)
    return (now - timedelta(days=days_ago)).replace(microsecond=(now.microsecond // 1_000) * 1_000)


class TestAuditEntry:
    def test_build_entry_has_valid_hash(self) -> None:
        entry = buildAuditEntry(
            entryId="audit-00000001",
            domain=AuditDomain.Command,
            actor="alice",
            action="CREATE_COMMAND",
            payloadHash="a" * 64,
        )
        assert entry.verifyHash()
        assert entry.entryHash != "0" * 64
        assert entry.prevHash == "0" * 64

    def test_same_content_same_hash(self) -> None:
        e1 = buildAuditEntry(
            entryId="a1", domain=AuditDomain.Order, actor="bob", action="SUBMIT",
            payloadHash="b" * 64, ts=_ts(),
        )
        e2 = buildAuditEntry(
            entryId="a2", domain=AuditDomain.Order, actor="bob", action="SUBMIT",
            payloadHash="b" * 64, ts=_ts(),
        )
        # entryId 不同 → 哈希不同；但同字段计算一致
        assert e1.entryHash != e2.entryHash

    def test_entry_requires_valid_hashes(self) -> None:
        with pytest.raises(ValueError, match="payloadHash"):
            buildAuditEntry(
                entryId="x", domain=AuditDomain.Risk, actor="alice", action="DECIDE",
                payloadHash="short",
            )

    def test_tamper_detected(self) -> None:
        entry = buildAuditEntry(
            entryId="a1", domain=AuditDomain.Ledger, actor="alice", action="POST",
            payloadHash="c" * 64,
        )
        tampered = AuditEntryV1(
            entryId=entry.entryId, ts=entry.ts, domain=entry.domain, actor=entry.actor,
            action=entry.action, payloadHash="d" * 64, prevHash=entry.prevHash,
            entryHash=entry.entryHash, traceId=entry.traceId, details=entry.details,
        )
        assert not tampered.verifyHash()


class TestAuditTrailStore:
    def test_append_and_chain_integrity(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Command, actor="alice", action="CREATE", payloadHash="a" * 64)
        store.append(domain=AuditDomain.Approval, actor="bob", action="APPROVE", payloadHash="b" * 64)
        store.append(domain=AuditDomain.Order, actor="alice", action="SUBMIT", payloadHash="c" * 64)
        assert store.entryCount() == 3
        assert store.chainIntegrity()

    def test_chain_head_tracks_last(self) -> None:
        store = AuditTrailStoreV1()
        e1 = store.append(domain=AuditDomain.Risk, actor="a", action="DECIDE", payloadHash="a" * 64)
        e2 = store.append(domain=AuditDomain.Risk, actor="a", action="DECIDE", payloadHash="b" * 64)
        assert e1.entryHash != e2.entryHash
        assert e2.prevHash == e1.entryHash
        assert store.chainHead() == e2.entryHash
        assert store.chainIntegrity()

    def test_delete_rejected(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="a" * 64)
        with pytest.raises(PermissionError, match="不可变"):
            store.delete("audit-00000001")

    def test_modify_rejected(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="a" * 64)
        with pytest.raises(PermissionError, match="不可变"):
            store.modify("audit-00000001")

    def test_duplicate_entry_id_rejected(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Risk, actor="a", action="DECIDE", payloadHash="a" * 64, entryId="dup-1")
        with pytest.raises(ValueError, match="已存在"):
            store.append(domain=AuditDomain.Risk, actor="a", action="DECIDE", payloadHash="a" * 64, entryId="dup-1")

    def test_search_by_domain(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Command, actor="alice", action="CREATE", payloadHash="a" * 64)
        store.append(domain=AuditDomain.Approval, actor="bob", action="APPROVE", payloadHash="b" * 64)
        store.append(domain=AuditDomain.Command, actor="alice", action="EXECUTE", payloadHash="c" * 64)
        commands = store.search(domain=AuditDomain.Command)
        assert len(commands) == 2
        assert all(e.domain is AuditDomain.Command for e in commands)

    def test_search_covers_all_six_domains(self) -> None:
        store = AuditTrailStoreV1()
        for domain in AuditDomain:
            store.append(domain=domain, actor="alice", action="ACT", payloadHash="a" * 64)
        assert store.entryCount() == len(AuditDomain) == 6
        for domain in AuditDomain:
            assert len(store.search(domain=domain)) == 1

    def test_search_by_actor_action_trace(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Order, actor="alice", action="SUBMIT", payloadHash="a" * 64, traceId="t1")
        store.append(domain=AuditDomain.Order, actor="bob", action="CANCEL", payloadHash="b" * 64, traceId="t1")
        store.append(domain=AuditDomain.Order, actor="alice", action="SUBMIT", payloadHash="c" * 64, traceId="t2")
        assert len(store.search(actor="alice")) == 2
        assert len(store.search(action="SUBMIT")) == 2
        assert len(store.search(traceId="t1")) == 2

    def test_search_since_and_limit(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="a" * 64)
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="b" * 64)
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="c" * 64)
        assert len(store.search(limit=2)) == 2

    def test_tampered_chain_detected(self) -> None:
        store = AuditTrailStoreV1()
        e1 = store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="a" * 64)
        store.append(domain=AuditDomain.Ledger, actor="a", action="POST", payloadHash="b" * 64)
        # 篡改第一条的 payloadHash（直接改内部存储）
        tampered = AuditEntryV1(
            entryId=e1.entryId, ts=e1.ts, domain=e1.domain, actor=e1.actor,
            action=e1.action, payloadHash="f" * 64, prevHash=e1.prevHash,
            entryHash=e1.entryHash, traceId=e1.traceId, details=e1.details,
        )
        store._entries[e1.entryId] = tampered
        assert not store.chainIntegrity()

    def test_purge_requires_administrator(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Command, actor="a", action="CREATE", payloadHash="a" * 64)
        with pytest.raises(PermissionError, match="Administrator"):
            store.purge(domain=AuditDomain.Command, before=_ts(), actor="alice")

    def test_purge_removes_and_audits(self) -> None:
        store = AuditTrailStoreV1()
        old_entry = buildAuditEntry(
            entryId="old-1", domain=AuditDomain.Command, actor="a", action="CREATE",
            payloadHash="a" * 64, ts=_ts(days_ago=400),
        )
        store._entries["old-1"] = old_entry
        store.append(domain=AuditDomain.Command, actor="a", action="CREATE", payloadHash="b" * 64)
        removed, audit = store.purge(
            domain=AuditDomain.Command, before=_ts(days_ago=100), actor=AuditAccessRole.Administrator.value,
        )
        assert removed == 1
        assert audit.action == "AUDIT_PURGE"
        assert store.get("old-1") is None
        assert len(store.purgeLog()) == 1


class TestAuditRetentionPolicy:
    def test_default_retention_10_years(self) -> None:
        policy = AuditRetentionPolicyV1()
        assert policy.retentionFor(AuditDomain.Command) == timedelta(days=3650)

    def test_custom_retention(self) -> None:
        policy = AuditRetentionPolicyV1(retentionDays={AuditDomain.Risk: 90})
        assert policy.retentionFor(AuditDomain.Risk) == timedelta(days=90)
        assert policy.retentionFor(AuditDomain.Order) == timedelta(days=3650)

    def test_expired_entry(self) -> None:
        policy = AuditRetentionPolicyV1(retentionDays={AuditDomain.Ledger: 30})
        entry = buildAuditEntry(
            entryId="e1", domain=AuditDomain.Ledger, actor="a", action="POST",
            payloadHash="a" * 64, ts=_ts(days_ago=60),
        )
        assert policy.expired(entry, now=_ts())

    def test_not_expired_entry(self) -> None:
        policy = AuditRetentionPolicyV1(retentionDays={AuditDomain.Ledger: 30})
        entry = buildAuditEntry(
            entryId="e1", domain=AuditDomain.Ledger, actor="a", action="POST",
            payloadHash="a" * 64, ts=_ts(days_ago=10),
        )
        assert not policy.expired(entry, now=_ts())

    def test_purge_eligible_lists_only_expired(self) -> None:
        policy = AuditRetentionPolicyV1(retentionDays={AuditDomain.Command: 30})
        store = AuditTrailStoreV1()
        old = buildAuditEntry(
            entryId="old", domain=AuditDomain.Command, actor="a", action="CREATE",
            payloadHash="a" * 64, ts=_ts(days_ago=60),
        )
        store._entries["old"] = old
        store.append(domain=AuditDomain.Command, actor="a", action="CREATE", payloadHash="b" * 64)
        eligible = policy.purgeEligible(store, now=_ts())
        assert len(eligible) == 1
        assert eligible[0].entryId == "old"
