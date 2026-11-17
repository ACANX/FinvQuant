"""P5-010 生产监控、分页告警和 24x7 联系树测试。"""

from __future__ import annotations

from datetime import timedelta

import pytest

from veritasquant.broker.Paging import (
    AlertSeverityLevel,
    DeliveryState,
    OnCallContactV1,
    PagedAlertV1,
    PagingError,
    PagingServiceV1,
)


def _contacts() -> dict[int, OnCallContactV1]:
    return {
        1: OnCallContactV1("oncall-1", "一级值班", 1, "pager"),
        2: OnCallContactV1("oncall-2", "二级值班", 2, "pager"),
        3: OnCallContactV1("oncall-3", "三级值班", 3, "phone"),
    }


def _service() -> PagingServiceV1:
    return PagingServiceV1(_contacts())


class TestPagingService:
    def test_page_s0(self) -> None:
        service = _service()
        alert = service.page(
            severity=AlertSeverityLevel.S0,
            title="券商断连",
            runbookLink="/runbook/broker-disconnect",
        )
        assert isinstance(alert, PagedAlertV1)
        assert alert.deliveryState is DeliveryState.Pending
        assert alert.runbookLink == "/runbook/broker-disconnect"  # 每个告警链接 Runbook
        assert len(service.alerts()) == 1

    def test_confirm_and_acknowledge(self) -> None:
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S1, title="时钟偏移", runbookLink="/rb/clock")
        service.confirmDelivery(alert.alertId)
        acknowledged = service.acknowledge(alert.alertId, "oncall-1")
        assert acknowledged.deliveryState is DeliveryState.Acknowledged
        assert acknowledged.acknowledgedBy == "oncall-1"

    def test_escalation_chain(self) -> None:
        """无人确认自动升级：1 -> 2 -> 3。"""
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S0, title="账本不平", runbookLink="/rb/ledger")
        escalated = service.escalate(alert.alertId, currentLevel=1)
        assert escalated.deliveryState is DeliveryState.Escalated
        assert escalated.escalatedTo == "oncall-2"
        escalated2 = service.escalate(alert.alertId, currentLevel=2)
        assert escalated2.escalatedTo == "oncall-3"

    def test_escalate_beyond_top_rejected(self) -> None:
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S0, title="t", runbookLink="/rb")
        service.escalate(alert.alertId, currentLevel=1)
        service.escalate(alert.alertId, currentLevel=2)
        with pytest.raises(PagingError, match="无更高级联系人"):
            service.escalate(alert.alertId, currentLevel=3)

    def test_escalate_acknowledged_rejected(self) -> None:
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S1, title="t", runbookLink="/rb")
        service.acknowledge(alert.alertId, "oncall-1")
        with pytest.raises(PagingError, match="已确认"):
            service.escalate(alert.alertId, currentLevel=1)

    def test_should_escalate_s0_after_delay(self) -> None:
        """S0/S1 无人确认到期自动升级。"""
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S0, title="t", runbookLink="/rb")
        assert service.shouldEscalate(alert.alertId) is False  # 未到期
        # 人为提前 createdAt 模拟超时
        service._alerts[alert.alertId] = PagedAlertV1(
            alertId=alert.alertId,
            severity=alert.severity,
            title=alert.title,
            runbookLink=alert.runbookLink,
            deliveryState=alert.deliveryState,
            createdAt=alert.createdAt - timedelta(minutes=30),
            detail=alert.detail,
        )
        assert service.shouldEscalate(alert.alertId) is True

    def test_should_not_escalate_s2(self) -> None:
        """非 S0/S1 不强制升级。"""
        service = _service()
        alert = service.page(severity=AlertSeverityLevel.S2, title="t", runbookLink="/rb")
        service._alerts[alert.alertId] = PagedAlertV1(
            alertId=alert.alertId,
            severity=alert.severity,
            title=alert.title,
            runbookLink=alert.runbookLink,
            deliveryState=alert.deliveryState,
            createdAt=alert.createdAt - timedelta(minutes=30),
            detail=alert.detail,
        )
        assert service.shouldEscalate(alert.alertId) is False

    def test_unknown_alert_rejected(self) -> None:
        service = _service()
        with pytest.raises(PagingError, match="不存在"):
            service.acknowledge("alert-unknown", "oncall-1")

    def test_empty_contact_tree_rejected(self) -> None:
        with pytest.raises(PagingError, match="联系树不能为空"):
            PagingServiceV1({})

    def test_contact_requires_fields(self) -> None:
        with pytest.raises(PagingError):
            OnCallContactV1("", "name", 1, "pager")
