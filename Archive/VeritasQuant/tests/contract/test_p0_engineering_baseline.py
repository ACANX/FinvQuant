from __future__ import annotations

from pathlib import Path

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]


def loadYaml(relativePath: str) -> dict[str, object]:
    value = yaml.safe_load((ROOT / relativePath).read_text(encoding="utf-8"))
    assert isinstance(value, dict)
    return value


@pytest.mark.stable_id("P0-004-001")
def test_solution_a_layout_has_required_roots_and_single_import_package() -> None:
    for directory in ("Apps", "Jobs", "Migrations", "Docker", "Configs", "Resources", "src", "tests"):
        assert (ROOT / directory).is_dir()
    assert (ROOT / "src" / "veritasquant" / "__init__.py").is_file()
    assert not list(ROOT.glob("*.py"))


@pytest.mark.stable_id("P0-008-001")
def test_development_compose_is_ephemeral_and_has_health_checks() -> None:
    compose = loadYaml("Docker/docker-compose.yml")
    services = compose["services"]
    assert isinstance(services, dict)
    assert set(services) == {"postgresql", "redis"}
    assert "volumes" not in compose
    for service in services.values():
        assert isinstance(service, dict)
        assert "tmpfs" in service
        assert "healthcheck" in service


@pytest.mark.stable_id("P0-010-001")
def test_all_p0_work_items_have_governance_fields_and_audit_history() -> None:
    register = loadYaml("Docs/DevelopmentWorkflow/Registers/WorkItemRegister.yml")
    records = register["Records"]
    assert isinstance(records, list)
    p0Records = {record["PlanTaskId"]: record for record in records if isinstance(record, dict) and str(record.get("PlanTaskId", "")).startswith("P0-")}
    assert set(p0Records) == {f"P0-{number:03d}" for number in range(1, 14)}
    required = {"WorkItemId", "PlanTaskId", "Title", "Description", "Owner", "Approver", "Dependencies", "AcceptanceCriteria", "TestEvidence", "RiskLinks", "Status", "StatusChangedTs", "AuditHistory"}
    for record in p0Records.values():
        assert required <= set(record)
        assert record["AuditHistory"]


@pytest.mark.stable_id("P0-010-002")
def test_all_governance_registers_declare_required_fields_and_unique_record_ids() -> None:
    registers = (
        "Docs/DevelopmentWorkflow/Registers/WorkItemRegister.yml",
        "Docs/DevelopmentWorkflow/Registers/BugRegister.yml",
        "Docs/DevelopmentWorkflow/Registers/RiskRegister.yml",
        "Docs/DevelopmentWorkflow/Registers/IncidentRegister.yml",
        "Docs/DevelopmentWorkflow/Registers/ChangeRegister.yml",
        "Docs/DevelopmentWorkflow/Registers/ActionRegister.yml",
    )
    for path in registers:
        register = loadYaml(path)
        assert register["RequiredFields"]
        records = register["Records"]
        assert isinstance(records, list)
        idFields = [field for field in register["RequiredFields"] if str(field).endswith("Id")]
        for record in records:
            assert isinstance(record, dict)
            assert any(record.get(field) for field in idFields)


@pytest.mark.stable_id("P0-012-001")
def test_traceability_matrix_covers_all_mandatory_review_contracts() -> None:
    matrix = loadYaml("Docs/DevelopmentWorkflow/Registers/TraceabilityMatrix.yml")
    rows = matrix["Rows"]
    assert isinstance(rows, list)
    requirementIds = {row["RequirementId"] for row in rows if isinstance(row, dict)}
    assert requirementIds == {f"R-{number:03d}" for number in range(1, 19)}
    for row in rows:
        assert row["PlanTaskIds"]
        assert row["TestIds"]
        assert row["LatestGate"]


@pytest.mark.stable_id("P0-005-001")
def test_ci_and_package_metadata_use_python_313_baseline() -> None:
    workflow = loadYaml(".github/workflows/Ci.yml")
    quality = workflow["jobs"]["quality"]
    assert quality["strategy"]["matrix"]["python-version"] == ["3.13"]
    project = (ROOT / "pyproject.toml").read_text(encoding="utf-8")
    assert 'requires-python = ">=3.13"' in project


