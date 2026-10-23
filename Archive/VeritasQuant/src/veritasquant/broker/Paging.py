"""P5-010 生产监控、分页告警和 24x7 联系树。

对齐 TechSpec 12.1/13 阶段 5：
- S0/S1 告警端到端送达；无人确认自动升级；
- 每个告警链接 Runbook。

- `AlertSeverityLevel`：S0/S1/S2/S3；
- `PagedAlertV1`：分页告警（严重度、送达状态、确认、升级、Runbook 链接）；
- `OnCallContactV1`：值班联系人（24x7 联系树节点）；
- `PagingServiceV1`：告警分页 + 自动升级（无人确认按升级路径提升）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from enum import StrEnum


class PagingError(ValueError):
    """分页告警不满足契约时抛出。"""


class AlertSeverityLevel(StrEnum):
    S0 = "S0"  # 立即停止/升级
    S1 = "S1"  # 高优先级
    S2 = "S2"
    S3 = "S3"


class DeliveryState(StrEnum):
    Pending = "PENDING"
    Delivered = "DELIVERED"
    Acknowledged = "ACKNOWLEDGED"
    Escalated = "ESCALATED"


@dataclass(frozen=True, slots=True)
class OnCallContactV1:
    """24x7 值班联系人（联系树节点）。"""

    contactId: str
    name: str
    level: int  # 1=一级值班，数字越大越高级
    channel: str

    def __post_init__(self) -> None:
        if not self.contactId or not self.name:
            raise PagingError("联系人标识字段不能为空")
        if self.level < 1:
            raise PagingError("联系等级必须为正")


@dataclass(frozen=True, slots=True)
class PagedAlertV1:
    """分页告警记录。"""

    alertId: str
    severity: AlertSeverityLevel
    title: str
    runbookLink: str  # 每个告警链接 Runbook
    deliveryState: DeliveryState
    createdAt: datetime
    escalatedTo: str | None = None
    acknowledgedBy: str | None = None
    acknowledgedAt: datetime | None = None
    detail: str = ""

    def __post_init__(self) -> None:
        if not self.alertId or not self.title:
            raise PagingError("告警标识字段不能为空")


class PagingServiceV1:
    """告警分页 + 自动升级：无人确认按升级路径提升。"""

    def __init__(self, contacts: dict[int, OnCallContactV1]) -> None:
        """contacts: level -> 联系人。"""
        if not contacts:
            raise PagingError("联系树不能为空")
        self._contacts = dict(contacts)
        self._alerts: dict[str, PagedAlertV1] = {}
        self._escalationDelay = timedelta(minutes=15)  # 15 分钟未确认升级
        self._counter = 0

    def page(
        self,
        *,
        severity: AlertSeverityLevel,
        title: str,
        runbookLink: str,
        detail: str = "",
    ) -> PagedAlertV1:
        """发出告警：初始投递给一级值班。"""
        self._counter += 1
        alert = PagedAlertV1(
            alertId=f"alert-{self._counter:06d}",
            severity=severity,
            title=title,
            runbookLink=runbookLink,
            deliveryState=DeliveryState.Pending,
            createdAt=datetime.now(timezone.utc),
            detail=detail,
        )
        self._alerts[alert.alertId] = alert
        return alert

    def confirmDelivery(self, alertId: str) -> PagedAlertV1:
        """确认送达（端到端送达确认）。"""
        alert = self._get(alertId)
        updated = self._replace(
            alert, deliveryState=DeliveryState.Delivered
        )
        self._alerts[alertId] = updated
        return updated

    def acknowledge(self, alertId: str, acknowledgedBy: str) -> PagedAlertV1:
        """值班确认（停止升级链）。"""
        alert = self._get(alertId)
        updated = self._replace(
            alert,
            deliveryState=DeliveryState.Acknowledged,
            acknowledgedBy=acknowledgedBy,
            acknowledgedAt=datetime.now(timezone.utc),
        )
        self._alerts[alertId] = updated
        return updated

    def escalate(self, alertId: str, currentLevel: int = 1) -> PagedAlertV1:
        """自动升级：提升到更高级联系人；S0/S1 升级路径。"""
        alert = self._get(alertId)
        if alert.deliveryState is DeliveryState.Acknowledged:
            raise PagingError("告警已确认，无需升级")
        nextContact = self._nextContact(currentLevel)
        updated = self._replace(
            alert,
            deliveryState=DeliveryState.Escalated,
            escalatedTo=nextContact.contactId,
        )
        self._alerts[alertId] = updated
        return updated

    def shouldEscalate(self, alertId: str) -> bool:
        """到期未确认应升级（S0/S1 强制升级路径）。"""
        alert = self._get(alertId)
        if alert.deliveryState is DeliveryState.Acknowledged:
            return False
        if alert.severity not in (AlertSeverityLevel.S0, AlertSeverityLevel.S1):
            return False
        return datetime.now(timezone.utc) - alert.createdAt > self._escalationDelay

    def get(self, alertId: str) -> PagedAlertV1 | None:
        return self._alerts.get(alertId)

    def alerts(self) -> tuple[PagedAlertV1, ...]:
        return tuple(self._alerts.values())

    def _get(self, alertId: str) -> PagedAlertV1:
        alert = self._alerts.get(alertId)
        if alert is None:
            raise PagingError(f"告警不存在: {alertId}")
        return alert

    def _nextContact(self, currentLevel: int) -> OnCallContactV1:
        for level in sorted(self._contacts):
            if level > currentLevel:
                return self._contacts[level]
        raise PagingError("无更高级联系人可升级")

    def _replace(self, alert: PagedAlertV1, **changes: object) -> PagedAlertV1:
        return PagedAlertV1(
            alertId=alert.alertId,
            severity=changes.get("severity", alert.severity),  # type: ignore[arg-type]
            title=changes.get("title", alert.title),  # type: ignore[arg-type]
            runbookLink=changes.get("runbookLink", alert.runbookLink),  # type: ignore[arg-type]
            deliveryState=changes.get("deliveryState", alert.deliveryState),  # type: ignore[arg-type]
            createdAt=changes.get("createdAt", alert.createdAt),  # type: ignore[arg-type]
            escalatedTo=changes.get("escalatedTo", alert.escalatedTo),  # type: ignore[arg-type]
            acknowledgedBy=changes.get("acknowledgedBy", alert.acknowledgedBy),  # type: ignore[arg-type]
            acknowledgedAt=changes.get("acknowledgedAt", alert.acknowledgedAt),  # type: ignore[arg-type]
            detail=changes.get("detail", alert.detail),  # type: ignore[arg-type]
        )
