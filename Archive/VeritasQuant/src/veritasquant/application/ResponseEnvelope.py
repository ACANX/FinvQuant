"""REST ResponseEnvelopeV1 与统一异常边界映射。"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Mapping

from pydantic import model_validator

from veritasquant.application.ApiErrors import ApiErrorCatalog, ApiErrorCatalogError, BusinessException
from veritasquant.core.Models import SnakeAlias, StrictModel


class ResponseErrorV1(StrictModel):
    """所有错误响应必需的稳定嵌套字段。"""

    errorCode: str = SnakeAlias("code", min_length=1)
    catalogVersion: str = SnakeAlias("catalog_version", min_length=1)
    retryable: bool = SnakeAlias("retryable")


class ResponseEnvelopeV1(StrictModel):
    """所有 REST JSON 响应统一使用的 v1 信封。"""

    code: int = SnakeAlias("code")
    message: str = SnakeAlias("message", min_length=1)
    data: Any | None = SnakeAlias("data", default=None)
    error: ResponseErrorV1 | None = SnakeAlias("error", default=None)
    details: Mapping[str, Any] | None = SnakeAlias("details", default=None)
    requestId: str | None = SnakeAlias("request_id", default=None)
    traceId: str | None = SnakeAlias("trace_id", default=None)

    @model_validator(mode="after")
    def validateCodeAndError(self) -> "ResponseEnvelopeV1":
        if self.code in {0, 1, 200, 202} or 2 <= self.code <= 999:
            if self.error is not None:
                raise ValueError("成功或非错误业务状态不得携带 error")
        elif self.code >= 1000:
            if self.error is None:
                raise ValueError("错误响应必须携带 error")
        else:
            raise ValueError("顶层 code 不属于允许空间")
        return self

    @classmethod
    def success(
        cls,
        code: int,
        message: str,
        data: Any | None = None,
        details: Mapping[str, Any] | None = None,
        requestId: str | None = None,
        traceId: str | None = None,
    ) -> "ResponseEnvelopeV1":
        return cls.model_validate(
            {
                "code": code,
                "message": message,
                "data": data,
                "details": details,
                "request_id": requestId,
                "trace_id": traceId,
            }
        )

    def toWire(self) -> dict[str, Any]:
        """省略未使用的可选字段，输出 snake_case JSON 结构。"""
        return self.model_dump(mode="json", by_alias=True, exclude_none=True)


@dataclass(frozen=True)
class MappedApiResponse:
    httpStatus: int
    envelope: ResponseEnvelopeV1


def mapException(
    exception: Exception,
    catalog: ApiErrorCatalog,
    requestId: str | None = None,
    traceId: str | None = None,
) -> MappedApiResponse:
    """将领域异常映射为目录冻结的 HTTP 与响应字段，未知码安全降级。"""
    if isinstance(exception, BusinessException):
        try:
            definition = catalog.getError(exception.code)
            details = catalog.filterPublicDetails(definition.code, exception.details)
            return _errorResponse(definition, catalog, details, requestId, traceId)
        except ApiErrorCatalogError:
            pass
    internal = catalog.getError(2006)
    return _errorResponse(internal, catalog, {}, requestId, traceId)


def _errorResponse(
    definition: Any,
    catalog: ApiErrorCatalog,
    details: Mapping[str, Any],
    requestId: str | None,
    traceId: str | None,
) -> MappedApiResponse:
    payload: dict[str, Any] = {
        "code": definition.code,
        "message": definition.messageKey,
        "error": ResponseErrorV1.model_validate(
            {
                "code": definition.errorCode,
                "catalog_version": catalog.catalogVersion,
                "retryable": definition.retryable,
            }
        ),
        "request_id": requestId,
        "trace_id": traceId,
    }
    if details:
        payload["details"] = dict(details)
    return MappedApiResponse(definition.httpStatus, ResponseEnvelopeV1.model_validate(payload))
