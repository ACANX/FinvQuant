from __future__ import annotations

from decimal import Decimal

import pytest
from pydantic import ValidationError

from veritasquant.accounts.Routing import AccountRoutingError, AccountScopeV1, AccountStateRouterV1


def test_account_scope_requires_explicit_account_id() -> None:
    with pytest.raises(ValidationError, match="AccountId"):
        AccountScopeV1.model_validate({})


def test_account_router_keeps_state_isolated() -> None:
    router = AccountStateRouterV1()
    first = AccountScopeV1(AccountId="account-1")
    second = AccountScopeV1(AccountId="account-2", SubaccountId="strategy-1")
    router.registerAccount(first)
    router.registerAccount(second)
    router.applyDelta(first, "CNY", Decimal("100"))
    router.applyDelta(second, "CNY", Decimal("50"))
    assert router.balanceFor(first, "CNY") == Decimal("100")
    assert router.balanceFor(second, "CNY") == Decimal("50")


def test_account_router_rejects_unregistered_state_change() -> None:
    with pytest.raises(AccountRoutingError, match="未注册"):
        AccountStateRouterV1().applyDelta(AccountScopeV1(AccountId="account-1"), "CNY", Decimal("1"))
