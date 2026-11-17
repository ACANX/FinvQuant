"""P5-014 依赖/镜像/策略源码/沙箱安全冻结测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.SupplyChainFreeze import (
    DependencyV1,
    GateStatus,
    ImageDigestV1,
    LicenseGateV1,
    SecurityFreezeServiceV1,
    SecurityFreezeV1,
    SourceArtifactV1,
    VulnerabilityGateV1,
    VulnerabilityV1,
)


def _dep(name: str = "requests", version: str = "2.32.0", license: str = "Apache-2.0") -> DependencyV1:
    return DependencyV1(name=name, version=version, sha256="a" * 64, license=license)


def _image(name: str = "veritasquant/app", tag: str = "1.0.0") -> ImageDigestV1:
    return ImageDigestV1(imageName=name, digest="sha256:" + "b" * 64, tag=tag)


def _source(artifactType: str = "STRATEGY", name: str = "momentum-v3") -> SourceArtifactV1:
    return SourceArtifactV1(artifactType=artifactType, name=name, sha256="c" * 64)


class TestDependency:
    def test_dependency_valid(self) -> None:
        dep = _dep()
        assert dep.name == "requests"

    def test_dependency_requires_hash(self) -> None:
        with pytest.raises(ValueError, match="sha256"):
            DependencyV1(name="x", version="1", sha256="short", license="MIT")

    def test_dependency_requires_fields(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            DependencyV1(name="", version="1", sha256="a" * 64, license="MIT")


class TestImageDigest:
    def test_image_digest_valid(self) -> None:
        img = _image()
        assert img.digest.startswith("sha256:")

    def test_image_digest_requires_format(self) -> None:
        with pytest.raises(ValueError, match="sha256:<64hex>"):
            ImageDigestV1(imageName="app", digest="sha256:short", tag="1.0")


class TestSourceArtifact:
    def test_source_artifact_valid(self) -> None:
        src = _source()
        assert src.artifactType == "STRATEGY"

    def test_source_artifact_requires_hash(self) -> None:
        with pytest.raises(ValueError, match="sha256"):
            SourceArtifactV1(artifactType="CONFIG", name="x", sha256="bad")


class TestVulnerabilityGate:
    def test_pass_no_findings(self) -> None:
        gate = VulnerabilityGateV1()
        assert gate.scan(()) is GateStatus.Pass

    def test_pass_low_severity(self) -> None:
        gate = VulnerabilityGateV1()
        findings = (VulnerabilityV1("lib", "LOW", 3.5),)
        assert gate.scan(findings) is GateStatus.Pass

    def test_fail_high_severity(self) -> None:
        gate = VulnerabilityGateV1()
        findings = (
            VulnerabilityV1("lib-a", "LOW", 3.5),
            VulnerabilityV1("lib-b", "HIGH", 7.5),
        )
        assert gate.scan(findings) is GateStatus.Fail
        assert len(gate.blockingFindings()) == 1
        assert gate.blockingFindings()[0].dependencyName == "lib-b"

    def test_fail_critical(self) -> None:
        gate = VulnerabilityGateV1()
        findings = (VulnerabilityV1("lib", "CRITICAL", 9.8),)
        assert gate.scan(findings) is GateStatus.Fail

    def test_custom_threshold(self) -> None:
        gate = VulnerabilityGateV1(blockThreshold=9.0)
        findings = (VulnerabilityV1("lib", "HIGH", 8.5),)
        assert gate.scan(findings) is GateStatus.Pass

    def test_cvss_bounds(self) -> None:
        with pytest.raises(ValueError, match="CVSS"):
            VulnerabilityV1("lib", "HIGH", 11.0)


class TestLicenseGate:
    def test_pass_all_allowed(self) -> None:
        gate = LicenseGateV1()
        deps = (_dep(license="MIT"), _dep(name="pandas", license="BSD-3-Clause"))
        assert gate.check(deps) is GateStatus.Pass

    def test_fail_unknown_license(self) -> None:
        gate = LicenseGateV1()
        deps = (_dep(license="MIT"), _dep(name="evil", license="Proprietary"))
        assert gate.check(deps) is GateStatus.Fail
        assert len(gate.violations()) == 1
        assert gate.violations()[0].name == "evil"

    def test_custom_whitelist(self) -> None:
        gate = LicenseGateV1(allowedLicenses=frozenset({"MIT"}))
        assert gate.check((_dep(license="MIT"),)) is GateStatus.Pass
        assert gate.check((_dep(license="Apache-2.0"),)) is GateStatus.Fail


class TestSecurityFreeze:
    def test_freeze_hash_stable(self) -> None:
        freeze = SecurityFreezeV1(
            freezeId="f1",
            dependencies=(_dep(),),
            images=(_image(),),
            sourceArtifacts=(_source(),),
            vulnerabilityGate=GateStatus.Pass,
            licenseGate=GateStatus.Pass,
            approvedBy=("alice", "bob"),
        )
        assert freeze.computeHash() == freeze.computeHash()
        assert freeze.gatesPassed()
        assert freeze.approvalsComplete()

    def test_gates_not_passed(self) -> None:
        freeze = SecurityFreezeV1(
            freezeId="f1",
            dependencies=(),
            images=(),
            sourceArtifacts=(),
            vulnerabilityGate=GateStatus.Fail,
            licenseGate=GateStatus.Pass,
            approvedBy=("alice", "bob"),
        )
        assert not freeze.gatesPassed()

    def test_approvals_require_two(self) -> None:
        freeze = SecurityFreezeV1(
            freezeId="f1",
            dependencies=(),
            images=(),
            sourceArtifacts=(),
            vulnerabilityGate=GateStatus.Pass,
            licenseGate=GateStatus.Pass,
            approvedBy=("alice",),
        )
        assert not freeze.approvalsComplete()


class TestSecurityFreezeService:
    def test_freeze_success(self) -> None:
        service = SecurityFreezeServiceV1()
        freeze = service.freeze(
            dependencies=(_dep(), _dep(name="pandas", license="BSD-3-Clause")),
            images=(_image(),),
            sourceArtifacts=(_source(),),
            vulnerabilityFindings=(VulnerabilityV1("lib", "LOW", 2.0),),
            approvedBy=("alice", "bob"),
        )
        assert freeze.gatesPassed()
        assert freeze.freezeHash == freeze.computeHash()
        assert service.current() is not None
        assert service.verifyIntegrity(freeze)

    def test_freeze_blocked_by_vulnerability(self) -> None:
        service = SecurityFreezeServiceV1()
        with pytest.raises(ValueError, match="漏洞 Gate 失败"):
            service.freeze(
                dependencies=(_dep(),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(VulnerabilityV1("lib", "CRITICAL", 9.8),),
                approvedBy=("alice", "bob"),
            )
        assert service.all() == ()

    def test_freeze_blocked_by_license(self) -> None:
        service = SecurityFreezeServiceV1()
        with pytest.raises(ValueError, match="许可证 Gate 失败"):
            service.freeze(
                dependencies=(_dep(license="Proprietary"),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(),
                approvedBy=("alice", "bob"),
            )
        assert service.all() == ()

    def test_freeze_requires_two_approvers(self) -> None:
        service = SecurityFreezeServiceV1()
        with pytest.raises(ValueError, match="两名审批人"):
            service.freeze(
                dependencies=(_dep(),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(),
                approvedBy=("alice",),
            )

    def test_freeze_duplicate_rejected(self) -> None:
        service = SecurityFreezeServiceV1()
        service.freeze(
            dependencies=(_dep(),),
            images=(_image(),),
            sourceArtifacts=(_source(),),
            vulnerabilityFindings=(),
            approvedBy=("alice", "bob"),
            freezeId="freeze-dup",
        )
        with pytest.raises(ValueError, match="已存在"):
            service.freeze(
                dependencies=(_dep(),),
                images=(_image(),),
                sourceArtifacts=(_source(),),
                vulnerabilityFindings=(),
                approvedBy=("alice", "bob"),
                freezeId="freeze-dup",
            )

    def test_verify_integrity_detects_tamper(self) -> None:
        service = SecurityFreezeServiceV1()
        freeze = service.freeze(
            dependencies=(_dep(),),
            images=(_image(),),
            sourceArtifacts=(_source(),),
            vulnerabilityFindings=(),
            approvedBy=("alice", "bob"),
        )
        tampered = SecurityFreezeV1(
            freezeId=freeze.freezeId,
            dependencies=(_dep(name="tampered"),),
            images=freeze.images,
            sourceArtifacts=freeze.sourceArtifacts,
            vulnerabilityGate=freeze.vulnerabilityGate,
            licenseGate=freeze.licenseGate,
            approvedBy=freeze.approvedBy,
            frozenAt=freeze.frozenAt,
            freezeHash=freeze.freezeHash,
        )
        assert not service.verifyIntegrity(tampered)

    def test_full_freeze_flow(self) -> None:
        """完整流程：依赖锁 + 镜像摘要 + 源码哈希 + Gate + 审批齐全。"""
        service = SecurityFreezeServiceV1()
        deps = (
            _dep("fastapi", "0.115.0", "MIT"),
            _dep("uvicorn", "0.30.0", "BSD-3-Clause"),
            _dep("pydantic", "2.8.0", "MIT"),
        )
        images = (
            _image("veritasquant/app", "2026.08.03"),
            _image("veritasquant/migrator", "2026.08.03"),
        )
        sources = (
            _source("STRATEGY", "momentum-v3"),
            _source("CONFIG", "live-config"),
            _source("POLICY", "risk-policy-v5"),
        )
        freeze = service.freeze(
            dependencies=deps,
            images=images,
            sourceArtifacts=sources,
            vulnerabilityFindings=(VulnerabilityV1("urllib3", "MEDIUM", 5.3),),
            approvedBy=("reviewer-qa", "live-approver"),
        )
        assert freeze.gatesPassed()
        assert freeze.approvalsComplete()
        assert len(freeze.dependencies) == 3
        assert len(freeze.images) == 2
        assert len(freeze.sourceArtifacts) == 3
        assert service.verifyIntegrity(freeze)
