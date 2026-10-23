"""P2-036 /metrics 抓取端点。

- 以 text/plain; version=0.0.4 返回 Prometheus 文本格式；
- 非 JSON 响应天然豁免统一信封中间件（ApiMiddleware 只处理 application/json）；
- 端点只读：抓取不修改任何交易状态；
- 路径固定 /metrics（Prometheus 抓取器默认路径），不挂 API_V1_PREFIX，
  避免与业务信封语义混淆。
"""

from __future__ import annotations

from typing import Protocol

from fastapi import APIRouter
from fastapi.responses import PlainTextResponse

from veritasquant.monitoring.PrometheusMetrics import MetricsRegistry

METRICS_CONTENT_TYPE = "text/plain; version=0.0.4; charset=utf-8"


class MetricsProvider(Protocol):
    """指标文本提供者；默认实现为 MetricsRegistry.textFormat()。"""

    def textFormat(self) -> str: ...


def buildMetricsRouter(registry: MetricsRegistry | MetricsProvider | None = None) -> APIRouter:
    """构建 /metrics 路由；registry 缺省时使用进程级默认注册表。"""
    router = APIRouter(tags=["metrics"])

    if registry is None:
        from veritasquant.monitoring import getDefaultRegistry

        provider: MetricsProvider = getDefaultRegistry()
    else:
        provider = registry

    @router.get("/metrics", include_in_schema=False)
    async def metrics() -> PlainTextResponse:
        """Prometheus 文本格式指标抓取端点（只读）。"""
        return PlainTextResponse(
            content=provider.textFormat(),
            media_type=METRICS_CONTENT_TYPE,
            headers={"Cache-Control": "no-store"},
        )

    return router
