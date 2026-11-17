"""vq-import-market-data 导入流程测试（dry-run 真实解析 + FakeConnection 落库）。"""

from __future__ import annotations

from contextlib import contextmanager
from pathlib import Path

from veritasquant.cli.ImportMarketData import main

_MVSV_CONTENT = """\
# Title : "TEST_518880_ETF"
# Format : "MVSV-1"
# Field : "ts|dt|o|c|l|h|v|t|cp|cr|p"
# Count : 3
# EffectiveTimeZone : "Asia/Shanghai"
# Code : "518880"
# Market : "SSE"
# MarketCode : 1
# CurrencyCode : 1
# PriceAccuracy : 3
# LotSize : 100

1785720600|20260803093000|7.001|7.002|6.999|7.003|100000|70010000000|0.001|0.000143|7.000
1785720660|20260803093100|7.002|7.001|7.000|7.003|80000|56010000000|0.000|-0.000143|7.002
1785720720|20260803093200|7.003|7.004|7.001|7.005|90000|63020000000|0.002|0.000286|7.001
"""

_CONFIG_TEMPLATE = """\
Source: "TEST"
InputDir: "{input_dir}"
TsPrecision: "Second"
UpsertMode: "FIELD"
ImportedBy: "test"
"""


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


def _setup(tmp_path: Path) -> tuple[Path, Path]:
    inputDir = tmp_path / "mvsv"
    inputDir.mkdir()
    (inputDir / "TEST_518880.mvsv").write_text(_MVSV_CONTENT, encoding="utf-8")
    configPath = tmp_path / "import.yml"
    configPath.write_text(_CONFIG_TEMPLATE.format(input_dir=inputDir.as_posix()), encoding="utf-8")
    return configPath, inputDir


def _patchConnect(monkeypatch, connection: FakeConnection) -> None:
    def fakeConnect(dsn: str | None = None):  # noqa: ARG001
        return connection

    monkeypatch.setattr("veritasquant.cli.ImportMarketData.connectQuoteDb", fakeConnect)


class TestImportMarketData:
    def test_dry_run_parses_records(self, tmp_path, capsys) -> None:
        configPath, _ = _setup(tmp_path)
        exitCode = main(["--config", str(configPath), "--dry-run"])
        output = capsys.readouterr().out
        assert exitCode == 0
        assert "518880" in output
        assert "3 条" in output
        assert "预览" in output

    def test_import_writes_to_store(self, tmp_path, monkeypatch) -> None:
        configPath, _ = _setup(tmp_path)
        connection = FakeConnection()
        connection.queue([(True,), (True,), (True,)])  # upsert：3 行全部新增
        connection.queue([])  # registerBatch
        _patchConnect(monkeypatch, connection)

        exitCode = main(["--config", str(configPath)])

        assert exitCode == 0
        upsertSql, paramsSeq = connection._lastCursor.executed[0]
        assert "ON CONFLICT (ts, market_code, secu_code) DO UPDATE SET" in upsertSql
        assert "COALESCE(EXCLUDED.close" in upsertSql
        assert len(paramsSeq) == 3
        first = paramsSeq[0]
        assert first[0] == 1 and first[1] == "518880" and first[2] == 1_785_720_600
        batchSql, batchParams = connection.sql_log[0]
        assert "INSERT INTO quote_ingest_batches" in batchSql
        assert batchParams[3] == "518880"

    def test_import_row_mode(self, tmp_path, monkeypatch) -> None:
        configPath, _ = _setup(tmp_path)
        connection = FakeConnection()
        connection.queue([(True,), (True,), (True,)])
        connection.queue([])
        _patchConnect(monkeypatch, connection)

        exitCode = main(["--config", str(configPath), "--mode", "ROW"])

        assert exitCode == 0
        upsertSql, _ = connection._lastCursor.executed[0]
        assert "close = EXCLUDED.close" in upsertSql
        assert "COALESCE" not in upsertSql

    def test_config_missing_required(self, tmp_path) -> None:
        configPath = tmp_path / "bad.yml"
        configPath.write_text("Source: only\n", encoding="utf-8")
        assert main(["--config", str(configPath), "--dry-run"]) == 3

    def test_input_dir_missing(self, tmp_path) -> None:
        configPath = tmp_path / "import.yml"
        configPath.write_text(_CONFIG_TEMPLATE.format(input_dir="/no/such/dir"), encoding="utf-8")
        assert main(["--config", str(configPath), "--dry-run"]) == 3

    def test_no_mvsv_files(self, tmp_path) -> None:
        inputDir = tmp_path / "empty"
        inputDir.mkdir()
        configPath = tmp_path / "import.yml"
        configPath.write_text(_CONFIG_TEMPLATE.format(input_dir=inputDir.as_posix()), encoding="utf-8")
        assert main(["--config", str(configPath), "--dry-run"]) == 3

    def test_bad_file_does_not_abort_batch(self, tmp_path, monkeypatch) -> None:
        inputDir = tmp_path / "mvsv"
        inputDir.mkdir()
        (inputDir / "GOOD.mvsv").write_text(_MVSV_CONTENT, encoding="utf-8")
        (inputDir / "BAD.mvsv").write_text("# Format : \"MVSV-1\"\nbroken\n", encoding="utf-8")
        configPath = tmp_path / "import.yml"
        configPath.write_text(_CONFIG_TEMPLATE.format(input_dir=inputDir.as_posix()), encoding="utf-8")

        connection = FakeConnection()
        connection.queue([(True,), (True,), (True,)])
        connection.queue([])
        _patchConnect(monkeypatch, connection)

        exitCode = main(["--config", str(configPath)])
        # 坏文件失败 → 业务失败退出码 3，但好文件已入库
        assert exitCode == 3
        upsertSql, _ = connection._lastCursor.executed[0]
        assert "ON CONFLICT" in upsertSql
