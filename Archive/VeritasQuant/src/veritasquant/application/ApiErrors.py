"""统一业务错误目录加载、验证与公开详情过滤。"""

from __future__ import annotations

import re
from collections.abc import Mapping
from dataclasses import dataclass
from importlib import resources
from types import MappingProxyType
from typing import Any

import yaml


class ApiErrorCatalogError(ValueError):
    """错误目录不满足稳定性或号段契约。"""


class BusinessException(Exception):
    """领域层唯一允许抛出的已注册业务错误。"""

    def __init__(self, code: int, details: Mapping[str, Any] | None = None):
        super().__init__(str(code))
        self.code = code
        self.details = dict(details or {})


@dataclass(frozen=True)
class ApiErrorDefinition:
    """目录中一个不可变的错误码定义。"""

    code: int
    errorCode: str
    domain: str
    httpStatus: int
    retryable: bool
    messageKey: str
    introducedVersion: str
    deprecated: bool
    detailSchema: Mapping[str, str]


@dataclass(frozen=True)
class BusinessStatusDefinition:
    """显式注册的 2-999 非错误业务状态。"""

    code: int
    routes: tuple[str, ...]
    messageKey: str
    allowData: bool
    allowDetails: bool


_SUCCESS_CODES = frozenset({0, 1, 200, 202})
_ERROR_CODE_PATTERN = re.compile(r"^[A-Z][A-Z0-9_]*$")
_DOMAIN_RANGES: dict[str, range] = {
    "General": range(3000, 4000),
    "Data": range(4000, 5000),
    "Strategy": range(5000, 6000),
    "Account": range(6000, 7000),
    "Risk": range(7000, 8000),
    "Execution": range(8000, 9000),
    "Fund": range(9000, 10000),
    "Instrument": range(10000, 11000),
    "Scheduling": range(11000, 12000),
    "Reporting": range(12000, 13000),
}
_PLATFORM_DOMAINS = frozenset({"Platform", "Security", "Dependency"})


