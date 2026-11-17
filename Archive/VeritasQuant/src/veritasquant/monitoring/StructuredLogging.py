"""标准库 logging 的结构化 JSON、上下文、脱敏与有界队列封装。"""

from __future__ import annotations

import json
import logging
import queue
import re
import sys
from collections.abc import Iterator, Mapping
from contextlib import contextmanager
from contextvars import ContextVar, Token
from datetime import datetime, timezone
from decimal import Decimal
from logging.handlers import QueueListener
from typing import Any, TextIO

from veritasquant.core.CanonicalJson import canonicalDecimal
from veritasquant.core.Time import TsPrecision, serializeUtcTimestamp


class StructuredLoggingError(ValueError):
    """结构化日志调用违反上下文字段约束时抛出。"""


# 这些字段是跨模块检索所需的唯一关联标识，避免任意上下文带入敏感信息。
_CONTEXT_FIELDS = frozenset(
    {
        "runId",
        "executionMode",
        "eventId",
        "correlationId",
        "accountId",
        "subaccountId",
        "strategyId",
        "orderId",
        "executionId",
        "alertId",
    }
)
_WIRE_CONTEXT_FIELDS = {
    "runId": "run_id",
    "executionMode": "execution_mode",
    "eventId": "event_id",
    "correlationId": "correlation_id",
    "accountId": "account_id",
    "subaccountId": "subaccount_id",
    "strategyId": "strategy_id",
    "orderId": "order_id",
    "executionId": "execution_id",
    "alertId": "alert_id",
}
_SENSITIVE_KEY = re.compile(r"password|token|secret|private.?key|credential|api.?key|authorization", re.IGNORECASE)
_SENSITIVE_VALUE = re.compile(r"(?i)(password|token|secret|api[_-]?key|authorization)\s*[:=]\s*[^\s,;]+")
_EMAIL_VALUE = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
_logContext: ContextVar[Mapping[str, Any]] = ContextVar("veritasquantLogContext", default={})


def bindLogContext(**context: Any) -> Token[Mapping[str, Any]]:
    """向当前异步上下文绑定允许的关联标识，并返回可恢复 token。"""
    unknown = set(context) - _CONTEXT_FIELDS
    if unknown:
        raise StructuredLoggingError(f"不允许的日志上下文字段: {', '.join(sorted(unknown))}")
    current = dict(_logContext.get())
    current.update(context)
    return _logContext.set(current)


def resetLogContext(token: Token[Mapping[str, Any]]) -> None:
    """恢复绑定前上下文，避免请求或账户间关联 ID 串扰。"""
    _logContext.reset(token)


@contextmanager
def logContext(**context: Any) -> Iterator[None]:
    """以作用域方式绑定日志上下文。"""
    token = bindLogContext(**context)
    try:
        yield
    finally:
        resetLogContext(token)


