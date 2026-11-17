"""P2-043 M2 Gate 检查项与报告测试。"""

from __future__ import annotations

from veritasquant.application.StageGateReport import (
    GateCheckItemV1,
    GateItemStatus,
    StageGateReportBuilderV1,
)


class TestM2Checks:
    def test_m2_platform_checks_present(self) -> None:
        checks = StageGateReportBuilderV1.m2PlatformChecks()
        assert len(checks) == 6
        ids = [c.checkId for c in checks]
        assert "M2-P01" in ids and "M2-P06" in ids
        assert all(c.status is GateItemStatus.Pass for c in checks)

    def test_m2_strategy_checks_present(self) -> None:
        checks = StageGateReportBuilderV1.m2StrategyChecks()
        assert len(checks) == 4
        ids = [c.checkId for c in checks]
        assert "M2-S01" in ids and "M2-S04" in ids

    def test_m2_report_pass_when_all_pass(self) -> None:
        builder = StageGateReportBuilderV1()
        report = builder.build(
            gateName="M2-GATE-PLATFORM",
            stageGatePolicyVersion="V1",
            checks=StageGateReportBuilderV1.m2PlatformChecks(),
            openS0=0,
            openS1=0,
            lookaheadHits=0,
            propertySequences=10_000,
            signer="ACANX",
        )
        assert report.verdict.value == "PASS"
        assert report.mandatoryPassed is True
        report.assertUniqueConclusion()  # PASS 才可进入阶段 3

    def test_m2_report_fail_when_check_failed(self) -> None:
        builder = StageGateReportBuilderV1()
        checks = list(StageGateReportBuilderV1.m2PlatformChecks())
        checks[0] = GateCheckItemV1("M2-P01", "60 日运行", GateItemStatus.Fail, None)
        report = builder.build(
            gateName="M2-GATE-PLATFORM",
            stageGatePolicyVersion="V1",
            checks=tuple(checks),
            openS0=0,
            openS1=0,
            lookaheadHits=0,
            propertySequences=10_000,
        )
        assert report.verdict.value == "FAIL"

    def test_m2_report_fail_with_open_s1(self) -> None:
        builder = StageGateReportBuilderV1()
        report = builder.build(
            gateName="M2-GATE-PLATFORM",
            stageGatePolicyVersion="V1",
            checks=StageGateReportBuilderV1.m2PlatformChecks(),
            openS0=0,
            openS1=1,
            lookaheadHits=0,
            propertySequences=10_000,
        )
        assert report.verdict.value == "FAIL"

    def test_platform_and_strategy_gates_separate(self) -> None:
        """平台与策略 Gate 分别给出结论。"""
        builder = StageGateReportBuilderV1()
        platform = builder.build(
            gateName="M2-GATE-PLATFORM",
            stageGatePolicyVersion="V1",
            checks=StageGateReportBuilderV1.m2PlatformChecks(),
            openS0=0, openS1=0, lookaheadHits=0, propertySequences=10_000,
        )
        strategy = builder.build(
            gateName="M2-GATE-STRATEGY",
            stageGatePolicyVersion="V1",
            checks=StageGateReportBuilderV1.m2StrategyChecks(),
            openS0=0, openS1=0, lookaheadHits=0, propertySequences=10_000,
        )
        assert platform.gateName == "M2-GATE-PLATFORM"
        assert strategy.gateName == "M2-GATE-STRATEGY"
        assert platform.verdict.value == "PASS"
        assert strategy.verdict.value == "PASS"
