"""P2-009 背压与 trading-readiness 单元测试。

验收标准映射：
- 70%/90% 阈值行为符合方案；
- 硬阈值时禁止新增风险且不丢关键事件。
"""

from __future__ import annotations

import pytest

from veritasquant.monitoring.TradingReadiness import (
    DiskSpacePolicyV1,
    QueueThresholdPolicyV1,
    ThresholdLevel,
    TradingReadinessGateV1,
    TradingReadinessState,
)


class TestQueueThreshold:
    def test_70_and_90_percent_thresholds(self) -> None:
        policy = QueueThresholdPolicyV1()
        assert policy.evaluate(0.50).level is ThresholdLevel.Normal
        assert policy.evaluate(0.70).level is ThresholdLevel.Warning
        assert policy.evaluate(0.90).level is ThresholdLevel.Critical
        assert policy.evaluate(1.00).level is ThresholdLevel.Critical

    def test_hard_threshold_blocks_new_risk(self) -> None:
        policy = QueueThresholdPolicyV1()
        assert policy.mayOpenNewRisk(0.89)
        assert not policy.mayOpenNewRisk(0.90)

    def test_critical_writes_never_dropped(self) -> None:
        """硬阈值时关键 inbox/账本/控制/审计写入不得丢弃。"""
        policy = QueueThresholdPolicyV1()
        assert policy.allowCriticalWrites(1.00)

    def test_invalid_utilization_rejected(self) -> None:
        with pytest.raises(ValueError):
            QueueThresholdPolicyV1().evaluate(1.5)


class TestDiskSpace:
    def test_20_and_10_percent_free_ratios(self) -> None:
        policy = DiskSpacePolicyV1()
        assert policy.evaluate(0.30).level is ThresholdLevel.Normal
        assert policy.evaluate(0.19).level is ThresholdLevel.Warning
        assert policy.evaluate(0.09).level is ThresholdLevel.Critical

    def test_invalid_ratio_rejected(self) -> None:
        with pytest.raises(ValueError):
            DiskSpacePolicyV1().evaluate(-0.1)


class TestTradingReadinessGate:
    def _healthy(self) -> dict:
        return {
            "marketFresh": True,
            "reconciliationComplete": True,
            "ledgerInvariantsHeld": True,
            "controlsRecovered": True,
            "outboxUtilization": 0.3,
            "queueUtilization": 0.4,
            "diskFreeRatio": 0.35,
            "clockSkewSeconds": 0.1,
        }

    def test_healthy_state_is_ready(self) -> None:
        report = TradingReadinessGateV1().evaluate(**self._healthy())
        assert report.state is TradingReadinessState.Ready
        assert report.ready
        assert report.failedChecks == ()

    def test_queue_hard_limit_blocks_readiness(self) -> None:
        kwargs = self._healthy()
        kwargs["queueUtilization"] = 0.95
        report = TradingReadinessGateV1().evaluate(**kwargs)
        assert report.state is TradingReadinessState.NotReady
        assert any(check.name == "queue_below_hard_limit" for check in report.failedChecks)

    def test_disk_hard_limit_blocks_readiness(self) -> None:
        kwargs = self._healthy()
        kwargs["diskFreeRatio"] = 0.05
        report = TradingReadinessGateV1().evaluate(**kwargs)
        assert not report.ready

    def test_stale_market_blocks_readiness(self) -> None:
        kwargs = self._healthy()
        kwargs["marketFresh"] = False
        assert not TradingReadinessGateV1().evaluate(**kwargs).ready

    def test_clock_skew_over_500ms_blocks_readiness(self) -> None:
        kwargs = self._healthy()
        kwargs["clockSkewSeconds"] = 0.6
        report = TradingReadinessGateV1().evaluate(**kwargs)
        assert not report.ready
        assert any(check.name == "clock_sync" for check in report.failedChecks)

    def test_reconciliation_missing_blocks_readiness(self) -> None:
        kwargs = self._healthy()
        kwargs["reconciliationComplete"] = False
        assert not TradingReadinessGateV1().evaluate(**kwargs).ready
