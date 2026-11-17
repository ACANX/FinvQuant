"""P2-036 Prometheus 文本格式指标注册表（纯标准库实现）。

- 不引入 prometheus-client 外部依赖（避免许可证审批与锁文件变更）；
- 支持 Counter / Gauge / Histogram 三类指标与标签；
- 导出 Prometheus 文本格式 0.0.4（text/plain; version=0.0.4），
  可直接被 Prometheus 抓取器解析；
- 线程安全：所有写操作使用互斥锁；导出在锁内取快照，避免半写状态。
"""

from __future__ import annotations

import threading
from dataclasses import dataclass
from typing import Iterable

# 默认直方图桶：覆盖事件延迟、账本提交等 p99 观察（秒）。
DEFAULT_BUCKETS_SECONDS = (
    0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0,
)


class MetricType:
    COUNTER = "counter"
    GAUGE = "gauge"
    HISTOGRAM = "histogram"


@dataclass(frozen=True, slots=True)
class LabelSet:
    """有序标签对；导出时按键名排序保证确定性输出。"""

    labels: tuple[tuple[str, str], ...] = ()

    @classmethod
    def fromMapping(cls, mapping: dict[str, str] | None) -> "LabelSet":
        if not mapping:
            return cls()
        return cls(tuple(sorted(mapping.items())))

    def merged(self, extra: dict[str, str]) -> "LabelSet":
        combined = dict(self.labels)
        combined.update(extra)
        return LabelSet.fromMapping(combined)

    def asText(self) -> str:
        if not self.labels:
            return ""
        return "{" + ",".join(
            f'{k}="{_escapeLabelValue(v)}"' for k, v in self.labels
        ) + "}"


def _escapeLabelValue(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


def _formatValue(value: float) -> str:
    if value == float("inf"):
        return "+Inf"
    if value != value:  # NaN
        return "NaN"
    return repr(float(value))


class MetricFamily:
    """单指标族：名称 + 帮助文本 + 类型 + 按标签聚合的样本。"""

    def __init__(
        self,
        name: str,
        helpText: str,
        metricType: str,
        buckets: tuple[float, ...] = DEFAULT_BUCKETS_SECONDS,
    ) -> None:
        if not name or any(ch in name for ch in "{} \t"):
            raise ValueError(f"非法指标名: {name!r}")
        self.name = name
        self.helpText = helpText
        self.metricType = metricType
        self.buckets = buckets
        # gauge/counter：labels -> 当前值
        self._values: dict[LabelSet, float] = {}
        # histogram：labels -> (sum, count, {le: cumulativeCount})
        self._hist: dict[LabelSet, tuple[float, float, dict[str, float]]] = {}
        self._lock = threading.Lock()

    # ---- 写操作 ----
    def inc(self, value: float = 1.0, labels: dict[str, str] | None = None) -> None:
        if self.metricType == MetricType.HISTOGRAM:
            raise TypeError("直方图不支持 inc，请使用 observe")
        key = LabelSet.fromMapping(labels)
        with self._lock:
            self._values[key] = self._values.get(key, 0.0) + value

    def set(self, value: float, labels: dict[str, str] | None = None) -> None:
        if self.metricType == MetricType.HISTOGRAM:
            raise TypeError("直方图不支持 set，请使用 observe")
        key = LabelSet.fromMapping(labels)
        with self._lock:
            self._values[key] = float(value)

    def observe(self, value: float, labels: dict[str, str] | None = None) -> None:
        """直方图观察：按桶累计、更新总和与计数。"""
        if self.metricType != MetricType.HISTOGRAM:
            raise TypeError("只有直方图支持 observe")
        key = LabelSet.fromMapping(labels)
        with self._lock:
            total, count, buckets = self._hist.get(key, (0.0, 0.0, {}))
            total += float(value)
            count += 1.0
            for b in self.buckets:
                bucketKey = _formatValue(b)
                current = buckets.get(bucketKey, 0.0)
                buckets[bucketKey] = current + (1.0 if value <= b else 0.0)
            infKey = _formatValue(float("inf"))
            buckets[infKey] = count
            self._hist[key] = (total, count, buckets)

    def snapshot(self) -> dict[LabelSet, float]:
        with self._lock:
            return dict(self._values)

    # ---- 文本导出 ----
    def render(self) -> str:
        lines: list[str] = []
        lines.append(f"# HELP {self.name} {self.helpText}")
        lines.append(f"# TYPE {self.name} {self.metricType}")
        if self.metricType == MetricType.HISTOGRAM:
            lines.extend(self._renderHistogram())
        else:
            values = self.snapshot()
            for key in sorted(values, key=lambda k: k.asText()):
                lines.append(f"{self.name}{key.asText()} {_formatValue(values[key])}")
        return "\n".join(lines) + "\n"

    def _renderHistogram(self) -> list[str]:
        with self._lock:
            hist = {k: (s, c, dict(b)) for k, (s, c, b) in self._hist.items()}
        lines: list[str] = []
        for key in sorted(hist, key=lambda k: k.asText()):
            total, count, buckets = hist[key]
            for b in self.buckets:
                bucketKey = _formatValue(b)
                sampleKey = key.merged({"le": bucketKey})
                lines.append(
                    f"{self.name}_bucket{sampleKey.asText()} "
                    f"{_formatValue(buckets.get(bucketKey, 0.0))}"
                )
            infKey = _formatValue(float("inf"))
            sampleKey = key.merged({"le": infKey})
            lines.append(f"{self.name}_bucket{sampleKey.asText()} {_formatValue(count)}")
            lines.append(f"{self.name}_sum{key.asText()} {_formatValue(total)}")
            lines.append(f"{self.name}_count{key.asText()} {_formatValue(count)}")
        return lines


class MetricsRegistry:
    """指标注册表：统一管理指标族并导出 Prometheus 文本。"""

    def __init__(self, namespace: str = "vq") -> None:
        self.namespace = namespace
        self._families: dict[str, MetricFamily] = {}
        self._lock = threading.Lock()

    def _fullName(self, name: str) -> str:
        if name.startswith(self.namespace + "_"):
            return name
        return f"{self.namespace}_{name}"

    def counter(
        self, name: str, helpText: str = "", labels: Iterable[str] = ()
    ) -> MetricFamily:
        return self._register(name, helpText, MetricType.COUNTER)

    def gauge(
        self, name: str, helpText: str = "", labels: Iterable[str] = ()
    ) -> MetricFamily:
        return self._register(name, helpText, MetricType.GAUGE)

    def histogram(
        self,
        name: str,
        helpText: str = "",
        buckets: tuple[float, ...] = DEFAULT_BUCKETS_SECONDS,
    ) -> MetricFamily:
        return self._register(name, helpText, MetricType.HISTOGRAM, buckets)

    def _register(
        self,
        name: str,
        helpText: str,
        metricType: str,
        buckets: tuple[float, ...] = DEFAULT_BUCKETS_SECONDS,
    ) -> MetricFamily:
        full = self._fullName(name)
        with self._lock:
            existing = self._families.get(full)
            if existing is not None:
                if existing.metricType != metricType:
                    raise ValueError(
                        f"指标 {full} 已注册为 {existing.metricType}，不能注册为 {metricType}"
                    )
                return existing
            family = MetricFamily(full, helpText, metricType, buckets)
            self._families[full] = family
            return family

    def render(self) -> str:
        with self._lock:
            families = list(self._families.values())
        parts = [f.render() for f in sorted(families, key=lambda f: f.name)]
        return "".join(parts)

    def textFormat(self) -> str:
        """Prometheus 文本格式 0.0.4。"""
        return self.render()
