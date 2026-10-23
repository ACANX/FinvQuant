"""防前视探针与变形测试（技术方案 13 章量化 gate）。

注入/修改未来数据或重排无关未来事件时当前决策不变，命中数为 0。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Callable

from veritasquant.core.CanonicalJson import canonicalHash


class LookaheadError(ValueError):
    """防前视探针配置或决策函数违反契约时抛出。"""


@dataclass(frozen=True, slots=True)
class LookaheadProbeReportV1:
    """防前视探针报告。"""

    probeId: str
    baselineDecisionHash: str
    mutatedDecisionHash: str
    hits: int
    passes: bool
    seed: int


class LookaheadProbeV1:
    """对决策函数注入未来/无关事件扰动，统计决策变化命中数。"""

    def __init__(self, seed: int = 20260802) -> None:
        self._seed = seed

    def probe(
        self,
        *,
        probeId: str,
        decisionFn: Callable[[tuple[dict[str, Any], ...]], str],
        baselineEvents: tuple[dict[str, Any], ...],
        futureEvent: dict[str, Any],
        unrelatedEvent: dict[str, Any],
    ) -> LookaheadProbeReportV1:
        """基线决策 vs 注入未来事件/重排无关事件后的决策。"""
        if not baselineEvents:
            raise LookaheadError("基线事件不能为空")
        baselineHash = decisionFn(baselineEvents)

        # 注入未来事件：事件 ts 晚于当前窗口最后事件
        withFuture = baselineEvents + (futureEvent,)
        futureHash = decisionFn(withFuture)

        # 重排无关未来事件：打乱与当前决策无关的事件顺序
        reordered = baselineEvents[:-1] + (unrelatedEvent, baselineEvents[-1]) if len(baselineEvents) > 1 else withFuture
        reorderedHash = decisionFn(reordered)

        hits = 0
        if futureHash != baselineHash:
            hits += 1
        if reorderedHash != baselineHash:
            hits += 1
        return LookaheadProbeReportV1(
            probeId=probeId,
            baselineDecisionHash=baselineHash,
            mutatedDecisionHash=canonicalHash({"future": futureHash, "reordered": reorderedHash}),
            hits=hits,
            passes=hits == 0,
            seed=self._seed,
        )


class FutureAwareDecisionFnV1:
    """示例决策函数：只使用窗口内最后事件（防前视基线）。"""

    @staticmethod
    def decide(events: tuple[dict[str, Any], ...]) -> str:
        if not events:
            return "noop"
        last = events[-1]
        return canonicalHash({"last_ts": last.get("ts"), "close": last.get("close")})
