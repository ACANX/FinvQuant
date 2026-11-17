"""P2-024 FastAPI 应用、依赖注入、版本路由与健康接口测试。"""

from __future__ import annotations

from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import ApiDependencies, createApp
from veritasquant.apps.server.ApiRuntime import (
    ErrorCatalogProbe,
    PackagedApiVersionProvider,
)
from veritasquant.application.ApiApp import ApiVersionProvider, ReadinessProbe
from veritasquant.application.ApiErrors import ApiErrorCatalog, BusinessException


class _FailingProbe:
    """readiness 探针：始终失败。"""

    def check(self) -> tuple[bool, str]:
        return False, "依赖不可用"


class _PassingProbe:
    """readiness 探针：始终通过。"""

    def check(self) -> tuple[bool, str]:
        return True, "依赖正常"


class _StubVersionProvider(ApiVersionProvider):
    @property
    def apiVersion(self) -> str:
        return "v1"

    @property
    def catalogVersion(self) -> str:
        return "9.9.9"


def _deps(probes: tuple[ReadinessProbe, ...] = ()) -> ApiDependencies:
    catalog = ApiErrorCatalog.loadPackaged()
    return ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
        readinessProbes=probes,
    )


def test_import_has_no_side_effects() -> None:
    """导入模块不创建应用、不连接外部服务。"""
    import veritasquant.apps.server.ApiApp  # noqa: F401
    import veritasquant.apps.server.ApiRuntime  # noqa: F401

    assert True


def test_liveness_returns_success_envelope() -> None:
    client = TestClient(createApp(_deps()))
    response = client.get("/health/live")
    assert response.status_code == 200
    payload = response.json()
    assert payload["code"] == 0
    assert payload["message"]
    assert payload["data"]["status"] == "ALIVE"
    assert "error" not in payload


def test_readiness_passes_when_all_probes_pass() -> None:
    client = TestClient(createApp(_deps(probes=(_PassingProbe(),))))
    response = client.get("/health/ready")
    assert response.status_code == 200
    payload = response.json()
    assert payload["code"] == 0
    assert payload["data"]["status"] == "READY"
    assert payload["data"]["checks"] == ["_PassingProbe"]


def test_readiness_fails_when_probe_fails() -> None:
    client = TestClient(createApp(_deps(probes=(_FailingProbe(),))))
    response = client.get("/health/ready")
    assert response.status_code == 503
    payload = response.json()
    assert payload["code"] == 2005
    assert payload["error"]["code"] == "NOT_TRADING_READY"
    assert payload["error"]["retryable"] is True  # 依赖恢复后可重试
    assert payload["data"]["status"] == "NOT_READY"
    assert payload["data"]["checks"][0]["passed"] is False


def test_version_route_under_api_v1() -> None:
    client = TestClient(createApp(_deps()))
    response = client.get("/api/v1/version")
    assert response.status_code == 200
    payload = response.json()
    assert payload["code"] == 0
    assert payload["data"]["api_version"] == "v1"
    assert payload["data"]["catalog_version"] == "9.9.9"
    assert payload["data"]["service"] == "veritasquant-api"


def test_unknown_route_returns_envelope_error() -> None:
    client = TestClient(createApp(_deps()))
    response = client.get("/api/v1/nonexistent")
    assert response.status_code == 404
    payload = response.json()
    assert payload["code"] == 1002
    assert payload["error"]["code"] == "RESOURCE_NOT_FOUND"


def test_business_exception_maps_to_registered_error() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
    )

    class _Boom:
        pass

    app = createApp(deps)

    @app.get("/boom", tags=["test"])
    async def boom() -> None:  # pragma: no cover - 测试专用路由
        raise BusinessException(code=6201, details={"requiredAmount": "1000.00", "availableCash": "800.00"})

    client = TestClient(app, raise_server_exceptions=False)
    response = client.get("/boom")
    assert response.status_code == 422
    payload = response.json()
    assert payload["code"] == 6201
    assert payload["error"]["code"] == "INSUFFICIENT_AVAILABLE_CASH"
    assert payload["error"]["catalog_version"] == catalog.catalogVersion
    assert payload["details"] == {"required_amount": "1000.00", "available_cash": "800.00"}


def test_internal_error_maps_to_safe_500() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_StubVersionProvider(),
    )
    app = createApp(deps)

    @app.get("/crash", tags=["test"])
    async def crash() -> None:  # pragma: no cover - 测试专用路由
        raise RuntimeError("内部堆栈不得泄露")

    client = TestClient(app, raise_server_exceptions=False)
    response = client.get("/crash")
    assert response.status_code == 500
    payload = response.json()
    assert payload["code"] == 2006
    assert payload["error"]["code"] == "INTERNAL_SERVER_ERROR"
    assert "堆栈" not in str(payload)
    assert "traceback" not in str(payload).lower()


def test_packaged_version_provider_reads_metadata() -> None:
    provider = PackagedApiVersionProvider()
    assert provider.apiVersion == "v1"
    assert provider.catalogVersion  # 非空（已安装包或 0.0.0）


def test_error_catalog_probe_passes_for_packaged_catalog() -> None:
    catalog = ApiErrorCatalog.loadPackaged()
    probe = ErrorCatalogProbe(catalog)
    passed, detail = probe.check()
    assert passed is True
    assert detail


def test_api_server_main_offline_validation() -> None:
    """入口离线校验：--help 返回 0，未知参数返回 2。"""
    from veritasquant.apps.server.ApiServer import main

    assert main(["--help"]) == 0
    assert main(["--unknown-option"]) == 2
