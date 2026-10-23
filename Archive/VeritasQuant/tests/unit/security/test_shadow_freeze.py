"""P5-017 影子运行账户、策略、额度和验收政策冻结测试。"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal

import pytest

from veritasquant.security.ShadowFreeze import (
    ShadowFreezeEntryV1,
    ShadowFreezeKind,
    ShadowFreezeRecordV1,
    ShadowFreezeServiceV1,
    buildShadowFreezeEntries,
)


def _utc() -> datetime:
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


def _entry(kind: ShadowFreezeKind = ShadowFreezeKind.Limit, objectId: str = "cap-1", **kw) -> ShadowFreezeEntryV1:
    defaults = dict(
        kind=kind,
        objectId=objectId,
        version="V1",
        capValue=Decimal("100000"),
        capUnit="CNY",
        signedBy=("alice", "bob"),
    )
    defaults.update(kw)
    return ShadowFreezeEntryV1(**defaults)


def _full_entries() -> tuple[ShadowFreezeEntryV1, ...]:
    return buildShadowFreezeEntries(
        accountId="shadow-001",
        strategyVersion="V3",
        strategyChecksum="s" * 64,
        initialFundCap=Decimal("500000"),
        orderCap=Decimal("100"),
        acceptancePolicyVersion="V5",
        signerA="alice",
        signerB="bob",
    )


class TestShadowFreezeEntry:
    def test_entry_valid(self) -> None:
        entry = _entry()
        assert entry.entryHash() == entry.entryHash()
        assert len(entry.entryHash()) == 64

    def test_entry_requires_two_signers(self) -> None:
        with pytest.raises(ValueError, match="两名签署人"):
            _entry(signedBy=("alice",))

    def test_entry_rejects_same_signer_twice(self) -> None:
        with pytest.raises(ValueError, match="互不相同"):
            _entry(signedBy=("alice", "alice"))

    def test_entry_rejects_negative_cap(self) -> None:
        with pytest.raises(ValueError, match="不得为负"):
            _entry(capValue=Decimal("-1"))

    def test_entry_requires_fields(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            _entry(objectId="", capUnit="CNY")


class TestShadowFreezeService:
    def test_freeze_success(self) -> None:
        service = ShadowFreezeServiceV1()
        record = service.freeze(entries=_full_entries(), frozenBy="operator")
        assert record.verify()
        assert service.current() is not None
        assert service.verifyIntegrity(record)

    def test_freeze_requires_all_kinds(self) -> None:
        service = ShadowFreezeServiceV1()
        entries = (_entry(ShadowFreezeKind.Account, "acc-1"),)  # 只有账户
        with pytest.raises(ValueError, match="缺少关键对象类型"):
            service.freeze(entries=entries, frozenBy="operator")

    def test_freeze_requires_dual_signed_each(self) -> None:
        service = ShadowFreezeServiceV1()
        with pytest.raises(ValueError, match="双人签署|两名签署人"):
            entries = (
                _entry(ShadowFreezeKind.Account, "acc-1", signedBy=("alice", "bob")),
                _entry(ShadowFreezeKind.Strategy, "s1", signedBy=("alice",)),  # 单签
                _entry(ShadowFreezeKind.Limit, "l1", signedBy=("alice", "bob")),
                _entry(ShadowFreezeKind.AcceptancePolicy, "p1", signedBy=("alice", "bob")),
            )
            service.freeze(entries=entries, frozenBy="operator")

    def test_freeze_empty_rejected(self) -> None:
        service = ShadowFreezeServiceV1()
        with pytest.raises(ValueError, match="不能为空"):
            service.freeze(entries=(), frozenBy="operator")

    def test_freeze_duplicate_rejected(self) -> None:
        service = ShadowFreezeServiceV1()
        service.freeze(entries=_full_entries(), frozenBy="operator", recordId="f1")
        with pytest.raises(ValueError, match="已存在"):
            service.freeze(entries=_full_entries(), frozenBy="operator", recordId="f1")

    def test_cap_for_returns_approved_value(self) -> None:
        service = ShadowFreezeServiceV1()
        service.freeze(entries=_full_entries(), frozenBy="operator")
        assert service.capFor(ShadowFreezeKind.Limit, "shadow-001.order-cap") == Decimal("100")
        assert service.capFor(ShadowFreezeKind.Account, "shadow-001") == Decimal("500000")
        assert service.capFor(ShadowFreezeKind.Limit, "unknown") is None

    def test_supersede_keeps_history(self) -> None:
        service = ShadowFreezeServiceV1()
        service.freeze(entries=_full_entries(), frozenBy="operator", recordId="f1")
        superseded = service.supersede("f1")
        # 新冻结取代旧的
        r2 = service.freeze(entries=_full_entries(), frozenBy="operator", recordId="f2")
        assert superseded.status.value == "SUPERSEDED"
        assert service.get("f1").status.value == "SUPERSEDED"
        assert r2.status.value == "FROZEN"
        assert service.current().recordId == "f2"
        assert len(service.all()) == 2

    def test_threshold_modification_detected(self) -> None:
        service = ShadowFreezeServiceV1()
        record = service.freeze(entries=_full_entries(), frozenBy="operator")
        # 篡改 capValue 后校验失败（阈值解释结果不可修改）
        tampered = ShadowFreezeRecordV1(
            recordId=record.recordId,
            entries=(_entry(ShadowFreezeKind.Limit, "shadow-001.order-cap", capValue=Decimal("99999")),),
            frozenAt=record.frozenAt,
            frozenBy=record.frozenBy,
            recordHash=record.recordHash,
        )
        assert not tampered.verify()
        assert not service.verifyIntegrity(tampered)
        assert service.thresholdModified(tampered)

    def test_verify_integrity_unknown(self) -> None:
        service = ShadowFreezeServiceV1()
        fake = ShadowFreezeRecordV1(
            recordId="nope", entries=(), frozenAt=_utc(), frozenBy="x", recordHash="0" * 64,
        )
        assert not service.verifyIntegrity(fake)


class TestBuildShadowFreezeEntries:
    def test_build_four_kinds(self) -> None:
        entries = _full_entries()
        assert {e.kind for e in entries} == set(ShadowFreezeKind)
        assert len(entries) == 4

    def test_build_rejects_same_signer(self) -> None:
        with pytest.raises(ValueError, match="两名不同人员"):
            buildShadowFreezeEntries(
                accountId="a", strategyVersion="V1", strategyChecksum="s" * 64,
                initialFundCap=Decimal("1"), orderCap=Decimal("1"),
                acceptancePolicyVersion="V1", signerA="alice", signerB="alice",
            )

    def test_build_all_dual_signed(self) -> None:
        for entry in _full_entries():
            assert len(entry.signedBy) == 2
            assert entry.signedBy[0] != entry.signedBy[1]
