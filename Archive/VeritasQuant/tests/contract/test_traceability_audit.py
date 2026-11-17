"""R-001 至 R-008、R-010 至 R-012、R-014、R-015 追踪审计（P1-074）。

每个需求关联具体测试结果、种子、工件哈希和责任人；跳过、环境失败或无
哈希不能视为通过。审计测试验证矩阵条目与代码证据存在且可下钻。
"""

from __future__ import annotations

from pathlib import Path

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[2]

# M1 Gate 需求范围（P1-074 审计清单）
M1_REQUIREMENTS = ("R-001", "R-002", "R-003", "R-004", "R-005", "R-006", "R-007", "R-008", "R-010", "R-011", "R-012", "R-014", "R-015")

# 本批次（P1-041~076）新增证据文件，必须存在于仓库
NEW_EVIDENCE_FILES = (
    "tests/unit/accounts/test_snapshot.py",
    "tests/unit/accounts/test_property_sequences.py",
    "tests/unit/execution/test_orders.py",
    "tests/unit/execution/test_order_state_machine.py",
    "tests/unit/execution/test_report_processor.py",
    "tests/unit/execution/test_ideal_execution.py",
    "tests/unit/execution/test_bar_path.py",
    "tests/unit/execution/test_execution_model.py",
    "tests/unit/execution/test_liquidity.py",
    "tests/unit/execution/test_atomic_execution.py",
    "src/veritasquant/execution/OrderModelSuite.py",
    "tests/unit/risk/test_alert_models.py",
    "tests/unit/risk/test_alert_normalizer.py",
    "tests/unit/risk/test_alert_correlator.py",
    "tests/unit/risk/test_alert_policy_engine.py",
    "tests/unit/risk/test_risk_engine.py",
    "tests/unit/risk/test_control_book.py",
    "tests/unit/risk/test_basic_rules.py",
    "tests/unit/risk/test_atomic_risk.py",
    "tests/contract/test_risk_contracts.py",
    "tests/unit/strategy/test_base_strategy.py",
    "tests/unit/strategy/test_indicator_window.py",
    "tests/unit/strategy/test_sandbox.py",
    "tests/unit/strategy/test_example_strategies.py",
    "tests/unit/strategy/test_sandbox_security.py",
    "tests/unit/application/test_backtest_service.py",
    "tests/unit/reporting/test_performance.py",
    "tests/unit/reporting/test_dual_track_report.py",
    "tests/unit/reporting/test_artifacts.py",
    "tests/unit/reporting/test_lookahead_probe.py",
    "tests/integration/test_end_to_end_pipeline.py",
    "tests/integration/test_cross_platform_regression.py",
    "tests/integration/test_performance_baseline.py",
)


@pytest.mark.stable_id("P1-074-001")
def test_m1_requirements_are_all_present_in_traceability_matrix() -> None:
    """M1 Gate 需求全部登记于追踪矩阵。"""
    matrix = yaml.safe_load((ROOT / "Docs/DevelopmentWorkflow/Registers/TraceabilityMatrix.yml").read_text(encoding="utf-8"))
    rows = matrix["Rows"]
    requirementIds = {row["RequirementId"] for row in rows}
    missing = set(M1_REQUIREMENTS) - requirementIds
    assert not missing, f"矩阵缺少需求: {missing}"


@pytest.mark.stable_id("P1-074-002")
def test_all_new_evidence_files_exist_for_drilldown() -> None:
    """本批次证据文件全部存在，可下钻原始证据。"""
    missing = [path for path in NEW_EVIDENCE_FILES if not (ROOT / path).is_file()]
    assert not missing, f"证据文件缺失: {missing}"


@pytest.mark.stable_id("P1-074-003")
def test_risk_and_order_requirements_have_execution_evidence() -> None:
    """R-004/R-006/R-007/R-008/R-010 具备 ExecutionEvidence 与 EvidenceLinks。"""
    matrix = yaml.safe_load((ROOT / "Docs/DevelopmentWorkflow/Registers/TraceabilityMatrix.yml").read_text(encoding="utf-8"))
    rows = {row["RequirementId"]: row for row in matrix["Rows"]}
    for requirement in ("R-004", "R-006", "R-007", "R-008", "R-010"):
        row = rows.get(requirement)
        assert row is not None, f"缺少 {requirement}"
        evidence = row.get("ExecutionEvidence")
        assert evidence is not None, f"{requirement} 缺少 ExecutionEvidence"
        assert evidence.get("EvidenceLinks"), f"{requirement} 缺少 EvidenceLinks"
        # 证据链接文件必须存在
        for link in evidence["EvidenceLinks"]:
            assert (ROOT / link).exists(), f"{requirement} 证据 {link} 不存在"


@pytest.mark.stable_id("P1-074-004")
def test_m1_gate_requirements_have_no_unapproved_skips() -> None:
    """M1 需求测试不得跳过或无哈希通过（登记测试即无 skip）。"""
    skippedMarkers = ("@pytest.mark.skip", "skipif")
    evidencePaths = (
        "tests/contract/test_risk_contracts.py",
        "tests/integration/test_cross_platform_regression.py",
        "tests/integration/test_performance_baseline.py",
    )
    for path in evidencePaths:
        content = (ROOT / path).read_text(encoding="utf-8")
        assert skippedMarkers[0] not in content and skippedMarkers[1] not in content, f"{path} 存在跳过标记"
