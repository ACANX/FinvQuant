"""P2-036 Prometheus 指标注册表与采集器单元测试。"""

from __future__ import annotations

import pytest

from veritasquant.monitoring.MetricsCollector import (
    CollectorInputs,
    MetricsCollector,
    WallClockLatencyRecorder,
)
from veritasquant.monitoring.PrometheusMetrics import (
    DEFAULT_BUCKETS_SECONDS,
    MetricsRegistry,
)


class _FakeReadiness:
    def __init__(self, ready: bool, failed: int, state: str = "TRADING_READY") -> None:
        self._ready = ready
        self._failed = failed
        self._state = state

    def ready(self) -> bool:
        return self._ready

    def state(self) -> str:
        return self._state

    def failedChecks(self) -> tuple[object, ...]:
        return (object(),) * self._failed


class _FakeOutbox:
    def __init__(self, age: float | None, count: int) -> None:
        self._age = age
        self._count = count

    def oldestUnconfirmedAgeSeconds(self) -> float | None:
        return self._age

    def unconfirmedCount(self) -> int:
        return self._count


class _FakeQueue:
    def __init__(self, utilization: float, connected: bool, pending: int) -> None:
        self._utilization = utilization
        self._connected = connected
        self._pending = pending

    def utilization(self) -> float:
        return self._utilization

    def connected(self) -> bool:
        return self._connected

    def pendingCount(self) -> int:
        return self._pending


class TestMetricsRegistry:
    def test_counter_inc_and_render(self) -> None:
        reg = MetricsRegistry()
        counter = reg.counter("orders_total", "订单计数")
        counter.inc(3.0, {"account_group": "a"})
        counter.inc(2.0, {"account_group": "a"})
        counter.inc(1.0, {"account_group": "b"})
        text = reg.render()
        assert "# HELP vq_orders_total 订单计数" in text
        assert "# TYPE vq_orders_total counter" in text
        assert 'vq_orders_total{account_group="a"} 5.0' in text
        assert 'vq_orders_total{account_group="b"} 1.0' in text

    def test_gauge_set(self) -> None:
        reg = MetricsRegistry()
        gauge = reg.gauge("queue_utilization", "队列利用率")
        gauge.set(0.75)
        assert 'vq_queue_utilization 0.75' in reg.render()
        gauge.set(0.5)
        assert 'vq_queue_utilization 0.5' in reg.render()

    def test_histogram_buckets_and_sum_count(self) -> None:
        reg = MetricsRegistry()
        hist = reg.histogram("latency_seconds", "延迟", buckets=(0.1, 0.5, 1.0))
        hist.observe(0.05)
        hist.observe(0.2)
        hist.observe(0.8)
        text = reg.render()
        assert "# TYPE vq_latency_seconds histogram" in text
        # 桶累计：0.05<=0.1 → 1；0.2 落入 0.5；0.8 落入 1.0；+Inf=3
        assert 'vq_latency_seconds_bucket{le="0.1"} 1.0' in text
        assert 'vq_latency_seconds_bucket{le="0.5"} 2.0' in text
        assert 'vq_latency_seconds_bucket{le="1.0"} 3.0' in text
        assert 'vq_latency_seconds_bucket{le="+Inf"} 3.0' in text
        assert "vq_latency_seconds_sum 1.05" in text
        assert "vq_latency_seconds_count 3.0" in text

    def test_histogram_with_labels(self) -> None:
        reg = MetricsRegistry()
        hist = reg.histogram("event_latency", "事件延迟", buckets=(0.1,))
        hist.observe(0.05, {"account_group": "g1"})
        text = reg.render()
        assert 'vq_event_latency_bucket{account_group="g1",le="0.1"} 1.0' in text
        assert 'vq_event_latency_sum{account_group="g1"} 0.05' in text

    def test_duplicate_register_same_type_returns_same(self) -> None:
        reg = MetricsRegistry()
        a = reg.counter("x_total")
        b = reg.counter("x_total")
        assert a is b

    def test_conflicting_type_raises(self) -> None:
        reg = MetricsRegistry()
        reg.counter("x_total")
        with pytest.raises(ValueError, match="已注册"):
            reg.gauge("x_total")

    def test_namespace_prefix(self) -> None:
        reg = MetricsRegistry(namespace="custom")
        reg.counter("hits_total").inc()
        assert "# TYPE custom_hits_total counter" in reg.render()

    def test_invalid_name_rejected(self) -> None:
        reg = MetricsRegistry()
        with pytest.raises(ValueError):
            reg.counter("bad name")


