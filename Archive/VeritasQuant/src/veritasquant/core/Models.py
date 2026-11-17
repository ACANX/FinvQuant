"""严格 Pydantic 模型与显式别名约束。"""

from __future__ import annotations

import re
from typing import Any

from pydantic import BaseModel, ConfigDict, Field


_PASCAL_CASE = re.compile(r"^[A-Z][A-Za-z0-9]*$")
_SNAKE_CASE = re.compile(r"^[a-z][a-z0-9_]*$")


class AliasContractError(TypeError):
    """模型字段没有唯一显式 wire alias 时抛出。"""


def PascalAlias(wireName: str, default: Any = ..., **kwargs: Any) -> Any:
    """声明项目 YAML 使用的唯一 PascalCase 双向 alias。"""
    if not _PASCAL_CASE.fullmatch(wireName):
        raise AliasContractError(f"PascalCase alias 非法: {wireName}")
    return Field(
        default,
        validation_alias=wireName,
        serialization_alias=wireName,
        **kwargs,
    )


def SnakeAlias(wireName: str, default: Any = ..., **kwargs: Any) -> Any:
    """声明 REST JSON 使用的唯一 snake_case 双向 alias。"""
    if not _SNAKE_CASE.fullmatch(wireName):
        raise AliasContractError(f"snake_case alias 非法: {wireName}")
    return Field(
        default,
        validation_alias=wireName,
        serialization_alias=wireName,
        **kwargs,
    )


class StrictModel(BaseModel):
    """禁止未知字段、隐式类型和按 Python 字段名回填的基类。"""

    model_config = ConfigDict(
        extra="forbid",
        strict=True,
        frozen=True,
        validate_by_alias=True,
        validate_by_name=False,
    )

    @classmethod
    def __pydantic_init_subclass__(cls, **kwargs: Any) -> None:
        super().__pydantic_init_subclass__(**kwargs)
        if cls is StrictModel:
            return
        cls.validateAliasContract()

    @classmethod
    def validateAliasContract(cls) -> None:
        """确认每个项目字段恰有一个相同的输入和输出 alias。"""
        seenAliases: set[str] = set()
        for fieldName, fieldInfo in cls.model_fields.items():
            validationAlias = fieldInfo.validation_alias
            serializationAlias = fieldInfo.serialization_alias
            if not isinstance(validationAlias, str) or not isinstance(serializationAlias, str):
                raise AliasContractError(
                    f"{cls.__name__}.{fieldName} 必须声明唯一字符串 alias"
                )
            if validationAlias != serializationAlias:
                raise AliasContractError(
                    f"{cls.__name__}.{fieldName} 的输入和输出 alias 必须相同"
                )
            if validationAlias in seenAliases:
                raise AliasContractError(
                    f"{cls.__name__} 存在重复 alias: {validationAlias}"
                )
            seenAliases.add(validationAlias)


class EventPayloadV1(StrictModel):
    """注册表中事件强类型载荷的共同基类。"""

