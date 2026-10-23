"""P2-028 账户、策略、数据、回测、基金计划和报告 API 测试。"""

from __future__ import annotations


from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import ApiDependencies, createApp
from veritasquant.apps.server.DomainRoutes import (
    AccountViewProvider,
    DomainApis,
    FundViewProvider,
    InstrumentViewProvider,
    ResourceNotFound,
    StrategyViewProvider,
)
from veritasquant.application.ApiApp import ApiVersionProvider
from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.BacktestService import BacktestApplicationServiceV1


class _StubVersionProvider(ApiVersionProvider):
    @property
    def apiVersion(self) -> str:
        return "v1"

    @property
    def catalogVersion(self) -> str:
        return "9.9.9"


class _StubAccounts(AccountViewProvider):
    def accounts(self) -> tuple[dict, ...]:
        return ({"account_id": "acc-1", "execution_mode": "PAPER", "run_id": None},)

    def account(self, accountId: str, runId: str) -> dict:
        if accountId != "acc-1":
            raise ResourceNotFound(f"账户不存在: {accountId}")
        return {
            "account_id": accountId,
            "run_id": runId,
            "execution_mode": "PAPER",
            "cash": "10000.00",
        }

    def ledgerEntries(self, accountId: str, runId: str) -> tuple[dict, ...]:
        if accountId != "acc-1":
            raise ResourceNotFound(f"账户不存在: {accountId}")
        return (
            {"journal_id": "j-1", "account_id": accountId, "amount": "100.00"},
            {"journal_id": "j-2", "account_id": accountId, "amount": "-50.00"},
        )

    def cashFlows(self, accountId: str, runId: str) -> tuple[dict, ...]:
        if accountId != "acc-1":
            raise ResourceNotFound(f"账户不存在: {accountId}")
        return ({"date": "2026-01-01", "amount": "-1000.00"},)

    def sharePositions(self, accountId: str, runId: str) -> tuple[dict, ...]:
        if accountId != "acc-1":
            raise ResourceNotFound(f"账户不存在: {accountId}")
        return ({"fund_symbol": "FUND-A", "quantity": "100.5"},)

    def analysis(self, accountId: str, runId: str) -> dict:
        if accountId != "acc-1":
            raise ResourceNotFound(f"账户不存在: {accountId}")
        return {"twr": "0.0123", "xirr": "0.0456", "principal": "10000.00"}


class _StubStrategies(StrategyViewProvider):
    def strategies(self) -> tuple[dict, ...]:
        return (
            {"strategy_id": "strat-1", "version": "1.0.0", "type": "FUND_SMART_INVEST"},
        )


class _StubInstruments(InstrumentViewProvider):
    def instruments(self) -> tuple[dict, ...]:
        return ({"symbol": "FUND-A", "kind": "OTC_FUND"},)


class _StubFunds(FundViewProvider):
    def funds(self) -> tuple[dict, ...]:
        return ({"fund_symbol": "FUND-A", "status": "OPEN"},)


def _client(**depsOverrides) -> TestClient:
    catalog = ApiErrorCatalog.loadPackaged()
    backtest = BacktestApplicationServiceV1()
    domainApis = DomainApis(
        catalog=catalog,
        backtest=backtest,
        accounts=_StubAccounts(),
        strategies=_StubStrategies(),
        instruments=_StubInstruments(),
        funds=_StubFunds(),
    )
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        domainApis=domainApis,
        **depsOverrides,
    )
    return TestClient(createApp(deps))


