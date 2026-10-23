"""P2-033 账户/结果分析/账本/监控测试：API 端点与多账户隔离。"""

from __future__ import annotations

import httpx
import pytest

from veritasquant.apps.guiclient.ApiClient import ApiClient
from veritasquant.apps.guiclient.Pages import AccountScope


class TestAccountScopedEndpoints:
    def test_ledger_cashflows_shares_analysis(self) -> None:
        """P2-033 四个账户域端点必须显式 account_id + run_id。"""
        seen: list[tuple[str, str, str]] = []

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            path = request.url.path
            runId = request.url.params.get("run_id", "")
            seen.append((request.method, path, runId))
            if path.endswith("/ledger"):
                data = {"entries": [{"journal_id": "j-1", "amount": "100.00"}]}
            elif path.endswith("/cashflows"):
                data = {"cashflows": [{"date": "2026-01-01", "amount": "-1000.00"}]}
            elif path.endswith("/shares"):
                data = {"shares": [{"fund_symbol": "FUND-A", "quantity": "100.5"}]}
            elif path.endswith("/analysis"):
                data = {"twr": "0.0123", "xirr": "0.0456", "principal": "10000.00"}
            else:
                data = {"account_id": "acc-1", "execution_mode": "PAPER"}
            return httpx.Response(200, json={"code": 0, "message": "ok", "data": data})

        client = ApiClient("http://test", transport=httpx.MockTransport(handler))
        entries = client.accountLedger("acc-1", "run-9")
        assert entries[0]["journal_id"] == "j-1"
        flows = client.accountCashFlows("acc-1", "run-9")
        assert flows[0]["amount"] == "-1000.00"
        shares = client.accountShares("acc-1", "run-9")
        assert shares[0]["quantity"] == "100.5"
        analysis = client.accountAnalysis("acc-1", "run-9")
        assert analysis["twr"] == "0.0123"

        # 所有请求都带 run_id
        assert all(path.endswith("/ledger") or path.endswith("/cashflows") or path.endswith("/shares") or path.endswith("/analysis") for _, path, _ in seen)
        assert all(runId == "run-9" for _, _, runId in seen)

    def test_analysis_requires_run_id(self) -> None:
        """分析查询缺少 run_id 时后端应拒绝（校验显式 run_id）。"""

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            runId = request.url.params.get("run_id", "")
            if not runId:
                return httpx.Response(
                    400,
                    json={
                        "code": 1001,
                        "message": "run_id 必填",
                        "error": {"code": "VALIDATION_ERROR", "catalog_version": "1.0", "retryable": False},
                    },
                )
            return httpx.Response(200, json={"code": 0, "message": "ok", "data": {}})

        client = ApiClient("http://test", transport=httpx.MockTransport(handler))
        with pytest.raises(Exception) as excinfo:  # noqa: B017
            client.accountAnalysis("acc-1", "")
        assert "1001" in str(excinfo.value) or "VALIDATION" in str(excinfo.value)


class TestAccountScopeIsolation:
    def test_require_account_rejects_empty(self) -> None:
        scope = AccountScope(accountId="")
        with pytest.raises(ValueError):
            scope.requireAccount()

    def test_require_account_returns_id(self) -> None:
        scope = AccountScope(accountId="acc-1", runId="run-2")
        assert scope.requireAccount() == "acc-1"
        assert scope.runId == "run-2"

    def test_scope_holds_run_id(self) -> None:
        assert AccountScope("acc-1", "run-7").runId == "run-7"
        assert AccountScope("acc-1").runId is None
