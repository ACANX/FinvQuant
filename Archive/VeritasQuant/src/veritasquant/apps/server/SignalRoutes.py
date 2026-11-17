"""P3-004 人工确认、忽略和成交登记 API 路由。

- POST /api/v1/signals/{signal_id}/actions：登记人工动作（CONFIRM/IGNORE/
  REGISTER_EXECUTION）；每个动作带身份、理由、ts、版本和审计；
- POST /api/v1/signals/{signal_id}/executions：登记人工成交。

动作登记只产生待执行意图，不直接修改内核或账本（P3-005 授权命令消费）。
"""

from __future__ import annotations

from typing import Any, Mapping

from fastapi import APIRouter
from fastapi.responses import JSONResponse

from veritasquant.application.ApiErrors import ApiErrorCatalog
from veritasquant.application.ResponseEnvelope import ResponseEnvelopeV1
from veritasquant.signals.ManualActionService import ManualActionError, ManualActionServiceV1
from veritasquant.signals.SignalReference import (
    IgnoreReasonV1,
    ManualReviewActionV1,
    SignalActionType,
    SignalContractError,
)

_SIGNALS_PREFIX = "/api/v1/signals"


class SignalApi:
    """信号路由依赖：封装动作登记服务与统一错误映射。"""

    def __init__(
        self,
        service: ManualActionServiceV1,
        catalog: ApiErrorCatalog,
        signalResolver: Any | None = None,
    ) -> None:
        self._service = service
        self._catalog = catalog
        # 信号查询端口：由 wiring 注入（如 InMemorySignalStoreV1 的 get）。
        self._signalResolver = signalResolver

    def recordAction(
        self,
        *,
        signalId: str,
        actionType: str,
        operatorId: str,
        reason: str,
        ignoreReasonCode: str | None = None,
        ignoreReasonDetail: str = "",
    ) -> tuple[ResponseEnvelopeV1, int]:
        try:
            actionTypeEnum = self._parseActionType(actionType)
            ignoreReason = None
            if ignoreReasonCode is not None:
                ignoreReason = IgnoreReasonV1.create(
                    reasonCode=ignoreReasonCode, detail=ignoreReasonDetail, source="manual"
                )
            action = self._service.recordAction(
                signal=self._requireSignal(signalId),
                actionType=actionTypeEnum,
                operatorId=operatorId,
                reason=reason,
                ignoreReason=ignoreReason,
            )
        except (ManualActionError, SignalContractError) as error:
            return self._error(1001, str(error)), 400
        return ResponseEnvelopeV1.success(0, "动作已登记", data=_wireAction(action)), 202

    def recordExecution(
        self,
        *,
        signalId: str,
        actionId: str,
        operatorId: str,
        direction: str,
        quantity: str,
        price: str,
        note: str = "",
    ) -> tuple[ResponseEnvelopeV1, int]:
        try:
            signal = self._requireSignal(signalId)
            action = self._service.getAction(actionId)
            if action is None:
                return self._error(1002, "动作不存在"), 404
            execution = self._service.recordExecution(
                signal=signal,
                action=action,
                operatorId=operatorId,
                direction=direction,
                quantity=quantity,
                price=price,
                note=note,
            )
        except (ManualActionError, SignalContractError) as error:
            return self._error(1001, str(error)), 400
        return ResponseEnvelopeV1.success(0, "成交已登记", data=_wireExecution(execution)), 202

    def _requireSignal(self, signalId: str) -> Any:
        """信号查询依赖注入点：由 wiring 提供信号存储查询。"""
        if self._signalResolver is None:
            raise ManualActionError(f"信号存储未配置，无法查询: {signalId}")
        signal = self._signalResolver(signalId)
        if signal is None:
            raise ManualActionError(f"信号不存在: {signalId}")
        return signal

    @staticmethod
    def _parseActionType(value: str) -> SignalActionType:
        try:
            return SignalActionType(value)
        except ValueError as error:
            raise SignalContractError(f"未知动作类型: {value}") from error

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


def _wireAction(action: ManualReviewActionV1) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "action_id": action.actionId,
        "signal_reference_id": action.signalReferenceId,
        "action_type": action.actionType.value,
        "operator_id": action.operatorId,
        "reason": action.reason,
        "acted_at": action.actedAt.isoformat(),
        "version": action.version,
        "audit_trail": list(action.auditTrail),
    }
    if action.ignoreReason is not None:
        payload["ignore_reason"] = {
            "reason_code": action.ignoreReason.reasonCode,
            "detail": action.ignoreReason.detail,
            "source": action.ignoreReason.source,
        }
    return payload


def _wireExecution(execution: Any) -> dict[str, Any]:
    return {
        "execution_id": execution.executionId,
        "signal_reference_id": execution.signalReferenceId,
        "action_id": execution.actionId,
        "operator_id": execution.operatorId,
        "executed_at": execution.executedAt.isoformat(),
        "direction": execution.direction,
        "quantity": execution.quantity,
        "price": execution.price,
        "note": execution.note,
    }


def buildSignalRouter(api: SignalApi) -> APIRouter:
    """注册信号路由；依赖通过构造参数注入。"""
    router = APIRouter(prefix=_SIGNALS_PREFIX, tags=["signals"])

    @router.post("/{signal_id}/actions")
    async def recordAction(signal_id: str, payload: dict[str, Any]) -> JSONResponse:
        try:
            envelope, status = api.recordAction(
                signalId=signal_id,
                actionType=_string(payload, "action_type"),
                operatorId=_string(payload, "operator_id"),
                reason=_string(payload, "reason"),
                ignoreReasonCode=_optionalString(payload, "ignore_reason_code"),
                ignoreReasonDetail=_optionalString(payload, "ignore_reason_detail") or "",
            )
        except (ManualActionError, SignalContractError) as error:
            envelope, status = api._error(1001, str(error)), 400  # noqa: SLF001
        return JSONResponse(status_code=status, content=envelope.toWire())

    @router.post("/{signal_id}/executions")
    async def recordExecution(signal_id: str, payload: dict[str, Any]) -> JSONResponse:
        try:
            envelope, status = api.recordExecution(
                signalId=signal_id,
                actionId=_string(payload, "action_id"),
                operatorId=_string(payload, "operator_id"),
                direction=_string(payload, "direction"),
                quantity=_string(payload, "quantity"),
                price=_string(payload, "price"),
                note=_optionalString(payload, "note") or "",
            )
        except (ManualActionError, SignalContractError) as error:
            envelope, status = api._error(1001, str(error)), 400  # noqa: SLF001
        return JSONResponse(status_code=status, content=envelope.toWire())

    return router


def _string(payload: Mapping[str, Any], key: str) -> str:
    value = payload.get(key)
    if not isinstance(value, str) or not value:
        raise SignalContractError(f"{key} 必须为非空字符串")
    return value


def _optionalString(payload: Mapping[str, Any], key: str) -> str | None:
    value = payload.get(key)
    if value is None:
        return None
    if not isinstance(value, str) or not value:
        raise SignalContractError(f"{key} 必须为非空字符串")
    return value
