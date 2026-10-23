"""P1-022 有界内存顺序迭代器与多源最小堆归并。

每个输入源提供顺序迭代器；归并器按 EventOrderingVersion V1 的完整排序键
（ts + phase + priority + source_rank + source_sequence + event_id）从最小堆
逐条弹出最早事件，保证跨文件结果符合完整排序键，且内存占用有界。
乱序来源被拒绝（严格模式）或隔离（宽容模式），绝不静默重排。
"""

from __future__ import annotations

from collections.abc import Callable, Iterable, Iterator
from dataclasses import dataclass
from heapq import heappop, heappush
from typing import TypeVar

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.Time import TsPrecision
from veritasquant.data.ParquetFile import (
    ParquetReadSummaryV1,
    ParquetReadError,
    readParquetSummary,
    readParquetBytesFromFile,
)

T = TypeVar("T")

SortKey = tuple[str, int, int, int, int, str]


class MergeError(ValueError):
    """输入源或归并序列不满足确定性契约。"""


class OrderedSourceError(MergeError):
    """单个来源违反顺序迭代器契约。"""


def makeEventSortKey(
    event: EventEnvelopeV1,
    tsPrecision: TsPrecision,
) -> SortKey:
    """构造 EventOrderingVersion V1 的完整排序键（与 EventOrdering 一致）。"""
    from veritasquant.core.EventOrdering import eventOrderingKey

    return eventOrderingKey(event, tsPrecision)


class SequentialIteratorV1(Iterator[T]):
    """有界内存顺序迭代器：逐条拉取并校验单调递增。

    迭代器每次只持有当前元素，绝不缓存整列数据；严格模式下发现排序键
    倒退立即抛出 OrderedSourceError。
    """

    def __init__(
        self,
        items: Iterable[T],
        sortKey: Callable[[T], object],
        *,
        strict: bool = True,
    ) -> None:
        self._iterator = iter(items)
        self._sortKey = sortKey
        self._strict = strict
        self._lastKey: object | None = None
        self._invalidCount = 0
        self._current: T | None = None
        self._currentKey: object | None = None
        self._exhausted = False
        self._advance()

    def _advance(self) -> None:
        """拉取下一条；返回 False 表示来源耗尽。"""
        self._current = None
        self._currentKey = None
        try:
            item = next(self._iterator)
        except StopIteration:
            self._exhausted = True
            return
        key = self._sortKey(item)
        if self._lastKey is not None and _compareKeys(key, self._lastKey) < 0:
            self._invalidCount += 1
            if self._strict:
                raise OrderedSourceError("来源顺序倒退，严格模式拒绝")
            self._advance()
            return
        self._current = item
        self._currentKey = key
        self._lastKey = key

    def __next__(self) -> T:
        if self._exhausted or self._current is None:
            raise StopIteration
        item = self._current
        self._advance()
        return item

    @property
    def current(self) -> T | None:
        """当前未消费元素（供归并器 peek）。"""
        return self._current

    @property
    def currentKey(self) -> object | None:
        """当前元素的排序键（供归并器比较）。"""
        return self._currentKey

    @property
    def invalidCount(self) -> int:
        """被拒绝或隔离的乱序元素计数。"""
        return self._invalidCount

    @property
    def exhausted(self) -> bool:
        return self._exhausted


class EventFileIteratorV1(Iterator[EventEnvelopeV1]):
    """从已校验 Parquet 文件流式读取事件（占位：事件序列由后续任务接入）。

    阶段 1 数据管道中事件文件按固定 schema 写入；此迭代器包装
    ``readParquetSummary`` 的逐条读取路径，保持与数据层一致。
    """

    def __init__(self, path: str, tsPrecision: TsPrecision) -> None:
        self._path = path
        self._tsPrecision = tsPrecision
        self._summary: ParquetReadSummaryV1 | None = None

    def open(self) -> "EventFileIteratorV1":
        try:
            content = readParquetBytesFromFile(__import__("pathlib").Path(self._path))
            self._summary = readParquetSummary(content, self._tsPrecision)
        except ParquetReadError as error:
            raise OrderedSourceError(f"事件文件不可解析: {error}") from error
        return self

    def __iter__(self) -> "EventFileIteratorV1":
        return self

    def __next__(self) -> EventEnvelopeV1:
        raise StopIteration  # 事件序列文件由 P1-026 事件总线任务定义

    @property
    def summary(self) -> ParquetReadSummaryV1 | None:
        return self._summary


@dataclass(slots=True)
class _HeapEntry:
    """最小堆条目：排序键 + 来源序号，保证同键时按来源稳定。"""

    key: tuple[object, ...]
    sourceIndex: int
    iterator: SequentialIteratorV1[EventEnvelopeV1]

    def __lt__(self, other: "_HeapEntry") -> bool:
        if self.key != other.key:
            return self.key < other.key
        return self.sourceIndex < other.sourceIndex


class MinHeapMergerV1:
    """多源最小堆归并器：内存占用 = 源数量，与文件大小无关。"""

    def __init__(
        self,
        sources: list[SequentialIteratorV1[EventEnvelopeV1]],
        tsPrecision: TsPrecision,
        *,
        strict: bool = True,
    ) -> None:
        if not sources:
            raise MergeError("至少需要一个输入来源")
        self._sources = sources
        self._tsPrecision = tsPrecision
        self._strict = strict
        self._heap: list[_HeapEntry] = []
        for index, source in enumerate(sources):
            if source.current is not None and source.currentKey is not None:
                key = _normalizeKey(source.currentKey)
                heappush(self._heap, _HeapEntry(key, index, source))

    def next(self) -> EventEnvelopeV1 | None:
        """弹出并返回当前最早事件；所有来源耗尽时返回 None。"""
        if not self._heap:
            return None
        entry = heappop(self._heap)
        event = entry.iterator.current
        if event is None:
            raise MergeError("堆条目缺少当前事件")
        entry.iterator._advance()  # noqa: SLF001 - 内部推进有界迭代器
        if entry.iterator.current is not None and entry.iterator.currentKey is not None:
            key = _normalizeKey(entry.iterator.currentKey)
            heappush(self._heap, _HeapEntry(key, entry.sourceIndex, entry.iterator))
        return event

    def drain(self) -> list[EventEnvelopeV1]:
        """依次弹出全部事件，返回完整有序序列。"""
        result: list[EventEnvelopeV1] = []
        while True:
            event = self.next()
            if event is None:
                return result
            result.append(event)

    def merge(
        self,
        sources: list[SequentialIteratorV1[EventEnvelopeV1]],
    ) -> list[EventEnvelopeV1]:
        """便捷入口：以当前 tsPrecision 归并指定来源。"""
        merger = MinHeapMergerV1(sources, self._tsPrecision, strict=self._strict)
        return merger.drain()


def _normalizeKey(key: object) -> tuple[object, ...]:
    """将排序键归一化为可哈希元组。"""
    if isinstance(key, tuple):
        return key
    return (key,)


def _compareKeys(left: object, right: object) -> int:
    """比较两个排序键；tuple 键逐项比较，否则直接比较。"""
    if isinstance(left, tuple) and isinstance(right, tuple):
        if left < right:
            return -1
        if left > right:
            return 1
        return 0
    if left < right:  # type: ignore[operator]
        return -1
    if left > right:  # type: ignore[operator]
        return 1
    return 0
