"""P3-003 通知路由、模板、重试和失败隔离测试。"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

import pytest

from veritasquant.signals.NotificationRouter import (
    ChannelKind,
    DeliveryStatus,
    InMemoryDeliveryStoreV1,
    NotificationError,
    NotificationRouterV1,
    NotificationTemplateV1,
    RecordingDeliverySinkV1,
)
from veritasquant.signals.SignalReference import SignalReferenceV1, SignalStatus

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _signal() -> SignalReferenceV1:
    return SignalReferenceV1.create(
        signalReferenceId="sig-ref-001",
        version=1,
        status=SignalStatus.Pending,
        accountId="acc-001",
        strategyId="strat-dual-ma",
        strategyChecksum="a" * 64,
        sourceEventId="evt-bar-001",
        sourceEventType="MarketBarEvent",
        direction="BUY",
        quantity="100.0000",
        priceLimit="5.0000",
        operatorId=None,
        generatedTs=_T0,
        expiresAt=_T0 + timedelta(minutes=15),
        previousSignalReferenceId=None,
    )


class TestNotificationTemplate:
    def test_render_deterministic(self) -> None:
        template = NotificationTemplateV1(
            subjectTemplate="[{direction}] {signal_reference_id}",
            bodyTemplate="{account_id}|{strategy_id}|{quantity}|{price_limit}",
        )
        subject, body = template.render(_signal())
        assert subject == "[BUY] sig-ref-001"
        assert body == "acc-001|strat-dual-ma|100.0000|5.0000"

    def test_render_price_limit_none(self) -> None:
        template = NotificationTemplateV1(
            subjectTemplate="s", bodyTemplate="{price_limit}"
        )
        signal = SignalReferenceV1.create(
            signalReferenceId="sig-ref-002",
            version=1,
            status=SignalStatus.Pending,
            accountId="acc-001",
            strategyId="strat-dual-ma",
            strategyChecksum="a" * 64,
            sourceEventId="evt-bar-002",
            sourceEventType="MarketBarEvent",
            direction="SELL",
            quantity="50.0000",
            priceLimit=None,
            operatorId=None,
            generatedTs=_T0,
            expiresAt=None,
            previousSignalReferenceId=None,
        )
        _, body = template.render(signal)
        assert body == "-"


class TestNotificationRouter:
    def test_route_delivers(self) -> None:
        store = InMemoryDeliveryStoreV1()
        sink = RecordingDeliverySinkV1()
        router = NotificationRouterV1(store, sink)
        delivery = router.route(_signal(), ChannelKind.Gui)
        assert delivery.status == DeliveryStatus.Delivered
        assert delivery.channel == "GUI"
        assert len(sink.calls) == 1

    def test_route_duplicate_does_not_recreate(self) -> None:
        """重试/重复路由不重复人工任务：同一信号同一渠道返回既有投递。"""
        store = InMemoryDeliveryStoreV1()
        sink = RecordingDeliverySinkV1()
        router = NotificationRouterV1(store, sink)
        first = router.route(_signal(), ChannelKind.Email)
        second = router.route(_signal(), ChannelKind.Email)
        assert second.deliveryId == first.deliveryId
        assert len(sink.calls) == 1
        assert len(store.all()) == 1

    def test_route_unknown_channel_rejected(self) -> None:
        store = InMemoryDeliveryStoreV1()
        router = NotificationRouterV1(store, RecordingDeliverySinkV1())
        with pytest.raises(NotificationError, match="未知通知渠道"):
            router.route(_signal(), "SMS")

    def test_retry_success_after_failures(self) -> None:
        """有界重试：前两次失败，第三次成功。"""
        store = InMemoryDeliveryStoreV1()
        sink = RecordingDeliverySinkV1(failFirst=2)
        router = NotificationRouterV1(store, sink, maxAttempts=3)
        delivery = router.route(_signal(), ChannelKind.DingTalk)
        assert delivery.status == DeliveryStatus.Delivered
        assert len(delivery.attempts) == 3
        assert delivery.attempts[0].status == DeliveryStatus.Failed
        assert delivery.attempts[2].status == DeliveryStatus.Delivered

    def test_failure_isolated(self) -> None:
        """通知失败不改变交易控制：路由层无交易副作用，仅记录 FAILED。"""
        store = InMemoryDeliveryStoreV1()
        sink = RecordingDeliverySinkV1(failFirst=99)
        router = NotificationRouterV1(store, sink, maxAttempts=2)
        delivery = router.route(_signal(), ChannelKind.Email)
        assert delivery.status == DeliveryStatus.Failed
        assert len(delivery.attempts) == 2
        # 信号本身保持 PENDING，交易控制不受影响
        assert _signal().status is SignalStatus.Pending

    def test_confirm_delivered(self) -> None:
        store = InMemoryDeliveryStoreV1()
        router = NotificationRouterV1(store, RecordingDeliverySinkV1())
        delivery = router.route(_signal(), ChannelKind.Gui)
        confirmed = router.confirm(delivery.deliveryId, "op-alice")
        assert confirmed.status == DeliveryStatus.Confirmed
        assert confirmed.confirmedBy == "op-alice"

    def test_confirm_failed_rejected(self) -> None:
        store = InMemoryDeliveryStoreV1()
        router = NotificationRouterV1(store, RecordingDeliverySinkV1(failFirst=99), maxAttempts=1)
        delivery = router.route(_signal(), ChannelKind.Gui)
        with pytest.raises(NotificationError, match="DELIVERED"):
            router.confirm(delivery.deliveryId, "op-alice")

    def test_confirm_unknown_delivery_rejected(self) -> None:
        store = InMemoryDeliveryStoreV1()
        router = NotificationRouterV1(store, RecordingDeliverySinkV1())
        with pytest.raises(NotificationError, match="不存在"):
            router.confirm("delivery-999", "op-alice")