class TestMetricsCollector:
    def test_collect_all_readiness_ready(self) -> None:
        reg = MetricsRegistry()
        collector = MetricsCollector(reg)
        inputs = CollectorInputs(
            readiness=_FakeReadiness(True, 0, "TRADING_READY"),
            outbox=_FakeOutbox(5.0, 3),
            queue=_FakeQueue(0.3, True, 10),
            logDegraded=False,
            logDroppedCount=0,
            errorCodeCounts={1001: 2, 2002: 1},
        )
        collector.collectAll(inputs)
        text = reg.render()
        assert 'vq_trading_readiness_state{state="TRADING_READY"} 1.0' in text
        assert "vq_readiness_checks_failed 0.0" in text
        assert "vq_outbox_oldest_unconfirmed_age_seconds 5.0" in text
        assert "vq_outbox_unconfirmed_count 3.0" in text
        assert "vq_queue_utilization 0.3" in text
        assert "vq_queue_connected 1.0" in text
        assert 'vq_api_error_codes_total{code="1001"} 2.0' in text
        assert 'vq_api_error_codes_total{code="2002"} 1.0' in text
        assert "vq_structured_log_degraded 0.0" in text

    def test_collect_readiness_not_ready(self) -> None:
        reg = MetricsRegistry()
        collector = MetricsCollector(reg)
        collector.collectReadiness(_FakeReadiness(False, 2, "PROTECTED"))
        text = reg.render()
        assert 'vq_trading_readiness_state{state="PROTECTED"} 0.0' in text
        assert "vq_readiness_checks_failed 2.0" in text

    def test_observe_order_and_ledger(self) -> None:
        reg = MetricsRegistry()
        collector = MetricsCollector(reg)
        collector.observeOrderTransition("FILLED", "g1")
        collector.observeOrderTransition("FILLED", "g1")
        collector.observeOrderRejection("RISK")
        collector.observeLedgerCommit(0.05, "g1")
        text = reg.render()
        assert 'vq_order_state_transitions_total{account_group="g1",state="FILLED"} 2.0' in text
        assert 'vq_order_rejections_total{reason="RISK"} 1.0' in text
        assert 'vq_ledger_commits_total{account_group="g1"} 1.0' in text
        assert 'vq_ledger_commit_latency_seconds_bucket{account_group="g1",le="0.1"} 1.0' in text

    def test_log_degradation(self) -> None:
        reg = MetricsRegistry()
        collector = MetricsCollector(reg)
        collector.collectLogState(True, 7)
        text = reg.render()
        assert "vq_structured_log_degraded 1.0" in text
        assert "vq_structured_log_dropped_total 7.0" in text

    def test_wall_clock_recorder(self) -> None:
        reg = MetricsRegistry()
        collector = MetricsCollector(reg)
        recorder = WallClockLatencyRecorder(collector, "g1")
        recorder.observeEventIngest()
        text = reg.render()
        assert 'vq_event_ingest_latency_seconds_count{account_group="g1"} 1.0' in text


class TestDefaultRegistry:
    def test_default_registry_singleton(self) -> None:
        from veritasquant.monitoring import getDefaultRegistry

        assert getDefaultRegistry() is getDefaultRegistry()
        assert getDefaultRegistry().namespace == "vq"

    def test_default_buckets_constant(self) -> None:
        assert DEFAULT_BUCKETS_SECONDS[0] == 0.005
        assert DEFAULT_BUCKETS_SECONDS[-1] == 10.0
