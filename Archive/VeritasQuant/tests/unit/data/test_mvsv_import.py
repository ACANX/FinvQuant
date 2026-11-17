"""MvsvImport 解析测试：字节流/文件 → QuoteRowV1 行。"""

from __future__ import annotations

from decimal import Decimal
from pathlib import Path

import pytest

from veritasquant.data.MvsvImport import MvsvImportError, parseMvsvContent, parseMvsvPath

_MVSV_CONTENT = (
    "# Format : \"MVSV-1\"\n"
    "# Field : \"ts|dt|o|c|l|h|v|t|cp|cr|p\"\n"
    "# Count : 3\n"
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
    "1785720720|20260803093200|7.003|7.004|7.001|7.005|90000|63020000000|0.002|0.000286|7.001\n"
)


class TestParseMvsv:
    def test_parse_content(self) -> None:
        result = parseMvsvContent(_MVSV_CONTENT.encode("utf-8"), sourceName="TEST.mvsv")
        assert result.marketCode == 1
        assert result.secuCode == "518880"
        assert result.recordCount == 3
        assert len(result.contentSha256) == 64

        first = result.rows[0]
        assert first.ts == 1_785_720_600
        assert first.date == 20260803
        assert first.time == 93000
        assert first.close == Decimal("7.002")
        assert first.volume == 100_000
        assert first.market_code == 1
        assert first.secu_code == "518880"

    def test_parse_path(self, tmp_path: Path) -> None:
        path = tmp_path / "TEST_518880.mvsv"
        path.write_text(_MVSV_CONTENT, encoding="utf-8")
        result = parseMvsvPath(path)
        assert result.recordCount == 3
        assert result.rows[2].ts == 1_785_720_720

    def test_empty_content_rejected(self) -> None:
        with pytest.raises(MvsvImportError):
            parseMvsvContent(b"", sourceName="empty.mvsv")

    def test_missing_market_code_rejected(self, tmp_path: Path) -> None:
        path = tmp_path / "no_market.mvsv"
        path.write_text(
            _MVSV_CONTENT.replace("# MarketCode : 1\n", ""), encoding="utf-8"
        )
        with pytest.raises(MvsvImportError):
            parseMvsvPath(path)

    def test_bad_format_rejected(self, tmp_path: Path) -> None:
        path = tmp_path / "bad.mvsv"
        path.write_text("# Format : \"MVSV-1\"\nbroken\n", encoding="utf-8")
        with pytest.raises(MvsvImportError):
            parseMvsvPath(path)
