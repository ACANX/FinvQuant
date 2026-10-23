"""历史分钟行情 PostgreSQL 存储（字段级覆盖式更新，基础设施层）。

对齐 `finv_quote_secu_kline_min`（参考 MySQL 表，V4 迁移）：
- 主键 `(ts, market_code, secu_code)`，允许修正（无不可变触发器）；
- 两种覆盖模式：
  - ``FIELD``（默认）：后到数据只覆盖"非 NULL 字段"，NULL 保留旧值（COALESCE）；
  - ``ROW``：整行覆盖，NULL 也覆盖。
- 程序层控制：导入批次登记表 + 修正审计日志，覆盖修正留痕可追溯。

psycopg3 实现，连接由外部注入（与既有 persistence store 风格一致）。
"""

from __future__ import annotations

import os
from collections.abc import Iterator, Sequence
from datetime import datetime, timezone
from typing import Any

import psycopg
from psycopg import Connection
from psycopg.types.json import Jsonb

from veritasquant.data.QuoteRow import QuoteRowV1, UpsertMode

# 表列（与 V4 迁移一一对应；键列在前，可更新列在后）
_INSERT_COLUMNS = (
    "market_code",
    "secu_code",
    "ts",
    "date",
    "time",
    "prev_close",
    "open",
    "high",
    "low",
    "close",
    "paocd",
    "volume",
    "turnover",
    "ext_field",
    "remark",
)

# 可被覆盖更新的字段（不含主键三列与 gmt_create/gmt_update）
_UPDATABLE_COLUMNS = (
    "date",
    "time",
    "prev_close",
    "open",
    "high",
    "low",
    "close",
    "paocd",
    "volume",
    "turnover",
    "ext_field",
    "remark",
)

_INSERT_SQL = (
    "INSERT INTO finv_quote_secu_kline_min\n"
    "    (" + ", ".join(_INSERT_COLUMNS) + ")\n"
    "VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)\n"
    "ON CONFLICT (ts, market_code, secu_code) DO UPDATE SET\n"
)


def _buildUpsertSql(mode: str) -> str:
    """生成 FIELD（COALESCE 保留旧值）或 ROW（整行覆盖）的 upsert SQL。

    使用 ``RETURNING (xmax = 0) AS is_insert`` 区分新增与覆盖行，
    覆盖行数写入修正审计日志。
    """
    assignments: list[str] = []
    for column in _UPDATABLE_COLUMNS:
        if mode == UpsertMode.Row:
            assignments.append(f'    {column} = EXCLUDED.{column}')
        else:
            assignments.append(
                f"    {column} = COALESCE(EXCLUDED.{column}, finv_quote_secu_kline_min.{column})"
            )
    return (
        _INSERT_SQL
        + ",\n".join(assignments)
        + "\nRETURNING (xmax = 0) AS is_insert"
    )


