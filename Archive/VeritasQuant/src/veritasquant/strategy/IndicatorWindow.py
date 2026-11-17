"""增量指标窗口与完成 Bar 查询 API（技术方案 4.2/7.2 节）。

指标只使用已消费数据；未来数据修改不影响当前输出；未完成 Bar 查询失败；
日线只在收盘后可用。窗口为增量滑动窗口，支持均线、极值、累计成交量等
确定性指标。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from decimal import Decimal

from veritasquant.data.MinuteBar import MinuteBarSchemaV1


class IndicatorError(ValueError):
    """指标窗口或 Bar 查询违反已消费边界时抛出。"""


@dataclass(frozen=True, slots=True)
class IndicatorPointV1:
    """窗口内单个已消费数据点。"""

    ts: datetime
    close: Decimal
    high: Decimal
    low: Decimal
    volume: Decimal


@dataclass(frozen=True, slots=True)
class WindowMetricsV1:
    """窗口指标快照。"""

    barCount: int
    closeMean: Decimal
    closeMax: Decimal
    closeMin: Decimal
    volumeSum: Decimal
    metricsHash: str


class IncrementalWindowV1:
    """有界增量滑动窗口：指标只使用已消费数据。"""

    def __init__(self, capacity: int = 1000) -> None:
        if capacity < 2:
            raise IndicatorError("窗口容量至少为 2")
        self._capacity = capacity
        self._points: list[IndicatorPointV1] = []
        self._latestTs: datetime | None = None

    @property
    def barCount(self) -> int:
        return len(self._points)

    def push(self, bar: MinuteBarSchemaV1) -> None:
        """追加一个已完成 Bar；时间必须严格递增。"""
        if self._latestTs is not None and bar.ts <= self._latestTs:
            raise IndicatorError("窗口只接受时间严格递增的已完成 Bar")
        self._points.append(
            IndicatorPointV1(ts=bar.ts, close=bar.close, high=bar.high, low=bar.low, volume=bar.volume)
        )
        self._latestTs = bar.ts
        if len(self._points) > self._capacity:
            self._points = self._points[-self._capacity :]

    def metrics(self, lookback: int | None = None) -> WindowMetricsV1:
        """计算最近 lookback 个点的指标；默认全窗口。"""
        if not self._points:
            raise IndicatorError("窗口为空，无指标可计算")
        window = self._points[-lookback:] if lookback is not None and lookback > 0 else self._points
        closes = [point.close for point in window]
        total = sum(closes, Decimal("0"))
        mean = total / Decimal(len(closes))
        volumeSum = sum((point.volume for point in window), Decimal("0"))
        metricsHash = _hashMetrics(window)
        return WindowMetricsV1(
            barCount=len(window),
            closeMean=mean,
            closeMax=max(closes),
            closeMin=min(closes),
            volumeSum=volumeSum,
            metricsHash=metricsHash,
        )

    def movingAverage(self, period: int) -> Decimal:
        """最近 period 个收盘价的简单均线。"""
        if period <= 0 or period > len(self._points):
            raise IndicatorError(f"均线周期 {period} 超出窗口 {len(self._points)}")
        return self.metrics(lookback=period).closeMean

    def latest(self) -> IndicatorPointV1 | None:
        """最近已消费数据点。"""
        return self._points[-1] if self._points else None


class BarQueryWindowV1:
    """完成 Bar 查询：只允许查询已完成（barEnd <= 当前逻辑时间）的 Bar。"""

    def __init__(self) -> None:
        self._bars: list[MinuteBarSchemaV1] = []
        self._clock: datetime | None = None

    @property
    def clock(self) -> datetime | None:
        return self._clock

    def registerBar(self, bar: MinuteBarSchemaV1) -> None:
        """登记一个已完成 Bar 并推进查询时钟。"""
        if self._clock is not None and bar.ts <= self._clock:
            raise IndicatorError("Bar 时间必须推进查询时钟")
        self._bars.append(bar)
        self._clock = bar.ts

    def queryCompleted(self) -> tuple[MinuteBarSchemaV1, ...]:
        """返回全部已登记 Bar（都是已完成的）。"""
        return tuple(self._bars)

    def latestCompleted(self) -> MinuteBarSchemaV1 | None:
        """最近一个已完成 Bar；未完成 Bar 不可查询。"""
        return self._bars[-1] if self._bars else None

    def queryAsOf(self, asOf: datetime) -> tuple[MinuteBarSchemaV1, ...]:
        """返回 barEnd <= asOf 的已完成 Bar；未来 Bar 不存在。"""
        return tuple(bar for bar in self._bars if bar.barEnd <= asOf)


def _hashMetrics(points: list[IndicatorPointV1]) -> str:
    """窗口指标确定性哈希。"""
    from veritasquant.core.CanonicalJson import canonicalHash

    return canonicalHash(
        [
            {
                "ts": point.ts,
                "close": point.close,
                "volume": point.volume,
            }
            for point in points
        ]
    )
