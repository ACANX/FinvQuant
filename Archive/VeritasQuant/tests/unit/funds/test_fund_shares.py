"""P2-015 基金份额 journal 单元测试。"""

from __future__ import annotations

from decimal import Decimal

import pytest

from veritasquant.funds.FundShares import (
    DistributionEventV1,
    FundShareError,
    FundShareLedgerV1,
    ShareConfirmationV1,
    ShareRedemptionV1,
)


def _confirm(
    confirmationId: str = "cf-1",
    shares: str = "1000",
    nav: str = "1.2345",
    fee: str = "0",
) -> ShareConfirmationV1:
    return ShareConfirmationV1(
        confirmationId=confirmationId,
        applicationId=f"app-{confirmationId}",
        fundSymbol="FUND-001",
        accountId="a1",
        shares=Decimal(shares),
        unitNav=Decimal(nav),
        currency="CNY",
        fee=Decimal(fee),
    )


class TestFundShareLedger:
    def test_confirmation_adds_shares_with_precision(self) -> None:
        ledger = FundShareLedgerV1(sharePrecision=2)
        # ROUND_HALF_EVEN：1000.005 -> 1000.00（0 为偶数保持），1000.015 -> 1000.02
        position = ledger.confirm(_confirm(shares="1000.005"))
        assert position.shares == Decimal("1000.00")
        position2 = ledger.confirm(_confirm(confirmationId="cf-even", shares="1000.015"))
        assert position2.shares == Decimal("2000.02")  # 累计：1000.00 + 1000.02
        assert position.costAmount == Decimal("1234.50")  # 1000.00 * 1.2345

    def test_duplicate_confirmation_is_idempotent(self) -> None:
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(confirmationId="cf-x", shares="100"))
        position = ledger.confirm(_confirm(confirmationId="cf-x", shares="100"))
        assert position.shares == Decimal("100")  # 重复确认不重复份额
        assert len(ledger.journal) == 1

    def test_redemption_requires_sufficient_shares(self) -> None:
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(shares="100"))
        with pytest.raises(FundShareError):
            ledger.redeem(
                ShareRedemptionV1("red-1", "FUND-001", "a1", Decimal("100.01"), "CNY")
            )

    def test_redemption_reduces_shares(self) -> None:
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(shares="100"))
        position = ledger.redeem(
            ShareRedemptionV1("red-1", "FUND-001", "a1", Decimal("40"), "CNY")
        )
        assert position.shares == Decimal("60")

    def test_cash_distribution_records_without_changing_shares(self) -> None:
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(shares="1000"))
        position = ledger.distribute(
            DistributionEventV1("dist-1", "FUND-001", "a1", Decimal("1000"), Decimal("0.1"), "CNY", False)
        )
        assert position.shares == Decimal("1000")  # 现金分红不改份额
        assert ledger.journal[-1]["type"] == "DISTRIBUTION_CASH"
        assert ledger.journal[-1]["cashAmount"] == Decimal("100.00")

    def test_reinvest_distribution_increases_shares(self) -> None:
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(shares="1000", nav="1.00"))
        position = ledger.distribute(
            DistributionEventV1("dist-2", "FUND-001", "a1", Decimal("1000"), Decimal("0.1"), "CNY", True)
        )
        assert position.shares > Decimal("1000")  # 再投资增加份额

    def test_invalid_inputs_rejected(self) -> None:
        ledger = FundShareLedgerV1()
        with pytest.raises(FundShareError):
            ledger.confirm(_confirm(shares="0"))
        with pytest.raises(FundShareError):
            ledger.confirm(_confirm(nav="0"))
        with pytest.raises(FundShareError):
            ledger.redeem(ShareRedemptionV1("r", "FUND-001", "a1", Decimal("0"), "CNY"))

    def test_account_isolation(self) -> None:
        """多账户份额互不串扰。"""
        ledger = FundShareLedgerV1()
        ledger.confirm(_confirm(confirmationId="cf-a", shares="100"))
        ledger.confirm(
            ShareConfirmationV1("cf-b", "app-b", "FUND-001", "a2", Decimal("50"), Decimal("1"), "CNY")
        )
        assert ledger.position("a1", "FUND-001").shares == Decimal("100")
        assert ledger.position("a2", "FUND-001").shares == Decimal("50")