class JsonLogFormatter(logging.Formatter):
    """输出稳定字段集合的单行 UTF-8 JSON，不输出异常堆栈。"""

    def __init__(self, service: str, environment: str, tsPrecision: TsPrecision) -> None:
        super().__init__()
        self.service = service
        self.environment = environment
        self.tsPrecision = tsPrecision

    def format(self, record: logging.LogRecord) -> str:
        # record.created 是日志采集时刻，只用于诊断，绝不参与事件因果排序。
        # 此处显式按运行精度对齐日志时钟；事件 ts 的输入路径仍禁止隐式降精度。
        ts = datetime.fromtimestamp(record.created, timezone.utc)
        if self.tsPrecision is TsPrecision.Second:
            ts = ts.replace(microsecond=0)
        else:
            ts = ts.replace(microsecond=(ts.microsecond // 1_000) * 1_000)
        # 异步队列由另一线程格式化，优先使用生产线程入队时冻结的上下文快照。
        context = dict(getattr(record, "structuredContext", _logContext.get()))
        for internalName in _CONTEXT_FIELDS:
            if hasattr(record, internalName):
                context[internalName] = getattr(record, internalName)
        payload: dict[str, Any] = {
            "ts": serializeUtcTimestamp(ts, self.tsPrecision),
            "level": record.levelname,
            "logger": record.name,
            "message": _redactValue(record.getMessage()),
            "service": self.service,
            "environment": self.environment,
            "run_id": _redactValue(context.get("runId")),
            "execution_mode": _redactValue(context.get("executionMode")),
        }
        for internalName, wireName in _WIRE_CONTEXT_FIELDS.items():
            if internalName not in {"runId", "executionMode"} and internalName in context:
                payload[wireName] = _redactValue(context[internalName])
        if hasattr(record, "structuredData"):
            payload["data"] = _redactValue(getattr(record, "structuredData"))
        if record.exc_info is not None:
            # 仅保留异常类型，禁止把可能含秘密或路径的堆栈写入诊断日志。
            exceptionType = record.exc_info[0]
            if exceptionType is not None:
                payload["error_type"] = exceptionType.__name__
        return json.dumps(_jsonSafe(payload, self.tsPrecision), ensure_ascii=False, separators=(",", ":"), sort_keys=True)


class BoundedQueueHandler(logging.Handler):
    """非阻塞日志入口；队列故障不可反向阻断交易保护动作。"""

    def __init__(self, logQueue: queue.Queue[logging.LogRecord], fallback: logging.Handler) -> None:
        super().__init__()
        self.logQueue = logQueue
        self.fallback = fallback
        self.droppedCount = 0

    def emit(self, record: logging.LogRecord) -> None:
        try:
            # contextvars 不会自动跨 QueueListener 线程传播，必须在调用线程捕获。
            record.structuredContext = dict(_logContext.get())
            self.logQueue.put_nowait(record)
        except Exception:
            self.droppedCount += 1
            # ERROR/CRITICAL 仍尽力直接输出；任何失败都被吞掉以保持调用方非阻塞。
            if record.levelno >= logging.ERROR:
                try:
                    self.fallback.handle(record)
                except Exception:
                    pass


class StructuredLoggingRuntime:
    """持有队列监听线程，显式 start/stop，模块导入本身没有副作用。"""

    def __init__(self, logger: logging.Logger, handler: BoundedQueueHandler, listener: QueueListener) -> None:
        self.logger = logger
        self.handler = handler
        self.listener = listener
        self._started = False

    def start(self) -> None:
        if not self._started:
            self.listener.start()
            self._started = True

    def stop(self) -> None:
        if self._started:
            self.listener.stop()
            self._started = False


def configureStructuredLogging(
    service: str,
    environment: str,
    tsPrecision: TsPrecision = TsPrecision.Second,
    maxQueueSize: int = 1_000,
    stream: TextIO | None = None,
) -> StructuredLoggingRuntime:
    """创建并启动有界 JSON 日志运行时；重复配置同名 logger 不累加 handler。"""
    if not service or not environment or maxQueueSize <= 0:
        raise StructuredLoggingError("service、environment 和正队列容量均为必填")
    output = logging.StreamHandler(stream or sys.stdout)
    output.setFormatter(JsonLogFormatter(service, environment, tsPrecision))
    fallback = logging.StreamHandler(stream or sys.stderr)
    fallback.setFormatter(JsonLogFormatter(service, environment, tsPrecision))
    logQueue: queue.Queue[logging.LogRecord] = queue.Queue(maxsize=maxQueueSize)
    handler = BoundedQueueHandler(logQueue, fallback)
    logger = logging.getLogger(f"veritasquant.{service}")
    logger.handlers.clear()
    logger.addHandler(handler)
    logger.setLevel(logging.INFO)
    logger.propagate = False
    runtime = StructuredLoggingRuntime(logger, handler, QueueListener(logQueue, output, respect_handler_level=True))
    runtime.start()
    return runtime


def _redactValue(value: Any, key: str | None = None) -> Any:
    if key is not None and _SENSITIVE_KEY.search(key):
        return "[REDACTED]"
    if isinstance(value, Mapping):
        return {str(itemKey): _redactValue(itemValue, str(itemKey)) for itemKey, itemValue in value.items()}
    if isinstance(value, list):
        return [_redactValue(item) for item in value]
    if isinstance(value, tuple):
        return tuple(_redactValue(item) for item in value)
    if isinstance(value, str):
        value = _SENSITIVE_VALUE.sub(lambda match: f"{match.group(1)}=[REDACTED]", value)
        return _EMAIL_VALUE.sub("[REDACTED_EMAIL]", value)
    return value


def _jsonSafe(value: Any, tsPrecision: TsPrecision) -> Any:
    if value is None or isinstance(value, (str, bool, int)):
        return value
    if isinstance(value, Decimal):
        return canonicalDecimal(value)
    if isinstance(value, datetime):
        return serializeUtcTimestamp(value, tsPrecision)
    if isinstance(value, Mapping):
        return {str(key): _jsonSafe(item, tsPrecision) for key, item in value.items()}
    if isinstance(value, (list, tuple)):
        return [_jsonSafe(item, tsPrecision) for item in value]
    return str(value)
