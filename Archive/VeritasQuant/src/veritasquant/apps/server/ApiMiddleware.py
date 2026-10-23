"""P2-025 统一响应中间件与异常边界强化。

保证：
- 所有 JSON 响应统一 ResponseEnvelopeV1（code/message 固定字段）；
- 错误 retryable 只出现在 error 对象内；
- 禁止空 204 响应（成功无数据也返回带 code/message 的信封）；
- 非信封 JSON 响应安全降级为注册的 2006，不泄露内部载荷。
"""

from __future__ import annotations

import json
from collections.abc import Awaitable, Callable

from fastapi import Request, Response
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.responses import JSONResponse

from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.ResponseEnvelope import (
    ResponseEnvelopeV1,
    mapException,
)


class ResponseEnvelopeMiddleware(BaseHTTPMiddleware):
    """把 204 与裸 JSON 响应统一为 v1 信封。"""

    def __init__(self, app: Response, catalog: ApiErrorCatalog) -> None:
        super().__init__(app)
        self._catalog = catalog

    async def dispatch(
        self,
        request: Request,
        call_next: Callable[[Request], Awaitable[Response]],
    ) -> Response:
        response = await call_next(request)
        if response.status_code == 204:
            # 禁止空 204：成功无数据也返回 code/message 信封
            envelope = ResponseEnvelopeV1.success(0, "成功")
            return JSONResponse(status_code=200, content=envelope.toWire())
        contentType = response.headers.get("content-type", "")
        if "application/json" not in contentType:
            return response
        # OpenAPI/docs 等框架自产 JSON 不套用业务信封
        if request.url.path in ("/openapi.json", "/docs", "/redoc") or request.url.path.endswith("/openapi.json"):
            return response
        body = b"".join([chunk async for chunk in response.body_iterator])  # type: ignore[attr-defined]
        try:
            payload = json.loads(body.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError):
            return self._internalError()
        if isinstance(payload, dict) and {"code", "message"} <= set(payload):
            return Response(
                status_code=response.status_code,
                content=body,
                media_type="application/json",
                headers=dict(response.headers),
            )
        return self._internalError()

    def _internalError(self) -> JSONResponse:
        mapped = mapException(RuntimeError("non-envelope json"), self._catalog)
        return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())