@pytest.mark.stable_id("P0-005-002")
def test_ci_uses_single_global_version_constant() -> None:
    """Ci.yml 必须通过全局常量引用包版本，禁止硬编码 wheel/sdist 版本号。

    升级版本时只需改两处：pyproject.toml 与 Ci.yml 顶层的 VQ_VERSION。
    """
    ci = (ROOT / ".github" / "workflows" / "Ci.yml").read_text(encoding="utf-8")
    # 全局常量必须存在
    assert "VQ_VERSION" in ci
    # 所有 wheel/sdist 引用必须通过常量（${{ env.VQ_VERSION }}），禁止硬编码版本
    assert "veritasquant-${{ env.VQ_VERSION }}-py3-none-any.whl" in ci
    assert "veritasquant-${{ env.VQ_VERSION }}.tar.gz" in ci
    # 常量值必须与 pyproject.toml 一致（单一来源原则）
    workflow = loadYaml(".github/workflows/Ci.yml")
    constant = workflow["env"]["VQ_VERSION"]
    project = (ROOT / "pyproject.toml").read_text(encoding="utf-8")
    assert f'version = "{constant}"' in project
    # 不得残留硬编码的 veritasquant-<版本> 文件名（常量引用除外）
    for line in ci.splitlines():
        if "veritasquant-" in line and "VQ_VERSION" not in line:
            raise AssertionError(f"发现硬编码版本引用: {line.strip()}")


@pytest.mark.stable_id("P0-005-003")
def test_ci_builds_and_publishes_ghcr_image() -> None:
    """CI 必须包含 ghcr.io 镜像构建 job（GitHub Packages 发布服务端镜像）。"""
    workflow = loadYaml(".github/workflows/Ci.yml")
    jobs = workflow["jobs"]
    assert "build-image" in jobs
    build_image = jobs["build-image"]
    # 需要 packages: write 权限（推送 ghcr.io）
    assert build_image["permissions"]["packages"] == "write"
    steps = "\n".join(
        step.get("uses", "") + "\n" + str(step.get("with", ""))
        for step in build_image["steps"]
    )
    # 登录 ghcr.io 使用内置 GITHUB_TOKEN（不引入外部凭据）
    assert "docker/login-action" in steps
    assert "secrets.GITHUB_TOKEN" in steps
    # 构建并推送（build-push-action）
    assert "docker/build-push-action" in steps
    # 镜像仓库名必须转小写（Docker 命名规范；github.repository 可能含大写）
    assert "steps.repo.outputs.image" in steps
    ci_text = (ROOT / ".github" / "workflows" / "Ci.yml").read_text(encoding="utf-8")
    assert "Normalize lowercase image repository" in ci_text
    assert "${GITHUB_REPOSITORY,,}" in ci_text  # bash 小写化
    assert "ghcr.io/${{ github.repository }}" not in ci_text  # 禁止大写直用
    # 版本 tag：V* 触发发布版本镜像；main/dev 刷新 latest
    assert "latest" in steps
    assert "refs/tags/V" in steps
    # 手动触发（workflow_dispatch）生成带版本前缀的 yyyyMMddHHmm 时间戳 tag
    assert "Generate timestamp tag for dev push and manual build" in "\n".join(
        step.get("name", "") for step in build_image["steps"]
    )
    assert "%Y%m%d%H%M" in ci_text
    assert "github.event_name == 'workflow_dispatch'" in ci_text
    # 时间戳版本必须带版本前缀（如 0.1.2-202608031217）
    assert 'echo "tag=${{ env.VQ_VERSION }}-$TS"' in ci_text
    # dev 推送生成时间戳 tag（0.1.2-<UTC时间戳>），main 推送生成版本 tag（0.1.2）
    assert "github.ref == 'refs/heads/dev'" in ci_text
    assert "github.ref == 'refs/heads/main'" in ci_text
    assert "type=sha,format=short" not in ci_text  # 禁止 sha-* 作为镜像 tag
    # 全局版本常量 VQ_VERSION 必须存在（版本单一来源）
    assert workflow["env"]["VQ_VERSION"]
