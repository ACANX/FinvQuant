"""P2-024 FastAPI 应用组装、依赖注入与统一异常边界。

- createApp() 纯函数：导入无副作用，测试可注入替身依赖；
- 基路径固定 /api/v1，所有 JSON 响应统一 ResponseEnvelopeV1；
- liveness/readiness 分层健康接口；领域模块不反向依赖本层。
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHttpException

from veritasquant.apps.server.ApiMiddleware import ResponseEnvelopeMiddleware
from veritasquant.apps.server.CommandRoutes import CommandApi, buildCommandRouter
from veritasquant.apps.server.DomainRoutes import DomainApis, buildDomainRouter
from veritasquant.apps.server.ImportRoutes import ImportApi, buildImportRouter
from veritasquant.apps.server.MetricsRoutes import buildMetricsRouter
from veritasquant.apps.server.SecurityMiddleware import SecurityMiddleware
from veritasquant.apps.server.StateStreamRoutes import (
    StreamDependencies,
    buildStreamRouter,
)
from veritasquant.application.ApiApp import (
    ApiVersionInfoV1,
    ApiVersionProvider,
    HealthService,
    ReadinessProbe,
)
from veritasquant.application.ApiErrors import ApiErrorCatalog, BusinessException
from veritasquant.application.ResponseEnvelope import (
    ResponseEnvelopeV1,
    mapException,
)
from veritasquant.application.Security import (
    AccessDeniedError,
    RateLimitExceededError,
    SecurityService,
    UnauthenticatedError,
)

SERVICE_NAME = "veritasquant-api"
API_V1_PREFIX = "/api/v1"


@dataclass(frozen=True, slots=True)
class ApiDependencies:
    """显式依赖注入容器；测试可替换任何探针或版本提供者。"""

    errorCatalog: ApiErrorCatalog
    versionProvider: ApiVersionProvider
    readinessProbes: tuple[ReadinessProbe, ...] = ()
    commandApi: CommandApi | None = None
    domainApis: DomainApis | None = None
    importApi: ImportApi | None = None
    streamDeps: StreamDependencies | None = None
    securityService: SecurityService | None = None
    requestIdExtractor: Callable[[Request], str | None] | None = None
    traceIdExtractor: Callable[[Request], str | None] | None = None
    _health: HealthService = field(init=False, repr=False)

    def __post_init__(self) -> None:
        object.__setattr__(self, "_health", HealthService(self.readinessProbes))

    @property
    def health(self) -> HealthService:
        return self._health


def _errorResponse(exception: Exception, deps: ApiDependencies, request: Request) -> JSONResponse:
    requestId = deps.requestIdExtractor(request) if deps.requestIdExtractor else None
    traceId = deps.traceIdExtractor(request) if deps.traceIdExtractor else None
    mapped = mapException(exception, deps.errorCatalog, requestId, traceId)
    return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())


def createApp(deps: ApiDependencies) -> FastAPI:
    """组装 FastAPI 应用；纯函数，不连接外部服务。"""
    app = FastAPI(
        title="VeritasQuant API",
        version=deps.versionProvider.apiVersion,
        docs_url=f"{API_V1_PREFIX}/docs",
        openapi_url=f"{API_V1_PREFIX}/openapi.json",
    )
    app.add_middleware(ResponseEnvelopeMiddleware, catalog=deps.errorCatalog)  # type: ignore[arg-type]
    if deps.securityService is not None:
        app.add_middleware(
            SecurityMiddleware,
            securityService=deps.securityService,
            errorCatalog=deps.errorCatalog,
        )

    @app.exception_handler(RequestValidationError)
    async def validationExceptionHandler(
        request: Request, exception: RequestValidationError
    ) -> JSONResponse:
        # 字段/Schema 校验失败统一映射为 VALIDATION_ERROR(1001)
        requestId = deps.requestIdExtractor(request) if deps.requestIdExtractor else None
        traceId = deps.traceIdExtractor(request) if deps.traceIdExtractor else None
        mapped = mapException(BusinessException(1001, {}), deps.errorCatalog, requestId, traceId)
        return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())

    if deps.securityService is not None:
        @app.exception_handler(UnauthenticatedError)
        async def unauthenticatedHandler(request: Request, exception: UnauthenticatedError) -> JSONResponse:
            # 2001 UNAUTHENTICATED：身份凭据缺失或无效
            requestId = getattr(request.state, "request_id", None) or (
                deps.requestIdExtractor(request) if deps.requestIdExtractor else None
            )
            traceId = getattr(request.state, "trace_id", None)
            mapped = mapException(
                BusinessException(2001, {}), deps.errorCatalog, requestId, traceId
            )
            return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())

        @app.exception_handler(AccessDeniedError)
        async def accessDeniedHandler(request: Request, exception: AccessDeniedError) -> JSONResponse:
            # 2002 FORBIDDEN：越权账户或动作无权限（隐藏资源存在性）
            requestId = getattr(request.state, "request_id", None) or (
                deps.requestIdExtractor(request) if deps.requestIdExtractor else None
            )
            traceId = getattr(request.state, "trace_id", None)
            mapped = mapException(
                BusinessException(2002, {}), deps.errorCatalog, requestId, traceId
            )
            return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())

        @app.exception_handler(RateLimitExceededError)
        async def rateLimitHandler(request: Request, exception: RateLimitExceededError) -> JSONResponse:
            # 2004 RATE_LIMITED：限频，响应携带 Retry-After
            requestId = getattr(request.state, "request_id", None) or (
                deps.requestIdExtractor(request) if deps.requestIdExtractor else None
            )
            traceId = getattr(request.state, "trace_id", None)
            mapped = mapException(
                BusinessException(2004, {}), deps.errorCatalog, requestId, traceId
            )
            response = JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())
            response.headers["Retry-After"] = str(exception.retryAfterSeconds)
            return response

    @app.exception_handler(StarletteHttpException)
    async def httpExceptionHandler(request: Request, exception: StarletteHttpException) -> JSONResponse:
        # 未命中路由等框架级 404/405 统一映射为 RESOURCE_NOT_FOUND 信封
        requestId = deps.requestIdExtractor(request) if deps.requestIdExtractor else None
        traceId = deps.traceIdExtractor(request) if deps.traceIdExtractor else None
        mapped = mapException(
            BusinessException(1002, {}) if exception.status_code == 404 else BusinessException(1001, {}),
            deps.errorCatalog,
            requestId,
            traceId,
        )
        return JSONResponse(status_code=mapped.httpStatus, content=mapped.envelope.toWire())

    @app.exception_handler(Exception)
    async def unhandledExceptionHandler(request: Request, exception: Exception) -> JSONResponse:
        return _errorResponse(exception, deps, request)

    @app.get("/health/live", tags=["health"])
    async def liveness() -> JSONResponse:
        """liveness：进程能响应即存活（决定是否重启）。"""
        envelope = ResponseEnvelopeV1.success(
            0,
            "存活",
            data={"status": "ALIVE", "service": SERVICE_NAME},
        )
        return JSONResponse(status_code=200, content=envelope.toWire())

    @app.get("/health/ready", tags=["health"])
    async def readiness() -> JSONResponse:
        """readiness：全部探针通过才接收流量。"""
        result = deps.health.readiness()
        if result.ready:
            envelope = ResponseEnvelopeV1.success(
                0,
                "就绪",
                data={"status": "READY", "checks": [name for name, _, _ in result.checks]},
            )
            return JSONResponse(status_code=200, content=envelope.toWire())
        # 2005 NOT_TRADING_READY：服务存活但就绪门禁未通过（>=1000 必须携带 error）
        definition = deps.errorCatalog.getError(2005)
        envelope = ResponseEnvelopeV1.model_validate(
            {
                "code": 2005,
                "message": definition.messageKey,
                "data": {
                    "status": "NOT_READY",
                    "checks": [
                        {"name": name, "passed": passed, "detail": detail}
                        for name, passed, detail in result.checks
                    ],
                },
                "error": {
                    "code": definition.errorCode,
                    "catalog_version": deps.errorCatalog.catalogVersion,
                    "retryable": definition.retryable,
                },
            }
        )
        return JSONResponse(status_code=503, content=envelope.toWire())

    @app.get(f"{API_V1_PREFIX}/version", tags=["api"])
    async def version() -> JSONResponse:
        """版本路由：返回 API 与错误目录版本。"""
        info = ApiVersionInfoV1(
            apiVersion=deps.versionProvider.apiVersion,
            catalogVersion=deps.versionProvider.catalogVersion,
            service=SERVICE_NAME,
        )
        envelope = ResponseEnvelopeV1.success(0, "版本信息", data={"api_version": info.apiVersion, "catalog_version": info.catalogVersion, "service": info.service})
        return JSONResponse(status_code=200, content=envelope.toWire())

    if deps.commandApi is not None:
        app.include_router(buildCommandRouter(deps.commandApi))

    if deps.importApi is not None:
        app.include_router(buildImportRouter(deps.importApi))

    if deps.domainApis is not None:
        app.include_router(buildDomainRouter(deps.domainApis))

    if deps.streamDeps is not None:
        app.include_router(buildStreamRouter(deps.streamDeps))

    app.include_router(buildMetricsRouter())

    return app


def buildApiDependencies(
    errorCatalog: ApiErrorCatalog,
    versionProvider: ApiVersionProvider,
    readinessProbes: tuple[ReadinessProbe, ...] = (),
    commandApi: CommandApi | None = None,
    domainApis: DomainApis | None = None,
    importApi: ImportApi | None = None,
    streamDeps: StreamDependencies | None = None,
    securityService: SecurityService | None = None,
    requestIdExtractor: Callable[[Request], str | None] | None = None,
    traceIdExtractor: Callable[[Request], str | None] | None = None,
) -> ApiDependencies:
    """便捷组装；保持字段名与 ApiDependencies 一致。"""
    return ApiDependencies(
        errorCatalog=errorCatalog,
        versionProvider=versionProvider,
        readinessProbes=readinessProbes,
        commandApi=commandApi,
        domainApis=domainApis,
        importApi=importApi,
        streamDeps=streamDeps,
        securityService=securityService,
        requestIdExtractor=requestIdExtractor,
        traceIdExtractor=traceIdExtractor,
    )
