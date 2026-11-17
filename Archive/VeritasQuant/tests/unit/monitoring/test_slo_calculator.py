"""P2-037 SLO 计算、错误预算与告警路由单元测试。"""

from __future__ import annotations

from datetime import date, timedelta

import pytest

from veritasquant.monitoring.PrometheusMetrics import MetricsRegistry
from veritasquant.monitoring.SloCalculator import (
    AlertRouteV1,
    AlertRouterV1,
    ExecutionMode,
    SliName,
    SliObservationV1,
    SloCalculatorV1,
    SloPolicyV1,
    SloStatus,
)


def _day(offset: int) -> str:
    return str(date.today() - timedelta(days=offset))


def _obs(
    sli: SliName,
    value: float,
    offset: int = 1,
    runId: str = "run-1",
    accountGroup: str = "g1",
) -> SliObservationV1:
    return SliObservationV1(
        sli=sli,
        value=value,
        tradingDay=_day(offset),
        runId=runId,
        accountGroup=accountGroup,
    )


class TestSloPolicy:
    def test_paper_targets_present(self) -> None:
        policy = SloPolicyV1()
        targets = policy.targetsFor(ExecutionMode.Paper)
        assert SliName.TradingReadinessAvailability in targets
        assert SliName.UnreconciledDifferences in targets

    def test_zero_budget_correctness(self) -> None:
        policy = SloPolicyV1()
        targets = policy.targetsFor(ExecutionMode.Paper)
        assert targets[SliName.UnreconciledDifferences].zeroBudget is True
        assert targets[SliName.ControlRecoveryCompleteness].zeroBudget is True
        assert targets[SliName.TradingReadinessAvailability].zeroBudget is False

    def test_backtest_has_no_runtime_targets(self) -> None:
        policy = SloPolicyV1()
        assert policy.targetsFor(ExecutionMode.Backtest) == {}

    def test_mode_thresholds_differ(self) -> None:
        policy = SloPolicyV1()
        paper = policy.targetsFor(ExecutionMode.Paper)[SliName.TradingReadinessAvailability]
        live = policy.targetsFor(ExecutionMode.Live)[SliName.TradingReadinessAvailability]
        assert paper.threshold < live.threshold


class TestSloCalculator:
    def test_insufficient_evidence_when_empty(self) -> None:
        calc = SloCalculatorV1()
        summary = calc.evaluate(ExecutionMode.Paper)
        assert summary.worst() is SloStatus.InsufficientEvidence
        assert all(r.status is SloStatus.InsufficientEvidence for r in summary.results)

    def test_all_within_budget(self) -> None:
        calc = SloCalculatorV1()
        # 5 天全部达标
        for i in range(1, 6):
            calc.record(_obs(SliName.LedgerCommitLatencyP99, 0.2, i))
            calc.record(_obs(SliName.UnreconciledDifferences, 0.0, i))
        summary = calc.evaluate(ExecutionMode.Paper)
        led = next(r for r in summary.results if r.sli is SliName.LedgerCommitLatencyP99)
        diff = next(r for r in summary.results if r.sli is SliName.UnreconciledDifferences)
        assert led.status is SloStatus.WithinBudget
        assert led.budgetRemaining == 1.0
        assert diff.status is SloStatus.WithinBudget

    def test_budget_depletion(self) -> None:
        calc = SloCalculatorV1()
        # 10 天全部违约可用率类指标
        for i in range(1, 11):
            calc.record(_obs(SliName.LedgerCommitLatencyP99, 5.0, i))
        summary = calc.evaluate(ExecutionMode.Paper)
        led = next(r for r in summary.results if r.sli is SliName.LedgerCommitLatencyP99)
        assert led.status is SloStatus.Exceeded
        assert led.budgetRemaining == 0.0

    def test_partial_violation_budget_remaining(self) -> None:
        calc = SloCalculatorV1()
        for i in range(1, 6):
            calc.record(_obs(SliName.LedgerCommitLatencyP99, 0.2, i))
        calc.record(_obs(SliName.LedgerCommitLatencyP99, 9.0, 6))
        summary = calc.evaluate(ExecutionMode.Paper)
        led = next(r for r in summary.results if r.sli is SliName.LedgerCommitLatencyP99)
        # 1/6 违约 → 预算 5/6
        assert led.budgetRemaining == pytest.approx(5 / 6)

    def test_zero_budget_any_violation_fails(self) -> None:
        calc = SloCalculatorV1()
        for i in range(1, 20):
            calc.record(_obs(SliName.UnreconciledDifferences, 0.0, i))
        # 第 21 天出现一次差异
        calc.record(_obs(SliName.UnreconciledDifferences, 1.0, 21))
        summary = calc.evaluate(ExecutionMode.Paper)
        diff = next(r for r in summary.results if r.sli is SliName.UnreconciledDifferences)
        assert diff.status is SloStatus.Exceeded
        assert diff.budgetRemaining == 0.0

    def test_window_filter_excludes_old_days(self) -> None:
        calc = SloCalculatorV1()
        # 40 天前违约 → 不在 30 天窗口内
        calc.record(_obs(SliName.UnreconciledDifferences, 1.0, 40))
        summary = calc.evaluate(ExecutionMode.Paper)
        diff = next(r for r in summary.results if r.sli is SliName.UnreconciledDifferences)
        assert diff.status is SloStatus.InsufficientEvidence

    def test_account_group_isolation(self) -> None:
        calc = SloCalculatorV1()
        # g1 违约，g2 正常
        calc.record(_obs(SliName.LedgerCommitLatencyP99, 9.0, 1, accountGroup="g1"))
        calc.record(_obs(SliName.LedgerCommitLatencyP99, 0.2, 1, accountGroup="g2"))
        g1 = calc.evaluateAccountGroup(ExecutionMode.Paper, "g1")
        g2 = calc.evaluateAccountGroup(ExecutionMode.Paper, "g2")
        led1 = next(r for r in g1.results if r.sli is SliName.LedgerCommitLatencyP99)
        led2 = next(r for r in g2.results if r.sli is SliName.LedgerCommitLatencyP99)
        assert led1.status is SloStatus.Exceeded
        assert led2.status is SloStatus.WithinBudget


