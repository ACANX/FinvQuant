from __future__ import annotations

from pathlib import Path

import pytest

from veritasquant.data.Mvsv import MvsvFormatError, MvsvReaderV1


SAMPLE = Path("Data/US_NSDQ_NVDA/US_NVDA_Min_V4_2026_2026072907_15000.mvsv")


def test_mvsv_reader_streams_repository_15000_row_sample() -> None:
    reader = MvsvReaderV1(SAMPLE)
    header = reader.readHeader()
    records = reader.iterRecords()
    first = next(records)
    count = 1 + sum(1 for _ in records)
    assert header.values["Format"] == "MVSV-1"
    assert header.count == count == 15_000
    assert first.sourceLine > 1
    assert first.sourceSequence == 1


def test_mvsv_reader_rejects_missing_header_and_wrong_column_count(tmp_path: Path) -> None:
    missingHeader = tmp_path / "missing.mvsv"
    missingHeader.write_text('# Format : MVSV-1\n\n', encoding="utf-8")
    with pytest.raises(MvsvFormatError, match="缺少必填头部"):
        MvsvReaderV1(missingHeader).readHeader()
    malformed = tmp_path / "malformed.mvsv"
    malformed.write_text(
        '# Format : MVSV-1\n# Field : "ts|dt|o|c|l|h|v|t|cp|cr|p"\n# Count : 1\n'
        '# EffectiveTimeZone : "UTC"\n# Code : "X"\n# Market : "X"\n# CurrencyCode : 1\n'
        '# PriceAccuracy : 1\n# LotSize : 1\n\n1|20260101000000\n', encoding="utf-8"
    )
    with pytest.raises(MvsvFormatError, match="11 列"):
        list(MvsvReaderV1(malformed).iterRecords())


def test_mvsv_reader_rejects_count_and_timezone_inconsistency(tmp_path: Path) -> None:
    content = (
        '# Format : MVSV-1\n# Field : "ts|dt|o|c|l|h|v|t|cp|cr|p"\n# Count : 2\n'
        '# EffectiveTimeZone : "UTC"\n# Code : "X"\n# Market : "X"\n# CurrencyCode : 1\n'
        '# PriceAccuracy : 1\n# LotSize : 1\n\n0|19700101000001|1|1|1|1|0|0|0|0|1\n'
    )
    path = tmp_path / "count.mvsv"
    path.write_text(content, encoding="utf-8")
    with pytest.raises(MvsvFormatError, match="不一致"):
        list(MvsvReaderV1(path).iterRecords())
