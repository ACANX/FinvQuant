"""P5-006 白名单和硬上限测试。"""

from __future__ import annotations

import pytest

from veritasquant.security.LiveOrderGuard import (
    GuardError,
    HardLimitV1,
    LiveOrderGuardV1,
    LiveWhitelistV1,
    OrderGuardRequestV1,
)


def _whitelist(**overrides: object) -> LiveWhitelistV1:
    values: dict[str, object] = {
        "version": "1.0",
        "approvedAccounts": frozenset({"acc-live-001"}),
        "approvedStrategies": frozenset({"strat-a", "strat-b"}),
        "approvedSymbols": frozenset({"518880"}),
        "maxApprovedAmountPerOrder": "100000",
    }
    values.update(overrides)
    return LiveWhitelistV1(**values)


def _limit(**overrides: object) -> HardLimitV1:
    values: dict[str, object] = {
        "accountId": "acc-live-001",
        "maxNotionalPerOrder": "50000",
        "maxDailyNotional": "200000",
        "maxOrderCountPerDay": 10,
    }
    values.update(overrides)
    return HardLimitV1(**values)


def _request(**overrides: object) -> OrderGuardRequestV1:
    values: dict[str, object] = {
        "clientOrderId": "co-001",
        "accountId": "acc-live-001",
        "strategyId": "strat-a",
        "symbol": "518880",
        "notional": "10000",
        "price": "5.0000",
    }
    values.update(overrides)
    return OrderGuardRequestV1(**values)


class TestLiveWhitelist:
    def test_approved_combination(self) -> None:
        whitelist = _whitelist()
        assert whitelist.isApproved(
            accountId="acc-live-001", strategyId="strat-a", symbol="518880"
        ) is True

    def test_unapproved_account_rejected(self) -> None:
        whitelist = _whitelist()
        assert whitelist.isApproved(
            accountId="acc-paper-001", strategyId="strat-a", symbol="518880"
        ) is False

    def test_unapproved_symbol_rejected(self) -> None:
        whitelist = _whitelist()
        assert whitelist.isApproved(
            accountId="acc-live-001", strategyId="strat-a", symbol="600000"
        ) is False


class TestLiveOrderGuard:
    def _guard(self, **overrides: object) -> LiveOrderGuardV1:
        values: dict[str, object] = {
            "whitelist": _whitelist(),
            "hardLimits": {"acc-live-001": _limit()},
        }
        values.update(overrides)
        return LiveOrderGuardV1(**values)

    def test_valid_order_passes(self) -> None:
        guard = self._guard()
        guard.validate(_request())  # 不抛
        guard.recordAccepted(_request())
        usage, count = guard.dailyUsage("acc-live-001")
        assert usage == "10000"
        assert count == 1

    def test_unapproved_combination_rejected(self) -> None:
        """非批准组合无法发单。"""
        guard = self._guard()
        with pytest.raises(GuardError, match="非批准组合"):
            guard.validate(_request(accountId="acc-paper-001"))

    def test_per_order_limit_rejected(self) -> None:
        guard = self._guard()
        with pytest.raises(GuardError, match="单笔上限"):
            guard.validate(_request(notional="60000"))

    def test_daily_notional_limit_rejected(self) -> None:
        guard = self._guard()
        # 6 笔 30000 累计 180000（未超 200000）
        for _ in range(6):
            guard.validate(_request(notional="30000"))
            guard.recordAccepted(_request(notional="30000"))
        # 第 7 笔：180000 + 30000 = 210000 > 200000
        with pytest.raises(GuardError, match="单日金额上限"):
            guard.validate(_request(notional="30000"))

    def test_daily_order_count_limit_rejected(self) -> None:
        guard = self._guard(hardLimits={"acc-live-001": _limit(maxOrderCountPerDay=2)})
        guard.validate(_request())
        guard.recordAccepted(_request())
        guard.validate(_request())
        guard.recordAccepted(_request())
        with pytest.raises(GuardError, match="单日订单数上限"):
            guard.validate(_request())

    def test_missing_hard_limit_rejected(self) -> None:
        """账户无硬上限记录：拒绝发单（硬上限不可绕过）。"""
        guard = self._guard(hardLimits={})
        with pytest.raises(GuardError, match="无硬上限"):
            guard.validate(_request())

    def test_hard_limit_requires_positive_amounts(self) -> None:
        with pytest.raises(GuardError):
            _limit(maxNotionalPerOrder="0")
        with pytest.raises(GuardError):
            _limit(maxOrderCountPerDay=0)
