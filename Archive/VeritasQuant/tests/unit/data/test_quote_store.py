"""MinuteQuoteStore / QuoteRowV1 单元测试（FakeConnection，不依赖真实 PG）。"""

from __future__ import annotations

from contextlib import contextmanager
from decimal import Decimal

import pytest

from veritasquant.data.QuoteRow import QuoteRowV1, UpsertMode
from veritasquant.infrastructure.persistence.QuoteStore import (
    MinuteQuoteStore,
    connectQuoteDb,
)


class FakeCursor:
    """记录 SQL 执行并回放预置结果。"""

    def __init__(self, rows: list[tuple] | None = None) -> None:
        self.rows = list(rows or [])
        self.executed: list[tuple] = []

    def __enter__(self) -> "FakeCursor":
        return self

    def __exit__(self, *exc) -> None:
        return None

    def execute(self, sql: str, params: tuple | None = None) -> "FakeCursor":
        self.executed.append((sql, params))
        return self

    def executemany(self, sql: str, paramsSeq: list[tuple]) -> "FakeCursor":
        self.executed.append((sql, paramsSeq))
        return self

    def fetchall(self) -> list[tuple]:
        rows, self.rows = self.rows, []
        return rows

    def fetchone(self) -> tuple | None:
        return self.rows.pop(0) if self.rows else None

    def fetchmany(self, size: int) -> list[tuple]:
        chunk, self.rows = self.rows[:size], self.rows[size:]
        return chunk


class FakeConnection:
    """最小 psycopg.Connection 替身：记录 SQL/参数，按队列回放结果。"""

    def __init__(self) -> None:
        self.sql_log: list[tuple] = []
        self.result_queue: list[list[tuple]] = []
        self.transaction_count = 0
        self.closed = False

    def queue(self, rows: list[tuple]) -> None:
        self.result_queue.append(rows)

    def execute(self, sql: str, params: tuple | None = None) -> FakeCursor:
        self.sql_log.append((sql, params))
        return FakeCursor(self.result_queue.pop(0) if self.result_queue else [])

    def executemany(self, sql: str, paramsSeq: list[tuple]) -> FakeCursor:
        self.sql_log.append((sql, paramsSeq))
        return FakeCursor(self.result_queue.pop(0) if self.result_queue else [])

    def cursor(self, name: str | None = None) -> FakeCursor:  # noqa: ARG002
        self._lastCursor = FakeCursor(self.result_queue.pop(0) if self.result_queue else [])
        return self._lastCursor

    @contextmanager
    def transaction(self):
        self.transaction_count += 1
        yield self

    def close(self) -> None:
        self.closed = True


def makeRow(**overrides) -> QuoteRowV1:
    values = {
        "MarketCode": 1,
        "SecuCode": "518880",
        "Ts": 1_777_505_400,
        "Date": 20260803,
        "Time": 93000,
        "PrevClose": Decimal("7.000"),
        "Open": Decimal("7.001"),
        "High": Decimal("7.003"),
        "Low": Decimal("6.999"),
        "Close": Decimal("7.002"),
        "Paocd": None,
        "Volume": 100_000,
        "Turnover": Decimal("70010000000"),
        "ExtField": None,
        "Remark": None,
    }
    values.update(overrides)
    return QuoteRowV1.model_validate(values)


# -- QuoteRowV1 校验 ------------------------------------------------------


class TestQuoteRowValidation:
    def test_valid_row(self) -> None:
        row = makeRow()
        assert row.market_code == 1
        assert row.close == Decimal("7.002")

    def test_market_code_range(self) -> None:
        with pytest.raises(ValueError):
            makeRow(market_code=100_000_000)

    def test_ts_negative(self) -> None:
        with pytest.raises(ValueError):
            makeRow(ts=-1)

    def test_date_range(self) -> None:
        with pytest.raises(ValueError):
            makeRow(date=19_491_231)  # 低于 19500101
        with pytest.raises(ValueError):
            makeRow(date=22_000_101)  # 高于 21001231

    def test_time_range(self) -> None:
        with pytest.raises(ValueError):
            makeRow(time=240000)

    def test_ohlc_relation(self) -> None:
        with pytest.raises(ValueError):
            makeRow(open=Decimal("9.000"))  # 超过 high=7.003

    def test_negative_volume(self) -> None:
        with pytest.raises(ValueError):
            makeRow(volume=-1)


# -- upsert SQL 与统计 ----------------------------------------------------


