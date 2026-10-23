"""QuoteImportService 测试：MVSV 内容 → 分批 upsert + 批次登记。"""

from __future__ import annotations

from contextlib import contextmanager

import pytest

from veritasquant.application.QuoteImportService import QuoteImportService
from veritasquant.data.MvsvImport import MvsvImportError

_MVSV_CONTENT = (
    "# Format : \"MVSV-1\"\n"
    "# Field : \"ts|dt|o|c|l|h|v|t|cp|cr|p\"\n"
    "# Count : 2\n"
    "# EffectiveTimeZone : \"Asia/Shanghai\"\n"
    "# Code : \"518880\"\n"
    "# Market : \"SSE\"\n"
    "# MarketCode : 1\n"
    "# CurrencyCode : 1\n"
    "# PriceAccuracy : 3\n"
    "# LotSize : 100\n"
    "\n"
    "1785720600|20260803093000|7.001|7.002|6.999|7.003|100000|70010000000|0.001|0.000143|7.000\n"
    "1785720660|20260803093100|7.002|7.001|7.000|7.003|80000|56010000000|0.000|-0.000143|7.002\n"
)


class FakeCursor:
    def __init__(self, rows: list[tuple] | None = None) -> None:
        self.rows = list(rows or [])
        self.executed: list[tuple] = []

    def fetchall(self) -> list[tuple]:
        rows, self.rows = self.rows, []
        return rows

    def fetchone(self) -> tuple | None:
        return self.rows.pop(0) if self.rows else None

    def execute(self, sql: str, params: tuple | None = None) -> "FakeCursor":
        self.executed.append((sql, params))
        return self

    def executemany(self, sql: str, paramsSeq: list[tuple]) -> "FakeCursor":
        self.executed.append((sql, paramsSeq))
        return self


class FakeConnection:
    def __init__(self) -> None:
        self.sql_log: list[tuple] = []
        self.result_queue: list[list[tuple]] = []
        self.closed = False

    def queue(self, rows: list[tuple]) -> None:
        self.result_queue.append(rows)

    def execute(self, sql: str, params: tuple | None = None) -> FakeCursor:
        self.sql_log.append((sql, params))
        return FakeCursor(self.result_queue.pop(0) if self.result_queue else [])

    def executemany(self, sql: str, paramsSeq: list[tuple]) -> FakeCursor:
        self.sql_log.append((sql, paramsSeq))
        return FakeCursor(self.result_queue.pop(0) if self.result_queue else [])

    @contextmanager
    def cursor(self):
        cursor = FakeCursor(self.result_queue.pop(0) if self.result_queue else [])
        self._lastCursor = cursor
        yield cursor

    @contextmanager
    def transaction(self):
        yield self

    def close(self) -> None:
        self.closed = True


def makeStore() -> tuple[QuoteImportService, FakeConnection]:
    connection = FakeConnection()
    from veritasquant.infrastructure.persistence.QuoteStore import MinuteQuoteStore

    return QuoteImportService(MinuteQuoteStore(connection)), connection


class TestQuoteImportService:
    def test_import_content_persists(self) -> None:
        service, connection = makeStore()
        connection.queue([(True,), (True,)])  # upsert：2 行新增
        connection.queue([])  # registerBatch

        result = service.importContent(
            _MVSV_CONTENT.encode("utf-8"),
            source="cn-feed",
            mode="FIELD",
            importedBy="test",
        )

        assert result["secu_code"] == "518880"
        assert result["market_code"] == 1
        assert result["record_count"] == 2
        assert result["mode"] == "FIELD"
        assert result["batch_id"].startswith("import_518880_")

        upsertSql, paramsSeq = connection._lastCursor.executed[0]
        assert "ON CONFLICT (ts, market_code, secu_code) DO UPDATE SET" in upsertSql
        assert "COALESCE(EXCLUDED.close" in upsertSql
        assert len(paramsSeq) == 2

        batchSql, batchParams = connection.sql_log[0]
        assert "INSERT INTO quote_ingest_batches" in batchSql
        assert batchParams[3] == "518880"

    def test_import_row_mode(self) -> None:
        service, connection = makeStore()
        connection.queue([(True,), (True,)])
        connection.queue([])

        result = service.importContent(
            _MVSV_CONTENT.encode("utf-8"),
            source="cn-feed",
            mode="ROW",
            importedBy="test",
        )

        assert result["mode"] == "ROW"
        upsertSql, _ = connection._lastCursor.executed[0]
        assert "close = EXCLUDED.close" in upsertSql
        assert "COALESCE" not in upsertSql

    def test_import_empty_content_rejected(self) -> None:
        service, _ = makeStore()
        with pytest.raises(MvsvImportError):
            service.importContent(b"", source="cn-feed")

    def test_import_bad_content_rejected(self) -> None:
        service, _ = makeStore()
        with pytest.raises(MvsvImportError):
            service.importContent(b"not a mvsv file", source="cn-feed")
