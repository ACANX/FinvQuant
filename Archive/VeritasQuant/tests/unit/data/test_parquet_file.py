"""P1-019 规范化 Parquet 写入与固定物理参数验证。"""

from __future__ import annotations

from datetime import datetime, timezone
from decimal import Decimal
from pathlib import Path

import pytest

from veritasquant.core.Time import TsPrecision
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.data.ParquetFile import (
    PARQUET_WRITER_VERSION,
    ParquetReadError,
    ParquetStoreV1,
    ParquetWriteError,
    logicalParquetPath,
    parquetContentHash,
    readParquetSummary,
    sortBars,
    writeParquetBytes,
)


def _bar(
    minute: int,
    *,
    symbol: str = "518880",
    market: str = "SSE",
    source: str = "fixture",
    amount: Decimal | None = Decimal("100.00000000"),
    tradeCount: int | None = 5,
) -> MinuteBarSchemaV1:
    start = datetime(2026, 8, 3, 9, minute, tzinfo=timezone.utc)
    end = datetime(2026, 8, 3, 9, minute + 1, tzinfo=timezone.utc)
    return MinuteBarSchemaV1.model_validate({
        "Ts": end,
        "BarStart": start,
        "BarEnd": end,
        "Symbol": symbol,
        "Market": market,
        "Open": Decimal("10.000"),
        "High": Decimal("10.500"),
        "Low": Decimal("9.900"),
        "Close": Decimal("10.200"),
        "Volume": Decimal("1000"),
        "Amount": amount,
        "TradeCount": tradeCount,
        "Currency": "CNY",
        "SessionId": "SSE-0900-1130",
        "Source": source,
        "SourceRecordId": f"fixture:{minute}",
        "SourceSequence": minute,
        "IsAdjusted": False,
        "AdjustmentVersion": None,
        "InstrumentMetadataVersion": "v1",
        "QualityFlags": 0,
    })


def test_write_and_readback_roundtrip_preserves_rows_and_order() -> None:
    bars = sortBars([_bar(1), _bar(3), _bar(2)])
    content = writeParquetBytes(bars, TsPrecision.Millisecond)
    assert content.startswith(b"PAR1") and content.endswith(b"PAR1")
    summary = readParquetSummary(content, TsPrecision.Millisecond)
    assert summary.numRows == 3
    assert summary.writerVersion == PARQUET_WRITER_VERSION
    assert summary.firstBar is not None and summary.lastBar is not None
    assert summary.firstBar.sourceSequence == 1
    assert summary.lastBar.sourceSequence == 3
    assert summary.firstBar.ts == _bar(1).ts
    assert summary.lastBar.amount == Decimal("100.00000000")
    assert summary.lastBar.tradeCount == 5
    # 固定 schema 列齐
    assert summary.schemaColumns[0] == "minute_bar_schema_v1"
    assert "open" in summary.schemaColumns and "quality_flags" in summary.schemaColumns


def test_write_is_byte_deterministic_across_calls() -> None:
    bars = sortBars([_bar(1), _bar(2)])
    first = writeParquetBytes(bars, TsPrecision.Millisecond)
    second = writeParquetBytes(sortBars([_bar(1), _bar(2)]), TsPrecision.Millisecond)
    assert first == second
    assert parquetContentHash(first) == parquetContentHash(second)


def test_write_rejects_unsorted_input() -> None:
    with pytest.raises(ParquetWriteError, match="严格递增"):
        writeParquetBytes([_bar(2), _bar(1)], TsPrecision.Millisecond)


def test_write_rejects_empty_and_duplicate_rows() -> None:
    with pytest.raises(ParquetWriteError, match="空"):
        writeParquetBytes([], TsPrecision.Millisecond)
    with pytest.raises(ParquetWriteError, match="严格递增"):
        writeParquetBytes(sortBars([_bar(1), _bar(1)]), TsPrecision.Millisecond)


def test_store_never_overwrites_and_returns_logical_path(tmp_path: Path) -> None:
    store = ParquetStoreV1(tmp_path, "MinuteBarV1", TsPrecision.Millisecond)
    bars = sortBars([_bar(1), _bar(2)])
    logical, contentHash = store.storeBars(bars)
    assert logical.as_posix().startswith("MinuteBarV1/SSE/518880/Year=2026/Month=08/")
    assert logical.name == f"{contentHash}.parquet"
    assert (tmp_path / logical).exists()
    # 相同内容重复存储：复用不覆盖
    logicalAgain, sameHash = store.storeBars(sortBars([_bar(1), _bar(2)]))
    assert logicalAgain == logical and sameHash == contentHash


def test_store_rejects_content_hash_conflict(tmp_path: Path) -> None:
    store = ParquetStoreV1(tmp_path, "MinuteBarV1", TsPrecision.Millisecond)
    store.storeBars(sortBars([_bar(1)]))
    # 修改物理文件后再次存储同路径不同字节应被拒绝
    target = tmp_path / "MinuteBarV1" / "SSE" / "518880" / "Year=2026" / "Month=08"
    for file in target.rglob("*.parquet"):
        file.chmod(0o666)
        file.write_bytes(file.read_bytes() + b"\x00")
    with pytest.raises(ParquetWriteError, match="拒绝覆盖"):
        store.storeBars(sortBars([_bar(1)]))


def test_logical_path_rejects_bad_dataset_and_hash() -> None:
    bar = _bar(1)
    with pytest.raises(ParquetWriteError, match="DatasetId"):
        logicalParquetPath("a/b", bar, "0" * 64)
    with pytest.raises(ParquetWriteError, match="SHA-256"):
        logicalParquetPath("ok", bar, "short")


def test_readback_rejects_tampered_magic() -> None:
    bars = sortBars([_bar(1)])
    content = writeParquetBytes(bars, TsPrecision.Millisecond)
    with pytest.raises(ParquetReadError, match="魔数"):
        readParquetSummary(b"XXXX" + content[4:], TsPrecision.Millisecond)


def test_optional_columns_null_roundtrip() -> None:
    bar = _bar(4, amount=None, tradeCount=None)
    content = writeParquetBytes(sortBars([bar]), TsPrecision.Millisecond)
    summary = readParquetSummary(content, TsPrecision.Millisecond)
    assert summary.firstBar is not None
    assert summary.firstBar.amount is None
    assert summary.firstBar.tradeCount is None
