"""P3-002 信号生成与幂等发布测试。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.signals.SignalGenerator import (
    InMemorySignalStoreV1,
    PublishResultV1,
    SignalGenerationError,
    SignalGeneratorV1,
    SignalIntentV1,
    SignalPublisherV1,
)
from veritasquant.signals.SignalReference import SignalStatus

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _intent(**overrides: object) -> SignalIntentV1:
    values: dict[str, object] = {
        "accountId": "acc-001",
        "strategyId": "strat-dual-ma",
        "strategyChecksum": "b" * 64,
        "sourceEventId": "evt-bar-001",
        "sourceEventType": "MarketBarEvent",
        "direction": "BUY",
        "quantity": "100.0000",
        "priceLimit": "5.0000",
        "availableTs": _T0,
    }
    values.update(overrides)
    return SignalIntentV1(**values)


class TestSignalGenerator:
    def test_generate_deterministic(self) -> None:
        generator = SignalGeneratorV1()
        first = generator.generate(_intent(), "sig-001")
        second = generator.generate(_intent(), "sig-002")
        # 相同输入：方向/数量/策略 checksum 一致
        assert first.direction == second.direction == "BUY"
        assert first.quantity == second.quantity
        assert first.strategyChecksum == second.strategyChecksum
        assert first.status is SignalStatus.Pending

    def test_content_checksum_stable(self) -> None:
        generator = SignalGeneratorV1()
        intentA = _intent()
        intentB = _intent(availableTs=datetime(2026, 8, 3, 3, 0, 0, tzinfo=timezone.utc))
        # 内容 checksum 与 availableTs 无关
        assert generator.contentChecksum(intentA) == generator.contentChecksum(intentB)

    def test_content_checksum_changes_with_strategy(self) -> None:
        generator = SignalGeneratorV1()
        intentA = _intent()
        intentB = _intent(strategyChecksum="c" * 64)
        assert generator.contentChecksum(intentA) != generator.contentChecksum(intentB)

    def test_generated_ts_from_available_ts(self) -> None:
        generator = SignalGeneratorV1()
        signal = generator.generate(_intent(), "sig-001", expirySeconds=300)
        assert signal.generatedTs == _T0
        assert signal.expiresAt is not None
        assert (signal.expiresAt - signal.generatedTs).total_seconds() == 300

    def test_rejects_invalid_intent(self) -> None:
        with pytest.raises(SignalGenerationError):
            _intent(direction="SHORT")
        with pytest.raises(SignalGenerationError):
            _intent(strategyChecksum="short")
        with pytest.raises(SignalGenerationError):
            _intent(quantity="")

    def test_rejects_non_positive_expiry(self) -> None:
        generator = SignalGeneratorV1()
        with pytest.raises(SignalGenerationError):
            generator.generate(_intent(), "sig-001", expirySeconds=0)


class TestSignalPublisher:
    def test_publish_new_signal(self) -> None:
        store = InMemorySignalStoreV1()
        publisher = SignalPublisherV1(store)
        result = publisher.publish(_intent(), "sig-001")
        assert isinstance(result, PublishResultV1)
        assert result.duplicate is False
        assert result.signal.signalReferenceId == "sig-001"
        assert len(store.all()) == 1

    def test_duplicate_event_no_duplicate_signal(self) -> None:
        """重复事件不重复信号：同键同内容返回既有信号。"""
        store = InMemorySignalStoreV1()
        publisher = SignalPublisherV1(store)
        publisher.publish(_intent(), "sig-001")
        second = publisher.publish(_intent(), "sig-002")
        assert second.duplicate is True
        assert second.signal.signalReferenceId == "sig-001"
        assert len(store.all()) == 1

    def test_conflict_same_key_different_content(self) -> None:
        """同键不同内容：拒绝并留档冲突。"""
        store = InMemorySignalStoreV1()
        publisher = SignalPublisherV1(store)
        publisher.publish(_intent(), "sig-001")
        with pytest.raises(SignalGenerationError, match="幂等冲突"):
            publisher.publish(_intent(quantity="200.0000"), "sig-002")
        conflicts = publisher.conflicts()
        assert len(conflicts) == 1
        assert conflicts[0].existingSignalId == "sig-001"
        assert conflicts[0].incomingHash != conflicts[0].existingHash

    def test_store_rejects_duplicate_id(self) -> None:
        store = InMemorySignalStoreV1()
        publisher = SignalPublisherV1(store)
        publisher.publish(_intent(), "sig-001")
        from veritasquant.signals.SignalReference import SignalContractError

        with pytest.raises(SignalContractError):
            store.save(publisher.publish(_intent(sourceEventId="evt-bar-002"), "sig-001").signal)