class ApiErrorCatalog:
    """冻结的数值码和符号码双向索引。"""

    def __init__(
        self,
        catalogVersion: str,
        errorsByCode: Mapping[int, ApiErrorDefinition],
        errorsBySymbol: Mapping[str, ApiErrorDefinition],
        businessStatuses: Mapping[int, BusinessStatusDefinition],
    ) -> None:
        self.catalogVersion = catalogVersion
        self.errorsByCode = MappingProxyType(dict(errorsByCode))
        self.errorsBySymbol = MappingProxyType(dict(errorsBySymbol))
        self.businessStatuses = MappingProxyType(dict(businessStatuses))

    @classmethod
    def loadPackaged(cls) -> "ApiErrorCatalog":
        """通过 importlib.resources 加载 wheel 内置目录，不依赖工作目录。"""
        resource = resources.files("veritasquant.resources").joinpath("Schemas", "ApiErrorCodes.yml")
        try:
            raw = yaml.safe_load(resource.read_text(encoding="utf-8"))
        except (OSError, yaml.YAMLError) as error:
            raise ApiErrorCatalogError("无法加载内置 ApiErrorCodes.yml") from error
        return cls.fromMapping(raw)

    @classmethod
    def fromMapping(cls, raw: Any) -> "ApiErrorCatalog":
        """严格校验完整 YAML 目录后构建索引。"""
        if not isinstance(raw, Mapping):
            raise ApiErrorCatalogError("错误目录根节点必须为对象")
        allowedTopLevel = {"ErrorCatalogVersion", "SuccessCodes", "BusinessStatuses", "Errors"}
        _rejectUnknown(raw, allowedTopLevel, "错误目录")
        catalogVersion = raw.get("ErrorCatalogVersion")
        if not isinstance(catalogVersion, str) or not catalogVersion:
            raise ApiErrorCatalogError("ErrorCatalogVersion 必须为非空字符串")
        successCodes = raw.get("SuccessCodes")
        if not isinstance(successCodes, list) or frozenset(successCodes) != _SUCCESS_CODES or len(successCodes) != len(_SUCCESS_CODES):
            raise ApiErrorCatalogError("SuccessCodes 必须且只能为 [0, 1, 200, 202]")
        rawErrors = raw.get("Errors")
        if not isinstance(rawErrors, list) or not rawErrors:
            raise ApiErrorCatalogError("Errors 必须为非空数组")
        errorsByCode: dict[int, ApiErrorDefinition] = {}
        errorsBySymbol: dict[str, ApiErrorDefinition] = {}
        for index, item in enumerate(rawErrors):
            definition = _parseError(item, index)
            if definition.code in errorsByCode:
                raise ApiErrorCatalogError(f"错误数值码重复: {definition.code}")
            if definition.errorCode in errorsBySymbol:
                raise ApiErrorCatalogError(f"错误符号码重复: {definition.errorCode}")
            errorsByCode[definition.code] = definition
            errorsBySymbol[definition.errorCode] = definition
        statuses = _parseBusinessStatuses(raw.get("BusinessStatuses", []), errorsByCode)
        if 2006 not in errorsByCode:
            raise ApiErrorCatalogError("错误目录必须注册 2006 INTERNAL_SERVER_ERROR")
        return cls(catalogVersion, errorsByCode, errorsBySymbol, statuses)

    def getError(self, code: int) -> ApiErrorDefinition:
        try:
            return self.errorsByCode[code]
        except KeyError as error:
            raise ApiErrorCatalogError(f"未注册错误码: {code}") from error

    def filterPublicDetails(self, code: int, details: Mapping[str, Any]) -> dict[str, Any]:
        """仅允许目录 DetailSchema 中声明的 lowerCamelCase 详情字段。"""
        definition = self.getError(code)
        expected = {_lowerCamel(key): (key, schema) for key, schema in definition.detailSchema.items()}
        filtered: dict[str, Any] = {}
        for key, value in details.items():
            if key not in expected:
                raise ApiErrorCatalogError(f"错误 {code} 包含未公开详情字段: {key}")
            _, schema = expected[key]
            _validateDetailValue(key, value, schema)
            filtered[_snakeCase(key)] = value
        return filtered


def _parseError(raw: Any, index: int) -> ApiErrorDefinition:
    if not isinstance(raw, Mapping):
        raise ApiErrorCatalogError(f"Errors[{index}] 必须为对象")
    required = {
        "Code",
        "ErrorCode",
        "Domain",
        "HttpStatus",
        "Retryable",
        "MessageKey",
        "IntroducedVersion",
        "Deprecated",
        "DetailSchema",
    }
    _rejectUnknown(raw, required, f"Errors[{index}]")
    if set(raw) != required:
        missing = sorted(required - set(raw))
        raise ApiErrorCatalogError(f"Errors[{index}] 缺少字段: {', '.join(missing)}")
    code = raw["Code"]
    errorCode = raw["ErrorCode"]
    domain = raw["Domain"]
    httpStatus = raw["HttpStatus"]
    retryable = raw["Retryable"]
    messageKey = raw["MessageKey"]
    introducedVersion = raw["IntroducedVersion"]
    deprecated = raw["Deprecated"]
    detailSchema = raw["DetailSchema"]
    if not isinstance(code, int) or isinstance(code, bool) or code < 1000:
        raise ApiErrorCatalogError("错误 Code 必须为 >=1000 的整数")
    if not isinstance(errorCode, str) or not _ERROR_CODE_PATTERN.fullmatch(errorCode):
        raise ApiErrorCatalogError(f"错误符号码非法: {errorCode}")
    if not isinstance(domain, str) or not _isCodeInDomain(code, domain):
        raise ApiErrorCatalogError(f"错误码 {code} 不属于 Domain {domain} 的号段")
    if not isinstance(httpStatus, int) or not 400 <= httpStatus <= 599:
        raise ApiErrorCatalogError("HttpStatus 必须为 400-599")
    if not isinstance(retryable, bool) or not isinstance(deprecated, bool):
        raise ApiErrorCatalogError("Retryable 和 Deprecated 必须为 boolean")
    if not isinstance(messageKey, str) or not messageKey or not isinstance(introducedVersion, str) or not introducedVersion:
        raise ApiErrorCatalogError("MessageKey 和 IntroducedVersion 必须为非空字符串")
    if not isinstance(detailSchema, Mapping) or any(not isinstance(key, str) or not isinstance(value, str) for key, value in detailSchema.items()):
        raise ApiErrorCatalogError("DetailSchema 必须是 PascalCase 到类型名的对象")
    return ApiErrorDefinition(code, errorCode, domain, httpStatus, retryable, messageKey, introducedVersion, deprecated, MappingProxyType(dict(detailSchema)))


