from __future__ import annotations

import io
import json
import logging
import queue

from veritasquant.core.Time import TsPrecision
from veritasquant.monitoring.StructuredLogging import BoundedQueueHandler, JsonLogFormatter, bindLogContext, configureStructuredLogging, resetLogContext


def test_json_logging_carries_context_and_redacts_secret_and_pii() -> None:
    output = io.StringIO()
    runtime = configureStructuredLogging("unit-test", "TEST", TsPrecision.Millisecond, stream=output)
    token = bindLogContext(runId="run-1", executionMode="BACKTEST", eventId="evt-1")
    try:
        runtime.logger.info("token=hidden contact user@example.com", extra={"structuredData": {"password": "hidden", "quantity": 1}})
    finally:
        resetLogContext(token)
        runtime.stop()
    record = json.loads(output.getvalue())
    assert record["run_id"] == "run-1"
    assert record["event_id"] == "evt-1"
    assert "hidden" not in output.getvalue()
    assert "user@example.com" not in output.getvalue()
    assert record["data"]["password"] == "[REDACTED]"


def test_bounded_queue_failure_does_not_raise_or_block_error_record() -> None:
    output = io.StringIO()
    fallback = logging.StreamHandler(output)
    fallback.setFormatter(JsonLogFormatter("unit-test", "TEST", TsPrecision.Second))
    logQueue: queue.Queue[logging.LogRecord] = queue.Queue(maxsize=1)
    logQueue.put(logging.LogRecord("test", logging.INFO, __file__, 1, "occupied", (), None))
    handler = BoundedQueueHandler(logQueue, fallback)
    handler.emit(logging.LogRecord("test", logging.ERROR, __file__, 1, "password=hidden", (), None))
    assert handler.droppedCount == 1
    assert "hidden" not in output.getvalue()
