"""分层配置、严格 YAML 加载与不可变 config_hash。"""

from __future__ import annotations

import re
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from decimal import Decimal, InvalidOperation
from pathlib import Path
from types import MappingProxyType
from typing import Any, TypeVar

import yaml
from pydantic import field_validator
from yaml.constructor import ConstructorError

from veritasquant.core.CanonicalJson import canonicalHash, canonicalize
from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision


class ConfigError(ValueError):
    """配置内容、层级或快照规则不满足时抛出。"""


class _StrictYamlLoader(yaml.SafeLoader):
    """拒绝重复键，并将 YAML 浮点字面量交给 Decimal 规范化路径。"""


def _constructMapping(loader: _StrictYamlLoader, node: yaml.MappingNode, deep: bool = False) -> dict[str, Any]:
    loader.flatten_mapping(node)
    mapping: dict[str, Any] = {}
    for keyNode, valueNode in node.value:
        key = loader.construct_object(keyNode, deep=deep)
        if not isinstance(key, str):
            raise ConstructorError("while constructing a mapping", node.start_mark, "配置键必须为字符串", keyNode.start_mark)
        if key in mapping:
            raise ConstructorError("while constructing a mapping", node.start_mark, f"发现重复键: {key}", keyNode.start_mark)
        mapping[key] = loader.construct_object(valueNode, deep=deep)
    return mapping


_StrictYamlLoader.add_constructor(yaml.resolver.BaseResolver.DEFAULT_MAPPING_TAG, _constructMapping)


def _constructDecimal(loader: _StrictYamlLoader, node: yaml.ScalarNode) -> Decimal:
    """保留 YAML 小数的十进制语义，避免 float 进入配置身份哈希。"""
    try:
        value = Decimal(loader.construct_scalar(node).replace("_", ""))
    except InvalidOperation as error:
        raise ConstructorError("while constructing a decimal", node.start_mark, "YAML 小数非法", node.start_mark) from error
    if not value.is_finite():
        raise ConstructorError("while constructing a decimal", node.start_mark, "YAML 小数必须有限", node.start_mark)
    return value


_StrictYamlLoader.add_constructor("tag:yaml.org,2002:float", _constructDecimal)


def loadYaml(path: Path) -> dict[str, Any]:
    """读取 UTF-8 YAML，拒绝空文档、根数组和重复键。"""
    try:
        contents = path.read_text(encoding="utf-8")
        loaded = yaml.load(contents, Loader=_StrictYamlLoader)
    except (OSError, yaml.YAMLError) as error:
        raise ConfigError(f"无法加载 YAML: {path}") from error
    if not isinstance(loaded, dict):
        raise ConfigError("配置 YAML 根节点必须为对象")
    return loaded


@dataclass(frozen=True)
class ConfigLayer:
    """一个从低到高优先级参与合并的原始配置层。"""

    name: str
    values: Mapping[str, Any]


@dataclass(frozen=True)
class FrozenConfig:
    """运行创建后不可变的配置快照与身份元数据。"""

    configHash: str
    configSchemaVersion: str
    parserVersion: str
    layerHashes: Mapping[str, str]
    snapshot: Mapping[str, Any]


class RuntimeConfigV1(StrictModel):
    """阶段 1 可用于配置机制验证的最小运行配置 Schema。"""

    configSchemaVersion: str = PascalAlias("ConfigSchemaVersion", min_length=1)
    executionMode: str = PascalAlias("ExecutionMode", min_length=1)
    tsPrecision: TsPrecision = PascalAlias("TsPrecision")
    riskPolicyVersion: str = PascalAlias("RiskPolicyVersion", min_length=1)

    @field_validator("tsPrecision", mode="before")
    @classmethod
    def parseTsPrecision(cls, value: Any) -> TsPrecision:
        if isinstance(value, TsPrecision):
            return value
        if isinstance(value, str):
            try:
                return TsPrecision(value)
            except ValueError as error:
                raise ConfigError("TsPrecision 只能为 Second 或 Millisecond") from error
        raise ConfigError("TsPrecision 必须为字符串")


_ModelType = TypeVar("_ModelType", bound=StrictModel)
_SECRET_KEYS = re.compile(r"password|token|secret|private.?key|credential|api.?key", re.IGNORECASE)
_SECRET_REFERENCE_SUFFIX = re.compile(r"(reference|ref|id|version)$", re.IGNORECASE)
_ABSOLUTE_PATH = re.compile(r"^(?:[A-Za-z]:[\\/]|\\\\|/)")


