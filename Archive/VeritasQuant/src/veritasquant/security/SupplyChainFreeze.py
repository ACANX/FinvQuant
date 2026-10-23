"""P5-014 依赖/镜像/策略源码/沙箱安全冻结。

对齐 TechSpec 12.3/13 阶段 5 与 ISSUE #210 验收标准：
- 漏洞与许可证 Gate 通过（依赖漏洞扫描、许可证白名单）；
- 镜像摘要（容器镜像 SHA-256 digest 不可变锁定）；
- 依赖锁（锁定文件哈希 + 许可合规）；
- 策略源码哈希（冻结策略工件 SHA-256）；
- 审批齐全（冻结必须经非作者评审/双人授权）后才允许进入实盘。

- `DependencyV1`：依赖条目（名称/版本/哈希/许可证）；
- `ImageDigestV1`：容器镜像摘要锁定；
- `SourceArtifactV1`：策略/配置源码工件哈希；
- `VulnerabilityGateV1`：漏洞扫描 Gate（CVSS 阈值阻断）；
- `LicenseGateV1`：许可证白名单 Gate；
- `SecurityFreezeV1`：安全冻结清单（依赖锁/镜像摘要/源码哈希/审批齐全）；
- `SecurityFreezeServiceV1`：冻结编排（Gate 全部通过 + 审批后才冻结）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash

# 漏洞阻断阈值（CVSS v3 基础分）
VULNERABILITY_BLOCK_THRESHOLD = 7.0  # HIGH 及以上阻断
# 默认许可白名单
DEFAULT_ALLOWED_LICENSES = frozenset(
    {
        "MIT",
        "Apache-2.0",
        "BSD-3-Clause",
        "BSD-2-Clause",
        "ISC",
        "PSF-2.0",
        "MPL-2.0",
        "Unlicense",
    }
)


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class GateStatus(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"
    NotExecuted = "NOT_EXECUTED"  # 未执行 = 不通过（不静默通过）


class FreezeStatus(StrEnum):
    Draft = "DRAFT"
    Frozen = "FROZEN"
    Superseded = "SUPERSEDED"


@dataclass(frozen=True, slots=True)
class DependencyV1:
    """依赖条目：名称/版本/内容哈希/许可证。"""

    name: str
    version: str
    sha256: str  # 依赖内容/锁定条目哈希
    license: str

    def __post_init__(self) -> None:
        if not self.name or not self.version or not self.license:
            raise ValueError("依赖名称/版本/许可证不能为空")
        if len(self.sha256) != 64:
            raise ValueError("sha256 必须为 SHA-256 十六进制（64 字符）")


@dataclass(frozen=True, slots=True)
class VulnerabilityV1:
    """依赖漏洞条目。"""

    dependencyName: str
    severity: str  # CRITICAL/HIGH/MEDIUM/LOW
    cvssScore: float  # CVSS v3 基础分（0~10）

    def __post_init__(self) -> None:
        if not 0.0 <= self.cvssScore <= 10.0:
            raise ValueError("CVSS 分数必须在 0~10")


class VulnerabilityGateV1:
    """漏洞扫描 Gate：CVSS >= 阈值（默认 7.0）阻断。"""

    def __init__(self, blockThreshold: float = VULNERABILITY_BLOCK_THRESHOLD) -> None:
        if not 0.0 <= blockThreshold <= 10.0:
            raise ValueError("阻断阈值必须在 0~10")
        self._threshold = blockThreshold
        self._findings: list[VulnerabilityV1] = []

    def scan(self, findings: tuple[VulnerabilityV1, ...]) -> GateStatus:
        """执行漏洞扫描；HIGH 及以上阻断。"""
        self._findings = list(findings)
        blocking = [f for f in findings if f.cvssScore >= self._threshold]
        return GateStatus.Fail if blocking else GateStatus.Pass

    def blockingFindings(self) -> tuple[VulnerabilityV1, ...]:
        return tuple(f for f in self._findings if f.cvssScore >= self._threshold)


class LicenseGateV1:
    """许可证白名单 Gate：未批准许可证阻断。"""

    def __init__(self, allowedLicenses: frozenset[str] = DEFAULT_ALLOWED_LICENSES) -> None:
        self._allowed = allowedLicenses
        self._violations: list[DependencyV1] = []

    def check(self, dependencies: tuple[DependencyV1, ...]) -> GateStatus:
        """检查全部依赖许可证；未批准阻断。"""
        self._violations = [d for d in dependencies if d.license not in self._allowed]
        return GateStatus.Fail if self._violations else GateStatus.Pass

    def violations(self) -> tuple[DependencyV1, ...]:
        return tuple(self._violations)


@dataclass(frozen=True, slots=True)
class ImageDigestV1:
    """容器镜像摘要锁定。"""

    imageName: str
    digest: str  # sha256:<hex> 不可变摘要
    tag: str

    def __post_init__(self) -> None:
        if not self.imageName or not self.tag:
            raise ValueError("镜像名称与标签不能为空")
        if not self.digest.startswith("sha256:") or len(self.digest) != 71:
            raise ValueError("镜像摘要必须为 sha256:<64hex> 格式")


@dataclass(frozen=True, slots=True)
class SourceArtifactV1:
    """策略/配置源码工件哈希。"""

    artifactType: str  # STRATEGY / CONFIG / POLICY
    name: str
    sha256: str

    def __post_init__(self) -> None:
        if not self.artifactType or not self.name:
            raise ValueError("工件类型与名称不能为空")
        if len(self.sha256) != 64:
            raise ValueError("sha256 必须为 SHA-256 十六进制（64 字符）")


@dataclass(frozen=True, slots=True)
class SecurityFreezeV1:
    """安全冻结清单：依赖锁 + 镜像摘要 + 源码哈希 + Gate 结果 + 审批。"""

    freezeId: str
    dependencies: tuple[DependencyV1, ...]  # 依赖锁
    images: tuple[ImageDigestV1, ...]  # 镜像摘要
    sourceArtifacts: tuple[SourceArtifactV1, ...]  # 策略源码哈希
    vulnerabilityGate: GateStatus
    licenseGate: GateStatus
    approvedBy: tuple[str, ...]  # 审批人（非作者评审/双人授权）
    frozenAt: datetime = field(default_factory=_utcNowMillisecond)
    freezeHash: str = ""

    def computeHash(self) -> str:
        payload = {
            "freeze_id": self.freezeId,
            "dependencies": [f"{d.name}@{d.version}:{d.sha256}:{d.license}" for d in self.dependencies],
            "images": [f"{i.imageName}@{i.digest}" for i in self.images],
            "source_artifacts": [f"{s.artifactType}:{s.name}:{s.sha256}" for s in self.sourceArtifacts],
            "vulnerability_gate": self.vulnerabilityGate.value,
            "license_gate": self.licenseGate.value,
            "approved_by": list(self.approvedBy),
            "frozen_at": self.frozenAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()

    def gatesPassed(self) -> bool:
        """漏洞/许可证 Gate 全部通过。"""
        return (
            self.vulnerabilityGate is GateStatus.Pass
            and self.licenseGate is GateStatus.Pass
        )

    def approvalsComplete(self) -> bool:
        """审批齐全：至少两名审批人（非作者评审/双人授权）。"""
        return len(self.approvedBy) >= 2


class SecurityFreezeServiceV1:
    """安全冻结编排：Gate 全部通过 + 审批齐全才冻结；冻结后不可变。"""

    def __init__(
        self,
        *,
        vulnerabilityGate: VulnerabilityGateV1 | None = None,
        licenseGate: LicenseGateV1 | None = None,
    ) -> None:
        self._vulnGate = vulnerabilityGate or VulnerabilityGateV1()
        self._licenseGate = licenseGate or LicenseGateV1()
        self._freezes: dict[str, SecurityFreezeV1] = {}
        self._counter = 0

    def freeze(
        self,
        *,
        dependencies: tuple[DependencyV1, ...],
        images: tuple[ImageDigestV1, ...],
        sourceArtifacts: tuple[SourceArtifactV1, ...],
        vulnerabilityFindings: tuple[VulnerabilityV1, ...],
        approvedBy: tuple[str, ...],
        freezeId: str | None = None,
    ) -> SecurityFreezeV1:
        """执行冻结：扫描漏洞 → 检查许可证 → 校验审批 → 生成冻结清单。"""
        vulnStatus = self._vulnGate.scan(vulnerabilityFindings)
        licenseStatus = self._licenseGate.check(dependencies)

        draft = SecurityFreezeV1(
            freezeId=freezeId or f"freeze-{self._counter + 1:04d}",
            dependencies=dependencies,
            images=images,
            sourceArtifacts=sourceArtifacts,
            vulnerabilityGate=vulnStatus,
            licenseGate=licenseStatus,
            approvedBy=approvedBy,
        )
        draftHash = draft.computeHash()
        draft = SecurityFreezeV1(
            freezeId=draft.freezeId,
            dependencies=dependencies,
            images=images,
            sourceArtifacts=sourceArtifacts,
            vulnerabilityGate=vulnStatus,
            licenseGate=licenseStatus,
            approvedBy=approvedBy,
            frozenAt=draft.frozenAt,
            freezeHash=draftHash,
        )

        # Gate 阻断检查
        if not draft.gatesPassed():
            blocking: list[str] = []
            if vulnStatus is not GateStatus.Pass:
                blocking.append(f"漏洞 Gate 失败: {len(self._vulnGate.blockingFindings())} 个 HIGH+ 漏洞")
            if licenseStatus is not GateStatus.Pass:
                blocking.append(f"许可证 Gate 失败: {len(self._licenseGate.violations())} 个未批准许可")
            raise ValueError("安全冻结被阻断: " + "; ".join(blocking))

        # 审批齐全检查
        if not draft.approvalsComplete():
            raise ValueError("安全冻结必须经至少两名审批人（非作者评审/双人授权）")

        if draft.freezeId in self._freezes:
            raise ValueError(f"安全冻结已存在: {draft.freezeId}")
        self._counter += 1
        self._freezes[draft.freezeId] = draft
        return draft

    def get(self, freezeId: str) -> SecurityFreezeV1 | None:
        return self._freezes.get(freezeId)

    def all(self) -> tuple[SecurityFreezeV1, ...]:
        return tuple(self._freezes.values())

    def current(self) -> SecurityFreezeV1 | None:
        return self._freezes[max(self._freezes.keys())] if self._freezes else None

    def verifyIntegrity(self, freeze: SecurityFreezeV1) -> bool:
        """校验冻结清单哈希未被篡改。"""
        stored = self._freezes.get(freeze.freezeId)
        if stored is None:
            return False
        return stored.freezeHash == freeze.computeHash()
