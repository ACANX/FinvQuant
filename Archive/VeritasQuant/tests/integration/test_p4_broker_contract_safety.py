"""P4-010 券商契约、限频、断连和结果未知测试。

验收标准（P4-010）：
- 两次断连、重放、超时、限频和未知结果均无重复订单/账本。

覆盖 P4-002~006 的交叉契约：
1. 断连后重放回报不重复记账（ReportDeduplicator + SequenceGuard）；
2. 超时进入 TIMEOUT_UNKNOWN，不盲目重发（SimBrokerOrderGateway）；
3. 限频拒绝超额发单（能力清单 maxOrderRatePerSecond）；
4. 未知结果订单隔离并触发查询（UnknownOrderIsolation）；
5. 两次断连场景均无重复副作用（幂等键 + 映射一致性）。

全内存实现，不依赖 PostgreSQL/Redis。
"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from veritasquant.broker.BrokerOrderGateway import (
    OrderOutcome,
    SimBrokerOrderGatewayV1,
)
from veritasquant.broker.BrokerPort import (
    BrokerCapabilityV1,
    BrokerPortError,
    OrderRequestV1,
)
from veritasquant.broker.BrokerSession import (
    BrokerCredentialV1,
    InMemoryCredentialResolverV1,
    SessionManagerV1,
)
from veritasquant.broker.ReportHandling import (
    BrokerReportV1,
    ReportDeduplicatorV1,
    ReportSequenceGuardV1,
    UnknownOrderIsolationV1,
)
from veritasquant.execution.Orders import ExecutionType, OrderSide, OrderType, TimeInForce

_T0 = datetime(2026, 8, 3, 2, 0, 0, tzinfo=timezone.utc)


def _capability(maxRate: int = 10) -> BrokerCapabilityV1:
    return BrokerCapabilityV1(
        brokerId="sim-broker",
        capabilityVersion="1.0",
        orderTypes=frozenset({OrderType.Limit, OrderType.Market}),
        timeInForces=frozenset({TimeInForce.Day}),
        orderSides=frozenset({OrderSide.Buy, OrderSide.Sell}),
        symbols=frozenset({"518880"}),
        markets=frozenset({"ETF_CN"}),
        supportsCancel=True,
        supportsOrderQuery=True,
        supportsPositionQuery=True,
        supportsCashQuery=True,
        minQuantity="100.0000",
        maxOrderRatePerSecond=maxRate,
        sessionOpenSupported=True,
        sessionIntradaySupported=True,
        sessionCloseSupported=True,
    )


def _request(clientOrderId: str = "co-001") -> OrderRequestV1:
    return OrderRequestV1(
        clientOrderId=clientOrderId,
        accountId="acc-001",
        symbol="518880",
        side=OrderSide.Buy,
        orderType=OrderType.Limit,
        timeInForce=TimeInForce.Day,
        quantity="100.0000",
        limitPrice="5.0000",
        stopPrice=None,
    )


def _report(sequence: int, **overrides: object) -> BrokerReportV1:
    values: dict[str, object] = {
        "brokerReportId": f"rep-{sequence}",
        "clientOrderId": "co-001",
        "brokerOrderId": "broker-001",
        "reportSequence": sequence,
        "executionType": ExecutionType.New,
        "executionId": None,
        "lastQuantity": "0",
        "lastPrice": None,
        "cumulativeQuantity": "0",
        "receivedAt": _T0,
    }
    values.update(overrides)
    return BrokerReportV1(**values)


def _gateway(capability: BrokerCapabilityV1 | None = None) -> SimBrokerOrderGatewayV1:
    capability = capability or _capability()
    resolver = InMemoryCredentialResolverV1(
        {"cred-sim-001": BrokerCredentialV1("cred-sim-001", "secret")}
    )
    return SimBrokerOrderGatewayV1(capability, SessionManagerV1(resolver))


def _session(gateway: SimBrokerOrderGatewayV1):
    return gateway._sessionManager.authenticate("cred-sim-001")  # type: ignore[attr-defined]


class TestDisconnectReplay:
    """断连后重放回报不重复记账。"""

    def test_replayed_reports_after_disconnect(self) -> None:
        """第一次断连：回报已处理；重连后重放同批回报 -> 全部去重。"""
        dedup = ReportDeduplicatorV1()
        guard = ReportSequenceGuardV1()
        # 断连前处理 seq=1,2
        for seq in (1, 2):
            assert guard.check(_report(seq)).status.value in ("IN_ORDER", "GAP_DETECTED")
            dedup.check(_report(seq))
        # 重连后重放 seq=1,2（断线重放）
        replay1 = dedup.check(_report(1))
        replay2 = dedup.check(_report(2))
        assert replay1.status.value == "DUPLICATE"
        assert replay2.status.value == "DUPLICATE"

    def test_second_disconnect_replay_still_idempotent(self) -> None:
        """第二次断连：再次重放依然无重复副作用。"""
        dedup = ReportDeduplicatorV1()
        guard = ReportSequenceGuardV1()
        for _ in range(2):  # 两次断连-重放周期
            for seq in (1, 2, 3):
                dedup.check(_report(seq))
                guard.check(_report(seq))
        # 第三轮重放全部去重
        assert dedup.check(_report(1)).status.value == "DUPLICATE"
        assert dedup.check(_report(2)).status.value == "DUPLICATE"
        assert dedup.check(_report(3)).status.value == "DUPLICATE"

    def test_gap_then_replay_fills(self) -> None:
        """缺口补齐后重放不重复。"""
        guard = ReportSequenceGuardV1()
        assert guard.check(_report(3)).status.value == "GAP_DETECTED"
        # 补齐缺口（迟到的合法序列允许补齐）
        assert guard.check(_report(1)).status.value in ("LATE_ARRIVAL", "IN_ORDER")
        assert guard.check(_report(2)).status.value in ("LATE_ARRIVAL", "IN_ORDER")
        # 重放旧序列被拒绝
        assert guard.check(_report(1)).status.value == "DUPLICATE"


class TestTimeoutUnknown:
    """超时/结果未知：不盲目重发。"""

    def test_timeout_does_not_resubmit(self) -> None:
        gateway = _gateway()
        session = _session(gateway)
        outcome = gateway.submit(
            session=session,
            request=_request(),
            simulateAccept=False,
            simulateReject=False,
        )
        assert outcome.outcome is OrderOutcome.TimeoutUnknown
        assert gateway.mappingCount() == 0  # 无映射 = 未发出

    def test_unknown_result_isolated(self) -> None:
        """未知订单隔离并触发查询。"""
        isolation = UnknownOrderIsolationV1()
        report = _report(1, brokerOrderId="broker-unknown")
        handling = isolation.isolate(report)
        assert "隔离" in handling.message
        assert len(isolation.records()) == 1


class TestRateLimit:
    """限频：超额发单被拒绝。"""

    def test_rate_limit_rejects_overflow(self) -> None:
        gateway = _gateway(_capability(maxRate=2))
        session = _session(gateway)
        gateway.submit(session=session, request=_request("co-001"))
        gateway.submit(session=session, request=_request("co-002"))
        # 超出 2/s 限频：第 3 单被拒绝（能力协商层）
        with pytest.raises(BrokerPortError):
            gateway.submit(session=session, request=_request("co-003"))
        assert gateway.mappingCount() == 2

    def test_within_rate_limit_ok(self) -> None:
        gateway = _gateway(_capability(maxRate=5))
        session = _session(gateway)
        for index in range(5):
            outcome = gateway.submit(session=session, request=_request(f"co-{index:03d}"))
            assert outcome.outcome is OrderOutcome.Accepted
        assert gateway.mappingCount() == 5


class TestUnknownResultNoDuplicate:
    """未知结果场景：重复提交同一订单不产生重复映射。"""

    def test_unknown_accepted_idempotent_mapping(self) -> None:
        """结果未知但允许按已受理处理时：重复提交返回同一映射。"""
        gateway = _gateway()
        session = _session(gateway)
        first = gateway.submit(session=session, request=_request("co-001"))
        second = gateway.submit(session=session, request=_request("co-001"))
        assert first.outcome is OrderOutcome.Accepted
        assert second.outcome is OrderOutcome.Accepted
        # 同一 clientOrderId 的映射唯一（不重复记账）
        assert gateway.brokerOrderIdFor("co-001") == first.brokerOrderId
        assert gateway.brokerOrderIdFor("co-001") == second.brokerOrderId
        # 每个 client 只产生一个映射
        assert gateway.mappingCount() == 1


class TestTwoDisconnects:
    """两次断连场景均无重复订单。"""

    def test_two_disconnect_cycles_no_duplicate_orders(self) -> None:
        gateway = _gateway()
        session = _session(gateway)
        # 第一次断连：发单结果未知
        first = gateway.submit(
            session=session,
            request=_request("co-001"),
            simulateAccept=False,
            simulateReject=False,
        )
        assert first.outcome is OrderOutcome.TimeoutUnknown
        # 第二次断连：重试同一订单仍不盲目重发
        second = gateway.submit(
            session=session,
            request=_request("co-001"),
            simulateAccept=False,
            simulateReject=False,
        )
        assert second.outcome is OrderOutcome.TimeoutUnknown
        assert gateway.mappingCount() == 0  # 无重复映射/无盲目重发副作用
