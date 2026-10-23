"""P5-006 账户/策略/标的/金额白名单和硬上限。

对齐 TechSpec 13 阶段 5：
- 非批准组合无法发单；
- 账户级/单笔/单日上限不能由普通配置放宽（硬上限独立于配置）。

- `LiveWhitelistV1`：账户/策略/标的/金额批准组合；
- `HardLimitV1`：账户级/单笔/单日硬上限（不可由普通配置放宽）；
- `LiveOrderGuardV1`：发单前白名单 + 硬上限校验。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal


class GuardError(ValueError):
    """白名单或硬上限校验不满足时抛出。"""


@dataclass(frozen=True, slots=True)
class LiveWhitelistV1:
    """实盘批准组合清单。"""

    version: str
    approvedAccounts: frozenset[str]
    approvedStrategies: frozenset[str]
    approvedSymbols: frozenset[str]
    maxApprovedAmountPerOrder: str  # Decimal 字符串（批准的单笔金额上限）

    def __post_init__(self) -> None:
        if not self.version:
            raise GuardError("白名单版本不能为空")
        if not self.approvedAccounts or not self.approvedSymbols:
            raise GuardError("白名单必须至少包含账户与标的")

    def isApproved(self, *, accountId: str, strategyId: str, symbol: str) -> bool:
        """非批准组合无法发单。"""
        return (
            accountId in self.approvedAccounts
            and strategyId in self.approvedStrategies
            and symbol in self.approvedSymbols
        )


@dataclass(frozen=True, slots=True)
class HardLimitV1:
    """硬上限：账户级/单笔/单日；不可由普通配置放宽。"""

    accountId: str
    maxNotionalPerOrder: str  # Decimal 字符串
    maxDailyNotional: str
    maxOrderCountPerDay: int

    def __post_init__(self) -> None:
        if not self.accountId:
            raise GuardError("硬上限账户不能为空")
        if self.maxOrderCountPerDay <= 0:
            raise GuardError("单日订单数上限必须为正")
        if Decimal(self.maxNotionalPerOrder) <= 0 or Decimal(self.maxDailyNotional) <= 0:
            raise GuardError("金额上限必须为正")


@dataclass(frozen=True, slots=True)
class OrderGuardRequestV1:
    """发单前校验请求。"""

    clientOrderId: str
    accountId: str
    strategyId: str
    symbol: str
    notional: str  # Decimal 字符串（名义金额）
    price: str

    def __post_init__(self) -> None:
        if not self.clientOrderId or not self.accountId or not self.strategyId or not self.symbol:
            raise GuardError("发单校验标识字段不能为空")


class LiveOrderGuardV1:
    """发单前白名单 + 硬上限校验。"""

    def __init__(self, whitelist: LiveWhitelistV1, hardLimits: dict[str, HardLimitV1]) -> None:
        if whitelist is None:
            raise GuardError("白名单不能为空")
        self._whitelist = whitelist
        self._hardLimits = dict(hardLimits)
        self._dailyNotional: dict[str, Decimal] = {}
        self._dailyOrderCount: dict[str, int] = {}

    def validate(self, request: OrderGuardRequestV1) -> None:
        """校验批准组合 + 硬上限；任何一项不满足即拒绝发单。"""
        # 1. 白名单：非批准组合无法发单
        if not self._whitelist.isApproved(
            accountId=request.accountId,
            strategyId=request.strategyId,
            symbol=request.symbol,
        ):
            raise GuardError(
                f"非批准组合无法发单: account={request.accountId} "
                f"strategy={request.strategyId} symbol={request.symbol}"
            )
        # 2. 硬上限：账户必须有硬上限记录
        limit = self._hardLimits.get(request.accountId)
        if limit is None:
            raise GuardError(f"账户无硬上限记录: {request.accountId}")
        notional = Decimal(request.notional)
        # 3. 单笔上限
        if notional > Decimal(limit.maxNotionalPerOrder):
            raise GuardError(
                f"单笔上限拒绝: {request.notional} > {limit.maxNotionalPerOrder}"
            )
        # 4. 单日累计上限
        dailyNotional = self._dailyNotional.get(request.accountId, Decimal("0"))
        if dailyNotional + notional > Decimal(limit.maxDailyNotional):
            raise GuardError(
                f"单日金额上限拒绝: 累计 {dailyNotional + notional} > {limit.maxDailyNotional}"
            )
        # 5. 单日订单数上限
        dailyCount = self._dailyOrderCount.get(request.accountId, 0)
        if dailyCount + 1 > limit.maxOrderCountPerDay:
            raise GuardError(
                f"单日订单数上限拒绝: {dailyCount + 1} > {limit.maxOrderCountPerDay}"
            )

    def recordAccepted(self, request: OrderGuardRequestV1) -> None:
        """记录已通过校验的订单（累计单日用量）。"""
        self._dailyNotional[request.accountId] = self._dailyNotional.get(
            request.accountId, Decimal("0")
        ) + Decimal(request.notional)
        self._dailyOrderCount[request.accountId] = self._dailyOrderCount.get(
            request.accountId, 0
        ) + 1

    def dailyUsage(self, accountId: str) -> tuple[str, int]:
        """返回 (累计名义金额, 订单数)。"""
        return (
            str(self._dailyNotional.get(accountId, Decimal("0"))),
            self._dailyOrderCount.get(accountId, 0),
        )