def _parseBusinessStatuses(raw: Any, errorsByCode: Mapping[int, ApiErrorDefinition]) -> dict[int, BusinessStatusDefinition]:
    if not isinstance(raw, list):
        raise ApiErrorCatalogError("BusinessStatuses 必须为数组")
    statuses: dict[int, BusinessStatusDefinition] = {}
    required = {"Code", "Routes", "MessageKey", "AllowData", "AllowDetails"}
    for index, item in enumerate(raw):
        if not isinstance(item, Mapping):
            raise ApiErrorCatalogError(f"BusinessStatuses[{index}] 必须为对象")
        _rejectUnknown(item, required, f"BusinessStatuses[{index}]")
        if set(item) != required:
            raise ApiErrorCatalogError(f"BusinessStatuses[{index}] 字段不完整")
        code = item["Code"]
        if not isinstance(code, int) or code in _SUCCESS_CODES or not 2 <= code <= 999 or code in errorsByCode or code in statuses:
            raise ApiErrorCatalogError(f"非错误业务状态 Code 非法或重复: {code}")
        routes = item["Routes"]
        if not isinstance(routes, list) or not routes or any(not isinstance(route, str) or not route for route in routes):
            raise ApiErrorCatalogError("BusinessStatuses.Routes 必须为非空字符串数组")
        if not isinstance(item["MessageKey"], str) or not item["MessageKey"] or not isinstance(item["AllowData"], bool) or not isinstance(item["AllowDetails"], bool):
            raise ApiErrorCatalogError("BusinessStatuses 字段类型非法")
        statuses[code] = BusinessStatusDefinition(code, tuple(routes), item["MessageKey"], item["AllowData"], item["AllowDetails"])
    return statuses


def _isCodeInDomain(code: int, domain: str) -> bool:
    if domain in _PLATFORM_DOMAINS:
        return 1000 <= code <= 2999
    return code in _DOMAIN_RANGES.get(domain, range(0))


def _rejectUnknown(raw: Mapping[str, Any], allowed: set[str], location: str) -> None:
    unknown = set(raw) - allowed
    if unknown:
        raise ApiErrorCatalogError(f"{location} 存在未知字段: {', '.join(sorted(unknown))}")


def _lowerCamel(value: str) -> str:
    return value[:1].lower() + value[1:]


def _snakeCase(value: str) -> str:
    return re.sub(r"(?<!^)([A-Z])", r"_\1", value).lower()


def _validateDetailValue(key: str, value: Any, schema: str) -> None:
    if schema == "DecimalString":
        if not isinstance(value, str) or not re.fullmatch(r"-?(?:0|[1-9]\d*)(?:\.\d+)?", value):
            raise ApiErrorCatalogError(f"详情 {key} 必须为 DecimalString")
    elif schema == "String" and not isinstance(value, str):
        raise ApiErrorCatalogError(f"详情 {key} 必须为 String")
    elif schema == "Integer" and (not isinstance(value, int) or isinstance(value, bool)):
        raise ApiErrorCatalogError(f"详情 {key} 必须为 Integer")
    elif schema == "Boolean" and not isinstance(value, bool):
        raise ApiErrorCatalogError(f"详情 {key} 必须为 Boolean")
