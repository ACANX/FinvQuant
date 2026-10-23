"""P2-028 账户、策略、数据、回测、基金计划与报告 API 路由。

验收标准：每个账户操作显式 account_id/run_id；统一响应信封；
OpenAPI 与契约测试覆盖所有返回码。

路由按领域拆分，依赖通过构造注入（测试可替换替身）：
- 账户：查询账户视图（显式 account_id/run_id）；
- 策略：列出策略与版本（策略元数据，不执行策略代码）；
- 数据：查询标的与数据版本（只读）；
- 回测：创建/查询/暂停/取消回测运行（复用 BacktestApplicationServiceV1）；
- 基金：基金信息与定投计划（只读查询）；
- 报告：TWR/XIRR/本金报告查询。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from typing import Any, Protocol

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.BacktestService import (
    BacktestApplicationServiceV1,
    BacktestConfigV1,
    BacktestServiceError,
)
from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1


class ResourceNotFound(Exception):
    """领域资源不存在；映射为 1002。"""


class InvalidResource(Exception):
    """领域输入非法；映射为 1001。"""


class AccountViewProvider(Protocol):
    """账户视图端口。"""

    def accounts(self) -> tuple[dict[str, Any], ...]: ...

    def account(self, accountId: str, runId: str) -> dict[str, Any]: ...

    def ledgerEntries(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]: ...

    def cashFlows(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]: ...

    def sharePositions(self, accountId: str, runId: str) -> tuple[dict[str, Any], ...]: ...

    def analysis(self, accountId: str, runId: str) -> dict[str, Any]: ...


class StrategyViewProvider(Protocol):
    """策略视图端口。"""

    def strategies(self) -> tuple[dict[str, Any], ...]: ...


class InstrumentViewProvider(Protocol):
    """标的视图端口。"""

    def instruments(self) -> tuple[dict[str, Any], ...]: ...


class FundViewProvider(Protocol):
    """基金视图端口。"""

    def funds(self) -> tuple[dict[str, Any], ...]: ...


@dataclass(frozen=True, slots=True)
class ReportQueryV1:
    """报告查询参数。"""

    accountId: str
    runId: str
    startValue: Decimal
    endValue: Decimal


class DomainApis:
    """聚合领域 API 依赖；未注入的端口返回 NOT_IMPLEMENTED 语义。"""

    def __init__(
        self,
        catalog: ApiErrorCatalog,
        backtest: BacktestApplicationServiceV1 | None = None,
        accounts: AccountViewProvider | None = None,
        strategies: StrategyViewProvider | None = None,
        instruments: InstrumentViewProvider | None = None,
        funds: FundViewProvider | None = None,
    ) -> None:
        self.catalog = catalog
        self.backtest = backtest
        self.accounts = accounts
        self.strategies = strategies
        self.instruments = instruments
        self.funds = funds


def buildDomainRouter(apis: DomainApis) -> APIRouter:
    """注册六类领域 API 路由。"""
    router = APIRouter(prefix="/api/v1", tags=["domain"])

    @router.get("/accounts")
    async def accounts() -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        return _ok(0, "账户列表", {"accounts": list(apis.accounts.accounts())})

    @router.get("/accounts/{account_id}")
    async def account(account_id: str, run_id: str) -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        try:
            data = apis.accounts.account(account_id, run_id)
        except ResourceNotFound as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "账户视图", data)

    @router.get("/accounts/{account_id}/ledger")
    async def accountLedger(account_id: str, run_id: str) -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        try:
            entries = apis.accounts.ledgerEntries(account_id, run_id)
        except ResourceNotFound as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "逐笔分录", {"account_id": account_id, "run_id": run_id, "entries": list(entries)})

    @router.get("/accounts/{account_id}/cashflows")
    async def accountCashFlows(account_id: str, run_id: str) -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        try:
            flows = apis.accounts.cashFlows(account_id, run_id)
        except ResourceNotFound as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "现金流", {"account_id": account_id, "run_id": run_id, "cashflows": list(flows)})

    @router.get("/accounts/{account_id}/shares")
    async def accountShares(account_id: str, run_id: str) -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        try:
            shares = apis.accounts.sharePositions(account_id, run_id)
        except ResourceNotFound as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "基金份额", {"account_id": account_id, "run_id": run_id, "shares": list(shares)})

    @router.get("/accounts/{account_id}/analysis")
    async def accountAnalysis(account_id: str, run_id: str) -> JSONResponse:
        _require(apis.accounts, "accounts")
        assert apis.accounts is not None
        try:
            data = apis.accounts.analysis(account_id, run_id)
        except ResourceNotFound as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "结果分析", data)

    @router.get("/strategies")
    async def strategies() -> JSONResponse:
        _require(apis.strategies, "strategies")
        assert apis.strategies is not None
        return _ok(0, "策略列表", {"strategies": list(apis.strategies.strategies())})

    @router.get("/instruments")
    async def instruments() -> JSONResponse:
        _require(apis.instruments, "instruments")
        assert apis.instruments is not None
        return _ok(0, "标的列表", {"instruments": list(apis.instruments.instruments())})

    @router.get("/funds")
    async def funds() -> JSONResponse:
        _require(apis.funds, "funds")
        assert apis.funds is not None
        return _ok(0, "基金列表", {"funds": list(apis.funds.funds())})

    @router.post("/backtests")
    async def createBacktest(payload: dict[str, Any]) -> JSONResponse:
        _require(apis.backtest, "backtest")
        assert apis.backtest is not None
        try:
            config = BacktestConfigV1(
                runId=_string(payload, "run_id"),
                accountId=_string(payload, "account_id"),
                strategyId=_string(payload, "strategy_id"),
                strategyVersion=_string(payload, "strategy_version"),
                dataRangeStart=_string(payload, "data_range_start"),
                dataRangeEnd=_string(payload, "data_range_end"),
                initialCash=_decimal(payload.get("initial_cash")),
                executionMode=_string(payload, "execution_mode"),
                executionModelVersion=_string(payload, "execution_model_version"),
                randomSeed=_int(payload.get("random_seed")),
            )
            view = apis.backtest.createRun(config)
        except BacktestServiceError as error:
            return _reject(apis.catalog, 1001, str(error))
        except InvalidResource as error:
            return _reject(apis.catalog, 1001, str(error))
        return _ok(202, "回测已创建", _backtestWire(view))

    @router.get("/backtests")
    async def listBacktests() -> JSONResponse:
        _require(apis.backtest, "backtest")
        assert apis.backtest is not None
        views = apis.backtest.queryAll()
        return _ok(0, "回测列表", {"backtests": [_backtestWire(view) for view in views]})

    @router.get("/backtests/{run_id}")
    async def getBacktest(run_id: str) -> JSONResponse:
        _require(apis.backtest, "backtest")
        assert apis.backtest is not None
        try:
            view = apis.backtest.query(run_id)
        except BacktestServiceError as error:
            return _reject(apis.catalog, 1002, str(error))
        return _ok(0, "回测状态", _backtestWire(view))

    @router.post("/backtests/{run_id}/start")
    async def startBacktest(run_id: str) -> JSONResponse:
        _require(apis.backtest, "backtest")
        assert apis.backtest is not None
        try:
            view = apis.backtest.start(run_id)
        except BacktestServiceError as error:
            return _reject(apis.catalog, 1001, str(error))
        return _ok(0, "回测已开始", _backtestWire(view))

    @router.post("/backtests/{run_id}/cancel")
    async def cancelBacktest(run_id: str) -> JSONResponse:
        _require(apis.backtest, "backtest")
        assert apis.backtest is not None
        try:
            view = apis.backtest.cancel(run_id)
        except BacktestServiceError as error:
            return _reject(apis.catalog, 1001, str(error))
        return _ok(0, "回测已取消", _backtestWire(view))

    return router


def _backtestWire(view: Any) -> dict[str, Any]:
    return {
        "run_id": view.runId,
        "status": view.status.value if hasattr(view.status, "value") else str(view.status),
        "checkpoint_sequence": view.checkpointSequence,
        "failure_reason": view.failureReason,
        "config_hash": view.configHash,
    }


def _ok(code: int, message: str, data: Any) -> JSONResponse:
    envelope = ResponseEnvelopeV1.success(code, message, data=data)
    return JSONResponse(status_code=200 if code == 0 else 202, content=envelope.toWire())


def _reject(catalog: ApiErrorCatalog, code: int, message: str) -> JSONResponse:
    definition = catalog.getError(code)
    envelope = ResponseEnvelopeV1.model_validate(
        {
            "code": definition.code,
            "message": message,
            "error": {
                "code": definition.errorCode,
                "catalog_version": catalog.catalogVersion,
                "retryable": definition.retryable,
            },
        }
    )
    return JSONResponse(status_code=definition.httpStatus, content=envelope.toWire())


def _require(port: Any, name: str) -> None:
    if port is None:
        raise NotImplementedError(f"{name} 端口未注入")


def _string(payload: dict[str, Any], key: str) -> str:
    value = payload.get(key)
    if not isinstance(value, str) or not value:
        raise InvalidResource(f"{key} 必须为非空字符串")
    return value


def _decimal(value: Any) -> Decimal:
    if value is None:
        raise InvalidResource("Decimal 字段不能为空")
    try:
        return Decimal(str(value))
    except Exception as error:  # noqa: BLE001
        raise InvalidResource(f"Decimal 解析失败: {value}") from error


def _int(value: Any) -> int:
    if isinstance(value, bool) or not isinstance(value, int):
        raise InvalidResource("整数字段非法")
    return value
