"""MVSV-1 外部历史行情文件的流式适配器。"""

from __future__ import annotations

from collections.abc import Iterator, Mapping
from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
from pathlib import Path
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError


class MvsvFormatError(ValueError):
    """MVSV-1 头部或记录不满足来源协议。"""


_REQUIRED_HEADERS = frozenset({
    "Format", "Field", "Count", "EffectiveTimeZone", "Code", "Market", "CurrencyCode", "PriceAccuracy", "LotSize"
})
_FIELDS = "ts|dt|o|c|l|h|v|t|cp|cr|p"


@dataclass(frozen=True, slots=True)
class MvsvHeaderV1:
    """保留来源头信息，未知键不参与协议解释。"""

    values: Mapping[str, str]
    count: int
    effectiveTimeZone: ZoneInfo


@dataclass(frozen=True, slots=True)
class MvsvRecordV1:
    """单行外部数据的严格解析结果，尚未映射到领域 MinuteBar。"""

    sourceTs: datetime
    sourceLocalTime: datetime
    open: Decimal
    close: Decimal
    low: Decimal
    high: Decimal
    volume: Decimal
    turnover: Decimal
    change: Decimal
    changeRate: Decimal
    previousClose: Decimal
    sourceLine: int
    sourceSequence: int


class MvsvReaderV1:
    """以固定内存读取一个 MVSV-1 文件。"""

    def __init__(self, path: Path) -> None:
        self._path = path

    def readHeader(self) -> MvsvHeaderV1:
        """读取并验证文件头，不扫描数据行。"""
        with self._path.open("r", encoding="utf-8-sig", newline=None) as source:
            headers: dict[str, str] = {}
            for lineNumber, rawLine in enumerate(source, start=1):
                line = rawLine.rstrip("\n\r")
                if line == "":
                    return _buildHeader(headers, self._path)
                _parseHeaderLine(line, lineNumber, headers)
        raise MvsvFormatError(f"{self._path}: 未找到头部与数据的空行分隔")

    def iterRecords(self) -> Iterator[MvsvRecordV1]:
        """顺序产生记录，并在流结束时校验声明 Count。"""
        with self._path.open("r", encoding="utf-8-sig", newline=None) as source:
            headerLines, header = _readHeaderFromStream(source, self._path)
            recordCount = 0
            for lineNumber, rawLine in enumerate(source, start=headerLines + 1):
                line = rawLine.rstrip("\n\r")
                if not line:
                    raise MvsvFormatError(f"{self._path}:{lineNumber}: 数据区不允许空行")
                recordCount += 1
                yield _parseRecord(line, lineNumber, recordCount, header, self._path)
            if recordCount != header.count:
                raise MvsvFormatError(
                    f"{self._path}: Count={header.count}，实际记录数={recordCount}"
                )


def _readHeaderFromStream(source: Iterator[str], path: Path) -> tuple[int, MvsvHeaderV1]:
    headers: dict[str, str] = {}
    for lineNumber, rawLine in enumerate(source, start=1):
        line = rawLine.rstrip("\n\r")
        if line == "":
            return lineNumber, _buildHeader(headers, path)
        _parseHeaderLine(line, lineNumber, headers)
    raise MvsvFormatError(f"{path}: 未找到头部与数据的空行分隔")


def _parseHeaderLine(line: str, lineNumber: int, headers: dict[str, str]) -> None:
    if not line.startswith("# ") or " : " not in line:
        raise MvsvFormatError(f"头部第 {lineNumber} 行格式必须为 '# Key : Value'")
    key, value = line[2:].split(" : ", maxsplit=1)
    if not key or key in headers:
        raise MvsvFormatError(f"头部第 {lineNumber} 行存在空键或重复键")
    headers[key] = _unquote(value)


def _buildHeader(headers: dict[str, str], path: Path) -> MvsvHeaderV1:
    missing = sorted(_REQUIRED_HEADERS - headers.keys())
    if missing:
        raise MvsvFormatError(f"{path}: 缺少必填头部: {', '.join(missing)}")
    if headers["Format"] != "MVSV-1":
        raise MvsvFormatError(f"{path}: Format 必须严格为 MVSV-1")
    if headers["Field"] != _FIELDS:
        raise MvsvFormatError(f"{path}: Field 必须严格为 {_FIELDS}")
    count = _parseInteger(headers["Count"], "Count")
    if count < 0:
        raise MvsvFormatError("Count 不得为负数")
    if _parseInteger(headers["PriceAccuracy"], "PriceAccuracy") < 0:
        raise MvsvFormatError("PriceAccuracy 不得为负数")
    if _parseDecimal(headers["LotSize"], "LotSize") <= 0:
        raise MvsvFormatError("LotSize 必须大于零")
    try:
        timeZone = ZoneInfo(headers["EffectiveTimeZone"])
    except ZoneInfoNotFoundError as error:
        raise MvsvFormatError("EffectiveTimeZone 必须是有效 IANA 时区") from error
    return MvsvHeaderV1(values=headers, count=count, effectiveTimeZone=timeZone)


def _parseRecord(line: str, lineNumber: int, sequence: int, header: MvsvHeaderV1, path: Path) -> MvsvRecordV1:
    columns = line.split("|")
    if len(columns) != 11:
        raise MvsvFormatError(f"{path}:{lineNumber}: 数据行必须恰有 11 列")
    sourceSeconds = _parseInteger(columns[0], "ts")
    sourceTs = datetime.fromtimestamp(sourceSeconds, tz=timezone.utc)
    try:
        localWallTime = datetime.strptime(columns[1], "%Y%m%d%H%M%S")
    except ValueError as error:
        raise MvsvFormatError(f"{path}:{lineNumber}: dt 必须为 yyyyMMddHHmmss") from error
    localTime = localWallTime.replace(tzinfo=header.effectiveTimeZone)
    if sourceTs.astimezone(header.effectiveTimeZone).replace(tzinfo=None) != localWallTime:
        raise MvsvFormatError(f"{path}:{lineNumber}: ts 与 dt/EffectiveTimeZone 不一致")
    decimalValues = tuple(
        _parseDecimal(value, fieldName)
        for value, fieldName in zip(columns[2:], _FIELDS.split("|")[2:], strict=True)
    )
    openPrice, closePrice, lowPrice, highPrice, volume, turnover, change, changeRate, previousClose = decimalValues
    return MvsvRecordV1(
        sourceTs=sourceTs,
        sourceLocalTime=localTime,
        open=openPrice,
        close=closePrice,
        low=lowPrice,
        high=highPrice,
        volume=volume,
        turnover=turnover,
        change=change,
        changeRate=changeRate,
        previousClose=previousClose,
        sourceLine=lineNumber,
        sourceSequence=sequence,
    )


def _parseInteger(value: str, fieldName: str) -> int:
    if not value.isascii() or not value.isdecimal():
        raise MvsvFormatError(f"{fieldName} 必须是非负十进制整数")
    return int(value)


def _parseDecimal(value: str, fieldName: str) -> Decimal:
    if not value or value.strip() != value:
        raise MvsvFormatError(f"{fieldName} 必须是无空白的 Decimal")
    try:
        parsed = Decimal(value)
    except InvalidOperation as error:
        raise MvsvFormatError(f"{fieldName} 不是合法 Decimal") from error
    if not parsed.is_finite():
        raise MvsvFormatError(f"{fieldName} 不得为 NaN 或 Infinity")
    return parsed


def _unquote(value: str) -> str:
    if len(value) >= 2 and value.startswith('"') and value.endswith('"'):
        return value[1:-1]
    return value
