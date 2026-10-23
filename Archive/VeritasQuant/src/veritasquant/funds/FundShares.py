"""P2-015 基金份额 journal：确认、赎回、分红与费用。

验收标准：
- 份额精度和舍入正确（ROUND_HALF_EVEN，按基金元数据份额精度）；
- 重复确认不重复份额（confirmationId 幂等）；
- 现金/份额逐单位平衡（复式语义，借记=贷记）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import ROUND_HALF_EVEN, Decimal
from enum import StrEnum


class FundShareError(ValueError):
    """份额 journal 不满足精度、幂等或平衡契约。"""


class ShareJournalType(StrEnum):
    SubscriptionConfirm = "SUBSCRIPTION_CONFIRM"
    Redemption = "REDEMPTION"
    DistributionReinvest = "DISTRIBUTION_REINVEST"
    DistributionCash = "DISTRIBUTION_CASH"
    Fee = "FEE"


@dataclass(frozen=True, slots=True)
class ShareConfirmationV1:
    """一次份额确认（对应 FundShareConfirmedEvent）。"""

    confirmationId: str
    applicationId: str
    fundSymbol: str
    accountId: str
    shares: Decimal
    unitNav: Decimal
    currency: str
    fee: Decimal = Decimal("0")
    ts: datetime | None = None


@dataclass(frozen=True, slots=True)
class ShareRedemptionV1:
    """一次份额赎回申请。"""

    applicationId: str
    fundSymbol: str
    accountId: str
    shares: Decimal
    currency: str
    ts: datetime | None = None


@dataclass(frozen=True, slots=True)
class DistributionEventV1:
    """一次基金分红。"""

    distributionId: str
    fundSymbol: str
    accountId: str
    shares: Decimal
    perShareAmount: Decimal
    currency: str
    reinvest: bool
    ts: datetime | None = None


@dataclass(frozen=True, slots=True)
class SharePositionV1:
    """某账户某基金的份额持仓快照。"""

    fundSymbol: str
    accountId: str
    shares: Decimal
    costAmount: Decimal
    currency: str


class FundShareLedgerV1:
    """份额账本：只读持仓由不可变 journal 序列派生。"""

    def __init__(self, sharePrecision: int = 2) -> None:
        """sharePrecision：份额小数位（基金元数据版本固定）。"""
        if sharePrecision < 0:
            raise FundShareError("份额精度必须非负")
        self._sharePrecision = sharePrecision
        self._positions: dict[tuple[str, str], SharePositionV1] = {}
        self._confirmedIds: set[str] = set()
        self._journal: list[dict[str, object]] = []

    @property
    def journal(self) -> tuple[dict[str, object], ...]:
        """不可变 journal 历史（可重放）。"""
        return tuple(self._journal)

    def position(self, accountId: str, fundSymbol: str) -> SharePositionV1:
        """返回持仓；无记录返回零持仓。"""
        return self._positions.get(
            (accountId, fundSymbol),
            SharePositionV1(fundSymbol, accountId, Decimal("0"), Decimal("0"), ""),
        )

    def confirm(self, confirmation: ShareConfirmationV1) -> SharePositionV1:
        """份额确认：同 confirmationId 幂等；确认后按净值入成本。"""
        if confirmation.confirmationId in self._confirmedIds:
            return self.position(confirmation.accountId, confirmation.fundSymbol)
        shares = self._roundShares(confirmation.shares)
        if shares <= 0:
            raise FundShareError("确认份额必须为正")
        if confirmation.unitNav <= 0:
            raise FundShareError("确认净值必须为正")
        current = self.position(confirmation.accountId, confirmation.fundSymbol)
        cost = (shares * confirmation.unitNav + confirmation.fee).quantize(
            Decimal("0.01"), rounding=ROUND_HALF_EVEN
        )
        updated = SharePositionV1(
            fundSymbol=confirmation.fundSymbol,
            accountId=confirmation.accountId,
            shares=self._roundShares(current.shares + shares),
            costAmount=current.costAmount + cost,
            currency=confirmation.currency,
        )
        self._positions[(confirmation.accountId, confirmation.fundSymbol)] = updated
        self._confirmedIds.add(confirmation.confirmationId)
        self._journal.append(
            {
                "type": ShareJournalType.SubscriptionConfirm.value,
                "confirmationId": confirmation.confirmationId,
                "fundSymbol": confirmation.fundSymbol,
                "accountId": confirmation.accountId,
                "shares": shares,
                "cost": cost,
                "currency": confirmation.currency,
            }
        )
        return updated

    def redeem(self, redemption: ShareRedemptionV1) -> SharePositionV1:
        """份额赎回：校验足够份额后扣减。"""
        shares = self._roundShares(redemption.shares)
        if shares <= 0:
            raise FundShareError("赎回份额必须为正")
        current = self.position(redemption.accountId, redemption.fundSymbol)
        if current.shares < shares:
            raise FundShareError(
                f"赎回份额 {shares} 超过持仓 {current.shares}"
            )
        updated = SharePositionV1(
            fundSymbol=redemption.fundSymbol,
            accountId=redemption.accountId,
            shares=self._roundShares(current.shares - shares),
            costAmount=current.costAmount,
            currency=redemption.currency,
        )
        self._positions[(redemption.accountId, redemption.fundSymbol)] = updated
        self._journal.append(
            {
                "type": ShareJournalType.Redemption.value,
                "applicationId": redemption.applicationId,
                "fundSymbol": redemption.fundSymbol,
                "accountId": redemption.accountId,
                "shares": shares,
            }
        )
        return updated

    def distribute(self, distribution: DistributionEventV1) -> SharePositionV1:
        """分红：再投资按净值折算份额；现金分红只记录（现金入账在账本层）。"""
        if distribution.shares <= 0 or distribution.perShareAmount < 0:
            raise FundShareError("分红份额必须为正且每份金额非负")
        cashAmount = (distribution.shares * distribution.perShareAmount).quantize(
            Decimal("0.01"), rounding=ROUND_HALF_EVEN
        )
        current = self.position(distribution.accountId, distribution.fundSymbol)
        if distribution.reinvest:
            if distribution.perShareAmount <= 0:
                raise FundShareError("再投资分红必须携带正每份金额")
            # 按单位净值折算份额（保守：净值须为正）
            nav = current.costAmount / current.shares if current.shares > 0 else Decimal("1")
            if nav <= 0:
                raise FundShareError("再投资需要正净值")
            additionalShares = self._roundShares(cashAmount / nav)
            updated = SharePositionV1(
                fundSymbol=distribution.fundSymbol,
                accountId=distribution.accountId,
                shares=self._roundShares(current.shares + additionalShares),
                costAmount=current.costAmount,
                currency=distribution.currency,
            )
            self._positions[(distribution.accountId, distribution.fundSymbol)] = updated
            self._journal.append(
                {
                    "type": ShareJournalType.DistributionReinvest.value,
                    "distributionId": distribution.distributionId,
                    "shares": additionalShares,
                }
            )
            return updated
        self._journal.append(
            {
                "type": ShareJournalType.DistributionCash.value,
                "distributionId": distribution.distributionId,
                "cashAmount": cashAmount,
                "currency": distribution.currency,
            }
        )
        return current

    def _roundShares(self, value: Decimal) -> Decimal:
        """按基金份额精度 ROUND_HALF_EVEN 舍入。"""
        quant = Decimal("1").scaleb(-self._sharePrecision)
        return value.quantize(quant, rounding=ROUND_HALF_EVEN)
