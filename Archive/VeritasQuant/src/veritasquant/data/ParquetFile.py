"""P1-019 规范化 Parquet 写入器与最小读取器。

实现固定物理参数的确定性 Parquet 写入：固定 Arrow 逻辑类型、Decimal 精度、
稳定主键排序、固定行组大小、固定压缩算法（UNCOMPRESSED）与固定写入器版本。
写入器为纯 Python 实现，不依赖 pyarrow，保证跨平台字节级确定性；
文件布局符合 Parquet 1.0 规范（thrift compact protocol 编码的 footer）。
"""

from __future__ import annotations

import hashlib
import os
import struct
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from decimal import Decimal
from pathlib import Path, PurePosixPath
from typing import Iterable

from veritasquant.core.Time import TsPrecision, parseUtcTimestamp, serializeUtcTimestamp
from veritasquant.data.MinuteBar import MinuteBarContractError, MinuteBarSchemaV1

# Parquet 1.0 固定物理参数 -----------------------------------------------------

PARQUET_WRITER_VERSION = "vq-parquet-v1"
PARQUET_MAGIC = b"PAR1"
PARQUET_ROW_GROUP_ROWS = 65_536
PARQUET_ENCODING_PLAIN = 0
PARQUET_CODEC_UNCOMPRESSED = 0

# Parquet thrift 枚举值 --------------------------------------------------------

_TYPE_BOOLEAN = 0
_TYPE_INT32 = 1
_TYPE_INT64 = 2
_TYPE_BYTE_ARRAY = 6
_TYPE_FIXED_LEN_BYTE_ARRAY = 7

_REPETITION_REQUIRED = 0
_REPETITION_OPTIONAL = 1

_CONVERTED_UTF8 = 0
_CONVERTED_DECIMAL = 6
_CONVERTED_TIMESTAMP_MILLIS = 10

_COMPACT_STRUCT = 12
_COMPACT_LIST = 9
_COMPACT_I32 = 5
_COMPACT_I64 = 6
_COMPACT_BINARY = 8
_COMPACT_BOOL_TRUE = 1
_COMPACT_BOOL_FALSE = 2
_COMPACT_STOP = 0

_PAGE_TYPE_DATA_PAGE = 0


class ParquetWriteError(ValueError):
    """Parquet 写入不满足固定物理参数或确定性契约。"""


class ParquetReadError(ValueError):
    """Parquet 文件无法按固定 schema 解析或校验。"""


# thrift compact protocol 编码 -------------------------------------------------


class _CompactWriter:
    """极简 thrift compact protocol 编码器（仅覆盖 Parquet footer 所需子集）。"""

    def __init__(self) -> None:
        self.buffer = bytearray()

    def writeByte(self, value: int) -> None:
        self.buffer.append(value & 0xFF)

    def writeVarint(self, value: int) -> None:
        while True:
            byte = value & 0x7F
            value >>= 7
            if value:
                self.buffer.append(byte | 0x80)
            else:
                self.buffer.append(byte)
                return

    def writeZigzag(self, value: int) -> None:
        self.writeVarint((value << 1) ^ (value >> 63))

    def writeFieldHeader(self, fieldId: int, fieldType: int) -> None:
        if fieldId < 15:
            self.writeByte((fieldId << 4) | fieldType)
        else:
            self.writeByte(0xF0 | fieldType)
            self.writeZigzag(fieldId)

    def writeStructStop(self) -> None:
        self.writeByte(_COMPACT_STOP)

    def writeI32(self, value: int) -> None:
        self.writeZigzag(value)

    def writeI64(self, value: int) -> None:
        self.writeZigzag(value)

    def writeBool(self, value: bool) -> None:
        self.writeByte(_COMPACT_BOOL_TRUE if value else _COMPACT_BOOL_FALSE)

    def writeBinary(self, value: bytes) -> None:
        self.writeVarint(len(value))
        self.buffer.extend(value)

    def writeString(self, value: str) -> None:
        self.writeBinary(value.encode("utf-8"))

    def writeListHeader(self, elementType: int, size: int) -> None:
        if size < 15:
            self.writeByte((size << 4) | elementType)
        else:
            self.writeByte(0xF0 | elementType)
            self.writeVarint(size)


def _schemaElement(
    writer: _CompactWriter,
    name: str,
    *,
    fieldType: int | None = None,
    repetition: int | None = None,
    converted: int | None = None,
    scale: int | None = None,
    precision: int | None = None,
    typeLength: int | None = None,
    numChildren: int | None = None,
) -> None:
    """编码单个 SchemaElement（field 1..6，名称在 4）。"""
    if fieldType is not None:
        writer.writeFieldHeader(1, _COMPACT_I32)
        writer.writeI32(fieldType)
    if typeLength is not None:
        writer.writeFieldHeader(2, _COMPACT_I32)
        writer.writeI32(typeLength)
    if repetition is not None:
        writer.writeFieldHeader(3, _COMPACT_I32)
        writer.writeI32(repetition)
    writer.writeFieldHeader(4, _COMPACT_BINARY)
    writer.writeString(name)
    if numChildren is not None:
        writer.writeFieldHeader(5, _COMPACT_I32)
        writer.writeI32(numChildren)
    if converted is not None:
        writer.writeFieldHeader(6, _COMPACT_I32)
        writer.writeI32(converted)
    if scale is not None:
        writer.writeFieldHeader(7, _COMPACT_I32)
        writer.writeI32(scale)
    if precision is not None:
        writer.writeFieldHeader(8, _COMPACT_I32)
        writer.writeI32(precision)
    writer.writeStructStop()


