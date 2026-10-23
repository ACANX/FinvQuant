"""运行日志、指标、通知与可观测性组件。"""

from __future__ import annotations

from veritasquant.monitoring.PrometheusMetrics import (
    DEFAULT_BUCKETS_SECONDS,
    LabelSet,
    MetricFamily,
    MetricType,
    MetricsRegistry,
)

# 进程级默认注册表：无显式注册表时 /metrics 与采集器共用此实例。
_defaultRegistry: MetricsRegistry | None = None


def getDefaultRegistry() -> MetricsRegistry:
    """返回进程级默认指标注册表（惰性创建）。"""
    global _defaultRegistry
    if _defaultRegistry is None:
        _defaultRegistry = MetricsRegistry()
    return _defaultRegistry


__all__ = [
    "DEFAULT_BUCKETS_SECONDS",
    "LabelSet",
    "MetricFamily",
    "MetricType",
    "MetricsRegistry",
    "getDefaultRegistry",
]
