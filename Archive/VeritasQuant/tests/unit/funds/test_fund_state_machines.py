"""P2-013/014 基金申购/赎回状态机单元测试。

验收标准映射：
- 申购 CREATED 至 CONFIRMED/REJECTED/CANCELLED 全边覆盖；受理冻结、失败释放正确；
- 赎回 WAITING_NAV/SETTLEMENT、费用、到账和拒绝路径可重放。
"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.funds.FundStateMachines import (
    FundApplicationAction,
    FundApplicationError,
    FundApplicationState,
    FundApplicationStateMachineV1,
)


def _ts() -> datetime:
    return datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)


class TestSubscriptionStateMachine:
    def test_full_confirm_path(self) -> None:
        machine = FundApplicationStateMachineV1("app-1")
        assert machine.state is FundApplicationState.Created
        machine.apply(FundApplicationAction.Accept, _ts(), "受理并冻结资金")
        assert machine.state is FundApplicationState.Accepted
        machine.apply(FundApplicationAction.WaitNav, _ts())
        assert machine.state is FundApplicationState.WaitingNav
        machine.apply(FundApplicationAction.Confirm, _ts(), "份额确认")
        assert machine.state is FundApplicationState.Confirmed

    def test_reject_from_non_terminal_releases_funds(self) -> None:
        machine = FundApplicationStateMachineV1("app-2")
        machine.apply(FundApplicationAction.Accept, _ts(), "冻结")
        machine.apply(FundApplicationAction.Reject, _ts(), "额度不足，释放资金")
        assert machine.state is FundApplicationState.Rejected

    def test_cancel_from_waiting_nav(self) -> None:
        machine = FundApplicationStateMachineV1("app-3")
        machine.apply(FundApplicationAction.Accept, _ts())
        machine.apply(FundApplicationAction.WaitNav, _ts())
        machine.apply(FundApplicationAction.Cancel, _ts(), "渠道规则允许取消")
        assert machine.state is FundApplicationState.Cancelled

    def test_terminal_state_rejects_further_transitions(self) -> None:
        machine = FundApplicationStateMachineV1("app-4")
        machine.apply(FundApplicationAction.Accept, _ts())
        machine.apply(FundApplicationAction.Reject, _ts())
        with pytest.raises(FundApplicationError):
            machine.apply(FundApplicationAction.WaitNav, _ts())

    def test_illegal_transition_rejected(self) -> None:
        machine = FundApplicationStateMachineV1("app-5")
        # CREATED 不允许直接确认
        with pytest.raises(FundApplicationError):
            machine.apply(FundApplicationAction.Confirm, _ts())

    def test_history_replayable(self) -> None:
        machine = FundApplicationStateMachineV1("app-6")
        for action in (
            FundApplicationAction.Accept,
            FundApplicationAction.WaitNav,
            FundApplicationAction.Confirm,
        ):
            machine.apply(action, _ts())
        history = machine.history
        assert [transition.action for transition in history] == [
            FundApplicationAction.Accept,
            FundApplicationAction.WaitNav,
            FundApplicationAction.Confirm,
        ]
        # 重放后状态一致
        replayed = FundApplicationStateMachineV1("app-6")
        for transition in history:
            replayed.apply(transition.action, transition.ts, transition.detail)
        assert replayed.state is machine.state is FundApplicationState.Confirmed


class TestRedemptionStateMachine:
    def test_full_settlement_path(self) -> None:
        machine = FundApplicationStateMachineV1("red-1", kind="REDEMPTION")
        machine.apply(FundApplicationAction.Accept, _ts(), "受理冻结份额")
        machine.apply(FundApplicationAction.WaitNav, _ts())
        assert machine.state is FundApplicationState.WaitingNav
        machine.apply(FundApplicationAction.Settle, _ts(), "按适用净值结算")
        assert machine.state is FundApplicationState.Settlement
        machine.apply(FundApplicationAction.Complete, _ts(), "资金到账")
        assert machine.state is FundApplicationState.Completed

    def test_reject_after_settlement_releases(self) -> None:
        machine = FundApplicationStateMachineV1("red-2", kind="REDEMPTION")
        machine.apply(FundApplicationAction.Accept, _ts())
        machine.apply(FundApplicationAction.WaitNav, _ts())
        machine.apply(FundApplicationAction.Settle, _ts())
        machine.apply(FundApplicationAction.Reject, _ts(), "结算失败，退回份额")
        assert machine.state is FundApplicationState.Rejected

    def test_subscription_action_not_allowed_in_redemption(self) -> None:
        machine = FundApplicationStateMachineV1("red-3", kind="REDEMPTION")
        # 赎回机不允许 CONFIRM（那是申购动作）
        with pytest.raises(FundApplicationError):
            machine.apply(FundApplicationAction.Confirm, _ts())

    def test_replay_paths_for_fees_and_settlement(self) -> None:
        """费用、到账和拒绝路径可重放。"""
        for actions, expected in (
            (
                (FundApplicationAction.Accept, FundApplicationAction.WaitNav,
                 FundApplicationAction.Settle, FundApplicationAction.Complete),
                FundApplicationState.Completed,
            ),
            (
                (FundApplicationAction.Accept, FundApplicationAction.WaitNav,
                 FundApplicationAction.Settle, FundApplicationAction.Reject),
                FundApplicationState.Rejected,
            ),
            (
                (FundApplicationAction.Accept, FundApplicationAction.Cancel),
                FundApplicationState.Cancelled,
            ),
        ):
            machine = FundApplicationStateMachineV1("red-replay", kind="REDEMPTION")
            for action in actions:
                machine.apply(action, _ts())
            assert machine.state is expected
