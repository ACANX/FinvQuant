"""账户与分账户的显式路由和隔离状态边界。"""

from __future__ import annotations

from decimal import Decimal

from veritasquant.core.Models import PascalAlias, StrictModel


class AccountRoutingError(ValueError):
    """账户范围缺失、未注册或跨账户操作时抛出。"""


class AccountScopeV1(StrictModel):
    """所有账户状态变更必须携带的不可省略路由范围。"""

    accountId: str = PascalAlias("AccountId", min_length=1)
    subaccountId: str | None = PascalAlias("SubaccountId", default=None, min_length=1)


class AccountStateRouterV1:
    """每个账户维护独立资源桶，禁止隐式跨账户汇总或调拨。"""

    def __init__(self) -> None:
        self._balances: dict[str, dict[str, Decimal]] = {}

    def registerAccount(self, scope: AccountScopeV1) -> None:
        if scope.accountId in self._balances:
            raise AccountRoutingError("账户已注册")
        self._balances[scope.accountId] = {}

    def applyDelta(self, scope: AccountScopeV1, unitId: str, amount: Decimal) -> Decimal:
        if not unitId or not isinstance(amount, Decimal):
            raise AccountRoutingError("状态变更必须包含计量单位和 Decimal 数量")
        balances = self._balances.get(scope.accountId)
        if balances is None:
            raise AccountRoutingError("账户未注册，拒绝状态变更")
        balances[unitId] = balances.get(unitId, Decimal("0")) + amount
        return balances[unitId]

    def balanceFor(self, scope: AccountScopeV1, unitId: str) -> Decimal:
        balances = self._balances.get(scope.accountId)
        if balances is None:
            raise AccountRoutingError("账户未注册")
        return balances.get(unitId, Decimal("0"))
