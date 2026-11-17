"""P2-025 统一响应中间件与异常边界测试。"""

from __future__ import annotations

from fastapi import Response
from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import ApiDependencies, createApp
from veritasquant.application.ApiApp import ApiVersionProvider
from veritasquant.application.ApiErrors import ApiErrorCatalog, BusinessException


class _StubVersionProvider(ApiVersionProvider):
    @property
    def apiVersion(self) -> str:
        return "v1"

    @property
    def catalogVersion(self) -> str:
        return "9.9.9"


def _deps() -> ApiDependencies:
    return ApiDependencies(
        errorCatalog=ApiErrorCatalog.loadPackaged(),
        versionProvider=_StubVersionProvider(),
    )


def _app():
    app = createApp(_deps())

    @app.get("/plain-json", tags=["test"])
    async def plainJson() -> dict:  # pragma: no cover - 测试专用裸 JSON 路由
        return {"hello": "world"}  # 非信封 -> 中间件降级

    @app.get("/empty-204", tags=["test"])
    async def empty204() -> Response:  # pragma: no cover - 测试专用 204 路由
        return Response(status_code=204)

    @app.get("/envelope-ok", tags=["test"])
    async def envelopeOk() -> dict:  # pragma: no cover - 测试专用信封路由
        from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1

        return ResponseEnvelopeV1.success(0, "成功", data={"x": 1}).toWire()

    @app.get("/envelope-error", tags=["test"])
    async def envelopeError() -> dict:  # pragma: no cover - 测试专用错误信封路由
        from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1

        return ResponseEnvelopeV1.model_validate(
            {
                "code": 4001,
                "message": "data.import_contract_invalid",
                "error": {
                    "code": "DATA_IMPORT_CONTRACT_INVALID",
                    "catalog_version": "1.0",
                    "retryable": False,
                },
            }
        ).toWire()

    return app


def test_empty_204_is_replaced_with_envelope() -> None:
    client = TestClient(_app())
    response = client.get("/empty-204")
    assert response.status_code == 200  # 204 被替换为 200
    payload = response.json()
    assert payload["code"] == 0
    assert payload["message"]
    assert "error" not in payload


def test_plain_json_response_is_downgraded_to_internal_error() -> None:
    client = TestClient(_app())
    response = client.get("/plain-json")
    assert response.status_code == 500
    payload = response.json()
    assert payload["code"] == 2006
    assert payload["error"]["code"] == "INTERNAL_SERVER_ERROR"
    assert "hello" not in str(payload)  # 内部载荷不泄露


def test_envelope_response_passes_through() -> None:
    client = TestClient(_app())
    response = client.get("/envelope-ok")
    assert response.status_code == 200
    payload = response.json()
    assert payload["code"] == 0
    assert payload["data"] == {"x": 1}


def test_envelope_error_keeps_retryable_inside_error_only() -> None:
    client = TestClient(_app())
    response = client.get("/envelope-error")
    assert response.status_code == 200  # 信封自身状态；业务码在 code 字段
    payload = response.json()
    assert payload["code"] == 4001
    assert "retryable" in payload["error"]
    assert "retryable" not in payload  # 顶层不得出现 retryable


def test_business_error_retryable_only_in_error() -> None:
    """6201 错误：retryable 仅存在于 error 内。"""
    app = createApp(_deps())

    @app.get("/boom", tags=["test"])
    async def boom() -> None:  # pragma: no cover - 测试专用路由
        raise BusinessException(code=6201, details={"requiredAmount": "1000.00", "availableCash": "800.00"})

    client = TestClient(app, raise_server_exceptions=False)
    response = client.get("/boom")
    payload = response.json()
    assert payload["code"] == 6201
    assert payload["message"]
    assert payload["error"]["retryable"] is False
    assert "retryable" not in payload
    assert {"code", "message"} <= set(payload)  # 固定字段始终存在


def test_all_json_routes_have_code_and_message() -> None:
    """内置路由全部满足固定 code/message。"""
    client = TestClient(createApp(_deps()))
    for path in ("/health/live", "/health/ready", "/api/v1/version"):
        response = client.get(path)
        payload = response.json()
        assert {"code", "message"} <= set(payload), path
        assert isinstance(payload["code"], int)
        assert isinstance(payload["message"], str) and payload["message"]
