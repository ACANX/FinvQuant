"""已发布错误目录与候选目录的稳定性比对。"""

from __future__ import annotations

from veritasquant.application.ApiErrors import ApiErrorCatalog, ApiErrorCatalogError


def validateCatalogCompatibility(previous: ApiErrorCatalog, candidate: ApiErrorCatalog) -> None:
    """阻止已发布错误码删除、复用或改变 HTTP/重试/领域语义。"""
    for code, previousDefinition in previous.errorsByCode.items():
        try:
            currentDefinition = candidate.getError(code)
        except ApiErrorCatalogError as error:
            raise ApiErrorCatalogError(f"已发布错误码不得删除: {code}") from error
        # 这些字段共同决定客户端处理和外部协议含义，任何变化都必须提升主版本并迁移。
        stableFields = ("errorCode", "domain", "httpStatus", "retryable", "messageKey")
        if any(
            getattr(previousDefinition, field) != getattr(currentDefinition, field)
            for field in stableFields
        ):
            raise ApiErrorCatalogError(f"已发布错误码不得复用或改变语义: {code}")
