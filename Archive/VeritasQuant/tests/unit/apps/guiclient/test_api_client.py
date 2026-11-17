"""P2-031 GUI API 客户端测试：信封解包、错误映射、凭据传播。"""

from __future__ import annotations

import httpx
import pytest

from veritasquant.apps.guiclient.ApiClient import ApiClient, ApiClientError


def _clientFor(handler) -> ApiClient:  # noqa: ANN001
    transport = httpx.MockTransport(handler)
    return ApiClient("http://test", credential="token-1", transport=transport)


def _envelope(code: int, message: str, data: dict | None = None, error: dict | None = None) -> dict:
    payload: dict = {"code": code, "message": message}
    if data is not None:
        payload["data"] = data
    if error is not None:
        payload["error"] = error
    return payload


class TestApiClientSuccess:
    def test_version_success(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            assert request.headers["authorization"] == "Bearer token-1"
            assert request.headers["x-request-id"].startswith("gui_")
            return httpx.Response(
                200,
                json=_envelope(0, "版本信息", {"api_version": "v1", "catalog_version": "0.1.0"}),
            )

        client = _clientFor(handler)
        info = client.version()
        assert info["api_version"] == "v1"

    def test_accounts_list(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            return httpx.Response(
                200,
                json=_envelope(0, "ok", {"accounts": [{"account_id": "acc-1", "execution_mode": "PAPER"}]}),
            )

        client = _clientFor(handler)
        accounts = client.accounts()
        assert accounts[0]["account_id"] == "acc-1"

    def test_account_with_run_id_param(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            assert request.url.params["run_id"] == "run-9"
            return httpx.Response(200, json=_envelope(0, "ok", {"account_id": "acc-1"}))

        client = _clientFor(handler)
        result = client.account("acc-1", runId="run-9")
        assert result["account_id"] == "acc-1"


class TestApiClientErrors:
    def test_business_error_raises_with_code(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            return httpx.Response(
                404,
                json=_envelope(
                    1002,
                    "resource_not_found",
                    error={"code": "RESOURCE_NOT_FOUND", "catalog_version": "1.0", "retryable": False},
                ),
            )

        client = _clientFor(handler)
        with pytest.raises(ApiClientError) as excinfo:
            client.account("acc-missing")
        assert excinfo.value.code == 1002
        assert excinfo.value.httpStatus == 404
        assert not excinfo.value.retryable

    def test_retryable_error_flags(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            return httpx.Response(
                429,
                json=_envelope(
                    2004,
                    "rate_limited",
                    error={"code": "RATE_LIMITED", "catalog_version": "1.0", "retryable": True},
                ),
            )

        client = _clientFor(handler)
        with pytest.raises(ApiClientError) as excinfo:
            client.submitCommand({})
        assert excinfo.value.retryable

    def test_network_error_wrapped(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            raise httpx.ConnectError("connection refused")

        client = _clientFor(handler)
        with pytest.raises(ApiClientError) as excinfo:
            client.version()
        assert excinfo.value.code == 2006
        assert excinfo.value.retryable

    def test_non_json_response(self) -> None:
        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            return httpx.Response(200, text="<html>oops</html>")

        client = _clientFor(handler)
        with pytest.raises(ApiClientError) as excinfo:
            client.version()
        assert excinfo.value.code == 2006


class TestApiClientCredential:
    def test_credential_update(self) -> None:
        seen: list[str] = []

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            seen.append(request.headers.get("authorization", ""))
            return httpx.Response(200, json=_envelope(0, "ok", {"api_version": "v1"}))

        client = _clientFor(handler)
        client.version()
        client.setCredential("token-2")
        client.version()
        assert seen == ["Bearer token-1", "Bearer token-2"]

    def test_no_credential_no_header(self) -> None:
        seen: list[str] = []

        def handler(request: httpx.Request) -> httpx.Response:  # noqa: ANN001
            seen.append(request.headers.get("authorization", ""))
            return httpx.Response(200, json=_envelope(0, "ok"))

        transport = httpx.MockTransport(handler)
        client = ApiClient("http://test", transport=transport)
        client.version()
        assert seen == [""]
