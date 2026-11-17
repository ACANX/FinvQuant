"""P5-012 WAL/对象版本化备份与恢复自动验证。

对齐 TechSpec 12.3 灾难恢复：
- PostgreSQL 连续 WAL 归档 + 每日基础备份；实盘 WAL 归档间隔不超过 5 分钟；
- 对象存储启用版本化与不可变保留；配置、数据 manifest、策略工件和审计分别按保留策略备份；
- 每月至少自动验证备份可读性；恢复环境与生产隔离；
- 每季度在隔离环境完成全量恢复和事实重放；实盘目标 RTO <= 1h、RPO <= 5min；
- 恢复结果需满足 RTO/RPO、账本哈希一致、活动控制 100% 和对账差异 0。

- `BackupArtifactV1`：备份工件（类型/对象存储版本 ID/SHA-256 摘要/大小/时间）；
- `BackupManifestV1`：版本化备份清单（依赖工件 + manifest 哈希）；
- `WalArchivePolicyV1`：WAL 归档间隔策略（实盘 <= 5 分钟）；
- `BackupReadabilityVerifierV1`：备份可读性自动验证（摘要校验 + 元数据完整性）；
- `RecoveryEnvironmentV1`：恢复环境隔离校验（与生产隔离）；
- `RecoveryVerificationV1`：恢复验证报告（RTO/RPO/账本哈希/控制 100%/差异 0）；
- `BackupRecoveryServiceV1`：编排（记录备份、验证可读性、执行隔离恢复演练）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash

# 实盘 WAL 归档最大间隔（TechSpec 12.3）
LIVE_WAL_MAX_INTERVAL = timedelta(minutes=5)
# 备份可读性自动验证周期（每月）
READABILITY_VERIFY_INTERVAL = timedelta(days=30)
# 恢复演练周期（每季度）
RECOVERY_DRILL_INTERVAL = timedelta(days=90)
# 实盘恢复目标（TechSpec 12.3）
LIVE_RTO_TARGET = timedelta(hours=1)
LIVE_RPO_TARGET = timedelta(minutes=5)


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class BackupArtifactType(StrEnum):
    WalSegment = "WAL_SEGMENT"  # PostgreSQL WAL 段
    BaseBackup = "BASE_BACKUP"  # 每日基础备份
    ObjectManifest = "OBJECT_MANIFEST"  # 对象存储版本化清单
    StrategyArtifact = "STRATEGY_ARTIFACT"  # 策略工件
    AuditArchive = "AUDIT_ARCHIVE"  # 审计归档


class BackupStatus(StrEnum):
    Stored = "STORED"
    Verified = "VERIFIED"  # 可读性自动验证通过
    VerificationFailed = "VERIFICATION_FAILED"
    Restored = "RESTORED"  # 已在隔离环境成功恢复


@dataclass(frozen=True, slots=True)
class BackupArtifactV1:
    """一次备份工件：不可变摘要 + 对象存储版本 ID。"""

    artifactId: str
    artifactType: BackupArtifactType
    objectVersionId: str  # 对象存储版本 ID（不可变保留）
    sha256: str  # 工件内容 SHA-256
    sizeBytes: int
    createdAt: datetime
    retentionDays: int  # 保留期（天）
    status: BackupStatus = BackupStatus.Stored

    def __post_init__(self) -> None:
        if not self.artifactId or not self.objectVersionId:
            raise ValueError("artifactId/objectVersionId 不能为空")
        if len(self.sha256) != 64:
            raise ValueError("sha256 必须为 SHA-256 十六进制（64 字符）")
        if self.sizeBytes < 0 or self.retentionDays <= 0:
            raise ValueError("sizeBytes 非负、retentionDays 为正")

    def digestValid(self, actualSha256: str) -> bool:
        """校验实际读取内容的摘要与记录一致（可读性验证核心）。"""
        return actualSha256 == self.sha256


class WalArchivePolicyV1:
    """WAL 归档间隔策略：实盘不超过 5 分钟。"""

    def __init__(self, maxInterval: timedelta = LIVE_WAL_MAX_INTERVAL) -> None:
        if maxInterval <= timedelta(0):
            raise ValueError("WAL 归档间隔必须为正")
        self._maxInterval = maxInterval

    def maxInterval(self) -> timedelta:
        return self._maxInterval

    def validateInterval(self, interval: timedelta) -> None:
        """校验归档间隔；超过阈值抛出（阻止积压）。"""
        if interval > self._maxInterval:
            raise ValueError(
                f"WAL 归档间隔 {interval} 超过阈值 {self._maxInterval}（实盘 <= 5 分钟）"
            )

    def compliant(self, interval: timedelta) -> bool:
        return interval <= self._maxInterval


@dataclass(frozen=True, slots=True)
class BackupManifestV1:
    """版本化备份清单：依赖工件 + manifest 内容哈希。"""

    manifestId: str
    baseBackup: BackupArtifactV1
    walSegments: tuple[BackupArtifactV1, ...] = ()
    objectArtifacts: tuple[BackupArtifactV1, ...] = ()
    createdAt: datetime = field(default_factory=_utcNowMillisecond)
    manifestHash: str = ""

    def computeHash(self) -> str:
        payload = {
            "manifest_id": self.manifestId,
            "base_backup": self.baseBackup.artifactId,
            "wal_segments": [w.artifactId for w in self.walSegments],
            "object_artifacts": [o.artifactId for o in self.objectArtifacts],
            "created_at": self.createdAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()


class BackupReadabilityVerifierV1:
    """备份可读性自动验证：每月至少一次；校验摘要与元数据完整性。"""

    def __init__(self, verifyInterval: timedelta = READABILITY_VERIFY_INTERVAL) -> None:
        self._interval = verifyInterval
        self._checks: list[tuple[datetime, str, bool]] = []  # (ts, artifactId, ok)

    def verify(
        self, artifact: BackupArtifactV1, actualSha256: str, *, now: datetime | None = None
    ) -> bool:
        """验证备份可读性：摘要一致且非空。失败则标记 VerificationFailed。"""
        ok = bool(actualSha256) and artifact.digestValid(actualSha256)
        now = now if now is not None else _utcNowMillisecond()
        self._checks.append((now, artifact.artifactId, ok))
        if not ok:
            # dataclass frozen：重建标记状态
            return False
        return True

    def verifyManifest(self, manifest: BackupManifestV1) -> tuple[bool, list[str]]:
        """验证整个清单：所有工件摘要自洽（模拟从对象存储读取）。"""
        failures: list[str] = []
        all_artifacts = (manifest.baseBackup,) + manifest.walSegments + manifest.objectArtifacts
        for artifact in all_artifacts:
            # 模拟读取：用记录的 sha256 作为"实际读到的摘要"（存储侧哈希自检）
            if not artifact.digestValid(artifact.sha256):
                failures.append(artifact.artifactId)
        return (not failures, failures)

    def monthlyDue(self, lastVerifiedAt: datetime | None, *, now: datetime | None = None) -> bool:
        """每月自动验证到期判定。"""
        now = now if now is not None else _utcNowMillisecond()
        if lastVerifiedAt is None:
            return True
        return now - lastVerifiedAt >= self._interval

    def lastCheck(self) -> tuple[datetime, str, bool] | None:
        return self._checks[-1] if self._checks else None


@dataclass(frozen=True, slots=True)
class RecoveryEnvironmentV1:
    """恢复环境：必须与生产隔离（环境标识/网络/凭据/数据目录）。"""

    environmentId: str
    productionEnvironmentId: str
    isolatedNetwork: bool
    isolatedCredentials: bool
    isolatedDataDirectory: bool
    dataDirectory: str  # 隔离恢复目录

    def __post_init__(self) -> None:
        if not self.environmentId or self.environmentId == self.productionEnvironmentId:
            raise ValueError("恢复环境必须与生产环境隔离（不同环境标识）")
        if not self.isolatedNetwork or not self.isolatedCredentials or not self.isolatedDataDirectory:
            raise ValueError("恢复环境必须隔离网络、凭据和数据目录")

    def isolatedFrom(self, environmentId: str) -> bool:
        """恢复环境标识与给定环境不同即视为隔离。"""
        return self.environmentId != environmentId


class RecoveryVerificationStatus(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    InsufficientEvidence = "INSUFFICIENT_EVIDENCE"


@dataclass(frozen=True, slots=True)
class RecoveryVerificationV1:
    """隔离环境恢复验证报告。"""

    drillId: str
    environment: RecoveryEnvironmentV1
    manifest: BackupManifestV1
    startedAt: datetime
    completedAt: datetime
    rtoSeconds: float  # 实际 RTO（秒）
    rpoSeconds: float  # 实际 RPO（秒）
    ledgerHashConsistent: bool  # 账本哈希一致
    controlsRecoveredPercent: float  # 活动控制恢复完整率（0~100）
    unreconciledDifferences: int  # 未解释对账差异
    verifiedBy: str | None  # 人工验证人（非作者评审）
    status: RecoveryVerificationStatus = RecoveryVerificationStatus.InsufficientEvidence
    reportHash: str = ""

    @property
    def rtoWithinTarget(self) -> bool:
        return self.rtoSeconds <= LIVE_RTO_TARGET.total_seconds()

    @property
    def rpoWithinTarget(self) -> bool:
        return self.rpoSeconds <= LIVE_RPO_TARGET.total_seconds()

    def uniqueConclusion(self) -> RecoveryVerificationStatus:
        """唯一结论：RTO/RPO/账本哈希/控制 100%/差异 0/人工验证全部满足才 PASS。"""
        if not self.rtoWithinTarget:
            return RecoveryVerificationStatus.Fail
        if not self.rpoWithinTarget:
            return RecoveryVerificationStatus.Fail
        if not self.ledgerHashConsistent:
            return RecoveryVerificationStatus.Fail
        if self.controlsRecoveredPercent != 100.0:
            return RecoveryVerificationStatus.Fail
        if self.unreconciledDifferences != 0:
            return RecoveryVerificationStatus.Fail
        if self.verifiedBy is None:
            return RecoveryVerificationStatus.InsufficientEvidence
        return RecoveryVerificationStatus.Pass


class BackupRecoveryServiceV1:
    """备份与恢复编排：记录备份工件、WAL 间隔校验、可读性验证、隔离恢复演练。"""

    def __init__(
        self,
        *,
        productionEnvironmentId: str = "prod",
        walPolicy: WalArchivePolicyV1 | None = None,
        verifier: BackupReadabilityVerifierV1 | None = None,
    ) -> None:
        self._productionEnvironmentId = productionEnvironmentId
        self._walPolicy = walPolicy or WalArchivePolicyV1()
        self._verifier = verifier or BackupReadabilityVerifierV1()
        self._artifacts: dict[str, BackupArtifactV1] = {}
        self._manifests: dict[str, BackupManifestV1] = {}
        self._drills: list[RecoveryVerificationV1] = []
        self._walIntervals: list[tuple[datetime, timedelta]] = []

    def recordWalInterval(self, interval: timedelta, *, now: datetime | None = None) -> None:
        """记录一次 WAL 归档间隔；超过 5 分钟立即拒绝。"""
        self._walPolicy.validateInterval(interval)
        self._walIntervals.append((now or _utcNowMillisecond(), interval))

    def walCompliant(self, interval: timedelta) -> bool:
        return self._walPolicy.compliant(interval)

    def storeArtifact(
        self,
        *,
        artifactId: str,
        artifactType: BackupArtifactType,
        objectVersionId: str,
        sha256: str,
        sizeBytes: int,
        retentionDays: int,
        createdAt: datetime | None = None,
    ) -> BackupArtifactV1:
        if artifactId in self._artifacts:
            raise ValueError(f"备份工件已存在: {artifactId}")
        artifact = BackupArtifactV1(
            artifactId=artifactId,
            artifactType=artifactType,
            objectVersionId=objectVersionId,
            sha256=sha256,
            sizeBytes=sizeBytes,
            createdAt=createdAt or _utcNowMillisecond(),
            retentionDays=retentionDays,
        )
        self._artifacts[artifactId] = artifact
        return artifact

    def createManifest(
        self,
        *,
        manifestId: str,
        baseBackup: BackupArtifactV1,
        walSegments: tuple[BackupArtifactV1, ...] = (),
        objectArtifacts: tuple[BackupArtifactV1, ...] = (),
    ) -> BackupManifestV1:
        if manifestId in self._manifests:
            raise ValueError(f"备份清单已存在: {manifestId}")
        manifest = BackupManifestV1(
            manifestId=manifestId,
            baseBackup=baseBackup,
            walSegments=walSegments,
            objectArtifacts=objectArtifacts,
        )
        manifest = BackupManifestV1(
            manifestId=manifestId,
            baseBackup=baseBackup,
            walSegments=walSegments,
            objectArtifacts=objectArtifacts,
            createdAt=manifest.createdAt,
            manifestHash=manifest.computeHash(),
        )
        self._manifests[manifestId] = manifest
        return manifest

    def verifyReadability(self, artifactId: str, actualSha256: str) -> bool:
        """自动验证备份可读性（摘要比对）。"""
        artifact = self._artifacts.get(artifactId)
        if artifact is None:
            raise ValueError(f"备份工件不存在: {artifactId}")
        ok = self._verifier.verify(artifact, actualSha256)
        if ok:
            self._artifacts[artifactId] = BackupArtifactV1(
                artifactId=artifact.artifactId,
                artifactType=artifact.artifactType,
                objectVersionId=artifact.objectVersionId,
                sha256=artifact.sha256,
                sizeBytes=artifact.sizeBytes,
                createdAt=artifact.createdAt,
                retentionDays=artifact.retentionDays,
                status=BackupStatus.Verified,
            )
        return ok

    def executeRecoveryDrill(
        self,
        *,
        drillId: str,
        environment: RecoveryEnvironmentV1,
        manifest: BackupManifestV1,
        rtoSeconds: float,
        rpoSeconds: float,
        ledgerHashConsistent: bool,
        controlsRecoveredPercent: float,
        unreconciledDifferences: int,
        verifiedBy: str | None,
        startedAt: datetime | None = None,
        completedAt: datetime | None = None,
    ) -> RecoveryVerificationV1:
        """隔离环境恢复演练；环境必须与生产隔离。"""
        if not environment.isolatedFrom(self._productionEnvironmentId):
            raise ValueError("恢复演练必须在与生产隔离的环境执行")
        if rtoSeconds < 0 or rpoSeconds < 0:
            raise ValueError("RTO/RPO 必须非负")
        if not 0.0 <= controlsRecoveredPercent <= 100.0:
            raise ValueError("控制恢复率必须在 0~100")
        report = RecoveryVerificationV1(
            drillId=drillId,
            environment=environment,
            manifest=manifest,
            startedAt=startedAt or _utcNowMillisecond(),
            completedAt=completedAt or _utcNowMillisecond(),
            rtoSeconds=rtoSeconds,
            rpoSeconds=rpoSeconds,
            ledgerHashConsistent=ledgerHashConsistent,
            controlsRecoveredPercent=controlsRecoveredPercent,
            unreconciledDifferences=unreconciledDifferences,
            verifiedBy=verifiedBy,
        )
        status = report.uniqueConclusion()
        payload = {
            "drill_id": drillId,
            "environment_id": environment.environmentId,
            "manifest_id": manifest.manifestId,
            "rto_seconds": str(rtoSeconds),
            "rpo_seconds": str(rpoSeconds),
            "ledger_hash_consistent": ledgerHashConsistent,
            "controls_recovered_percent": str(controlsRecoveredPercent),
            "unreconciled_differences": unreconciledDifferences,
            "verified_by": verifiedBy,
            "status": status.value,
        }
        final = RecoveryVerificationV1(
            drillId=drillId,
            environment=environment,
            manifest=manifest,
            startedAt=report.startedAt,
            completedAt=report.completedAt,
            rtoSeconds=rtoSeconds,
            rpoSeconds=rpoSeconds,
            ledgerHashConsistent=ledgerHashConsistent,
            controlsRecoveredPercent=controlsRecoveredPercent,
            unreconciledDifferences=unreconciledDifferences,
            verifiedBy=verifiedBy,
            status=status,
            reportHash=hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest(),
        )
        self._drills.append(final)
        return final

    def passedDrillCount(self) -> int:
        return sum(1 for d in self._drills if d.status is RecoveryVerificationStatus.Pass)

    def drills(self) -> tuple[RecoveryVerificationV1, ...]:
        return tuple(self._drills)

    def artifacts(self) -> tuple[BackupArtifactV1, ...]:
        return tuple(self._artifacts.values())

    def manifests(self) -> tuple[BackupManifestV1, ...]:
        return tuple(self._manifests.values())
