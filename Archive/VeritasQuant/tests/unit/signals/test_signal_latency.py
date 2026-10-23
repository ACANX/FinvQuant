"""P3-006 信号端到端延迟 SLI 与告警测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.signals.SignalLatency import (
    InMemoryLatencyStoreV1,
    SignalDeliverySampleV1,
    SignalLatencyAlertV1,
    SignalLatencyError,
    SignalLatencyEvaluatorV1,
)

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _sample(delaySeconds: float, **overrides: object) -> SignalDeliverySampleV1:
    values: dict[str, object] = {
        "sampleId": f"s-{delaySeconds}",
        "signalReferenceId": "sig-ref-001",
        "accountId": "acc-001",
        "runId": "run-001",
        "eventAvailableTs": _T0,
        "deliveredTs": _T0 + timedelta(seconds=delaySeconds),
        "channel": "GUI",
    }
    values.update(overrides)
    return SignalDeliverySampleV1(**values)


class TestSignalDeliverySample:
    def test_valid(self) -> None:
        sample = _sample(2.5)
        assert sample.latencySeconds == 2.5

    def test_delivered_before_available_rejected(self) -> None:
        with pytest.raises(SignalLatencyError, match="不得早于"):
            _sample(-1)

    def test_requires_identity_fields(self) -> None:
        with pytest.raises(SignalLatencyError):
            _sample(1.0, signalReferenceId="")


class TestSignalLatencyEvaluator:
    def test_no_samples_insufficient_evidence(self) -> None:
        """缺样本不判通过。"""
        evaluator = SignalLatencyEvaluatorV1()
        evaluation = evaluator.evaluate([])
        assert evaluation.passed is False
        assert evaluation.evidenceStatus == "INSUFFICIENT_EVIDENCE"
        assert evaluation.sli.p50Seconds is None

    def test_all_within_target_passes(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        samples = [_sample(1.0), _sample(2.0), _sample(3.0), _sample(4.0)]
        evaluation = evaluator.evaluate(samples)
        assert evaluation.passed is True
        assert evaluation.evidenceStatus == "SUFFICIENT"
        assert evaluation.sli.withinTargetRatio == 1.0

    def test_percentiles_correct(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        samples = [_sample(i) for i in range(1, 101)]  # 1..100 秒
        evaluation = evaluator.evaluate(samples)
        assert evaluation.sli.p50Seconds is not None
        assert evaluation.sli.p95Seconds is not None
        assert evaluation.sli.p99Seconds is not None
        assert 49 <= evaluation.sli.p50Seconds <= 51
        assert 94 <= evaluation.sli.p95Seconds <= 96
        assert 98 <= evaluation.sli.p99Seconds <= 100

    def test_ratio_below_target_fails(self) -> None:
        """99.5% 目标：98/100 达标 -> FAIL。"""
        evaluator = SignalLatencyEvaluatorV1()
        samples = [_sample(9.0) for _ in range(98)] + [_sample(15.0), _sample(20.0)]
        evaluation = evaluator.evaluate(samples)
        assert evaluation.passed is False
        assert evaluation.sli.withinTargetRatio == 0.98

    def test_single_sample(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        evaluation = evaluator.evaluate([_sample(5.0)])
        assert evaluation.sli.p50Seconds == 5.0
        assert evaluation.passed is True

    def test_alert_on_violation(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        samples = [_sample(9.0) for _ in range(98)] + [_sample(15.0), _sample(20.0)]
        evaluation = evaluator.evaluate(samples)
        alert = evaluator.buildAlert(
            accountId="acc-001",
            runId="run-001",
            evaluation=evaluation,
            dispositionLink="/ops/run-001",
        )
        assert alert is not None
        assert isinstance(alert, SignalLatencyAlertV1)
        assert alert.accountId == "acc-001"
        assert alert.runId == "run-001"
        assert alert.dispositionLink == "/ops/run-001"

    def test_no_alert_when_passed(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        evaluation = evaluator.evaluate([_sample(1.0)])
        alert = evaluator.buildAlert(accountId="acc-001", runId="run-001", evaluation=evaluation)
        assert alert is None

    def test_no_alert_when_insufficient(self) -> None:
        evaluator = SignalLatencyEvaluatorV1()
        evaluation = evaluator.evaluate([])
        alert = evaluator.buildAlert(accountId="acc-001", runId="run-001", evaluation=evaluation)
        assert alert is None

    def test_invalid_config(self) -> None:
        with pytest.raises(SignalLatencyError):
            SignalLatencyEvaluatorV1(targetSeconds=0)
        with pytest.raises(SignalLatencyError):
            SignalLatencyEvaluatorV1(passRatio=1.5)


class TestInMemoryLatencyStore:
    def test_record_and_clear(self) -> None:
        store = InMemoryLatencyStoreV1()
        store.record(_sample(1.0))
        store.record(_sample(2.0))
        assert len(store.all()) == 2
        store.clear()
        assert len(store.all()) == 0