class TestUpsert:
    def test_field_mode_sql(self) -> None:
        conn = FakeConnection()
        conn.queue([(True,), (False,), (True,)])  # RETURNING (xmax=0)
        conn.queue([])  # revision log 无返回
        store = MinuteQuoteStore(conn)
        store.upsertRows([makeRow(), makeRow(), makeRow()], "batch-1", mode=UpsertMode.Field)

        sql, paramsSeq = conn._lastCursor.executed[0]
        assert "INSERT INTO finv_quote_secu_kline_min" in sql
        assert "ON CONFLICT (ts, market_code, secu_code) DO UPDATE SET" in sql
        assert "COALESCE(EXCLUDED.close, finv_quote_secu_kline_min.close)" in sql
        assert "RETURNING (xmax = 0) AS is_insert" in sql
        assert len(paramsSeq) == 3

    def test_row_mode_sql(self) -> None:
        conn = FakeConnection()
        conn.queue([(True,), (True,)])
        store = MinuteQuoteStore(conn)
        store.upsertRows([makeRow(), makeRow()], "batch-1", mode=UpsertMode.Row)

        sql, _ = conn._lastCursor.executed[0]
        assert "close = EXCLUDED.close" in sql
        assert "COALESCE" not in sql

    def test_upsert_counts_and_revision_log(self) -> None:
        conn = FakeConnection()
        conn.queue([(True,), (False,), (True,)])  # 1 覆盖
        conn.queue([])  # revision log
        store = MinuteQuoteStore(conn)
        stats = store.upsertRows([makeRow(), makeRow(), makeRow()], "batch-1")

        assert stats == {"inserted": 2, "updated": 1}
        revisionSql, revisionParams = conn.sql_log[0]
        assert "INSERT INTO quote_revision_log" in revisionSql
        assert revisionParams[0] == "batch-1"
        assert revisionParams[3] == 1  # affected_rows

    def test_upsert_no_revision_when_nothing_overwritten(self) -> None:
        conn = FakeConnection()
        conn.queue([(True,), (True,)])
        store = MinuteQuoteStore(conn)
        store.upsertRows([makeRow(), makeRow()], "batch-1")

        assert len(conn.sql_log) == 0  # 只有 upsert（cursor 执行），无修正日志

    def test_upsert_empty_rows(self) -> None:
        store = MinuteQuoteStore(FakeConnection())
        assert store.upsertRows([], "batch-1") == {"inserted": 0, "updated": 0}


# -- 批次登记与读取 ------------------------------------------------------


class TestBatchAndRead:
    def test_register_batch(self) -> None:
        conn = FakeConnection()
        conn.queue([])
        store = MinuteQuoteStore(conn)
        store.registerBatch(
            ingestBatchId="import_NVDA_20260804120000",
            source="FT",
            marketCode=11,
            secuCode="NVDA",
            dataVersionId="a" * 64,
            fileCount=1,
            recordCount=15000,
            mode=UpsertMode.Field,
            tsPrecision="Second",
            configHash="c" * 64,
            importedBy="bee-agent",
            notes="example",
        )
        sql, params = conn.sql_log[0]
        assert "INSERT INTO quote_ingest_batches" in sql
        assert params[0] == "import_NVDA_20260804120000"
        assert params[3] == "NVDA"

    def test_iter_rows_sql_and_order(self) -> None:
        conn = FakeConnection()
        conn.queue(
            [
                (11, "NVDA", 100, 20260803, 93000, None, Decimal("1"), Decimal("2"), Decimal("0.5"), Decimal("1.5"), None, 10, None, None, None),
                (11, "NVDA", 101, 20260803, 93100, None, Decimal("1"), Decimal("2"), Decimal("0.5"), Decimal("1.6"), None, 11, None, None, None),
            ]
        )
        store = MinuteQuoteStore(conn)
        rows = list(store.iterRows(symbol="NVDA", marketCode=11, startTs=100, endTs=200))

        sql, params = conn._lastCursor.executed[0]
        assert "FROM finv_quote_secu_kline_min" in sql
        assert "secu_code = %s" in sql and "market_code = %s" in sql
        assert "ts >= %s" in sql and "ts < %s" in sql
        assert "ORDER BY secu_code, ts" in sql
        assert params == ["NVDA", 11, 100, 200]
        assert len(rows) == 2
        assert rows[0].secu_code == "NVDA"

    def test_count_rows(self) -> None:
        conn = FakeConnection()
        conn.queue([(42,)])
        store = MinuteQuoteStore(conn)
        assert store.countRows(symbol="NVDA") == 42


# -- 连接 helper ----------------------------------------------------------


class TestConnect:
    def test_connect_uses_dsn(self, monkeypatch) -> None:
        import psycopg

        called: list[str] = []

        def fakeConnect(dsn: str):
            called.append(dsn)
            raise RuntimeError("stop")

        monkeypatch.setattr(psycopg, "connect", fakeConnect)
        monkeypatch.delenv("VQ_POSTGRES_DSN", raising=False)
        with pytest.raises(RuntimeError):
            connectQuoteDb("host=localhost dbname=test")
        assert called == ["host=localhost dbname=test"]

    def test_connect_builds_from_env(self, monkeypatch) -> None:
        import psycopg

        called: list[str] = []

        def fakeConnect(dsn: str):
            called.append(dsn)
            raise RuntimeError("stop")

        monkeypatch.setattr(psycopg, "connect", fakeConnect)
        monkeypatch.setenv("VQ_POSTGRES_HOST", "pg1")
        monkeypatch.setenv("VQ_POSTGRES_PORT", "5433")
        monkeypatch.setenv("VQ_POSTGRES_DB", "vqdb")
        monkeypatch.setenv("VQ_POSTGRES_USER", "vquser")
        monkeypatch.setenv("VQ_POSTGRES_PASSWORD", "secret")
        monkeypatch.delenv("VQ_POSTGRES_DSN", raising=False)
        with pytest.raises(RuntimeError):
            connectQuoteDb(None)
        assert called == ["host=pg1 port=5433 dbname=vqdb user=vquser password=secret"]
