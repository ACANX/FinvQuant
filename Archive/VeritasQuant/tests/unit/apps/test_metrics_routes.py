"""P2-036 /metrics 端点测试：文本格式、豁免信封、只读。"""

from __future__ import annotations

from fastapi.testclient import TestClient

from veritasquant.apps.server.ApiApp import (
    ApiDependencies,
    createApp,
)
from veritasquant.apps.server.MetricsRoutes import METRICS_CONTENT_TYPE
from veritasquant.application.ApiErrors import ApiErrorCatalog


class _VersionProvider:
    def apiVersion(self) -> str:
        return "test"

    def catalogVersion(self) -> str:
        return "test-catalog"


def _makeApp() -> TestClient:
    catalog = ApiErrorCatalog.loadPackaged()
    deps = ApiDependencies(
        errorCatalog=catalog,
        versionProvider=_VersionProvider(),
    )
    return TestClient(createApp(deps))


def test_metrics_endpoint_returns_prometheus_text() -> None:
    client = _makeApp()
    response = client.get("/metrics")
    assert response.status_code == 200
    assert response.headers["content-type"].startswith("text/plain")
    assert METRICS_CONTENT_TYPE.split(";")[0] in response.headers["content-type"]
    assert "Cache-Control" in response.headers


def test_metrics_endpoint_not_enveloped() -> None:
    """/metrics 是 text/plain，不被统一信封中间件包装。"""
    client = _makeApp()
    response = client.get("/metrics")
    body = response.text
    assert "code" not in body or "vq_" in body  # 不是业务信封 JSON
    assert not body.lstrip().startswith("{")


def test_metrics_endpoint_readonly_does_not_affect_api() -> None:
    client = _makeApp()
    first = client.get("/metrics").text
    second = client.get("/metrics").text
    # 抓取不产生副作用：两次文本一致（无采集器写入时）
    assert first == second


def test_metrics_namespace_prefix_present() -> None:
    from veritasquant.monitoring import getDefaultRegistry

    # 预注册指标，验证默认注册表内容出现在 /metrics 文本中
    getDefaultRegistry().counter("probe_marker_total").inc()
    client = _makeApp()
    text = client.get("/metrics").text
    assert "vq_probe_marker_total" in text
