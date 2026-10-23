"""P2-026/027 命令 API 路由：幂等提交、查询、版本冲突与取消。

- POST /api/v1/commands：202 返回 command 引用；同键同载荷返回原命令；
  同键异载荷返回 1003；
- GET /api/v1/commands/{command_id}：查询命令状态（含失败快照）；
- POST /api/v1/commands/{command_id}/cancel：合法取消。
依赖通过 buildCommandRouter(api) 注入，测试可替换替身。
"""

from __future__ import annotations

from typing import Any, Mapping

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from veritasquant.application.CommandResource import (
    CommandError,
    CommandRecordV1,
    CommandService,
    CommandStateConflict,
    CommandStatus,
    IdempotencyConflict,
)
from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1

_COMMANDS_PREFIX = "/api/v1/commands"


def _wireStatus(record: CommandRecordV1) -> dict[str, Any]:
    """命令资源 wire 视图：版本 + 状态 + 失败快照。"""
    payload: dict[str, Any] = {
        "command_id": record.commandId,
        "command_type": record.commandType,
        "account_id": record.accountId,
        "run_id": record.runId,
        "status": record.status.value,
        "created_ts": record.createdTs.isoformat(),
        "updated_ts": record.updatedTs.isoformat(),
    }
    if record.expectedVersion is not None:
        payload["expected_version"] = record.expectedVersion
    if record.resultReference is not None:
        payload["result_reference"] = record.resultReference
    if record.failure is not None:
        payload["failure"] = {
            "code": record.failure.code,
            "error_code": record.failure.errorCode,
            "catalog_version": record.failure.catalogVersion,
            "retryable": record.failure.retryable,
        }
        if record.failure.details:
            payload["failure"]["details"] = dict(record.failure.details)
    return payload


class CommandApi:
    """命令路由依赖：封装 CommandService 与统一错误映射。"""

    def __init__(self, service: CommandService, catalog: ApiErrorCatalog) -> None:
        self._service = service
        self._catalog = catalog

    def submit(
        self,
        *,
        commandId: str,
        commandType: str,
        accountId: str,
        runId: str,
        requestedBy: str,
        idempotencyKey: str,
        payload: Mapping[str, Any],
        expectedVersion: str | None = None,
    ) -> tuple[ResponseEnvelopeV1, int]:
        """提交命令；返回 (信封, HTTP 状态)。"""
        try:
            record, _ = self._service.submit(
                commandId=commandId,
                commandType=commandType,
                accountId=accountId,
                runId=runId,
                requestedBy=requestedBy,
                idempotencyKey=idempotencyKey,
                route=_COMMANDS_PREFIX,
                payload=payload,
                expectedVersion=expectedVersion,
            )
        except IdempotencyConflict as error:
            return self._error(1003, str(error)), 409
        except CommandError as error:
            return self._error(1001, str(error)), 400
        envelope = ResponseEnvelopeV1.success(
            202,
            "命令已受理",
            data={"command_id": record.commandId, "status": record.status.value},
        )
        return envelope, 202

    def get(self, commandId: str) -> tuple[ResponseEnvelopeV1, int]:
        record = self._service.get(commandId)
        if record is None:
            return self._error(1002, "命令不存在"), 404
        return ResponseEnvelopeV1.success(0, "命令状态", data=_wireStatus(record)), 200

    def cancel(self, commandId: str) -> tuple[ResponseEnvelopeV1, int]:
        try:
            record = self._service.transition(commandId, CommandStatus.CancelRequested)
        except CommandStateConflict as error:
            return self._error(3000, str(error)), 422
        except CommandError as error:
            return self._error(1002, str(error)), 404
        return ResponseEnvelopeV1.success(0, "取消请求已受理", data=_wireStatus(record)), 200

    def _error(self, code: int, message: str) -> ResponseEnvelopeV1:
        definition = self._catalog.getError(code)
        return ResponseEnvelopeV1.model_validate(
            {
                "code": definition.code,
                "message": message,
                "error": {
                    "code": definition.errorCode,
                    "catalog_version": self._catalog.catalogVersion,
                    "retryable": definition.retryable,
                },
            }
        )


def buildCommandRouter(api: CommandApi) -> APIRouter:
    """注册命令路由；依赖通过构造参数注入。"""
    router = APIRouter(prefix=_COMMANDS_PREFIX, tags=["commands"])

    @router.post("")
    async def submitCommand(payload: dict[str, Any]) -> JSONResponse:
        try:
            envelope, status = api.submit(
                commandId=_string(payload, "command_id"),
                commandType=_string(payload, "command_type"),
                accountId=_string(payload, "account_id"),
                runId=_string(payload, "run_id"),
                requestedBy=_string(payload, "requested_by"),
                idempotencyKey=_string(payload, "idempotency_key"),
                payload=payload.get("payload") or {},
                expectedVersion=_optionalString(payload, "expected_version"),
            )
        except CommandError as error:
            envelope, status = api._error(1001, str(error)), 400  # noqa: SLF001 - 校验失败统一 400
        return JSONResponse(status_code=status, content=envelope.toWire())

    @router.get("/{command_id}")
    async def getCommand(command_id: str) -> JSONResponse:
        envelope, status = api.get(command_id)
        return JSONResponse(status_code=status, content=envelope.toWire())

    @router.post("/{command_id}/cancel")
    async def cancelCommand(command_id: str) -> JSONResponse:
        envelope, status = api.cancel(command_id)
        return JSONResponse(status_code=status, content=envelope.toWire())

    return router


def _string(payload: Mapping[str, Any], key: str) -> str:
    value = payload.get(key)
    if not isinstance(value, str) or not value:
        raise CommandError(f"{key} 必须为非空字符串")
    return value


def _optionalString(payload: Mapping[str, Any], key: str) -> str | None:
    value = payload.get(key)
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise CommandError(f"{key} 必须为非空字符串")
    return value