class TestAccountApi:
    def test_accounts_list(self) -> None:
        """GET /api/v1/accounts 返回账户列表。"""
        client = _client()
        response = client.get("/api/v1/accounts")
        assert response.status_code == 200
        payload = response.json()
        assert payload["code"] == 0
        assert payload["data"]["accounts"] == [
            {"account_id": "acc-1", "execution_mode": "PAPER", "run_id": None}
        ]

    def test_account_requires_explicit_ids(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1?run_id=run-1")
        assert response.status_code == 200
        payload = response.json()
        assert payload["code"] == 0
        assert payload["data"]["account_id"] == "acc-1"
        assert payload["data"]["run_id"] == "run-1"

    def test_missing_run_id_is_validation_error(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1")
        assert response.status_code == 400  # 缺失查询参数 -> VALIDATION_ERROR(1001)

    def test_unknown_account_404(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/nope?run_id=run-1")
        assert response.status_code == 404
        assert response.json()["code"] == 1002


class TestStrategyApi:
    def test_list_strategies(self) -> None:
        client = _client()
        response = client.get("/api/v1/strategies")
        assert response.status_code == 200
        payload = response.json()
        assert payload["data"]["strategies"][0]["strategy_id"] == "strat-1"


class TestInstrumentApi:
    def test_list_instruments(self) -> None:
        client = _client()
        response = client.get("/api/v1/instruments")
        assert response.status_code == 200
        assert response.json()["data"]["instruments"][0]["symbol"] == "FUND-A"


class TestFundApi:
    def test_list_funds(self) -> None:
        client = _client()
        response = client.get("/api/v1/funds")
        assert response.status_code == 200
        assert response.json()["data"]["funds"][0]["fund_symbol"] == "FUND-A"


class TestBacktestApi:
    def _body(self) -> dict:
        return {
            "run_id": "bt-1",
            "account_id": "acc-1",
            "strategy_id": "strat-1",
            "strategy_version": "1.0.0",
            "data_range_start": "2026-01-01",
            "data_range_end": "2026-06-30",
            "initial_cash": "100000.00",
            "execution_mode": "IDEAL",
            "execution_model_version": "1.0",
            "random_seed": 42,
        }

    def test_create_backtest_202(self) -> None:
        client = _client()
        response = client.post("/api/v1/backtests", json=self._body())
        assert response.status_code == 202
        payload = response.json()
        assert payload["code"] == 202
        assert payload["data"]["run_id"] == "bt-1"
        assert payload["data"]["status"] == "CREATED"

    def test_create_requires_positive_cash(self) -> None:
        client = _client()
        body = self._body()
        body["initial_cash"] = "-5.00"
        response = client.post("/api/v1/backtests", json=body)
        assert response.status_code == 400
        assert response.json()["code"] == 1001

    def test_get_backtest(self) -> None:
        client = _client()
        client.post("/api/v1/backtests", json=self._body())
        response = client.get("/api/v1/backtests/bt-1")
        assert response.status_code == 200
        assert response.json()["data"]["status"] == "CREATED"

    def test_get_unknown_backtest_404(self) -> None:
        client = _client()
        response = client.get("/api/v1/backtests/missing")
        assert response.status_code == 404
        assert response.json()["code"] == 1002

    def test_start_and_cancel_backtest(self) -> None:
        client = _client()
        client.post("/api/v1/backtests", json=self._body())
        start = client.post("/api/v1/backtests/bt-1/start")
        assert start.status_code == 200
        assert start.json()["data"]["status"] == "RUNNING"
        cancel = client.post("/api/v1/backtests/bt-1/cancel")
        assert cancel.status_code == 200
        assert cancel.json()["data"]["status"] == "CANCELLED"

    def test_list_backtests(self) -> None:
        client = _client()
        client.post("/api/v1/backtests", json=self._body())
        response = client.get("/api/v1/backtests")
        assert response.status_code == 200
        assert len(response.json()["data"]["backtests"]) == 1


class TestEnvelopeContract:
    def test_all_routes_return_code_and_message(self) -> None:
        """OpenAPI 覆盖：所有领域路由返回固定 code/message。"""
        client = _client()
        paths = (
            ("GET", "/api/v1/accounts/acc-1?run_id=run-1"),
            ("GET", "/api/v1/strategies"),
            ("GET", "/api/v1/instruments"),
            ("GET", "/api/v1/funds"),
            ("GET", "/api/v1/backtests"),
        )
        for method, path in paths:
            response = client.request(method, path)
            payload = response.json()
            assert {"code", "message"} <= set(payload), path
            assert isinstance(payload["code"], int), path

    def test_openapi_schema_available(self) -> None:
        client = _client()
        response = client.get("/api/v1/openapi.json")
        assert response.status_code == 200
        schema = response.json()
        assert "/api/v1/accounts/{account_id}" in schema["paths"]
        assert "/api/v1/backtests" in schema["paths"]
        assert "/api/v1/funds" in schema["paths"]


class TestAccountDetailEndpoints:
    """P2-033 账户账本/现金流/份额/分析端点。"""

    def test_ledger_entries(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1/ledger?run_id=run-9")
        assert response.status_code == 200
        body = response.json()
        assert body["code"] == 0
        assert len(body["data"]["entries"]) == 2
        assert body["data"]["account_id"] == "acc-1"

    def test_cashflows(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1/cashflows?run_id=run-9")
        assert response.status_code == 200
        assert response.json()["data"]["cashflows"][0]["amount"] == "-1000.00"

    def test_shares(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1/shares?run_id=run-9")
        assert response.status_code == 200
        assert response.json()["data"]["shares"][0]["quantity"] == "100.5"

    def test_analysis(self) -> None:
        client = _client()
        response = client.get("/api/v1/accounts/acc-1/analysis?run_id=run-9")
        assert response.status_code == 200
        data = response.json()["data"]
        assert data["twr"] == "0.0123"
        assert data["xirr"] == "0.0456"

    def test_unknown_account_returns_404(self) -> None:
        client = _client()
        for path in (
            "/api/v1/accounts/acc-9/ledger?run_id=run-9",
            "/api/v1/accounts/acc-9/cashflows?run_id=run-9",
            "/api/v1/accounts/acc-9/shares?run_id=run-9",
            "/api/v1/accounts/acc-9/analysis?run_id=run-9",
        ):
            response = client.get(path)
            assert response.status_code == 404, path
            assert response.json()["code"] == 1002, path
