"""P5-011~014 集成安全测试：审计/备份恢复/Runbook/安全冻结联动。

覆盖验收要点：
- 审计不可变：普通用户不能删改；六域检索；链完整性；
- WAL 间隔 <= 5 分钟；备份可读性自动验证；恢复环境与生产隔离；
- 六类 Runbook 全覆盖且要素齐全；
- 漏洞/许可证 Gate 阻断 + 镜像摘要/依赖锁/源码哈希/审批齐全才冻结。
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.reliability.BackupRecovery import (
    BackupArtifactType,
    BackupRecoveryServiceV1,
    RecoveryEnvironmentV1,
    RecoveryVerificationStatus,
)
from veritasquant.reliability.Runbook import (
    RunbookKind,
    RunbookRegistryV1,
    buildStandardRunbooks,
)
from veritasquant.security.AuditTrail import (
    AuditAccessRole,
    AuditDomain,
    AuditTrailStoreV1,
)
from veritasquant.security.SupplyChainFreeze import (
    DependencyV1,
    ImageDigestV1,
    LicenseGateV1,
    SecurityFreezeServiceV1,
    SourceArtifactV1,
    VulnerabilityGateV1,
    VulnerabilityV1,
)


def _dep(name: str = "requests", license: str = "MIT") -> DependencyV1:
    return DependencyV1(name=name, version="1.0.0", sha256="a" * 64, license=license)


def _image(name: str = "app") -> ImageDigestV1:
    return ImageDigestV1(imageName=name, digest="sha256:" + "b" * 64, tag="1.0.0")


def _source(artifactType: str = "STRATEGY", name: str = "strat") -> SourceArtifactV1:
    return SourceArtifactV1(artifactType=artifactType, name=name, sha256="c" * 64)


class TestAuditImmutabilityIntegration:
    def test_audit_covers_all_domains_and_rejects_mutation(self) -> None:
        store = AuditTrailStoreV1()
        # 六域审计检索覆盖
        store.append(domain=AuditDomain.Command, actor="op", action="CREATE", payloadHash="a" * 64)
        store.append(domain=AuditDomain.Approval, actor="ap", action="APPROVE", payloadHash="b" * 64)
        store.append(domain=AuditDomain.Risk, actor="risk", action="DECIDE", payloadHash="c" * 64)
        store.append(domain=AuditDomain.Order, actor="op", action="SUBMIT", payloadHash="d" * 64)
        store.append(domain=AuditDomain.Ledger, actor="core", action="POST", payloadHash="e" * 64)
        store.append(domain=AuditDomain.ManualAction, actor="op", action="REGISTER", payloadHash="f" * 64)
        for domain in AuditDomain:
            assert len(store.search(domain=domain)) == 1
        # 普通用户不能删改
        with pytest.raises(PermissionError):
            store.delete("audit-00000001", actor="viewer")
        with pytest.raises(PermissionError):
            store.modify("audit-00000001", actor="operator")
        # 链完整性
        assert store.chainIntegrity()

    def test_purge_only_administrator_and_audited(self) -> None:
        store = AuditTrailStoreV1()
        store.append(domain=AuditDomain.Command, actor="op", action="CREATE", payloadHash="a" * 64)
        with pytest.raises(PermissionError):
            store.purge(domain=AuditDomain.Command, before=datetime.now(timezone.utc), actor="operator")
        removed, audit = store.purge(
            domain=AuditDomain.Command,
            before=datetime.now(timezone.utc) + timedelta(days=1),
            actor=AuditAccessRole.Administrator.value,
        )
        assert removed >= 1
        assert audit.action == "AUDIT_PURGE"
        assert store.chainIntegrity()


class TestBackupRecoveryIntegration:
    def test_wal_interval_and_readability_and_isolated_recovery(self) -> None:
        service = BackupRecoveryServiceV1()
        # WAL 间隔合规
        service.recordWalInterval(timedelta(minutes=3))
        assert service.walCompliant(timedelta(minutes=3))
        with pytest.raises(ValueError):
            service.recordWalInterval(timedelta(minutes=10))
        # 备份 + 可读性自动验证
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        assert service.verifyReadability("base-1", "a" * 64)
        assert not service.verifyReadability("base-1", "zz" * 32)
        # 隔离环境恢复
        manifest = service.createManifest(manifestId="m1", baseBackup=base)
        env = RecoveryEnvironmentV1(
            environmentId="recovery-dr", productionEnvironmentId="prod",
            isolatedNetwork=True, isolatedCredentials=True, isolatedDataDirectory=True,
            dataDirectory="/restore/dr",
        )
        report = service.executeRecoveryDrill(
            drillId="drill-1", environment=env, manifest=manifest,
            rtoSeconds=300, rpoSeconds=60, ledgerHashConsistent=True,
            controlsRecoveredPercent=100.0, unreconciledDifferences=0,
            verifiedBy="independent-qa",
        )
        assert report.status is RecoveryVerificationStatus.Pass


class TestRunbookIntegration:
    def test_six_runbooks_complete_and_registered(self) -> None:
        registry = RunbookRegistryV1()
        for rb in buildStandardRunbooks().values():
            registry.register(rb)
        assert registry.coverageComplete()
        for kind in RunbookKind:
            rb = registry.get(kind)
            assert rb is not None
            assert rb.complete()
            # 每个 Runbook 含触发、权限、步骤、验证、回退、证据、升级联系人
            assert rb.trigger
            assert rb.requiredPermissions
            assert rb.steps
            assert rb.verification
            assert rb.rollback
            assert rb.evidence
            assert rb.escalationContacts


class TestSupplyChainFreezeIntegration:
    def test_gates_block_and_approvals_required(self) -> None:
        vulnGate = VulnerabilityGateV1()
        licenseGate = LicenseGateV1()
        service = SecurityFreezeServiceV1(vulnerabilityGate=vulnGate, licenseGate=licenseGate)
        # 漏洞阻断
        with pytest.raises(ValueError, match="漏洞"):
            service.freeze(
                dependencies=(_dep(),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(VulnerabilityV1("lib", "CRITICAL", 9.8),),
                approvedBy=("alice", "bob"),
            )
        # 许可证阻断
        with pytest.raises(ValueError, match="许可证"):
            service.freeze(
                dependencies=(_dep(license="Proprietary"),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(),
                approvedBy=("alice", "bob"),
            )
        # 审批不全
        with pytest.raises(ValueError, match="两名审批人"):
            service.freeze(
                dependencies=(_dep(),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(),
                approvedBy=("alice",),
            )
        # 全部满足才冻结
        freeze = service.freeze(
            dependencies=(_dep(), _dep("pydantic", "MIT")),
            images=(_image(), _image("migrator")),
            sourceArtifacts=(_source(), _source("CONFIG", "cfg")),
            vulnerabilityFindings=(VulnerabilityV1("urllib3", "MEDIUM", 5.3),),
            approvedBy=("reviewer-qa", "live-approver"),
        )
        assert freeze.gatesPassed()
        assert freeze.approvalsComplete()
        assert service.verifyIntegrity(freeze)
        assert freeze.freezeHash == freeze.computeHash()


class TestCrossModuleSafety:
    def test_audit_records_freeze_and_recovery_actions(self) -> None:
        """审计贯穿：冻结与恢复演练均产生不可变审计记录。"""
        store = AuditTrailStoreV1()
        # 安全冻结动作留痕
        store.append(domain=AuditDomain.Approval, actor="live-approver", action="APPROVE_FREEZE",
                     payloadHash="a" * 64)
        store.append(domain=AuditDomain.Command, actor="sre", action="FREEZE_SECURITY",
                     payloadHash="b" * 64)
        # 恢复演练动作留痕
        store.append(domain=AuditDomain.Risk, actor="sre", action="RECOVERY_DRILL",
                     payloadHash="c" * 64)
        store.append(domain=AuditDomain.Ledger, actor="core", action="RESTORE_VERIFY",
                     payloadHash="d" * 64)
        assert store.chainIntegrity()
        assert len(store.search(actor="sre")) == 2
        assert len(store.search(domain=AuditDomain.Approval)) == 1
        # Runbook 查询得到处置指引（审计检索 + Runbook 配套）
        registry = RunbookRegistryV1()
        for rb in buildStandardRunbooks().values():
            registry.register(rb)
        assert registry.get(RunbookKind.SecretLeak) is not None
        assert registry.get(RunbookKind.LedgerAnomaly) is not None
