"""UTC 时间精度契约。"""

from __future__ import annotations

import re
from datetime import datetime, timezone
from enum import StrEnum


class TsPrecision(StrEnum):
    """运行级事件时间精度。"""

    Second = "Second"
    Millisecond = "Millisecond"


class TimestampPrecisionError(ValueError):
    """时间不是 UTC 或无法由当前运行精度表示。"""


_SECOND_PATTERN = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
_MILLISECOND_PATTERN = re.compile(
    r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$"
)


def validateUtcTimestamp(value: datetime, tsPrecision: TsPrecision) -> datetime:
    """校验 UTC 和运行精度，绝不隐式改变输入时间。"""
    if value.tzinfo is None or value.utcoffset() != timezone.utc.utcoffset(value):
        raise TimestampPrecisionError("时间必须为带 UTC 时区的 datetime")
    if value.microsecond % 1_000 != 0:
        raise TimestampPrecisionError("时间精度超过 Millisecond")
    if tsPrecision is TsPrecision.Second and value.microsecond != 0:
        raise TimestampPrecisionError("Second 运行不接受含毫秒的时间")
    return value.astimezone(timezone.utc)


def parseUtcTimestamp(value: str | datetime, tsPrecision: TsPrecision) -> datetime:
    """解析严格 Z 格式的秒或毫秒 UTC 时间。"""
    if isinstance(value, datetime):
        return validateUtcTimestamp(value, tsPrecision)
    if not isinstance(value, str):
        raise TimestampPrecisionError("时间输入必须是 datetime 或 UTC 字符串")
    if _SECOND_PATTERN.fullmatch(value):
        parsed = datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(
            tzinfo=timezone.utc
        )
    elif _MILLISECOND_PATTERN.fullmatch(value):
        parsed = datetime.strptime(value, "%Y-%m-%dT%H:%M:%S.%fZ").replace(
            tzinfo=timezone.utc
        )
    else:
        raise TimestampPrecisionError("时间必须为 YYYY-MM-DDTHH:mm:ss[.SSS]Z")
    return validateUtcTimestamp(parsed, tsPrecision)


def serializeUtcTimestamp(value: datetime, tsPrecision: TsPrecision) -> str:
    """按运行精度输出固定宽度 UTC 时间。"""
    normalized = validateUtcTimestamp(value, tsPrecision)
    if tsPrecision is TsPrecision.Second:
        return normalized.strftime("%Y-%m-%dT%H:%M:%SZ")
    return normalized.strftime("%Y-%m-%dT%H:%M:%S.") + f"{normalized.microsecond // 1_000:03d}Z"
