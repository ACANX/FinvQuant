"""P1-026 确定性事件总线与订阅路由。

相同输入必须产生相同投递顺序；订阅顺序在首次投递前冻结；消费者异常、
重试与失败策略显式定义，且不得破坏已提交状态或绕过 inbox/outbox 边界。
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from enum import StrEnum

from veritasquant.core.Events import EventEnvelopeV1
from veritasquant.core.LogicalClock import UtcLogicalClockV1


class BusError(ValueError):
    """总线生命周期、订阅或投递违反确定性契约。"""


class ConsumerFailurePolicy(StrEnum):
    """消费者异常时的显式失败策略。"""

    StopRun = "StopRun"  # 抛给调用方，终止当前运行（默认，最安全）
    IsolateConsumer = "IsolateConsumer"  # 隔离该消费者，其余继续；记录失败
    RetryFixed = "RetryFixed"  # 固定次数重试后按 StopRun 处理


class SubscriptionOrder(StrEnum):
    """订阅路由的冻结顺序策略。"""

    RegistrationOrder = "RegistrationOrder"  # 按注册先后固定（默认）
    SourceRankOrder = "SourceRankOrder"  # 按消费者 sourceRank 固定


Consumer = Callable[[EventEnvelopeV1], None]


@dataclass(frozen=True, slots=True)
class SubscriptionV1:
    """冻结的订阅条目。"""

    consumerId: str
    eventType: str
    handler: Consumer
    sourceRank: int = 0
    policy: ConsumerFailurePolicy = ConsumerFailurePolicy.StopRun
    maxRetries: int = 0


@dataclass(slots=True)
class _ConsumerState:
    subscription: SubscriptionV1
    failedCount: int = 0
    isolated: bool = False


@dataclass(frozen=True, slots=True)
class DeliveryResultV1:
    """单事件投递结果，供审计与测试断言。"""

    eventId: str
    deliveredTo: tuple[str, ...]
    isolatedConsumers: tuple[str, ...]


class DeterministicEventBusV1:
    """确定性事件总线：订阅冻结、顺序固定、失败策略显式。"""

    def __init__(
        self,
        clock: UtcLogicalClockV1,
        order: SubscriptionOrder = SubscriptionOrder.RegistrationOrder,
    ) -> None:
        self._clock = clock
        self._order = order
        self._subscriptions: list[SubscriptionV1] = []
        self._states: dict[str, _ConsumerState] = {}
        self._frozen = False
        self._delivered: list[str] = []

    def subscribe(
        self,
        consumerId: str,
        eventType: str,
        handler: Consumer,
        *,
        sourceRank: int = 0,
        policy: ConsumerFailurePolicy = ConsumerFailurePolicy.StopRun,
        maxRetries: int = 0,
    ) -> None:
        """注册订阅；总线冻结后禁止新增或变更。"""
        if self._frozen:
            raise BusError("总线已冻结，禁止在投递后修改订阅")
        if consumerId in self._states:
            raise BusError(f"重复消费者: {consumerId}")
        if policy is ConsumerFailurePolicy.RetryFixed and maxRetries < 1:
            raise BusError("RetryFixed 策略必须指定正数重试次数")
        subscription = SubscriptionV1(
            consumerId=consumerId,
            eventType=eventType,
            handler=handler,
            sourceRank=sourceRank,
            policy=policy,
            maxRetries=maxRetries,
        )
        self._subscriptions.append(subscription)
        self._states[consumerId] = _ConsumerState(subscription)

    def freeze(self) -> None:
        """冻结订阅路由；同输入投递顺序从此固定。"""
        self._frozen = True

    def deliver(self, event: EventEnvelopeV1) -> DeliveryResultV1:
        """按冻结路由投递单事件；消费者异常按显式策略处置。"""
        if not self._frozen:
            self.freeze()
        self._clock.observe(event)
        self._delivered.append(event.eventId)
        consumers = self._orderedConsumers(event.eventType)
        delivered: list[str] = []
        for state in consumers:
            if state.isolated:
                continue
            self._invokeWithPolicy(state, event)
            delivered.append(state.subscription.consumerId)
        isolated = tuple(
            consumerId for consumerId, state in self._states.items() if state.isolated
        )
        return DeliveryResultV1(event.eventId, tuple(delivered), isolated)

    def _orderedConsumers(self, eventType: str) -> list[_ConsumerState]:
        """返回匹配事件类型的消费者，顺序冻结。"""
        matching = [
            self._states[subscription.consumerId]
            for subscription in self._subscriptions
            if subscription.eventType == eventType
        ]
        if self._order is SubscriptionOrder.SourceRankOrder:
            matching.sort(key=lambda state: state.subscription.sourceRank)
        return matching

    def _invokeWithPolicy(self, state: _ConsumerState, event: EventEnvelopeV1) -> None:
        """按失败策略调用消费者；StopRun 默认且永不绕过已提交边界。"""
        subscription = state.subscription
        attempts = subscription.maxRetries + 1 if subscription.policy is ConsumerFailurePolicy.RetryFixed else 1
        lastError: Exception | None = None
        for _ in range(attempts):
            try:
                subscription.handler(event)
                return
            except Exception as error:  # noqa: BLE001 - 消费者异常按策略处置
                lastError = error
        if subscription.policy is ConsumerFailurePolicy.StopRun:
            raise BusError(f"消费者 {subscription.consumerId} 失败: {lastError}") from lastError
        if subscription.policy is ConsumerFailurePolicy.IsolateConsumer:
            state.isolated = True
            state.failedCount += 1
            return
        # RetryFixed 达到上限后升级为 StopRun
        raise BusError(f"消费者 {subscription.consumerId} 重试耗尽: {lastError}") from lastError

    @property
    def deliveredEventIds(self) -> tuple[str, ...]:
        return tuple(self._delivered)

    @property
    def frozen(self) -> bool:
        return self._frozen