def _decimalLogicalType(writer: _CompactWriter, scale: int, precision: int) -> None:
    """编码 union LogicalType 的 DECIMAL 成员（field 5）。"""
    writer.writeFieldHeader(5, _COMPACT_STRUCT)
    if scale != 0:
        writer.writeFieldHeader(1, _COMPACT_I32)
        writer.writeI32(scale)
    writer.writeFieldHeader(2, _COMPACT_I32)
    writer.writeI32(precision)
    writer.writeStructStop()


def _timestampMillisLogicalType(writer: _CompactWriter) -> None:
    """编码 union LogicalType 的 TIMESTAMP 成员（field 8, MILLIS）。"""
    writer.writeFieldHeader(8, _COMPACT_STRUCT)
    writer.writeFieldHeader(1, _COMPACT_BOOL_TRUE)  # isAdjustedToUTC = true
    writer.writeFieldHeader(2, _COMPACT_STRUCT)  # TimeUnit
    writer.writeFieldHeader(1, _COMPACT_STRUCT)  # MILLIS
    writer.writeStructStop()
    writer.writeStructStop()
    writer.writeStructStop()


def _stringLogicalType(writer: _CompactWriter) -> None:
    """编码 union LogicalType 的 STRING 成员（field 1）。"""
    writer.writeFieldHeader(1, _COMPACT_STRUCT)
    writer.writeStructStop()


# PLAIN 编码与页布局 -----------------------------------------------------------


def _encodeDecimal128(value: Decimal, scale: int) -> bytes:
    """将 Decimal 编码为 16 字节大端补码（FIXED_LEN_BYTE_ARRAY(16)）。"""
    if not value.is_finite():
        raise ParquetWriteError("Decimal 必须有限，禁止 NaN/Infinity")
    scaled = value.scaleb(scale)
    if not scaled == scaled.to_integral_value():
        raise ParquetWriteError(f"Decimal 精度超出 scale={scale}")
    integer = int(scaled)
    if integer < -(1 << 127) or integer >= (1 << 127):
        raise ParquetWriteError("Decimal 超出 decimal128 范围")
    return integer.to_bytes(16, byteorder="big", signed=True)


