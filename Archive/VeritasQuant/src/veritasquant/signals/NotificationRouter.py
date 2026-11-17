"""P3-003 通知路由、模板、重试和失败隔离。

- `NotificationTemplateV1`：从 SignalReference 渲染通知内容（确定性模板）。
- `NotificationDeliveryV1`：单次投递记录（渠道、状态、尝试次数、时间、审计）。
- `NotificationRouterV1`：按渠道路由 + 有界重试 + 失败隔离。

契约（P3-003 验收标准）：
1. 通知失败不改变交易控制 —— 路由层不触碰 RiskEngine/账本/订单；
2. 重试不重复人工任务 —— 投递以 (signalReferenceId, channel) 为幂等键，
   同一信号同一渠道最多创建一个人工任务；
3. 投递结果可审计 —— 每次尝试、送达状态、确认人都留档。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Protocol

from veritasquant.signals.SignalReference import SignalReferenceV1


class NotificationError(ValueError):
    """通知路由或投递不满足契约时抛出。"""


class DeliveryStatus:
    Pending = "PENDING"
    Delivered = "DELIVERED"
    Failed = "FAILED"
    Confirmed = "CONFIRMED"


class ChannelKind:
    """受控通知渠道。"""

    Gui = "GUI"
    Email = "EMAIL"
    DingTalk = "DINGTALK"
    WeCom = "WECOM"


@dataclass(frozen=True, slots=True)
class NotificationTemplateV1:
    """确定性通知模板：同一信号渲染出同一内容。"""

    subjectTemplate: str
    bodyTemplate: str

    def __post_init__(self) -> None:
        if not self.subjectTemplate or not self.bodyTemplate:
            raise NotificationError("模板标题与正文不能为空")

    def render(self, signal: SignalReferenceV1) -> tuple[str, str]:
        """渲染标题与正文；字段缺失时模板抛 KeyError -> 上层失败隔离。"""
        values = {
            "signal_reference_id": signal.signalReferenceId,
            "account_id": signal.accountId,
            "strategy_id": signal.strategyId,
            "direction": signal.direction,
            "quantity": signal.quantity,
            "price_limit": signal.priceLimit or "-",
            "generated_ts": signal.generatedTs.isoformat(),
        }
        return self.subjectTemplate.format(**values), self.bodyTemplate.format(**values)


@dataclass(frozen=True, slots=True)
class DeliveryAttemptV1:
    """一次投递尝试记录（审计留档）。"""

    attemptNumber: int
    attemptedAt: datetime
    status: str
    detail: str = ""


@dataclass(frozen=True, slots=True)
class NotificationDeliveryV1:
    """信号通知投递记录：幂等键 = signalReferenceId + channel。"""

    deliveryId: str
    signalReferenceId: str
    channel: str
    recipient: str
    subject: str
    body: str
    status: str
    attempts: tuple[DeliveryAttemptV1, ...]
    createdTs: datetime
    updatedTs: datetime
    confirmedBy: str | None = None

    def __post_init__(self) -> None:
        if not self.deliveryId or not self.signalReferenceId or not self.channel:
            raise NotificationError("投递标识字段不能为空")
        if not self.attempts:
            raise NotificationError("投递记录必须包含至少一次尝试")


class DeliverySink(Protocol):
    """实际投递端口；实现可为 GUI/邮件/钉钉适配器。"""

    def deliver(self, channel: str, recipient: str, subject: str, body: str) -> None: ...


class DeliveryStore(Protocol):
    """投递记录持久化端口。"""

    def get(self, deliveryId: str) -> NotificationDeliveryV1 | None: ...

    def getBySignalChannel(self, signalReferenceId: str, channel: str) -> NotificationDeliveryV1 | None: ...

    def save(self, delivery: NotificationDeliveryV1) -> None: ...


@dataclass(slots=True)
class InMemoryDeliveryStoreV1:
    """内存投递记录存储（模拟盘/测试）。"""

    _deliveries: dict[str, NotificationDeliveryV1] = field(default_factory=dict)
    _bySignalChannel: dict[tuple[str, str], str] = field(default_factory=dict)

    def get(self, deliveryId: str) -> NotificationDeliveryV1 | None:
        return self._deliveries.get(deliveryId)

    def getBySignalChannel(self, signalReferenceId: str, channel: str) -> NotificationDeliveryV1 | None:
        deliveryId = self._bySignalChannel.get((signalReferenceId, channel))
        if deliveryId is None:
            return None
        return self._deliveries.get(deliveryId)

    def save(self, delivery: NotificationDeliveryV1) -> None:
        self._deliveries[delivery.deliveryId] = delivery
        self._bySignalChannel[(delivery.signalReferenceId, delivery.channel)] = delivery.deliveryId

    def all(self) -> tuple[NotificationDeliveryV1, ...]:
        return tuple(self._deliveries.values())


class NotificationRouterV1:
    """通知路由：模板渲染 -> 渠道投递 -> 有界重试 -> 失败隔离。

    路由层只投递通知，不改变任何交易控制；投递失败仅记录 FAILED 状态，
    重试不重复创建人工任务（同一信号同一渠道只保留一条投递记录）。
    """

    def __init__(
        self,
        store: DeliveryStore,
        sink: DeliverySink,
        *,
        template: NotificationTemplateV1 | None = None,
        maxAttempts: int = 3,
        recipient: str = "signal-ops",
    ) -> None:
        if store is None or sink is None:
            raise NotificationError("投递存储与渠道适配器不能为空")
        if maxAttempts < 1:
            raise NotificationError("最大尝试次数必须为正")
        self._store = store
        self._sink = sink
        self._template = template or NotificationTemplateV1(
            subjectTemplate="[信号] {signal_reference_id} {direction} {quantity}",
            bodyTemplate=(
                "账户: {account_id}\n策略: {strategy_id}\n方向: {direction}\n"
                "数量: {quantity}\n价格上限: {price_limit}\n生成时间: {generated_ts}"
            ),
        )
        self._maxAttempts = maxAttempts
        self._recipient = recipient
        self._counter = 0

    def route(self, signal: SignalReferenceV1, channel: str) -> NotificationDeliveryV1:
        """路由一条信号通知；重复路由返回既有投递记录（不重复人工任务）。"""
        if channel not in (ChannelKind.Gui, ChannelKind.Email, ChannelKind.DingTalk, ChannelKind.WeCom):
            raise NotificationError(f"未知通知渠道: {channel}")
        existing = self._store.getBySignalChannel(signal.signalReferenceId, channel)
        if existing is not None:
            return existing
        subject, body = self._template.render(signal)
        return self._deliverWithRetry(
            signalReferenceId=signal.signalReferenceId,
            channel=channel,
            subject=subject,
            body=body,
        )

    def _deliverWithRetry(
        self,
        *,
        signalReferenceId: str,
        channel: str,
        subject: str,
        body: str,
    ) -> NotificationDeliveryV1:
        self._counter += 1
        deliveryId = f"delivery-{self._counter:06d}"
        now = datetime.now(timezone.utc)
        attempts: list[DeliveryAttemptV1] = []
        status = DeliveryStatus.Pending
        detail = ""
        for attemptNumber in range(1, self._maxAttempts + 1):
            try:
                self._sink.deliver(channel, self._recipient, subject, body)
                status = DeliveryStatus.Delivered
                detail = ""
                attempts.append(
                    DeliveryAttemptV1(
                        attemptNumber=attemptNumber,
                        attemptedAt=datetime.now(timezone.utc),
                        status=DeliveryStatus.Delivered,
                    )
                )
                break
            except Exception as error:  # noqa: BLE001 - 投递失败必须隔离，不得影响交易控制
                status = DeliveryStatus.Failed
                detail = str(error)
                attempts.append(
                    DeliveryAttemptV1(
                        attemptNumber=attemptNumber,
                        attemptedAt=datetime.now(timezone.utc),
                        status=DeliveryStatus.Failed,
                        detail=detail,
                    )
                )
        delivery = NotificationDeliveryV1(
            deliveryId=deliveryId,
            signalReferenceId=signalReferenceId,
            channel=channel,
            recipient=self._recipient,
            subject=subject,
            body=body,
            status=status,
            attempts=tuple(attempts),
            createdTs=now,
            updatedTs=datetime.now(timezone.utc),
            confirmedBy=None,
        )
        self._store.save(delivery)
        return delivery

    def confirm(self, deliveryId: str, confirmedBy: str) -> NotificationDeliveryV1:
        """人工确认送达；未投递成功不允许确认。"""
        delivery = self._store.get(deliveryId)
        if delivery is None:
            raise NotificationError(f"投递记录不存在: {deliveryId}")
        if delivery.status != DeliveryStatus.Delivered:
            raise NotificationError("只有 DELIVERED 状态可以确认")
        confirmed = NotificationDeliveryV1(
            deliveryId=delivery.deliveryId,
            signalReferenceId=delivery.signalReferenceId,
            channel=delivery.channel,
            recipient=delivery.recipient,
            subject=delivery.subject,
            body=delivery.body,
            status=DeliveryStatus.Confirmed,
            attempts=delivery.attempts,
            createdTs=delivery.createdTs,
            updatedTs=datetime.now(timezone.utc),
            confirmedBy=confirmedBy,
        )
        self._store.save(confirmed)
        return confirmed


class RecordingDeliverySinkV1:
    """测试/演示用记录型投递适配器：记录每次投递，可注入失败。"""

    def __init__(self, failFirst: int = 0) -> None:
        self._failFirst = failFirst
        self._calls: list[tuple[str, str, str, str]] = []
        self._failures = 0

    def deliver(self, channel: str, recipient: str, subject: str, body: str) -> None:
        if self._failures < self._failFirst:
            self._failures += 1
            raise NotificationError(f"注入投递失败 #{self._failures}")
        self._calls.append((channel, recipient, subject, body))

    @property
    def calls(self) -> tuple[tuple[str, str, str, str], ...]:
        return tuple(self._calls)
