"""API 服务 console script 入口。

- 默认离线参数校验（无副作用，供 packaging 契约测试）；
- `--serve` 时组装依赖并启动 uvicorn；
- 导入模块不连接数据库、不启动线程。
"""

from __future__ import annotations

import argparse
import logging
from collections.abc import Sequence

from veritasquant.application.Entrypoints import configureStandardStreams

logger = logging.getLogger("veritasquant.apps.server.api_server")


def main(argv: Sequence[str] | None = None) -> int:
    """解析 API 服务参数；--serve 时启动 uvicorn 服务。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(prog="vq-api-server", description="启动 VeritasQuant API 服务")
    parser.add_argument("--config", help="显式绝对配置文件路径")
    parser.add_argument("--resource-dir", help="显式绝对部署资源目录")
    parser.add_argument("--runtime-dir", help="显式绝对运行产物目录")
    parser.add_argument("--host", default="127.0.0.1", help="监听地址（默认 127.0.0.1）")
    parser.add_argument("--port", type=int, default=8000, help="监听端口（默认 8000）")
    parser.add_argument(
        "--serve",
        action="store_true",
        help="实际启动服务；缺省时仅做离线参数校验",
    )
    try:
        arguments = parser.parse_args(argv)
    except SystemExit as error:
        return error.code if isinstance(error.code, int) else 1
    if not arguments.serve:
        return 0
    return _serve(arguments)


def _buildDomainApis(catalog):  # noqa: ANN001 - 延迟导入保持无副作用
    """组装生产领域 API（P2-028 接线；PAPER/仿真最小视图实现）。

    此前生产入口未注入 domainApis，导致领域端点（/accounts、/strategies、
    /backtests 等）在生产镜像中全部 404（1002）。账户/目录视图为最小实现
    （见 DomainViewProviders），命令 API 接线后续接入。
    """
    from veritasquant.application.BacktestService import BacktestApplicationServiceV1
    from veritasquant.apps.server.DomainRoutes import DomainApis
    from veritasquant.apps.server.DomainViewProviders import (
        ServerAccountViewV1,
        ServerFundViewV1,
        ServerInstrumentViewV1,
        ServerStrategyViewV1,
    )

    return DomainApis(
        catalog=catalog,
        backtest=BacktestApplicationServiceV1(),
        accounts=ServerAccountViewV1(),
        strategies=ServerStrategyViewV1(),
        instruments=ServerInstrumentViewV1(),
        funds=ServerFundViewV1(),
    )


def _serve(arguments: argparse.Namespace) -> int:
    """组装依赖并启动 uvicorn；运行错误返回非零退出码。"""
    import uvicorn

    from veritasquant.apps.server.ApiApp import buildApiDependencies, createApp
    from veritasquant.apps.server.ApiRuntime import (
        ErrorCatalogProbe,
        PackagedApiVersionProvider,
    )
    from veritasquant.application.ApiErrors import ApiErrorCatalog
    from veritasquant.application.Security import (
        InMemoryAuditSink,
        SecurityService,
        SimpleRequestIdGenerator,
        TokenBucketRateLimiter,
    )

    catalog = ApiErrorCatalog.loadPackaged()
    provider = PackagedApiVersionProvider()

    def requestIdFromState(request) -> str | None:  # noqa: ANN001
        return getattr(request.state, "request_id", None)

    def traceIdFromState(request) -> str | None:  # noqa: ANN001
        return getattr(request.state, "trace_id", None)

    # 模拟盘默认安全装配：进程内主体提供者需由部署侧注入真实凭据源；
    # 此处为可运行的默认（未配置时拒绝一切业务调用，保证默认拒绝）。
    from veritasquant.application.Security import Principal, Role, UnauthenticatedError

    class _DefaultPrincipalProvider:
        def __init__(self) -> None:
            self._admin = Principal(
                principalId="deploy-admin", roles=(Role.Administrator,), environment="PAPER"
            )

        def resolve(self, credential: str | None) -> Principal:
            if credential == "dev-admin-token":
                return self._admin
            raise UnauthenticatedError("凭据无效或未配置")

    securityService = SecurityService(
        principalProvider=_DefaultPrincipalProvider(),
        auditSink=InMemoryAuditSink(),
        requestIdGenerator=SimpleRequestIdGenerator(),
        rateLimiter=TokenBucketRateLimiter(capacity=120, refillPerSecond=10.0),
    )

    # P2-030 SSE 状态流：进程内事件源（模拟盘默认）
    from veritasquant.application.StateStream import (
        InMemoryStreamEventSource,
        StreamService,
    )
    from veritasquant.apps.server.StateStreamRoutes import StreamDependencies

    streamService = StreamService(InMemoryStreamEventSource())
    streamDeps = StreamDependencies(
        principalProvider=_DefaultPrincipalProvider(),
        streamService=streamService,
    )

    # 行情导入：用户上传 MVSV → PostgreSQL（finv_quote_secu_kline_min）
    from veritasquant.application.QuoteImportService import QuoteImportService
    from veritasquant.apps.server.ImportRoutes import ImportApi
    from veritasquant.infrastructure.persistence.QuoteStore import MinuteQuoteStore, connectQuoteDb

    try:
        quoteConnection = connectQuoteDb()
    except Exception:  # noqa: BLE001 - 未配置 PG 时导入端点不可用，其余 API 照常
        logger.warning("行情导入未接线：PG 连接不可用（设置 VQ_POSTGRES_* 环境变量）")
        quoteConnection = None
    importApi = None
    if quoteConnection is not None:
        importApi = ImportApi(
            service=QuoteImportService(MinuteQuoteStore(quoteConnection)),
            catalog=catalog,
        )

    deps = buildApiDependencies(
        errorCatalog=catalog,
        versionProvider=provider,
        readinessProbes=(ErrorCatalogProbe(catalog),),
        securityService=securityService,
        domainApis=_buildDomainApis(catalog),
        importApi=importApi,
        streamDeps=streamDeps,
        requestIdExtractor=requestIdFromState,
        traceIdExtractor=traceIdFromState,
    )
    app = createApp(deps)
    try:
        uvicorn.run(app, host=arguments.host, port=arguments.port, log_level="info")
    except Exception:  # noqa: BLE001 - 启动失败统一记录并返回非零
        return 1
    return 0
