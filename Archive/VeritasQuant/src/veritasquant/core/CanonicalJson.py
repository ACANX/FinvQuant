"""跨平台稳定的规范 JSON 与 SHA-256。"""

from __future__ import annotations

import hashlib
import json
from datetime import datetime
from decimal import Decimal
from typing import Any, Mapping

from pydantic import BaseModel

from veritasquant.core.Time import TsPrecision, serializeUtcTimestamp


def canonicalDecimal(value: Decimal) -> str:
    """输出无指数且没有无意义末尾零的 Decimal 字符串。"""
    if not value.is_finite():
        raise ValueError("规范 JSON 不接受非有限 Decimal")
    text = format(value, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    if text in {"", "-0"}:
        return "0"
    return text


def canonicalize(value: Any, tsPrecision: TsPrecision = TsPrecision.Millisecond) -> Any:
    """转换为只含 JSON 基础类型的确定性结构。"""
    if isinstance(value, BaseModel):
        return canonicalize(
            value.model_dump(mode="python", by_alias=False, exclude_none=False), tsPrecision
        )
    if value is None or isinstance(value, (str, bool, int)):
        return value
    if isinstance(value, float):
        raise TypeError("float 不得进入金额、价格或身份哈希路径")
    if isinstance(value, Decimal):
        return canonicalDecimal(value)
    if isinstance(value, datetime):
        return serializeUtcTimestamp(value, tsPrecision)
    if isinstance(value, Mapping):
        normalized: dict[str, Any] = {}
        for key in sorted(value.keys(), key=lambda item: str(item).encode("utf-8")):
            if not isinstance(key, str):
                raise TypeError("规范 JSON 对象键必须是字符串")
            normalized[key] = canonicalize(value[key], tsPrecision)
        return normalized
    if isinstance(value, (list, tuple)):
        return [canonicalize(item, tsPrecision) for item in value]
    raise TypeError(f"规范 JSON 不支持类型: {type(value).__name__}")


def canonicalJsonBytes(value: Any, tsPrecision: TsPrecision = TsPrecision.Millisecond) -> bytes:
    """返回 UTF-8、排序且无空白的规范 JSON 字节。"""
    return json.dumps(
        canonicalize(value, tsPrecision),
        ensure_ascii=False,
        separators=(",", ":"),
        allow_nan=False,
    ).encode("utf-8")


def canonicalHash(value: Any, tsPrecision: TsPrecision = TsPrecision.Millisecond) -> str:
    """计算规范 JSON 的 SHA-256。"""
    return hashlib.sha256(canonicalJsonBytes(value, tsPrecision)).hexdigest()
