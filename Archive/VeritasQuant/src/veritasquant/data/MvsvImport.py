"""MVSV-1 行情内容 → QuoteRowV1 解析（领域层，无基础设施依赖）。

供 CLI（vq-import-market-data）与 API 上传导入（POST /api/v1/imports/upload）
复用：把 MVSV-1 文件/字节流解析为与 `finv_quote_secu_kline_min` 对齐的行。
"""

from __future__ import annotations

import hashlib
import tempfile
from dataclasses import dataclass
from pathlib import Path

from veritasquant.data.Mvsv import MvsvReaderV1, MvsvRecordV1
from veritasquant.data.QuoteRow import QuoteRowV1


class MvsvImportError(ValueError):
    """MVSV-1 内容无法解析为行情行（头部缺失、记录非法等）。"""


@dataclass(frozen=True, slots=True)
class MvsvImportResult:
    """一次 MVSV-1 解析结果（单文件单证券）。"""

    marketCode: int
    secuCode: str
    rows: list[QuoteRowV1]
    contentSha256: str

    @property
    def recordCount(self) -> int:
        return len(self.rows)


def parseMvsvContent(content: bytes, sourceName: str = "upload") -> MvsvImportResult:
    """从字节流解析 MVSV-1 行情（API 上传场景）。

    通过临时文件包装以复用流式读取器；解析失败抛 ``MvsvImportError``。
    """
    if not content:
        raise MvsvImportError("上传内容为空")
    with tempfile.TemporaryDirectory(prefix="vq-mvsv-") as temporary:
        path = Path(temporary) / sourceName
        path.write_bytes(content)
        try:
            return parseMvsvPath(path)
        except Exception as error:  # noqa: BLE001 - 统一包装为导入错误
            raise MvsvImportError(f"MVSV 解析失败: {error}") from error


def parseMvsvPath(path: Path) -> MvsvImportResult:
    """从文件解析 MVSV-1 行情（CLI/服务端文件目录场景）。"""
    try:
        reader = MvsvReaderV1(path)
        header = reader.readHeader()
    except Exception as error:  # noqa: BLE001 - 统一包装为导入错误
        raise MvsvImportError(f"MVSV 解析失败: {error}") from error

    headerValues = header.values
    if "MarketCode" not in headerValues:
        raise MvsvImportError(f"{path}: 头部缺少 MarketCode（市场数字代码）")
    marketCode = int(headerValues["MarketCode"])
    secuCode = headerValues["Code"]

    rows: list[QuoteRowV1] = []
    for record in reader.iterRecords():
        rows.append(_rowFromRecord(marketCode, secuCode, record))

    return MvsvImportResult(
        marketCode=marketCode,
        secuCode=secuCode,
        rows=rows,
        contentSha256=_sha256(path.read_bytes()),
    )


def _rowFromRecord(
    marketCode: int,
    secuCode: str,
    record: MvsvRecordV1,
) -> QuoteRowV1:
    """将 MVSV-1 记录映射为行情表行。"""
    local = record.sourceLocalTime
    return QuoteRowV1.model_validate({
        "MarketCode": marketCode,
        "SecuCode": secuCode,
        "Ts": int(record.sourceTs.timestamp()),
        "Date": local.year * 10_000 + local.month * 100 + local.day,
        "Time": local.hour * 10_000 + local.minute * 100 + local.second,
        "PrevClose": record.previousClose,
        "Open": record.open,
        "High": record.high,
        "Low": record.low,
        "Close": record.close,
        "Paocd": None,
        "Volume": int(record.volume),
        "Turnover": record.turnover,
        "ExtField": None,
        "Remark": None,
    })


def _sha256(content: bytes) -> str:
    return hashlib.sha256(content).hexdigest()
