"""P5-012 WAL/对象版本化备份与恢复自动验证测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.reliability.BackupRecovery import (
    LIVE_RPO_TARGET,
    LIVE_RTO_TARGET,
    BackupArtifactType,
    BackupArtifactV1,
    BackupManifestV1,
    BackupReadabilityVerifierV1,
    BackupRecoveryServiceV1,
    RecoveryEnvironmentV1,
    RecoveryVerificationStatus,
    WalArchivePolicyV1,
)


def _ts(days_ago: int = 0) -> datetime:
    now = datetime.now(timezone.utc)
    return (now - timedelta(days=days_ago)).replace(microsecond=(now.microsecond // 1_000) * 1_000)


def _artifact(artifactId: str = "base-1", sha256: str | None = None, **kw) -> BackupArtifactV1:
    return BackupArtifactV1(
        artifactId=artifactId,
        artifactType=kw.get("artifactType", BackupArtifactType.BaseBackup),
        objectVersionId=kw.get("objectVersionId", f"v-{artifactId}"),
        sha256=sha256 or "a" * 64,
        sizeBytes=kw.get("sizeBytes", 1024),
        createdAt=kw.get("createdAt", _ts()),
        retentionDays=kw.get("retentionDays", 3650),
    )


def _env(environmentId: str = "recovery-dr", production: str = "prod") -> RecoveryEnvironmentV1:
    return RecoveryEnvironmentV1(
        environmentId=environmentId,
        productionEnvironmentId=production,
        isolatedNetwork=True,
        isolatedCredentials=True,
        isolatedDataDirectory=True,
        dataDirectory=f"/restore/{environmentId}",
    )


class TestWalArchivePolicy:
    def test_live_max_interval_is_5_minutes(self) -> None:
        policy = WalArchivePolicyV1()
        assert policy.maxInterval() == timedelta(minutes=5)

    def test_compliant_interval(self) -> None:
        policy = WalArchivePolicyV1()
        assert policy.compliant(timedelta(minutes=4, seconds=30))
        assert policy.compliant(timedelta(minutes=5))

    def test_over_interval_rejected(self) -> None:
        policy = WalArchivePolicyV1()
        assert not policy.compliant(timedelta(minutes=6))
        with pytest.raises(ValueError, match="5 分钟"):
            policy.validateInterval(timedelta(minutes=6))

    def test_custom_max_interval(self) -> None:
        policy = WalArchivePolicyV1(maxInterval=timedelta(minutes=1))
        assert policy.compliant(timedelta(seconds=59))
        assert not policy.compliant(timedelta(minutes=2))


class TestBackupArtifact:
    def test_artifact_validates_hash(self) -> None:
        with pytest.raises(ValueError, match="sha256"):
            BackupArtifactV1(
                artifactId="a1", artifactType=BackupArtifactType.WalSegment,
                objectVersionId="v1", sha256="bad", sizeBytes=10, createdAt=_ts(), retentionDays=30,
            )

    def test_artifact_digest_check(self) -> None:
        artifact = _artifact(sha256="b" * 64)
        assert artifact.digestValid("b" * 64)
        assert not artifact.digestValid("c" * 64)

    def test_artifact_requires_identity(self) -> None:
        with pytest.raises(ValueError):
            _artifact(artifactId="", objectVersionId="")


class TestBackupReadabilityVerifier:
    def test_verify_matches_digest(self) -> None:
        verifier = BackupReadabilityVerifierV1()
        artifact = _artifact(sha256="d" * 64)
        assert verifier.verify(artifact, "d" * 64)
        assert verifier.lastCheck()[2] is True

    def test_verify_mismatch_fails(self) -> None:
        verifier = BackupReadabilityVerifierV1()
        artifact = _artifact(sha256="d" * 64)
        assert not verifier.verify(artifact, "e" * 64)
        assert verifier.lastCheck()[2] is False

    def test_verify_rejects_empty_read(self) -> None:
        verifier = BackupReadabilityVerifierV1()
        artifact = _artifact(sha256="d" * 64)
        assert not verifier.verify(artifact, "")

    def test_verify_manifest_all_artifacts(self) -> None:
        verifier = BackupReadabilityVerifierV1()
        manifest = BackupManifestV1(
            manifestId="m1",
            baseBackup=_artifact(sha256="a" * 64),
            walSegments=(_artifact("w1", "b" * 64, artifactType=BackupArtifactType.WalSegment),),
            objectArtifacts=(_artifact("o1", "c" * 64, artifactType=BackupArtifactType.ObjectManifest),),
        )
        ok, failures = verifier.verifyManifest(manifest)
        assert ok
        assert failures == []

    def test_monthly_due(self) -> None:
        verifier = BackupReadabilityVerifierV1()
        assert verifier.monthlyDue(None)
        assert not verifier.monthlyDue(_ts(), now=_ts())
        assert verifier.monthlyDue(_ts(days_ago=31), now=_ts())


class TestRecoveryEnvironment:
    def test_environment_must_be_isolated(self) -> None:
        with pytest.raises(ValueError, match="隔离"):
            RecoveryEnvironmentV1(
                environmentId="prod", productionEnvironmentId="prod",
                isolatedNetwork=True, isolatedCredentials=True, isolatedDataDirectory=True,
                dataDirectory="/restore",
            )

    def test_environment_requires_isolation_flags(self) -> None:
        with pytest.raises(ValueError, match="隔离"):
            RecoveryEnvironmentV1(
                environmentId="dr", productionEnvironmentId="prod",
                isolatedNetwork=True, isolatedCredentials=False, isolatedDataDirectory=True,
                dataDirectory="/restore",
            )

    def test_isolated_from_production(self) -> None:
        env = _env()
        assert env.isolatedFrom("prod")
        assert not env.isolatedFrom("recovery-dr")
        # 环境标识与生产不同即隔离；相同标识不隔离
        assert env.isolatedFrom(env.productionEnvironmentId) == (env.environmentId != env.productionEnvironmentId)


class TestRecoveryVerification:
    def test_unique_conclusion_pass(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Pass

    def test_unique_conclusion_rto_exceeded(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, rtoSeconds=LIVE_RTO_TARGET.total_seconds() + 60)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Fail

    def test_unique_conclusion_rpo_exceeded(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, rpoSeconds=LIVE_RPO_TARGET.total_seconds() + 60)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Fail

    def test_unique_conclusion_ledger_hash_mismatch(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, ledgerHashConsistent=False)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Fail

    def test_unique_conclusion_controls_not_full(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, controlsRecoveredPercent=99.5)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Fail

    def test_unique_conclusion_differences(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, unreconciledDifferences=1)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.Fail

    def test_unique_conclusion_missing_verification(self) -> None:
        env = _env()
        manifest = BackupManifestV1(manifestId="m1", baseBackup=_artifact())
        report = _make_report(env, manifest, verifiedBy=None)
        assert report.uniqueConclusion() is RecoveryVerificationStatus.InsufficientEvidence


def _make_report(
    env: RecoveryEnvironmentV1,
    manifest: BackupManifestV1,
    *,
    rtoSeconds: float = 120,
    rpoSeconds: float = 30,
    ledgerHashConsistent: bool = True,
    controlsRecoveredPercent: float = 100.0,
    unreconciledDifferences: int = 0,
    verifiedBy: str | None = "sre-qa",
) -> "object":
    from veritasquant.reliability.BackupRecovery import RecoveryVerificationV1

    return RecoveryVerificationV1(
        drillId="drill-1",
        environment=env,
        manifest=manifest,
        startedAt=_ts(),
        completedAt=_ts(),
        rtoSeconds=rtoSeconds,
        rpoSeconds=rpoSeconds,
        ledgerHashConsistent=ledgerHashConsistent,
        controlsRecoveredPercent=controlsRecoveredPercent,
        unreconciledDifferences=unreconciledDifferences,
        verifiedBy=verifiedBy,
    )


class TestBackupRecoveryService:
    def test_record_wal_interval(self) -> None:
        service = BackupRecoveryServiceV1()
        service.recordWalInterval(timedelta(minutes=3))
        assert service.walCompliant(timedelta(minutes=3))

    def test_record_over_interval_rejected(self) -> None:
        service = BackupRecoveryServiceV1()
        with pytest.raises(ValueError, match="5 分钟"):
            service.recordWalInterval(timedelta(minutes=10))

    def test_store_and_verify_artifact(self) -> None:
        service = BackupRecoveryServiceV1()
        service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        assert service.verifyReadability("base-1", "a" * 64)
        stored = service.artifacts()[0]
        assert stored.status.value == "VERIFIED"

    def test_verify_mismatch_fails(self) -> None:
        service = BackupRecoveryServiceV1()
        service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        assert not service.verifyReadability("base-1", "f" * 64)

    def test_verify_unknown_artifact(self) -> None:
        service = BackupRecoveryServiceV1()
        with pytest.raises(ValueError, match="不存在"):
            service.verifyReadability("nope", "a" * 64)

    def test_create_manifest_with_hash(self) -> None:
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        wal = service.storeArtifact(
            artifactId="wal-1", artifactType=BackupArtifactType.WalSegment,
            objectVersionId="v2", sha256="b" * 64, sizeBytes=512, retentionDays=3650,
        )
        manifest = service.createManifest(manifestId="m1", baseBackup=base, walSegments=(wal,))
        assert manifest.manifestHash == manifest.computeHash()
        assert manifest.manifestHash != ""

    def test_execute_recovery_drill_pass(self) -> None:
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        manifest = service.createManifest(manifestId="m1", baseBackup=base)
        report = service.executeRecoveryDrill(
            drillId="drill-1",
            environment=_env(),
            manifest=manifest,
            rtoSeconds=120,
            rpoSeconds=30,
            ledgerHashConsistent=True,
            controlsRecoveredPercent=100.0,
            unreconciledDifferences=0,
            verifiedBy="sre-qa",
        )
        assert report.status is RecoveryVerificationStatus.Pass
        assert report.reportHash != ""
        assert service.passedDrillCount() == 1

    def test_execute_recovery_drill_rejects_production_env(self) -> None:
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        manifest = service.createManifest(manifestId="m1", baseBackup=base)
        # 环境标识与生产相同：构造即拒绝（不允许把生产环境当作恢复环境）
        with pytest.raises(ValueError, match="隔离"):
            RecoveryEnvironmentV1(
                environmentId="prod", productionEnvironmentId="prod",
                isolatedNetwork=True, isolatedCredentials=True, isolatedDataDirectory=True,
                dataDirectory="/restore",
            )
        # 环境标识与生产不同但隔离标志不全：也拒绝
        with pytest.raises(ValueError, match="隔离"):
            RecoveryEnvironmentV1(
                environmentId="dr", productionEnvironmentId="prod",
                isolatedNetwork=True, isolatedCredentials=False, isolatedDataDirectory=True,
                dataDirectory="/restore",
            )
        assert manifest.manifestHash != ""

    def test_execute_recovery_drill_fail_status(self) -> None:
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        manifest = service.createManifest(manifestId="m1", baseBackup=base)
        report = service.executeRecoveryDrill(
            drillId="drill-1",
            environment=_env(),
            manifest=manifest,
            rtoSeconds=7200,  # 2h > 1h
            rpoSeconds=30,
            ledgerHashConsistent=True,
            controlsRecoveredPercent=100.0,
            unreconciledDifferences=0,
            verifiedBy="sre-qa",
        )
        assert report.status is RecoveryVerificationStatus.Fail
        assert service.passedDrillCount() == 0

    def test_execute_recovery_drill_insufficient_evidence(self) -> None:
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=2048, retentionDays=3650,
        )
        manifest = service.createManifest(manifestId="m1", baseBackup=base)
        report = service.executeRecoveryDrill(
            drillId="drill-1",
            environment=_env(),
            manifest=manifest,
            rtoSeconds=120,
            rpoSeconds=30,
            ledgerHashConsistent=True,
            controlsRecoveredPercent=100.0,
            unreconciledDifferences=0,
            verifiedBy=None,
        )
        assert report.status is RecoveryVerificationStatus.InsufficientEvidence

    def test_full_quarterly_drill_flow(self) -> None:
        """季度演练流程：备份 → 可读性验证 → 隔离恢复 → PASS。"""
        service = BackupRecoveryServiceV1()
        base = service.storeArtifact(
            artifactId="base-q1", artifactType=BackupArtifactType.BaseBackup,
            objectVersionId="v1", sha256="a" * 64, sizeBytes=8192, retentionDays=3650,
        )
        wals = []
        for i in range(3):
            wals.append(
                service.storeArtifact(
                    artifactId=f"wal-q1-{i}", artifactType=BackupArtifactType.WalSegment,
                    objectVersionId=f"wv{i}", sha256=chr(ord("b") + i) * 64, sizeBytes=256, retentionDays=3650,
                )
            )
        manifest = service.createManifest(manifestId="m-q1", baseBackup=base, walSegments=tuple(wals))
        # 可读性自动验证
        assert service.verifyReadability("base-q1", "a" * 64)
        for i in range(3):
            assert service.verifyReadability(f"wal-q1-{i}", chr(ord("b") + i) * 64)
        # 隔离环境恢复
        report = service.executeRecoveryDrill(
            drillId="drill-q1",
            environment=_env(),
            manifest=manifest,
            rtoSeconds=300,
            rpoSeconds=60,
            ledgerHashConsistent=True,
            controlsRecoveredPercent=100.0,
            unreconciledDifferences=0,
            verifiedBy="independent-qa",
        )
        assert report.status is RecoveryVerificationStatus.Pass
        assert service.passedDrillCount() == 1