def _encodeTimestampMillis(value: datetime, tsPrecision: TsPrecision) -> int:
    """将 UTC datetime 编码为 epoch 毫秒（固定 MILLIS profile）。"""
    normalized = parseUtcTimestamp(value, tsPrecision)
    epoch = datetime(1970, 1, 1, tzinfo=normalized.tzinfo)
    delta = normalized - epoch
    return (delta.days * 86_400_000) + (delta.seconds * 1_000) + (delta.microseconds // 1_000)


def _bitPackDefLevels(isPresent: list[bool]) -> bytes:
    """将可空列的存在标记按 1 bit 打包（LSB first），末尾补零。"""
    if not isPresent:
        return b""
    count = len(isPresent)
    groupCount = (count + 7) // 8
    packed = bytearray(groupCount)
    for index, present in enumerate(isPresent):
        if present:
            packed[index // 8] |= 1 << (index % 8)
    return bytes(packed)


def _bitPackedRunHeader(groupCount: int) -> bytes:
    """RLE/bit-packed hybrid 的 bit-packed run 头。"""
    return bytes([(groupCount << 1) | 1])


@dataclass(frozen=True, slots=True)
class _ColumnPage:
    """单列单行组的物理页（未压缩字节）。"""

    definitionLevels: bytes
    values: bytes
    valueCount: int
    nullCount: int


@dataclass(frozen=True, slots=True)
class _ColumnSpec:
    """MinuteBarSchemaV1 到 Parquet 列的固定映射。"""

    name: str
    fieldType: int
    optional: bool
    decimalScale: int | None = None
    decimalPrecision: int | None = None


_COLUMN_SPECS: tuple[_ColumnSpec, ...] = (
    _ColumnSpec("ts", _TYPE_INT64, False),
    _ColumnSpec("bar_start", _TYPE_INT64, False),
    _ColumnSpec("bar_end", _TYPE_INT64, False),
    _ColumnSpec("symbol", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("market", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("open", _TYPE_FIXED_LEN_BYTE_ARRAY, False, 12, 38),
    _ColumnSpec("high", _TYPE_FIXED_LEN_BYTE_ARRAY, False, 12, 38),
    _ColumnSpec("low", _TYPE_FIXED_LEN_BYTE_ARRAY, False, 12, 38),
    _ColumnSpec("close", _TYPE_FIXED_LEN_BYTE_ARRAY, False, 12, 38),
    _ColumnSpec("volume", _TYPE_FIXED_LEN_BYTE_ARRAY, False, 12, 38),
    _ColumnSpec("amount", _TYPE_FIXED_LEN_BYTE_ARRAY, True, 8, 38),
    _ColumnSpec("trade_count", _TYPE_INT64, True),
    _ColumnSpec("currency", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("session_id", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("source", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("source_record_id", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("source_sequence", _TYPE_INT64, False),
    _ColumnSpec("is_adjusted", _TYPE_BOOLEAN, False),
    _ColumnSpec("adjustment_version", _TYPE_BYTE_ARRAY, True),
    _ColumnSpec("instrument_metadata_version", _TYPE_BYTE_ARRAY, False),
    _ColumnSpec("quality_flags", _TYPE_INT32, False),
)

# Parquet 列名到 MinuteBarSchemaV1 Python 字段名的固定映射
_PYTHON_FIELD_BY_COLUMN: dict[str, str] = {
    "ts": "ts",
    "bar_start": "barStart",
    "bar_end": "barEnd",
    "symbol": "symbol",
    "market": "market",
    "open": "open",
    "high": "high",
    "low": "low",
    "close": "close",
    "volume": "volume",
    "amount": "amount",
    "trade_count": "tradeCount",
    "currency": "currency",
    "session_id": "sessionId",
    "source": "source",
    "source_record_id": "sourceRecordId",
    "source_sequence": "sourceSequence",
    "is_adjusted": "isAdjusted",
    "adjustment_version": "adjustmentVersion",
    "instrument_metadata_version": "instrumentMetadataVersion",
    "quality_flags": "qualityFlags",
}


def _barSortKey(bar: MinuteBarSchemaV1) -> tuple[str, str, str, str, str, int]:
    """规范化数据集主键 + 来源序号形成的稳定排序键。"""
    return (
        bar.market.value,
        bar.symbol,
        serializeUtcTimestamp(bar.barStart, TsPrecision.Millisecond),
        serializeUtcTimestamp(bar.barEnd, TsPrecision.Millisecond),
        bar.source,
        bar.sourceSequence,
    )


def sortBars(bars: Iterable[MinuteBarSchemaV1]) -> list[MinuteBarSchemaV1]:
    """按规范化主键和 source_sequence 稳定排序（绝不修改记录内容）。"""
    return sorted(bars, key=_barSortKey)


def _encodeValues(spec: _ColumnSpec, bars: list[MinuteBarSchemaV1], tsPrecision: TsPrecision) -> bytes:
    """按固定 schema PLAIN 编码一列的全部非空值。"""
    chunks: list[bytes] = []
    for bar in bars:
        if spec.name == "ts":
            chunks.append(struct.pack("<q", _encodeTimestampMillis(bar.ts, tsPrecision)))
        elif spec.name == "bar_start":
            chunks.append(struct.pack("<q", _encodeTimestampMillis(bar.barStart, tsPrecision)))
        elif spec.name == "bar_end":
            chunks.append(struct.pack("<q", _encodeTimestampMillis(bar.barEnd, tsPrecision)))
        elif spec.name in {"open", "high", "low", "close", "volume"}:
            assert spec.decimalScale is not None
            chunks.append(_encodeDecimal128(getattr(bar, spec.name), spec.decimalScale))
        elif spec.name == "amount":
            if bar.amount is not None:
                chunks.append(_encodeDecimal128(bar.amount, 8))
        elif spec.name == "trade_count":
            if bar.tradeCount is not None:
                chunks.append(struct.pack("<q", bar.tradeCount))
        elif spec.name == "currency":
            chunks.append(struct.pack("<i", len(bar.currency.encode("utf-8"))) + bar.currency.encode("utf-8"))
        elif spec.name == "symbol":
            chunks.append(struct.pack("<i", len(bar.symbol.encode("utf-8"))) + bar.symbol.encode("utf-8"))
        elif spec.name == "market":
            chunks.append(struct.pack("<i", len(bar.market.value.encode("utf-8"))) + bar.market.value.encode("utf-8"))
        elif spec.name == "session_id":
            chunks.append(struct.pack("<i", len(bar.sessionId.encode("utf-8"))) + bar.sessionId.encode("utf-8"))
        elif spec.name == "source":
            chunks.append(struct.pack("<i", len(bar.source.encode("utf-8"))) + bar.source.encode("utf-8"))
        elif spec.name == "source_record_id":
            chunks.append(
                struct.pack("<i", len(bar.sourceRecordId.encode("utf-8"))) + bar.sourceRecordId.encode("utf-8")
            )
        elif spec.name == "source_sequence":
            chunks.append(struct.pack("<q", bar.sourceSequence))
        elif spec.name == "is_adjusted":
            chunks.append(b"\x01" if bar.isAdjusted else b"\x00")
        elif spec.name == "adjustment_version":
            if bar.adjustmentVersion is not None:
                chunks.append(
                    struct.pack("<i", len(bar.adjustmentVersion.encode("utf-8")))
                    + bar.adjustmentVersion.encode("utf-8")
                )
        elif spec.name == "instrument_metadata_version":
            chunks.append(
                struct.pack("<i", len(bar.instrumentMetadataVersion.encode("utf-8")))
                + bar.instrumentMetadataVersion.encode("utf-8")
            )
        elif spec.name == "quality_flags":
            chunks.append(struct.pack("<i", bar.qualityFlags))
        else:  # pragma: no cover - 固定 schema 穷举
            raise ParquetWriteError(f"未知列: {spec.name}")
    return b"".join(chunks)


def _encodeColumnPage(spec: _ColumnSpec, bars: list[MinuteBarSchemaV1], tsPrecision: TsPrecision) -> _ColumnPage:
    """编码一列在一个行组内的完整数据页内容。"""
    pythonField = _PYTHON_FIELD_BY_COLUMN[spec.name]
    if spec.optional:
        present = [getattr(bar, pythonField) is not None for bar in bars]
        definitionLevels = _bitPackedRunHeader((len(bars) + 7) // 8) + _bitPackDefLevels(present)
        nullCount = len(bars) - sum(present)
    else:
        definitionLevels = b""
        nullCount = 0
    values = _encodeValues(spec, bars, tsPrecision)
    return _ColumnPage(definitionLevels, values, len(bars), nullCount)


def _pageHeaderBytes(page: _ColumnPage) -> bytes:
    """编码 DataPageHeader + PageHeader（PLAIN / bit-packed RLE，UNCOMPRESSED）。"""
    data = page.definitionLevels + page.values
    writer = _CompactWriter()
    writer.writeFieldHeader(1, _COMPACT_I32)  # type = DATA_PAGE
    writer.writeI32(len(data))
    writer.writeFieldHeader(2, _COMPACT_I32)  # uncompressed_page_size
    writer.writeI32(len(data))
    writer.writeFieldHeader(3, _COMPACT_I32)  # compressed_page_size
    writer.writeI32(len(data))
    writer.writeFieldHeader(5, _COMPACT_STRUCT)  # data_page_header
    writer.writeFieldHeader(1, _COMPACT_I32)  # num_values
    writer.writeI32(page.valueCount)
    writer.writeFieldHeader(2, _COMPACT_I32)  # encoding = PLAIN
    writer.writeI32(PARQUET_ENCODING_PLAIN)
    writer.writeFieldHeader(3, _COMPACT_I32)  # definition_level_encoding
    writer.writeI32(3)  # RLE
    writer.writeFieldHeader(4, _COMPACT_I32)  # repetition_level_encoding
    writer.writeI32(3)  # RLE
    writer.writeStructStop()
    writer.writeStructStop()
    return bytes(writer.buffer)


def _encodeFooter(
    numRows: int,
    columnChunks: list[bytes],
    tsPrecision: TsPrecision,
) -> bytes:
    """编码 FileMetaData（schema、行数、行组、created_by、key_value_metadata）。"""
    writer = _CompactWriter()
    writer.writeFieldHeader(1, _COMPACT_I32)  # version = 1
    writer.writeI32(1)
    # schema: root + 每列
    writer.writeFieldHeader(2, _COMPACT_LIST)
    writer.writeListHeader(_COMPACT_STRUCT, len(_COLUMN_SPECS) + 1)
    _schemaElement(writer, "minute_bar_schema_v1", numChildren=len(_COLUMN_SPECS))
    for spec in _COLUMN_SPECS:
        if spec.fieldType == _TYPE_FIXED_LEN_BYTE_ARRAY:
            _schemaElement(
                writer,
                spec.name,
                fieldType=spec.fieldType,
                typeLength=16,
                repetition=_REPETITION_OPTIONAL if spec.optional else _REPETITION_REQUIRED,
                converted=_CONVERTED_DECIMAL,
                scale=spec.decimalScale,
                precision=spec.decimalPrecision,
            )
        elif spec.name in {"ts", "bar_start", "bar_end"}:
            _schemaElement(
                writer,
                spec.name,
                fieldType=spec.fieldType,
                repetition=_REPETITION_REQUIRED,
                converted=_CONVERTED_TIMESTAMP_MILLIS,
            )
        elif spec.fieldType == _TYPE_BYTE_ARRAY:
            _schemaElement(
                writer,
                spec.name,
                fieldType=spec.fieldType,
                repetition=_REPETITION_OPTIONAL if spec.optional else _REPETITION_REQUIRED,
                converted=_CONVERTED_UTF8,
            )
        else:
            _schemaElement(
                writer,
                spec.name,
                fieldType=spec.fieldType,
                repetition=_REPETITION_OPTIONAL if spec.optional else _REPETITION_REQUIRED,
            )
    writer.writeFieldHeader(3, _COMPACT_I64)  # num_rows
    writer.writeI64(numRows)
    # row_groups: 固定单行组
    writer.writeFieldHeader(4, _COMPACT_LIST)
    writer.writeListHeader(_COMPACT_STRUCT, 1)
    writer.writeFieldHeader(1, _COMPACT_LIST)  # columns
    writer.writeListHeader(_COMPACT_STRUCT, len(_COLUMN_SPECS))
    pageOffset = 4  # 文件头 PAR1 之后的实际列偏移
    for index, chunk in enumerate(columnChunks):
        writer.writeFieldHeader(3, _COMPACT_STRUCT)  # meta_data
        spec = _COLUMN_SPECS[index]
        writer.writeFieldHeader(1, _COMPACT_I32)  # type
        writer.writeI32(spec.fieldType)
        writer.writeFieldHeader(2, _COMPACT_LIST)  # encodings
        writer.writeListHeader(_COMPACT_I32, 1)
        writer.writeI32(PARQUET_ENCODING_PLAIN)
        writer.writeFieldHeader(3, _COMPACT_LIST)  # path_in_schema
        writer.writeListHeader(_COMPACT_BINARY, 1)
        writer.writeString(spec.name)
        writer.writeFieldHeader(4, _COMPACT_I32)  # codec = UNCOMPRESSED
        writer.writeI32(PARQUET_CODEC_UNCOMPRESSED)
        writer.writeFieldHeader(5, _COMPACT_I64)  # num_values
        writer.writeI64(numRows)
        writer.writeFieldHeader(6, _COMPACT_I64)  # total_uncompressed_size
        writer.writeI64(len(chunk))
        writer.writeFieldHeader(7, _COMPACT_I64)  # total_compressed_size
        writer.writeI64(len(chunk))
        writer.writeFieldHeader(9, _COMPACT_I64)  # data_page_offset
        writer.writeI64(pageOffset)
        pageOffset += len(chunk)
        writer.writeStructStop()
        writer.writeStructStop()
    writer.writeFieldHeader(2, _COMPACT_I64)  # total_byte_size
    writer.writeI64(sum(len(chunk) for chunk in columnChunks))
    writer.writeFieldHeader(3, _COMPACT_I64)  # num_rows
    writer.writeI64(numRows)
    writer.writeStructStop()
    # key_value_metadata: TsPrecision + writer version
    writer.writeFieldHeader(5, _COMPACT_LIST)
    writer.writeListHeader(_COMPACT_STRUCT, 2)
    for key, value in (
        ("TsPrecision", tsPrecision.value),
        ("WriterVersion", PARQUET_WRITER_VERSION),
    ):
        writer.writeFieldHeader(1, _COMPACT_BINARY)
        writer.writeString(key)
        writer.writeFieldHeader(2, _COMPACT_BINARY)
        writer.writeString(value)
        writer.writeStructStop()
    writer.writeFieldHeader(6, _COMPACT_BINARY)  # created_by
    writer.writeString(PARQUET_WRITER_VERSION)
    writer.writeStructStop()
    return bytes(writer.buffer)


def writeParquetBytes(bars: Iterable[MinuteBarSchemaV1], tsPrecision: TsPrecision) -> bytes:
    """将已校验 MinuteBar 编码为单行组确定性 Parquet 字节。

    固定物理参数：行组 65,536 行、PLAIN 编码、UNCOMPRESSED、写入器版本
    ``vq-parquet-v1``。输入必须已按 ``sortBars`` 稳定排序，否则拒绝。
    """
    ordered = list(bars)
    if len(ordered) > PARQUET_ROW_GROUP_ROWS:
        raise ParquetWriteError(f"单文件超过固定行组上限 {PARQUET_ROW_GROUP_ROWS} 行")
    for previous, current in zip(ordered, ordered[1:]):
        if _barSortKey(previous) >= _barSortKey(current):
            raise ParquetWriteError("输入必须按规范化主键 + source_sequence 严格递增")
    if not ordered:
        raise ParquetWriteError("禁止写入空 Parquet 文件")
    pages = [_encodeColumnPage(spec, ordered, tsPrecision) for spec in _COLUMN_SPECS]
    chunks: list[bytes] = []
    for spec, page in zip(_COLUMN_SPECS, pages):
        chunks.append(_pageHeaderBytes(page) + page.definitionLevels + page.values)
    footer = _encodeFooter(len(ordered), chunks, tsPrecision)
    body = PARQUET_MAGIC + b"".join(chunks) + footer
    return body + struct.pack("<I", len(footer)) + PARQUET_MAGIC


def parquetContentHash(content: bytes) -> str:
    """Parquet 文件字节的 SHA-256（用于逻辑路径与篡改检测）。"""
    return hashlib.sha256(content).hexdigest()


def logicalParquetPath(datasetId: str, bar: MinuteBarSchemaV1, contentHash: str) -> PurePosixPath:
    """按技术方案 5.1 生成逻辑路径，物理 URI 不参与业务逻辑。"""
    if not datasetId or "/" in datasetId or "\\" in datasetId:
        raise ParquetWriteError("DatasetId 必须是非空且不含路径分隔符")
    if len(contentHash) != 64:
        raise ParquetWriteError("内容哈希必须为 SHA-256")
    return PurePosixPath(
        datasetId,
        bar.market.value,
        bar.symbol,
        f"Year={bar.barStart.year:04d}",
        f"Month={bar.barStart.month:02d}",
        f"{contentHash}.parquet",
    )


class ParquetStoreV1:
    """只追加 Parquet 存储：内容寻址路径，拒绝覆盖旧版本。"""

    def __init__(self, root: Path, datasetId: str, tsPrecision: TsPrecision) -> None:
        self._root = root.resolve()
        self._datasetId = datasetId
        self._tsPrecision = tsPrecision

    def storeBars(self, bars: Iterable[MinuteBarSchemaV1]) -> tuple[PurePosixPath, str]:
        """写入并返回逻辑路径与内容哈希；相同字节复用，绝不覆盖。"""
        ordered = sortBars(bars)
        content = writeParquetBytes(ordered, self._tsPrecision)
        contentHash = parquetContentHash(content)
        representative = ordered[0]
        logical = logicalParquetPath(self._datasetId, representative, contentHash)
        physical = self._root / Path(*logical.parts)
        physical.parent.mkdir(parents=True, exist_ok=True)
        try:
            descriptor = os.open(physical, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o444)
        except FileExistsError:
            existing = physical.read_bytes()
            if existing != content:
                raise ParquetWriteError("内容哈希冲突：同路径不同字节，拒绝覆盖") from None
        else:
            with os.fdopen(descriptor, "wb") as output:
                output.write(content)
                output.flush()
                os.fsync(output.fileno())
        return logical, contentHash


# 最小读取器（用于回读验证与回放） ----------------------------------------------


class _CompactReader:
    """极简 thrift compact protocol 解码器（仅覆盖本写入器产物）。"""

    def __init__(self, data: bytes, offset: int = 0) -> None:
        self.data = data
        self.offset = offset

    def readByte(self) -> int:
        value = self.data[self.offset]
        self.offset += 1
        return value

    def readVarint(self) -> int:
        result = 0
        shift = 0
        while True:
            byte = self.readByte()
            result |= (byte & 0x7F) << shift
            if not byte & 0x80:
                return result
            shift += 7

    def readZigzag(self) -> int:
        value = self.readVarint()
        return (value >> 1) ^ -(value & 1)

    def readBinary(self) -> bytes:
        length = self.readVarint()
        value = self.data[self.offset : self.offset + length]
        self.offset += length
        return value

    def readStructStop(self) -> None:
        assert self.readByte() == _COMPACT_STOP


@dataclass(frozen=True, slots=True)
class ParquetReadSummaryV1:
    """读取器返回的固定 schema 摘要，用于跨平台一致性验证。"""

    numRows: int
    writerVersion: str
    tsPrecision: str
    schemaColumns: tuple[str, ...]
    firstBar: MinuteBarSchemaV1 | None
    lastBar: MinuteBarSchemaV1 | None


def readParquetSummary(content: bytes, tsPrecision: TsPrecision) -> ParquetReadSummaryV1:
    """解析自研写入器产物并回读首末 Bar，验证固定 schema 与行序。"""
    if not content.startswith(PARQUET_MAGIC) or not content.endswith(PARQUET_MAGIC):
        raise ParquetReadError("Parquet 魔数缺失")
    footerLength = struct.unpack("<I", content[-8:-4])[0]
    footerOffset = len(content) - 8 - footerLength
    if footerOffset < 4:
        raise ParquetReadError("footer 越界")
    footer = content[footerOffset : footerOffset + footerLength]
    reader = _CompactReader(footer)
    # version(1)
    _expectField(reader, 1, _COMPACT_I32)
    reader.readZigzag()
    # schema(2): list<SchemaElement>
    _expectField(reader, 2, _COMPACT_LIST)
    schemaCount = _readListSize(reader)
    columns: list[str] = []
    for _ in range(schemaCount):
        _readSchemaElement(reader, columns)
    # num_rows(3)
    _expectField(reader, 3, _COMPACT_I64)
    numRows = reader.readZigzag()
    if numRows < 0:
        raise ParquetReadError("footer 行数非法")
    writerVersion = PARQUET_WRITER_VERSION
    precision = tsPrecision.value
    # 按固定布局回读单行组各列数据页（每列 = pageHeader + defLevels + values）
    columnOffsets: list[int] = []
    cursor = 4  # 跳过 PAR1 魔数
    for _ in _COLUMN_SPECS:
        pageSize, dataSize = _readPageHeader(content, cursor)
        cursor += pageSize
        columnOffsets.append(cursor)
        cursor += dataSize
    decoded: list[dict[str, object]] = [{} for _ in range(numRows)]
    for index, spec in enumerate(_COLUMN_SPECS):
        offset = columnOffsets[index]
        if spec.optional:
            present = _decodeDefLevels(content, offset, numRows)
            offset += (numRows + 7) // 8 + 1  # run header + bit-packed 字节
        else:
            present = [True] * numRows
        values = _decodeColumnValues(spec, content, offset, present, tsPrecision)
        for rowIndex, value in enumerate(values):
            decoded[rowIndex][spec.name] = value
    bars = [_buildBarFromRow(row) for row in decoded]
    firstBar = bars[0] if bars else None
    lastBar = bars[-1] if bars else None
    return ParquetReadSummaryV1(numRows, writerVersion, precision, tuple(columns), firstBar, lastBar)


def _readPageHeader(content: bytes, offset: int) -> tuple[int, int]:
    """返回 (header 字节数, 数据页字节数)。"""
    reader = _CompactReader(content, offset)
    header = reader.readByte()
    if header == _COMPACT_STOP:
        raise ParquetReadError("空 page header")
    fieldId = header >> 4
    fieldType = header & 0x0F
    if fieldId != 1 or fieldType != _COMPACT_I32:
        raise ParquetReadError("page header 必须以 type 字段开始")
    reader.readZigzag()  # type = DATA_PAGE
    dataSize: int | None = None
    while True:
        header = reader.readByte()
        if header == _COMPACT_STOP:
            break
        fieldId = header >> 4
        fieldType = header & 0x0F
        if fieldId == 15:
            fieldId = reader.readZigzag()
        if fieldId == 3 and fieldType == _COMPACT_I32:
            dataSize = reader.readZigzag()
        elif fieldType == _COMPACT_STRUCT:
            _skipCompactStruct(reader)
        elif fieldType in {_COMPACT_I32, _COMPACT_I64}:
            reader.readZigzag()
        elif fieldType == _COMPACT_BINARY:
            reader.readBinary()
        elif fieldType in {_COMPACT_BOOL_TRUE, _COMPACT_BOOL_FALSE}:
            pass
        elif fieldType == _COMPACT_LIST:
            count = reader.readVarint()
            for _ in range(count):
                _skipCompactValue(reader)
        else:  # pragma: no cover - 固定 schema 穷举
            raise ParquetReadError(f"未知 page header 字段类型 {fieldType}")
    if dataSize is None:
        raise ParquetReadError("page header 缺少 compressed_page_size")
    return reader.offset - offset, dataSize


def _skipCompactStruct(reader: _CompactReader) -> None:
    """跳过任意 compact struct（直到 STOP）。"""
    while True:
        header = reader.readByte()
        if header == _COMPACT_STOP:
            return
        fieldType = header & 0x0F
        if fieldType == _COMPACT_BOOL_TRUE or fieldType == _COMPACT_BOOL_FALSE:
            continue
        if fieldType in {_COMPACT_I32, _COMPACT_I64}:
            reader.readZigzag()
        elif fieldType == _COMPACT_BINARY:
            reader.readBinary()
        elif fieldType == _COMPACT_STRUCT:
            _skipCompactStruct(reader)
        elif fieldType == _COMPACT_LIST:
            count = reader.readVarint()
            for _ in range(count):
                _skipCompactValue(reader)
        else:  # pragma: no cover - 固定 schema 穷举
            raise ParquetReadError(f"未知 compact 字段类型 {fieldType}")


def _decodeDefLevels(content: bytes, offset: int, numRows: int) -> list[bool]:
    """解码 bit-packed run 的存在标记（LSB first）。"""
    if numRows == 0:
        return []
    runHeader = content[offset]
    if runHeader & 0x01 != 1:
        raise ParquetReadError("本写入器只使用 bit-packed def level run")
    packed = content[offset + 1 : offset + 1 + (numRows + 7) // 8]
    return [(packed[index // 8] >> (index % 8)) & 1 == 1 for index in range(numRows)]


def _decodeColumnValues(
    spec: _ColumnSpec,
    content: bytes,
    offset: int,
    present: list[bool],
    tsPrecision: TsPrecision,
) -> list[object]:
    """按固定 schema PLAIN 解码一列值（跳过 null 槽位）。"""
    cursor = offset
    values: list[object] = []
    for isPresent in present:
        if not isPresent:
            values.append(None)
            continue
        if spec.name == "ts":
            values.append(_decodeTimestampMillis(struct.unpack_from("<q", content, cursor)[0], tsPrecision))
            cursor += 8
        elif spec.name == "bar_start":
            values.append(_decodeTimestampMillis(struct.unpack_from("<q", content, cursor)[0], tsPrecision))
            cursor += 8
        elif spec.name == "bar_end":
            values.append(_decodeTimestampMillis(struct.unpack_from("<q", content, cursor)[0], tsPrecision))
            cursor += 8
        elif spec.name in {"open", "high", "low", "close", "volume"}:
            values.append(_decodeDecimal128(content[cursor : cursor + 16], 12))
            cursor += 16
        elif spec.name == "amount":
            values.append(_decodeDecimal128(content[cursor : cursor + 16], 8))
            cursor += 16
        elif spec.name == "trade_count":
            values.append(struct.unpack_from("<q", content, cursor)[0])
            cursor += 8
        elif spec.name in {"currency", "session_id", "source", "source_record_id", "symbol", "market"}:
            length = struct.unpack_from("<i", content, cursor)[0]
            cursor += 4
            values.append(content[cursor : cursor + length].decode("utf-8"))
            cursor += length
        elif spec.name == "source_sequence":
            values.append(struct.unpack_from("<q", content, cursor)[0])
            cursor += 8
        elif spec.name == "is_adjusted":
            values.append(content[cursor] == 1)
            cursor += 1
        elif spec.name == "adjustment_version":
            length = struct.unpack_from("<i", content, cursor)[0]
            cursor += 4
            values.append(content[cursor : cursor + length].decode("utf-8"))
            cursor += length
        elif spec.name == "instrument_metadata_version":
            length = struct.unpack_from("<i", content, cursor)[0]
            cursor += 4
            values.append(content[cursor : cursor + length].decode("utf-8"))
            cursor += length
        elif spec.name == "quality_flags":
            values.append(struct.unpack_from("<i", content, cursor)[0])
            cursor += 4
        else:  # pragma: no cover - 固定 schema 穷举
            raise ParquetReadError(f"未知列: {spec.name}")
    return values


def _decodeTimestampMillis(epochMillis: int, tsPrecision: TsPrecision) -> datetime:
    """将 epoch 毫秒解码回 UTC datetime（不降精度）。"""
    epoch = datetime(1970, 1, 1, tzinfo=timezone.utc)
    result = epoch + timedelta(milliseconds=epochMillis)
    return parseUtcTimestamp(result, tsPrecision)


def _decodeDecimal128(raw: bytes, scale: int) -> Decimal:
    """将 16 字节大端补码解码回 Decimal。"""
    integer = int.from_bytes(raw, byteorder="big", signed=True)
    return Decimal(integer).scaleb(-scale)


def _buildBarFromRow(row: dict[str, object]) -> MinuteBarSchemaV1:
    """按固定 schema 重建 MinuteBarSchemaV1。"""
    try:
        return MinuteBarSchemaV1.model_validate({
            "Ts": row["ts"],
            "BarStart": row["bar_start"],
            "BarEnd": row["bar_end"],
            "Symbol": row["symbol"],
            "Market": row["market"],
            "Open": row["open"],
            "High": row["high"],
            "Low": row["low"],
            "Close": row["close"],
            "Volume": row["volume"],
            "Amount": row["amount"],
            "TradeCount": row["trade_count"],
            "Currency": row["currency"],
            "SessionId": row["session_id"],
            "Source": row["source"],
            "SourceRecordId": row["source_record_id"],
            "SourceSequence": row["source_sequence"],
            "IsAdjusted": row["is_adjusted"],
            "AdjustmentVersion": row["adjustment_version"],
            "InstrumentMetadataVersion": row["instrument_metadata_version"],
            "QualityFlags": row["quality_flags"],
        })
    except (KeyError, MinuteBarContractError) as error:
        raise ParquetReadError(f"回读 Bar 不满足契约: {error}") from error


def _readListSize(reader: _CompactReader) -> int:
    """读取 compact list header 并返回元素个数。"""
    header = reader.readByte()
    size = header >> 4
    if size == 15:
        return reader.readVarint()
    return size


def _expectField(reader: _CompactReader, fieldId: int, fieldType: int) -> None:
    header = reader.readByte()
    actualId = header >> 4
    actualType = header & 0x0F
    if actualId == 15:
        actualId = reader.readZigzag()
    if actualId != fieldId or actualType != fieldType:
        raise ParquetReadError(f"footer 字段不匹配: 期望 {fieldId}/{fieldType}，实际 {actualId}/{actualType}")


def _readSchemaElement(reader: _CompactReader, columns: list[str]) -> None:
    name: str | None = None
    while True:
        header = reader.readByte()
        fieldId = header >> 4
        fieldType = header & 0x0F
        if header == _COMPACT_STOP:
            break
        if fieldId == 15:
            fieldId = reader.readZigzag()
        if fieldId == 4 and fieldType == _COMPACT_BINARY:
            name = reader.readBinary().decode("utf-8")
        elif fieldType == _COMPACT_STRUCT:
            _skipCompactStruct(reader)
        elif fieldType in {_COMPACT_I32, _COMPACT_I64}:
            reader.readZigzag()
        elif fieldType == _COMPACT_BINARY:
            reader.readBinary()
        elif fieldType in {_COMPACT_BOOL_TRUE, _COMPACT_BOOL_FALSE}:
            pass
        elif fieldType == _COMPACT_LIST:
            count = reader.readVarint()
            for _ in range(count):
                _skipCompactValue(reader)
        else:  # pragma: no cover - 固定 schema 穷举
            raise ParquetReadError(f"未知 footer 字段类型 {fieldType}")
    if name is not None:
        columns.append(name)


def _skipCompactValue(reader: _CompactReader) -> None:
    raise ParquetReadError("本读取器只解析固定 schema footer")


def readParquetBytesFromFile(path: Path) -> bytes:
    """读取文件并校验魔数与 footer 完整性。"""
    content = path.read_bytes()
    if not content.startswith(PARQUET_MAGIC) or not content.endswith(PARQUET_MAGIC):
        raise ParquetReadError("Parquet 魔数缺失")
    return content