def mergeConfigLayers(
    layers: Sequence[ConfigLayer], overrideablePaths: frozenset[tuple[str, ...]] = frozenset()
) -> dict[str, Any]:
    """按低到高优先级递归合并，并拒绝未声明的覆盖。"""
    if not layers:
        raise ConfigError("至少需要一个配置层")
    result: dict[str, Any] = {}
    for layer in layers:
        if not layer.name:
            raise ConfigError("配置层必须有名称")
        result = _mergeMapping(result, dict(layer.values), overrideablePaths, ())
    return result


def _mergeMapping(
    lower: Mapping[str, Any],
    higher: Mapping[str, Any],
    overrideablePaths: frozenset[tuple[str, ...]],
    prefix: tuple[str, ...],
) -> dict[str, Any]:
    merged = dict(lower)
    for key, highValue in higher.items():
        if not isinstance(key, str):
            raise ConfigError("配置键必须为字符串")
        path = prefix + (key,)
        if key not in merged:
            merged[key] = highValue
            continue
        lowValue = merged[key]
        if isinstance(lowValue, Mapping) and isinstance(highValue, Mapping):
            merged[key] = _mergeMapping(lowValue, highValue, overrideablePaths, path)
            continue
        if lowValue != highValue and path not in overrideablePaths:
            raise ConfigError(f"字段未标记为可覆盖: {'.'.join(path)}")
        merged[key] = highValue
    return merged


def freezeConfig(
    config: _ModelType, layers: Sequence[ConfigLayer], parserVersion: str = "V1"
) -> FrozenConfig:
    """展开 Schema 后生成不含密钥或机器绝对路径的规范快照。"""
    if not isinstance(config, StrictModel):
        raise ConfigError("配置必须使用 StrictModel")
    snapshot = config.model_dump(mode="python", by_alias=True, exclude_none=False)
    _rejectSecretsAndAbsolutePaths(snapshot)
    normalized = canonicalize(snapshot, config.tsPrecision if hasattr(config, "tsPrecision") else TsPrecision.Millisecond)
    layerHashes = {layer.name: canonicalHash(dict(layer.values)) for layer in layers}
    return FrozenConfig(
        configHash=canonicalHash(normalized, config.tsPrecision if hasattr(config, "tsPrecision") else TsPrecision.Millisecond),
        configSchemaVersion=str(getattr(config, "configSchemaVersion", "")),
        parserVersion=parserVersion,
        layerHashes=MappingProxyType(layerHashes),
        snapshot=_freezeMapping(normalized),
    )


def loadAndFreezeConfig(
    modelType: type[_ModelType],
    layers: Sequence[ConfigLayer],
    overrideablePaths: frozenset[tuple[str, ...]] = frozenset(),
    parserVersion: str = "V1",
) -> FrozenConfig:
    """合并、严格校验并冻结一个版本化配置。"""
    merged = mergeConfigLayers(layers, overrideablePaths)
    try:
        config = modelType.model_validate(merged)
    except Exception as error:
        raise ConfigError("配置未通过严格 Pydantic Schema 校验") from error
    return freezeConfig(config, layers, parserVersion)


def _rejectSecretsAndAbsolutePaths(value: Any, path: tuple[str, ...] = ()) -> None:
    if isinstance(value, Mapping):
        for key, nestedValue in value.items():
            if not isinstance(key, str):
                raise ConfigError("配置键必须为字符串")
            if _SECRET_KEYS.search(key) and not _SECRET_REFERENCE_SUFFIX.search(key):
                raise ConfigError(f"配置快照不得包含密钥值: {'.'.join(path + (key,))}")
            _rejectSecretsAndAbsolutePaths(nestedValue, path + (key,))
        return
    if isinstance(value, list):
        for index, nestedValue in enumerate(value):
            _rejectSecretsAndAbsolutePaths(nestedValue, path + (str(index),))
        return
    if isinstance(value, str) and _ABSOLUTE_PATH.match(value):
        raise ConfigError(f"配置快照不得包含机器绝对路径: {'.'.join(path)}")


def _freezeMapping(value: Any) -> Any:
    if isinstance(value, dict):
        return MappingProxyType({key: _freezeMapping(item) for key, item in value.items()})
    if isinstance(value, list):
        return tuple(_freezeMapping(item) for item in value)
    return value
