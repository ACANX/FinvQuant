"""P2-012 基金事件载荷（EventPayloadV1 子类）。

启用基金定投回测前必须注册：`FundNavPublishedEvent`、`InvestmentPlanDueEvent`、
`FundSubscriptionEvent`、`FundRedemptionEvent` 和 `FundShareConfirmedEvent`
（TechSpec 4.1）。核心事件禁止使用无约束 dict 作为最终载荷。
"""

from __future__ import annotations

from datetime import date, datetime
from decimal import Decimal

from pydantic import field_validator

from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class FundEventError(ValueError):
    """基金事件载荷契约失败。"""


class FundNavPublishedEventV1(EventPayloadV1):
    """净值发布事件：发布当时可用的基金净值。"""

    symbol: str = PascalAlias("Symbol", min_length=1)
    navDate: date = PascalAlias("NavDate")
    unitNav: Decimal = PascalAlias("UnitNav")
    accumulatedNav: Decimal | None = PascalAlias("AccumulatedNav", default=None)
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    navAvailabilityPolicy: str = PascalAlias("NavAvailabilityPolicy", min_length=1)
    fundMetadataVersion: str = PascalAlias("FundMetadataVersion", min_length=1)

    @field_validator("unitNav", "accumulatedNav", mode="before")
    @classmethod
    def parseNav(cls, value: object) -> Decimal | None:
        if value is None:
            return None
        if not isinstance(value, Decimal):
            raise FundEventError("净值必须为 Decimal，禁止 float")
        if value <= 0:
            raise FundEventError("净值必须为正数")
        return value


class InvestmentPlanDueEventV1(EventPayloadV1):
    """计划到期事件：计划时间确定性转 UTC，历史触发不依赖服务器当前时间。"""

    planId: str = PascalAlias("PlanId", min_length=1)
    planVersion: str = PascalAlias("PlanVersion", min_length=1)
    fundSymbol: str = PascalAlias("FundSymbol", min_length=1)
    dueDate: date = PascalAlias("DueDate")
    scheduledUtcTs: datetime = PascalAlias("ScheduledUtcTs")
    amountRuleVersion: str = PascalAlias("AmountRuleVersion", min_length=1)

    @field_validator("scheduledUtcTs", mode="before")
    @classmethod
    def parseTs(cls, value: object) -> datetime:
        if not isinstance(value, datetime):
            raise FundEventError("计划到期时间必须是 UTC datetime")
        return validateUtcTimestamp(value, TsPrecision.Millisecond)


class FundSubscriptionEventV1(EventPayloadV1):
    """基金申购申请：受理时冻结资金，只有份额确认后才增加持仓。"""

    applicationId: str = PascalAlias("ApplicationId", min_length=1)
    planId: str = PascalAlias("PlanId", min_length=1)
    fundSymbol: str = PascalAlias("FundSymbol", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    amount: Decimal = PascalAlias("Amount")
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    feeRateVersion: str = PascalAlias("FeeRateVersion", min_length=1)
    idempotencyKey: str = PascalAlias("IdempotencyKey", min_length=1)

    @field_validator("amount", mode="before")
    @classmethod
    def parseAmount(cls, value: object) -> Decimal:
        if not isinstance(value, Decimal):
            raise FundEventError("申购金额必须为 Decimal")
        if value <= 0:
            raise FundEventError("申购金额必须为正数")
        return value


class FundRedemptionEventV1(EventPayloadV1):
    """基金赎回申请：按份额赎回，结算依赖适用净值。"""

    applicationId: str = PascalAlias("ApplicationId", min_length=1)
    fundSymbol: str = PascalAlias("FundSymbol", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    shares: Decimal = PascalAlias("Shares")
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")
    feeRateVersion: str = PascalAlias("FeeRateVersion", min_length=1)

    @field_validator("shares", mode="before")
    @classmethod
    def parseShares(cls, value: object) -> Decimal:
        if not isinstance(value, Decimal):
            raise FundEventError("赎回份额必须为 Decimal")
        if value <= 0:
            raise FundEventError("赎回份额必须为正数")
        return value


class FundShareConfirmedEventV1(EventPayloadV1):
    """份额确认事件：账户内唯一，重复确认不得重复记账。"""

    confirmationId: str = PascalAlias("ConfirmationId", min_length=1)
    applicationId: str = PascalAlias("ApplicationId", min_length=1)
    fundSymbol: str = PascalAlias("FundSymbol", min_length=1)
    accountId: str = PascalAlias("AccountId", min_length=1)
    shares: Decimal = PascalAlias("Shares")
    unitNav: Decimal = PascalAlias("UnitNav")
    currency: str = PascalAlias("Currency", pattern=r"^[A-Z]{3}$")

    @field_validator("shares", "unitNav", mode="before")
    @classmethod
    def parseDecimal(cls, value: object) -> Decimal:
        if not isinstance(value, Decimal):
            raise FundEventError("份额与净值必须为 Decimal")
        if value <= 0:
            raise FundEventError("份额与净值必须为正数")
        return value
