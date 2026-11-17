from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]


def runTool(toolName: str, *arguments: str) -> subprocess.CompletedProcess[str]:
    """以正式解释器调用维护工具，验证 CLI 的退出码和定位信息。"""
    return subprocess.run(
        [sys.executable, str(ROOT / "scripts" / toolName), *arguments],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


@pytest.mark.stable_id("P0-007-001")
def test_preflight_rejects_invalid_project_yaml_and_timestamp_alias(tmp_path: Path) -> None:
    configs = tmp_path / "Configs"
    configs.mkdir()
    (configs / "Invalid.yml").write_text("bad_field: value\nTimestamp: 2026-01-01\n", encoding="utf-8")

    result = runTool("Preflight.py", "--root", str(tmp_path))

    assert result.returncode == 1
    assert "bad_field" in result.stdout
    assert "Timestamp" in result.stdout


@pytest.mark.stable_id("P0-007-002")
def test_preflight_rejects_root_level_python_business_package(tmp_path: Path) -> None:
    package = tmp_path / "ParallelPackage"
    package.mkdir()
    (package / "__init__.py").write_text("", encoding="utf-8")

    result = runTool("Preflight.py", "--root", str(tmp_path))

    assert result.returncode == 1
    assert "平行业务 Python 包" in result.stdout


@pytest.mark.stable_id("P0-007-003")
def test_preflight_supports_cp1252_output_for_violations(tmp_path: Path) -> None:
    configs = tmp_path / "Configs"
    configs.mkdir()
    (configs / "Invalid.yml").write_text("invalid_field: value\n", encoding="utf-8")
    environment = dict(os.environ)
    environment["PYTHONIOENCODING"] = "cp1252"

    result = subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "Preflight.py"), "--root", str(tmp_path)],
        cwd=ROOT,
        env=environment,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 1
    assert "invalid_field" in result.stdout
    assert "UnicodeEncodeError" not in result.stderr


@pytest.mark.stable_id("P0-011-001")
def test_secret_scanner_blocks_high_confidence_assignment_without_echoing_value(tmp_path: Path) -> None:
    (tmp_path / "Source.py").write_text('api_key = "vq_test_credential_123456"\n', encoding="utf-8")

    result = runTool("ScanSecrets.py", "--root", str(tmp_path))

    assert result.returncode == 1
    assert "sensitive-assignment" in result.stdout
    assert "vq_test_credential_123456" not in result.stdout


@pytest.mark.stable_id("P0-009-001")
def test_evidence_collector_records_junit_environment_seed_and_artifact_hash(tmp_path: Path) -> None:
    junit = tmp_path / "JUnit.xml"
    coverage = tmp_path / "Coverage.xml"
    artifact = tmp_path / "Package.whl"
    output = tmp_path / "TestEvidence.json"
    junit.write_text('<testsuite tests="2" failures="0" errors="0" skipped="0"/>', encoding="utf-8")
    coverage.write_text("<coverage/>", encoding="utf-8")
    artifact.write_bytes(b"wheel")

    result = runTool(
        "CollectTestEvidence.py",
        "--junit",
        str(junit),
        "--coverage",
        str(coverage),
        "--artifact",
        str(artifact),
        "--work-item",
        "P0-009",
        "--seed",
        "20260731",
        "--output",
        str(output),
    )

    evidence = json.loads(output.read_text(encoding="utf-8"))
    assert result.returncode == 0
    assert evidence["junit"]["tests"] == 2
    assert evidence["random_seed"] == "20260731"
    assert evidence["artifacts"][0]["sha256"]
    assert evidence["environment"]["python"]


@pytest.mark.stable_id("P0-011-002")
def test_license_policy_requires_approval_before_it_can_pass(tmp_path: Path) -> None:
    requirements = tmp_path / "Requirements"
    security = tmp_path / "Configs" / "Security"
    requirements.mkdir()
    security.mkdir(parents=True)
    (requirements / "Runtime.lock").write_text("pydantic==2.12.5\n", encoding="utf-8")
    (requirements / "Development.lock").write_text("-r Runtime.lock\n", encoding="utf-8")
    policy = security / "LicensePolicy.yml"
    policy.write_text("ApprovalStatus: PENDING_APPROVAL\nAllowedLicenses: [MIT]\nPackageExceptions: {}\n", encoding="utf-8")

    result = runTool("VerifyLicenses.py", "--root", str(tmp_path), "--policy", str(policy))

    assert result.returncode == 1
    assert "尚未获得批准" in result.stdout


def test_verify_package_supports_cp1252_output_for_failures(tmp_path: Path) -> None:
    environment = dict(os.environ)
    environment["PYTHONIOENCODING"] = "cp1252"
    result = subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "VerifyPackage.py"), "--wheel", str(tmp_path / "missing.whl")],
        cwd=ROOT,
        env=environment,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 1
    assert b"UnicodeEncodeError" not in result.stderr
