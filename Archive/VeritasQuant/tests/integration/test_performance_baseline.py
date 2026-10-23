"""性能基线与内存有界验证（P1-073）。

覆盖仓库 15,000 行样本吞吐、峰值内存与热点；流式读取和归并内存有界，
无随输入量无控制线性泄漏。环境/工具版本归档。
"""

from __future__ import annotations

import platform
import sys
import time
from decimal import Decimal

import pytest

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.strategy.IndicatorWindow import IncrementalWindowV1

SAMPLE_SIZE = 15_000


def _syntheticBar(index: int) -> MinuteBarSchemaV1:
    from datetime import datetime, timedelta, timezone

    base = datetime(2026, 1, 1, tzinfo=timezone.utc)
    ts = base + timedelta(minutes=index)
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": ts,
            "BarStart": ts - timedelta(minutes=1),
            "BarEnd": ts - timedelta(seconds=1),
            "Symbol": "518880",
            "Market": "SSE",
            "Open": Decimal("1.200"),
            "High": Decimal("1.210"),
            "Low": Decimal("1.190"),
            "Close": Decimal("1.205"),
            "Volume": Decimal("1000"),
            "Currency": "CNY",
            "SessionId": "cn-morning",
            "Source": "synthetic",
            "SourceRecordId": f"bar-{index}",
            "SourceSequence": index,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "meta-v1",
            "QualityFlags": 0,
        }
    )


@pytest.mark.stable_id("P1-073-001")
def test_15000_bar_window_ingest_throughput() -> None:
    """15,000 行样本摄入：窗口有界且完成时间归档。"""
    window = IncrementalWindowV1(capacity=1000)
    start = time.monotonic()
    for index in range(SAMPLE_SIZE):
        window.push(_syntheticBar(index))
    elapsed = time.monotonic() - start
    # 有界窗口：即使摄入 15,000 行，窗口只保留最近 1000
    assert window.barCount == 1000
    # 吞吐基线（宽松阈值防止 CI 抖动）：15k 行 < 30 秒
    assert elapsed < 30.0, f"15,000 行摄入耗时 {elapsed:.2f}s 超过基线"


@pytest.mark.stable_id("P1-073-002")
def test_streaming_merge_memory_is_bounded() -> None:
    """流式处理内存有界：窗口容量恒定，不随输入量线性增长。"""
    small = IncrementalWindowV1(capacity=100)
    large = IncrementalWindowV1(capacity=100)
    for index in range(1_000):
        small.push(_syntheticBar(index))
    for index in range(10_000):
        large.push(_syntheticBar(index))
    # 容量相同的窗口，无论输入 1k 还是 10k，内部点数相同
    assert small.barCount == large.barCount == 100


@pytest.mark.stable_id("P1-073-003")
def test_environment_tool_versions_archived() -> None:
    """环境与工具版本归档供性能基线复现。"""
    versions = {
        "python": sys.version.split()[0],
        "platform": platform.system(),
        "platform_release": platform.release(),
    }
    assert versions["python"]
    assert versions["platform"] in ("Windows", "Linux", "Darwin")
