"""生产领域视图提供者（DomainViewProviders）单元测试。"""

from __future__ import annotations

import pytest

from veritasquant.apps.server.DomainRoutes import ResourceNotFound
from veritasquant.apps.server.DomainViewProviders import (
    ServerAccountViewV1,
    ServerFundViewV1,
    ServerInstrumentViewV1,
    ServerStrategyViewV1,
)


class TestServerAccountView:
    def test_accounts_empty_by_default(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.delenv("VQ_ACCOUNTS", raising=False)
        view = ServerAccountViewV1()
        assert view.accounts() == ()

    def test_accounts_from_environment(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("VQ_ACCOUNTS", "acc-1, acc-2 ,")
        view = ServerAccountViewV1()
        ids = [item["account_id"] for item in view.accounts()]
        assert ids == ["acc-1", "acc-2"]

    def test_account_detail_returns_metadata(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("VQ_ACCOUNTS", "acc-1")
        monkeypatch.setenv("VQ_ENVIRONMENT", "SIMULATION")
        view = ServerAccountViewV1()
        detail = view.account("acc-1", "run-9")
        assert detail["account_id"] == "acc-1"
        assert detail["execution_mode"] == "SIMULATION"
        assert detail["run_id"] == "run-9"

    def test_account_unknown_raises_not_found(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("VQ_ACCOUNTS", "acc-1")
        view = ServerAccountViewV1()
        with pytest.raises(ResourceNotFound, match="账户不存在"):
            view.account("ghost", "")

    def test_ledger_cashflows_shares_empty(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("VQ_ACCOUNTS", "acc-1")
        view = ServerAccountViewV1()
        assert view.ledgerEntries("acc-1", "") == ()
        assert view.cashFlows("acc-1", "") == ()
        assert view.sharePositions("acc-1", "") == ()
        assert view.analysis("acc-1", "")["principal"] == "0.00"

    def test_illegal_environment_rejected(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("VQ_ACCOUNTS", "acc-1")
        monkeypatch.setenv("VQ_ENVIRONMENT", "LIVE")
        view = ServerAccountViewV1()
        with pytest.raises(ValueError, match="非法运行环境"):
            view.accounts()


class TestServerCatalogViews:
    def test_strategies_instruments_funds_empty(self) -> None:
        assert ServerStrategyViewV1().strategies() == ()
        assert ServerInstrumentViewV1().instruments() == ()
        assert ServerFundViewV1().funds() == ()
