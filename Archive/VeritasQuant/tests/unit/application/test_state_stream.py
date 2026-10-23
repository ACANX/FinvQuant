"""P2-030 状态流应用层测试：replay cursor、积压控制与权限撤销。"""

from __future__ import annotations

from veritasquant.application.StateStream import (
    InMemoryStreamEventSource,
    ReplayCursor,
    StreamCloseReason,
    StreamEventType,
    StreamEventV1,
    StreamService,
)


def _event(sequence: int, accountId: str = "acc-1") -> StreamEventV1:
    return StreamEventV1(
        sequence=sequence,
        eventType=StreamEventType.CommandStatus,
        accountId=accountId,
        payload={"command_id": f"cmd-{sequence}"},
        occurredAtIso="2026-08-03T00:00:00Z",
    )


def _service(maxBacklog: int = 500, replayWindow: int = 10) -> StreamService:
    source = InMemoryStreamEventSource(retentionLimit=100)
    for i in range(1, 6):
        source.append(_event(i))
    return StreamService(source, maxBacklog=maxBacklog, replayWindow=replayWindow)


class TestReplayCursor:
    def test_parse_none_means_no_replay(self) -> None:
        assert ReplayCursor.parse(None).sequence == -1

    def test_parse_int(self) -> None:
        assert ReplayCursor.parse("42").sequence == 42

    def test_parse_invalid_falls_back_to_latest(self) -> None:
        assert ReplayCursor.parse("abc").sequence == -1


class TestStreamServiceOpen:
    def test_open_replays_events_after_cursor(self) -> None:
        service = _service()
        result = service.open("u-1", frozenset({"acc-1"}), "2")
        assert len(result.events) == 3  # seq 3,4,5
        assert result.events[0].sequence == 3
        assert result.subscriptionId.startswith("sub_")

    def test_open_no_cursor_replays_nothing(self) -> None:
        service = _service()
        result = service.open("u-1", frozenset({"acc-1"}), None)
        assert result.events == ()
        assert result.closeReason is None

    def test_open_cursor_beyond_window_closes(self) -> None:
        service = _service(replayWindow=10)
        # 构造超过窗口的 cursor：latest=5，窗口 10 -> cursor=1 仍在窗口内
        # 先推进 latest 到 20
        source = InMemoryStreamEventSource()
        for i in range(1, 21):
            source.append(_event(i))
        service = StreamService(source, replayWindow=10)
        result = service.open("u-1", frozenset({"acc-1"}), "2")
        # latest=20，窗口 10 -> cursor 2 < 10 -> 超出
        assert result.closeReason is StreamCloseReason.BacklogExceeded
        assert "超出" in result.message

    def test_open_account_filter(self) -> None:
        service = _service()
        # acc-2 无事件
        result = service.open("u-1", frozenset({"acc-2"}), "0")
        assert result.events == ()


class TestStreamServiceDelivery:
    def test_deliver_to_matching_subscription(self) -> None:
        service = _service()
        result = service.open("u-1", frozenset({"acc-1"}), None)
        service.deliver(_event(6, "acc-1"))
        pending = service.takePending(result.subscriptionId)
        assert pending is not None
        assert [e.sequence for e in pending] == [6]

    def test_deliver_filters_by_account(self) -> None:
        service = _service()
        result = service.open("u-1", frozenset({"acc-1"}), None)
        service.deliver(_event(6, "acc-2"))
        pending = service.takePending(result.subscriptionId)
        assert pending == ()

    def test_backlog_exceeded_closes_subscription(self) -> None:
        service = _service(maxBacklog=3)
        result = service.open("u-1", frozenset({"acc-1"}), None)
        for i in range(6, 10):
            service.deliver(_event(i, "acc-1"))
        # 第 4 个投递触发积压超限
        assert service.subscriptionCloseReason(result.subscriptionId) is StreamCloseReason.BacklogExceeded
        assert service.takePending(result.subscriptionId) is None


class TestStreamServiceRevoke:
    def test_revoke_principal_closes_subscriptions(self) -> None:
        service = _service()
        result = service.open("u-1", frozenset({"acc-1"}), None)
        count = service.revokePrincipal("u-1")
        assert count == 1
        assert service.subscriptionCloseReason(result.subscriptionId) is StreamCloseReason.PermissionRevoked
        assert service.takePending(result.subscriptionId) is None

    def test_revoke_only_target_principal(self) -> None:
        service = _service()
        r1 = service.open("u-1", frozenset({"acc-1"}), None)
        r2 = service.open("u-2", frozenset({"acc-1"}), None)
        service.revokePrincipal("u-1")
        assert service.subscriptionCloseReason(r1.subscriptionId) is StreamCloseReason.PermissionRevoked
        assert service.subscriptionCloseReason(r2.subscriptionId) is None
        assert service.takePending(r2.subscriptionId) is not None
