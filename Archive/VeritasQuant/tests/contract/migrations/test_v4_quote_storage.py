"""V4 历史行情迁移的静态契约测试（不依赖真实数据库）。

验证迁移文件命名、主表/审计表/索引/触发器声明，以及"允许修正"的
关键属性（不绑定不可变触发器）。
"""

from __future__ import annotations

import re
from pathlib import Path

import pytest

_REPO_ROOT = Path(__file__).resolve().parents[3]
_MIGRATIONS_DIR = _REPO_ROOT / "Migrations" / "postgresql"

_MIGRATION_NAME = re.compile(r"^V(\d+)__[A-Za-z0-9_\-]+\.sql$")


@pytest.fixture(scope="module")
def v4Sql() -> str:
    candidates = sorted(_MIGRATIONS_DIR.glob("V4__*.sql"))
    assert candidates, "缺少 V4 迁移文件"
    return candidates[0].read_text(encoding="utf-8")


class TestV4MigrationContract:
    def test_file_naming(self) -> None:
        files = sorted(_MIGRATIONS_DIR.glob("V4__*.sql"))
        assert len(files) == 1
        assert _MIGRATION_NAME.match(files[0].name)

    def test_main_table_declared(self, v4Sql: str) -> None:
        assert "CREATE TABLE finv_quote_secu_kline_min" in v4Sql

    def test_primary_key(self, v4Sql: str) -> None:
        assert "PRIMARY KEY (ts, market_code, secu_code)" in v4Sql

    def test_market_code_integer_checked(self, v4Sql: str) -> None:
        assert re.search(
            r"market_code\s+INTEGER\s+NOT NULL\s+CHECK\s*\(\s*market_code\s+BETWEEN\s+0\s+AND\s+99999999\s*\)",
            v4Sql,
        ), "market_code 必须为 INTEGER NOT NULL CHECK (market_code BETWEEN 0 AND 99999999)"

    def test_price_columns_numeric(self, v4Sql: str) -> None:
        for column in ("prev_close", "open", "high", "low", "close", "paocd"):
            assert re.search(rf"{column}\s+NUMERIC\(20,\s*6\)", v4Sql), f"{column} 必须为 NUMERIC(20,6)"

    def test_turnover_numeric(self, v4Sql: str) -> None:
        assert re.search(r"turnover\s+NUMERIC\(30,\s*8\)", v4Sql)

    def test_gmt_update_trigger(self, v4Sql: str) -> None:
        assert "CREATE TRIGGER trg_finv_quote_secu_kline_min_gmt_update" in v4Sql
        assert "BEFORE UPDATE ON finv_quote_secu_kline_min" in v4Sql

    def test_no_immutable_trigger(self, v4Sql: str) -> None:
        # 历史行情允许修正：不得绑定 prevent_fact_mutation 不可变触发器
        assert "prevent_fact_mutation" not in v4Sql

    def test_indexes(self, v4Sql: str) -> None:
        assert "idx_finv_quote_secu_secu_ts" in v4Sql
        assert "idx_finv_quote_secu_market_ts" in v4Sql

    def test_audit_tables(self, v4Sql: str) -> None:
        assert "CREATE TABLE quote_ingest_batches" in v4Sql
        assert "CREATE TABLE quote_revision_log" in v4Sql

    def test_single_transaction(self, v4Sql: str) -> None:
        assert re.search(r"^BEGIN;$", v4Sql, re.M)
        assert v4Sql.rstrip().endswith("COMMIT;")
