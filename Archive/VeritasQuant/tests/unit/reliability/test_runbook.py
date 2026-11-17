"""P5-013 启动/停机/断连/对账/账本异常/密钥泄漏 Runbook 测试。"""

from __future__ import annotations

import pytest

from veritasquant.reliability.Runbook import (
    EscalationContactV1,
    RunbookKind,
    RunbookRegistryV1,
    RunbookSeverity,
    RunbookStepV1,
    RunbookV1,
    buildStandardRunbooks,
)


def _contact(role: str = "OnCall-SRE") -> EscalationContactV1:
    return EscalationContactV1(role=role, name="SRE-值班", channel="电话", priority=1)


def _runbook(kind: RunbookKind = RunbookKind.Startup, **kw) -> RunbookV1:
    defaults = dict(
        kind=kind,
        title="测试 Runbook",
        trigger="测试触发",
        requiredPermissions=("Operator",),
        steps=(RunbookStepV1(1, "执行动作", "期望结果"),),
        verification="验证完成",
        rollback="回退方案",
        evidence="证据记录",
        escalationContacts=(_contact(),),
    )
    defaults.update(kw)
    return RunbookV1(**defaults)


class TestRunbookStep:
    def test_step_requires_order_positive(self) -> None:
        with pytest.raises(ValueError, match="序号"):
            RunbookStepV1(0, "动作", "期望")

    def test_step_requires_content(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            RunbookStepV1(1, "", "期望")


class TestEscalationContact:
    def test_contact_requires_fields(self) -> None:
        with pytest.raises(ValueError, match="不能为空"):
            EscalationContactV1(role="", name="x", channel="电话", priority=1)

    def test_contact_requires_priority(self) -> None:
        with pytest.raises(ValueError, match="优先级"):
            EscalationContactV1(role="r", name="x", channel="电话", priority=0)


class TestRunbook:
    def test_runbook_complete(self) -> None:
        rb = _runbook()
        assert rb.complete()

    def test_runbook_requires_trigger(self) -> None:
        with pytest.raises(ValueError, match="触发"):
            _runbook(trigger="")

    def test_runbook_requires_steps(self) -> None:
        with pytest.raises(ValueError, match="步骤"):
            _runbook(steps=())

    def test_runbook_requires_verification_rollback_evidence(self) -> None:
        with pytest.raises(ValueError, match="验证"):
            _runbook(verification="")

    def test_runbook_requires_escalation(self) -> None:
        with pytest.raises(ValueError, match="升级联系人"):
            _runbook(escalationContacts=())

    def test_runbook_requires_permissions(self) -> None:
        with pytest.raises(ValueError, match="权限"):
            _runbook(requiredPermissions=())

    def test_ordered_steps_valid(self) -> None:
        rb = _runbook(
            steps=(
                RunbookStepV1(1, "第一步", "结果1"),
                RunbookStepV1(2, "第二步", "结果2"),
                RunbookStepV1(3, "第三步", "结果3"),
            )
        )
        assert [s.order for s in rb.orderedSteps()] == [1, 2, 3]

    def test_ordered_steps_rejects_gap(self) -> None:
        rb = _runbook(
            steps=(
                RunbookStepV1(1, "第一步", "结果1"),
                RunbookStepV1(3, "第三步", "结果3"),
            )
        )
        with pytest.raises(ValueError, match="连续递增"):
            rb.orderedSteps()


class TestRunbookRegistry:
    def test_register_incomplete_rejected(self) -> None:
        registry = RunbookRegistryV1()
        # 不完整 Runbook（缺升级联系人）在构造/登记时即被拒绝
        with pytest.raises(ValueError, match="升级联系人|不完整"):
            rb = _runbook(escalationContacts=())
            registry.register(rb)

    def test_register_duplicate_rejected(self) -> None:
        registry = RunbookRegistryV1()
        registry.register(_runbook())
        with pytest.raises(ValueError, match="已登记"):
            registry.register(_runbook())

    def test_coverage_incomplete_initially(self) -> None:
        registry = RunbookRegistryV1()
        assert not registry.coverageComplete()
        assert len(registry.missingKinds()) == 6

    def test_standard_runbooks_cover_all_six(self) -> None:
        registry = RunbookRegistryV1()
        for rb in buildStandardRunbooks().values():
            registry.register(rb)
        assert registry.coverageComplete()
        assert registry.missingKinds() == ()

    def test_all_six_kinds_present(self) -> None:
        runbooks = buildStandardRunbooks()
        assert set(runbooks.keys()) == {
            RunbookKind.Startup,
            RunbookKind.Shutdown,
            RunbookKind.Disconnect,
            RunbookKind.Reconciliation,
            RunbookKind.LedgerAnomaly,
            RunbookKind.SecretLeak,
        }

    def test_get_by_kind(self) -> None:
        registry = RunbookRegistryV1()
        registry.register(_runbook(RunbookKind.Shutdown))
        assert registry.get(RunbookKind.Shutdown) is not None
        assert registry.get(RunbookKind.Startup) is None

    def test_all_returns_ordered(self) -> None:
        registry = RunbookRegistryV1()
        for rb in buildStandardRunbooks().values():
            registry.register(rb)
        kinds = [rb.kind for rb in registry.all()]
        assert kinds == list(RunbookKind)


class TestStandardRunbooks:
    def test_startup_runbook_content(self) -> None:
        from veritasquant.reliability.Runbook import buildStartupRunbook

        rb = buildStartupRunbook()
        assert rb.kind is RunbookKind.Startup
        assert rb.complete()
        assert any("trading-readiness" in s.action for s in rb.steps)

    def test_shutdown_runbook_stops_orders_first(self) -> None:
        from veritasquant.reliability.Runbook import buildShutdownRunbook

        rb = buildShutdownRunbook()
        assert rb.kind is RunbookKind.Shutdown
        assert "禁止新订单" in rb.steps[0].action or "停止" in rb.steps[0].action

    def test_disconnect_runbook_s0(self) -> None:
        from veritasquant.reliability.Runbook import buildDisconnectRunbook

        rb = buildDisconnectRunbook()
        assert rb.severity is RunbookSeverity.S0
        assert any("不盲目重发" in s.action or "查询" in s.action for s in rb.steps)

    def test_reconciliation_runbook_requires_zero_diff(self) -> None:
        from veritasquant.reliability.Runbook import buildReconciliationRunbook

        rb = buildReconciliationRunbook()
        assert "差异为 0" in rb.verification

    def test_ledger_anomaly_runbook_forbids_manual_edit(self) -> None:
        from veritasquant.reliability.Runbook import buildLedgerAnomalyRunbook

        rb = buildLedgerAnomalyRunbook()
        assert any("禁止手工改账" in s.action or "重放" in s.action for s in rb.steps)
        assert rb.severity is RunbookSeverity.S0

    def test_secret_leak_runbook_revokes_first(self) -> None:
        from veritasquant.reliability.Runbook import buildSecretLeakRunbook

        rb = buildSecretLeakRunbook()
        assert rb.severity is RunbookSeverity.S0
        assert any("撤销" in s.action for s in rb.steps)
        assert any("轮换" in s.action for s in rb.steps)

    def test_every_runbook_has_escalation_and_evidence(self) -> None:
        for rb in buildStandardRunbooks().values():
            assert rb.complete()
            assert len(rb.escalationContacts) >= 1
            assert bool(rb.evidence)
            assert bool(rb.rollback)
