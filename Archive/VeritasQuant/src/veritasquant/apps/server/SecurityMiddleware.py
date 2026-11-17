"""P2-029 安全中间件：请求/追踪 ID 提取、鉴权、RBAC 与限频。

- 从 Header 提取凭据（Authorization Bearer）、X-Request-Id、X-Trace-Id；
- 未提供 request_id 时生成（响应回填 X-Request-Id）；
- 路由权限表按方法+路径前缀匹配，缺失鉴权 2001、越权 2002、限频 2004；
- 安全异常在中间件内直接映射为信封响应（中间件层异常不会到达
  FastAPI exception handler，必须在 dispatch 内捕获）。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse, Response

from veritasquant.application.ApiErrors import ApiErrorCatalog, BusinessException
from veritasquant.application.ResponseEnvelope import mapException
from veritasquant.application.Security import (
    AccessDeniedError,
    Permission,
    RateLimitExceededError,
    RoutePermission,
    SecurityService,
    UnauthenticatedError,
)

# 免鉴权路径（健康检查与版本路由；OpenAPI 文档由中间件豁免）
_PUBLIC_PREFIXES = ("/health/live", "/health/ready")


@dataclass(frozen=True, slots=True)
class _RouteRule:
    """方法+路径前缀 -> 权限规则。"""

    methods: frozenset[str]
    prefix: str
    route: RoutePermission


_ROUTE_RULES: tuple[_RouteRule, ...] = (
    # 账户（读/写）
    _RouteRule(frozenset({"GET"}), "/api/v1/accounts/", RoutePermission(Permission.AccountRead, "account:read")),
    # 策略（读/写）
    _RouteRule(frozenset({"GET"}), "/api/v1/strategies", RoutePermission(Permission.StrategyRead, "strategy:read")),
    _RouteRule(frozenset({"POST", "PUT", "PATCH", "DELETE"}), "/api/v1/strategies", RoutePermission(Permission.StrategyWrite, "strategy:write")),
    # 数据（读/导入）
    _RouteRule(frozenset({"GET"}), "/api/v1/instruments", RoutePermission(Permission.DataRead, "data:read")),
    _RouteRule(frozenset({"POST"}), "/api/v1/data/", RoutePermission(Permission.DataImport, "data:import", 30, 0.5)),
    # 回测
    _RouteRule(frozenset({"GET"}), "/api/v1/backtests", RoutePermission(Permission.BacktestRun, "backtest:read")),
    _RouteRule(frozenset({"POST"}), "/api/v1/backtests", RoutePermission(Permission.BacktestRun, "backtest:run", 10, 0.2)),
    # 基金计划
    _RouteRule(frozenset({"GET"}), "/api/v1/funds", RoutePermission(Permission.FundPlanRead, "fund_plan:read")),
    # 命令（写）
    _RouteRule(frozenset({"POST"}), "/api/v1/commands", RoutePermission(Permission.CommandSubmit, "command:submit", 20, 0.5)),
    _RouteRule(frozenset({"GET"}), "/api/v1/commands/", RoutePermission(Permission.CommandSubmit, "command:read", 60, 1.0)),
    _RouteRule(frozenset({"POST"}), "/api/v1/commands/", RoutePermission(Permission.CommandCancel, "command:cancel", 20, 0.5)),
    # 报告
    _RouteRule(frozenset({"GET"}), "/api/v1/reports", RoutePermission(Permission.ReportRead, "report:read")),
    _RouteRule(frozenset({"POST"}), "/api/v1/reports", RoutePermission(Permission.ReportGenerate, "report:generate", 10, 0.2)),
)


def _matchRule(method: str, path: str) -> _RouteRule | None:
    """最长前缀匹配路由规则；未匹配返回 None（放行，由路由层兜底 404）。"""
    best: _RouteRule | None = None
    for rule in _ROUTE_RULES:
        if method in rule.methods and path.startswith(rule.prefix):
            if best is None or len(rule.prefix) > len(best.prefix):
                best = rule
    return best


def _bearerToken(request: Request) -> str | None:
    authorization = request.headers.get("authorization")
    if not authorization:
        return None
    scheme, _, credential = authorization.partition(" ")
    if scheme.lower() != "bearer" or not credential:
        return None
    return credential.strip()


class SecurityMiddleware(BaseHTTPMiddleware):
    """强制请求上下文与路由授权。"""

    def __init__(self, app: Any, securityService: SecurityService, errorCatalog: ApiErrorCatalog) -> None:
        super().__init__(app)
        self._security = securityService
        self._catalog = errorCatalog

    def _securityErrorResponse(
        self, request: Request, exception: Exception, code: int, httpStatus: int
    ) -> JSONResponse:
        requestId = getattr(request.state, "request_id", None)
        traceId = getattr(request.state, "trace_id", None)
        mapped = mapException(BusinessException(code, {}), self._catalog, requestId, traceId)
        response = JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())
        if isinstance(exception, RateLimitExceededError):
            response.headers["Retry-After"] = str(exception.retryAfterSeconds)
        return response

    async def dispatch(self, request: Request, call_next: Any) -> Response:
        path = request.url.path
        method = request.method.upper()

        # 健康检查免鉴权
        if path.startswith(_PUBLIC_PREFIXES):
            return await call_next(request)

        # OpenAPI / docs 免业务鉴权（框架自产文档）
        if path.startswith("/api/v1/openapi.json") or "/docs" in path:
            return await call_next(request)

        credential = _bearerToken(request)
        incomingRequestId = request.headers.get("x-request-id")
        traceId = request.headers.get("x-trace-id")
        context = self._security.newRequestContext(credential, traceId)

        # 先记录请求上下文，再执行授权（未提供 request_id 也照常授权）
        request.state.request_id = incomingRequestId or context.requestId
        request.state.trace_id = traceId
        request.state.principal = context.principal
        request.state.context = context

        rule = _matchRule(method, path)
        if rule is not None:
            # 从路径提取 account_id（/api/v1/accounts/{account_id}）
            accountId = None
            if path.startswith("/api/v1/accounts/"):
                parts = path.split("/")
                if len(parts) >= 5 and parts[4]:
                    accountId = parts[4]
            try:
                self._security.authorize(context, rule.route, path, accountId)
            except UnauthenticatedError as error:
                return self._securityErrorResponse(request, error, 2001, 401)
            except AccessDeniedError as error:
                return self._securityErrorResponse(request, error, 2002, 403)
            except RateLimitExceededError as error:
                return self._securityErrorResponse(request, error, 2004, 429)

        response = await call_next(request)
        # 回填 request_id（未提供则返回生成的）
        response.headers["X-Request-Id"] = request.state.request_id
        if traceId:
            response.headers["X-Trace-Id"] = traceId
        return response