class MinuteQuoteStore:
    """历史分钟行情 PostgreSQL 存储：字段级覆盖 upsert + 批次/修正审计 + 流式读取。"""

    def __init__(self, connection: Connection) -> None:
        self._connection = connection

    # -- 写入 ------------------------------------------------------------

    def upsertRows(
        self,
        rows: Sequence[QuoteRowV1],
        ingestBatchId: str,
        mode: str = UpsertMode.Field,
        reason: str | None = None,
        revisedBy: str = "import",
    ) -> dict[str, int]:
        """批量 upsert（同键覆盖），返回 {"inserted": n, "updated": n}。

        - mode=FIELD：字段级覆盖（非 NULL 覆盖，NULL 保留旧值）；
        - mode=ROW：整行覆盖。
        发生覆盖（updated > 0）时自动写入修正审计日志。
        """
        if not rows:
            return {"inserted": 0, "updated": 0}
        normalized = UpsertMode.Row if mode == UpsertMode.Row else UpsertMode.Field
        sql = _buildUpsertSql(normalized)
        params = [
            (
                row.market_code,
                row.secu_code,
                row.ts,
                row.date,
                row.time,
                row.prev_close,
                row.open,
                row.high,
                row.low,
                row.close,
                row.paocd,
                row.volume,
                row.turnover,
                row.ext_field,
                row.remark,
            )
            for row in rows
        ]
        with self._connection.transaction():
            with self._connection.cursor() as cursor:
                cursor.executemany(sql, params)
                isInsert = [row[0] for row in cursor.fetchall()]
                inserted = sum(1 for flag in isInsert if flag)
                updated = len(isInsert) - inserted
                if updated > 0:
                    self._logRevision(
                        ingestBatchId=ingestBatchId,
                        marketCode=rows[0].market_code,
                        secuCode=rows[0].secu_code,
                        affectedRows=updated,
                        reason=reason or "同键数据覆盖更新",
                        revisedBy=revisedBy,
                        mode=normalized,
                        rows=rows,
                    )
        return {"inserted": inserted, "updated": updated}

    def registerBatch(
        self,
        ingestBatchId: str,
        source: str,
        marketCode: int,
        secuCode: str,
        dataVersionId: str,
        fileCount: int,
        recordCount: int,
        mode: str,
        tsPrecision: str,
        configHash: str,
        importedBy: str,
        notes: str | None = None,
    ) -> None:
        """登记一次导入批次（审计：谁、何时、导入了什么）。"""
        with self._connection.transaction():
            self._connection.execute(
                "INSERT INTO quote_ingest_batches ("
                " ingest_batch_id, source, market_code, secu_code, data_version_id,"
                " file_count, record_count, upsert_mode, ts_precision, config_hash,"
                " imported_by, notes) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
                (
                    ingestBatchId,
                    source,
                    marketCode,
                    secuCode,
                    dataVersionId,
                    fileCount,
                    recordCount,
                    mode,
                    tsPrecision,
                    configHash,
                    importedBy,
                    notes,
                ),
            )

    def _logRevision(
        self,
        *,
        ingestBatchId: str,
        marketCode: int,
        secuCode: str,
        affectedRows: int,
        reason: str,
        revisedBy: str,
        mode: str,
        rows: Sequence[QuoteRowV1],
    ) -> None:
        """写入修正审计日志（覆盖前后摘要）。"""
        previous = {"mode": mode, "rows": affectedRows, "note": "被同键新数据覆盖"}
        new = {
            "mode": mode,
            "rows": len(rows),
            "min_ts": min(row.ts for row in rows),
            "max_ts": max(row.ts for row in rows),
        }
        self._connection.execute(
            "INSERT INTO quote_revision_log ("
            " ingest_batch_id, market_code, secu_code, affected_rows, reason,"
            " revised_by, previous_summary, new_summary)"
            " VALUES (%s, %s, %s, %s, %s, %s, %s, %s)",
            (
                ingestBatchId,
                marketCode,
                secuCode,
                affectedRows,
                reason,
                revisedBy,
                Jsonb(previous),
                Jsonb(new),
            ),
        )

    # -- 读取 ------------------------------------------------------------

    def iterRows(
        self,
        symbol: str | None = None,
        marketCode: int | None = None,
        startTs: int | None = None,
        endTs: int | None = None,
        batch: int = 4096,
    ) -> Iterator[QuoteRowV1]:
        """按 ``(secu_code, ts)`` 顺序流式读取（回放/分析）。

        支持按证券、市场、时间范围过滤；使用 fetchmany 分批，内存可控。
        """
        conditions: list[str] = []
        params: list[Any] = []
        if symbol is not None:
            conditions.append("secu_code = %s")
            params.append(symbol)
        if marketCode is not None:
            conditions.append("market_code = %s")
            params.append(marketCode)
        if startTs is not None:
            conditions.append("ts >= %s")
            params.append(startTs)
        if endTs is not None:
            conditions.append("ts < %s")
            params.append(endTs)
        where = (" WHERE " + " AND ".join(conditions)) if conditions else ""
        sql = (
            "SELECT market_code, secu_code, ts, date, \"time\", prev_close, open,"
            " high, low, close, paocd, volume, turnover, ext_field, remark"
            f" FROM finv_quote_secu_kline_min{where}"
            " ORDER BY secu_code, ts"
        )
        with self._connection.cursor() as cursor:
            cursor.execute(sql, params)
            while chunk := cursor.fetchmany(batch):
                for record in chunk:
                    yield self._rowFromRecord(record)

    def countRows(
        self,
        symbol: str | None = None,
        marketCode: int | None = None,
        startTs: int | None = None,
        endTs: int | None = None,
    ) -> int:
        """统计满足过滤条件的行数。"""
        conditions: list[str] = []
        params: list[Any] = []
        if symbol is not None:
            conditions.append("secu_code = %s")
            params.append(symbol)
        if marketCode is not None:
            conditions.append("market_code = %s")
            params.append(marketCode)
        if startTs is not None:
            conditions.append("ts >= %s")
            params.append(startTs)
        if endTs is not None:
            conditions.append("ts < %s")
            params.append(endTs)
        where = (" WHERE " + " AND ".join(conditions)) if conditions else ""
        row = self._connection.execute(
            f"SELECT COUNT(*) FROM finv_quote_secu_kline_min{where}", params
        ).fetchone()
        return int(row[0]) if row is not None else 0

    @staticmethod
    def _rowFromRecord(record: tuple[Any, ...]) -> QuoteRowV1:
        return QuoteRowV1.model_validate({
            "MarketCode": record[0],
            "SecuCode": record[1],
            "Ts": record[2],
            "Date": record[3],
            "Time": record[4],
            "PrevClose": record[5],
            "Open": record[6],
            "High": record[7],
            "Low": record[8],
            "Close": record[9],
            "Paocd": record[10],
            "Volume": record[11],
            "Turnover": record[12],
            "ExtField": record[13],
            "Remark": record[14],
        })


def connectQuoteDb(dsn: str | None = None) -> Connection:
    """建立 PG 连接；优先显式 DSN，其次环境变量（VQ_POSTGRES_*）。"""
    if dsn is None:
        dsn = os.environ.get("VQ_POSTGRES_DSN", "")
        if not dsn:
            dsn = (
                f"host={os.environ.get('VQ_POSTGRES_HOST', 'localhost')} "
                f"port={os.environ.get('VQ_POSTGRES_PORT', '5432')} "
                f"dbname={os.environ.get('VQ_POSTGRES_DB', 'veritasquant')} "
                f"user={os.environ.get('VQ_POSTGRES_USER', 'veritasquant')} "
                f"password={os.environ.get('VQ_POSTGRES_PASSWORD', '')}"
            )
    return psycopg.connect(dsn)


def utcNowIso() -> str:
    """UTC ISO 时间（批次 ID 等使用）。"""
    return datetime.now(timezone.utc).isoformat()