class TestAlertRouter:
    def test_route_generates_remediation_link(self) -> None:
        calc = SloCalculatorV1()
        calc.record(_obs(SliName.UnreconciledDifferences, 1.0, 1))
        summary = calc.evaluate(ExecutionMode.Paper)
        router = AlertRouterV1()
        alerts = router.route(summary, runId="run-9", accountGroup="g1")
        assert len(alerts) == 1
        alert = alerts[0]
        assert alert.severity == "P1"  # 正确性指标 → P1
        assert alert.runId == "run-9"
        assert alert.accountGroup == "g1"
        assert "run-9/accounts/g1" in alert.remediationLink
        assert len(alert.dedupeKey) == 16

    def test_route_non_correctness_is_p2(self) -> None:
        calc = SloCalculatorV1()
        for i in range(1, 6):
            calc.record(_obs(SliName.LedgerCommitLatencyP99, 9.0, i))
        summary = calc.evaluate(ExecutionMode.Paper)
        router = AlertRouterV1()
        alerts = router.route(summary, runId="run-1", accountGroup="g1")
        p2 = [a for a in alerts if a.severity == "P2"]
        assert p2, "延迟类违约应为 P2"

    def test_resolve_removes_alert(self) -> None:
        calc = SloCalculatorV1()
        calc.record(_obs(SliName.UnreconciledDifferences, 1.0, 1))
        summary = calc.evaluate(ExecutionMode.Paper)
        router = AlertRouterV1()
        alerts = router.route(summary, runId="run-1", accountGroup="g1")
        assert len(router.alerts()) == 1
        router.resolve(alerts[0])
        assert router.alerts() == ()

    def test_router_writes_metrics(self) -> None:
        reg = MetricsRegistry()
        calc = SloCalculatorV1()
        calc.record(_obs(SliName.UnreconciledDifferences, 1.0, 1))
        summary = calc.evaluate(ExecutionMode.Paper)
        router = AlertRouterV1(reg)
        router.route(summary, runId="run-1", accountGroup="g1")
        text = reg.render()
        assert 'vq_slo_alerts_total{severity="P1"} 1.0' in text
        assert "vq_slo_alerts_pending 1.0" in text

    def test_dedupe_key_stable(self) -> None:
        a = AlertRouteV1(
            runId="r", accountGroup="g", sli=SliName.OutboxMaxAge,
            severity="P2", message="m", remediationLink="x",
        )
        b = AlertRouteV1(
            runId="r", accountGroup="g", sli=SliName.OutboxMaxAge,
            severity="P2", message="m", remediationLink="x",
        )
        assert a.dedupeKey == b.dedupeKey
