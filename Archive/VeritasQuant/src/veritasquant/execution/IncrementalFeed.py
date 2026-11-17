"""P2-006 增量市场事件接入与断流保护。

模拟盘使用增量行情（每次只推送新 Bar），而不是全量历史回放。
增量接入要求：

- 每个有效交易日/Bar 只增量推进一次，bar_start 必须严格递增；
- 超过预期间隔未收到增量事件（断流）进入保护状态；
- 断流状态下禁止新发单，直到恢复并确认数据连续性。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone

from veritasquant.data.MinuteBar import MinuteBarSchemaV1


class IncrementalFeedError(ValueError):
    """增量事件连续性或保护状态不满足契约。"""


@dataclass(frozen=True, slots=True)
class IncrementalFeedStateV1:
    """增量接入的当前状态。"""

    lastBarStart: datetime | None
    lastIngestedAt: datetime | None
    gapCount: int
    protected: bool


class IncrementalMarketFeedV1:
    """增量 Bar 接入：连续性校验与断流保护。"""

    def __init__(self, maxGapSeconds: int = 300) -> None:
        """maxGapSeconds：允许的最大事件间隔；超过即判定断流。"""
        if maxGapSeconds <= 0:
            raise IncrementalFeedError("最大间隔必须为正")
        self._maxGapSeconds = maxGapSeconds
        self._lastBarStart: datetime | None = None
        self._lastIngestedAt: datetime | None = None
        self._gapCount = 0
        self._protected = False

    @property
    def state(self) -> IncrementalFeedStateV1:
        return IncrementalFeedStateV1(
            self._lastBarStart,
            self._lastIngestedAt,
            self._gapCount,
            self._protected,
        )

    @property
    def protected(self) -> bool:
        """断流保护状态：为 True 时禁止新发单。"""
        return self._protected

    def ingest(self, bar: MinuteBarSchemaV1, ingestedAt: datetime | None = None) -> None:
        """增量接入一根新 Bar；断流或乱序进入保护状态。"""
        if self._protected:
            raise IncrementalFeedError("增量行情处于保护状态，拒绝接入直至恢复确认")
        now = ingestedAt or datetime.now(timezone.utc)
        if self._lastBarStart is not None:
            if bar.barStart <= self._lastBarStart:
                self._enterProtection(f"bar_start 未递增: {bar.barStart} <= {self._lastBarStart}")
                return
            gapSeconds = (bar.barStart - self._lastBarStart).total_seconds()
            if gapSeconds > self._maxGapSeconds:
                self._gapCount += 1
                self._enterProtection(f"增量断流：间隔 {gapSeconds}s 超过上限 {self._maxGapSeconds}s")
                return
        self._lastBarStart = bar.barStart
        self._lastIngestedAt = now

    def recover(self) -> None:
        """数据连续性确认后恢复；必须显式调用，禁止自动恢复发单。"""
        if not self._protected:
            raise IncrementalFeedError("未处于保护状态，无需恢复")
        self._protected = False
        self._gapCount = 0

    def _enterProtection(self, reason: str) -> None:
        self._protected = True
        self._lastReason = reason

    @property
    def lastProtectionReason(self) -> str | None:
        """最近一次进入保护状态的原因（诊断/审计）。"""
        return getattr(self, "_lastReason", None)
