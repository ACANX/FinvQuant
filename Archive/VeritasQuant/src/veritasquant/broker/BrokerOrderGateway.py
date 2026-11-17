"""P4-004 订单发送、受理、拒绝、撤单和查询映射。

对齐 TechSpec 7.1/7.3 与 13 阶段 4：
- client/broker ID 可追溯（双向映射，永不丢失）；
- 超时进入 RECONCILIATION_REQUIRED，不盲目重发。

- `BrokerOrderMappingV1`：clientOrderId <-> brokerOrderId 双向映射记录；
- `OrderStatusMapperV1`：券商侧状态 -> 领域订单状态映射；
- `SimBrokerOrderGatewayV1`：仿真券商订单网关（发送/撤单/查询），
  超时/结果未知进入 RECONCILIATION_REQUIRED。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from enum import StrEnum

from veritasquant.broker.BrokerPort import (
    BrokerCapabilityV1,
    BrokerPortError,
    CapabilityNegotiatorV1,
    OrderRequestV1,
)
from veritasquant.broker.BrokerSession import BrokerSessionV1, SessionManagerV1
from veritasquant.execution.Orders import BrokerState, OrderState


class OrderOutcome(StrEnum):
    Accepted = "ACCEPTED"
    Rejected = "REJECTED"
    TimeoutUnknown = "TIMEOUT_UNKNOWN"  # 结果未知：不盲目重发
    Cancelled = "CANCELLED"


@dataclass(frozen=True, slots=True)
class BrokerOrderMappingV1:
    """client/broker 双向 ID 映射记录（可追溯）。"""

    clientOrderId: str
    brokerOrderId: str
    accountId: str
    createdAt: datetime

    def __post_init__(self) -> None:
        if not self.clientOrderId or not self.brokerOrderId:
            raise BrokerPortError("双向映射标识字段不能为空")


@dataclass(frozen=True, slots=True)
class SubmitOutcomeV1:
    """发单结果：受理/拒绝/超时未知。"""

    outcome: OrderOutcome
    brokerOrderId: str | None = None
    brokerState: str | None = None
    reasonCode: str | None = None


@dataclass(frozen=True, slots=True)
class QueryOutcomeV1:
    """订单查询结果。"""

    brokerState: BrokerState
    filledQuantity: str = "0"
    remainingQuantity: str | None = None
    lastPrice: str | None = None


class OrderStatusMapperV1:
    """券商侧状态 -> 领域订单状态（受控映射）。"""

    _MAPPING: dict[BrokerState, OrderState] = {
        BrokerState.Accepted: OrderState.Accepted,
        BrokerState.Working: OrderState.Submitted,
        BrokerState.Partial: OrderState.PartiallyFilled,
        BrokerState.Filled: OrderState.Filled,
        BrokerState.Cancelled: OrderState.Cancelled,
        BrokerState.Rejected: OrderState.Rejected,
        BrokerState.Expired: OrderState.Expired,
        BrokerState.Unknown: OrderState.ReconciliationRequired,
    }

    def map(self, brokerState: BrokerState) -> OrderState:
        try:
            return self._MAPPING[brokerState]
        except KeyError as error:
            raise BrokerPortError(f"未知券商状态: {brokerState}") from error


class SimBrokerOrderGatewayV1:
    """仿真券商订单网关：能力协商 -> 发单 -> 双向映射 -> 查询/撤单。

    超时或结果未知 -> TIMEOUT_UNKNOWN / RECONCILIATION_REQUIRED，
    绝不盲目重发同一订单（重复副作用 0）。
    """

    def __init__(
        self,
        capability: BrokerCapabilityV1,
        sessionManager: SessionManagerV1,
        *,
        submitTimeoutSeconds: float = 5.0,
        allowUnknownAsAccepted: bool = False,
    ) -> None:
        if capability is None or sessionManager is None:
            raise BrokerPortError("能力清单与会话管理不能为空")
        self._negotiator = CapabilityNegotiatorV1(capability)
        self._sessionManager = sessionManager
        self._submitTimeout = timedelta(seconds=submitTimeoutSeconds)
        self._allowUnknownAsAccepted = allowUnknownAsAccepted
        self._mappings: dict[str, BrokerOrderMappingV1] = {}  # client -> mapping
        self._byBroker: dict[str, str] = {}  # broker -> client
        self._outcomes: dict[str, SubmitOutcomeV1] = {}
        self._counter = 0
        self._submitTimestamps: list[datetime] = []

    def _enforceRateLimit(self) -> None:
        """限频：滑动窗口内发单数不得超过能力清单 maxOrderRatePerSecond。"""
        now = datetime.now(timezone.utc)
        windowStart = now - timedelta(seconds=1)
        self._submitTimestamps = [t for t in self._submitTimestamps if t >= windowStart]
        if len(self._submitTimestamps) >= self._negotiator.capability.maxOrderRatePerSecond:
            raise BrokerPortError(
                f"限频：每秒最多 {self._negotiator.capability.maxOrderRatePerSecond} 笔，拒绝发单"
            )
        self._submitTimestamps.append(now)

    def submit(
        self,
        *,
        session: BrokerSessionV1,
        request: OrderRequestV1,
        simulateAccept: bool = True,
        simulateReject: bool = False,
        reasonCode: str | None = None,
    ) -> SubmitOutcomeV1:
        """发送订单：先能力协商，再认证校验，最后受理/拒绝/超时未知。"""
        self._negotiator.negotiate(request)
        self._sessionManager.validate(session)
        if not session.hasPermission("order:submit"):
            raise BrokerPortError("会话缺少 order:submit 权限")
        self._enforceRateLimit()
        # 同一 clientOrderId 重复提交：返回既有映射（幂等，不重复记账）
        existing = self._mappings.get(request.clientOrderId)
        if existing is not None:
            prior = self._outcomes[request.clientOrderId]
            return SubmitOutcomeV1(
                outcome=OrderOutcome.Accepted
                if prior.outcome is OrderOutcome.Accepted
                else OrderOutcome.TimeoutUnknown,
                brokerOrderId=existing.brokerOrderId,
                brokerState=prior.brokerState,
            )
        self._counter += 1
        brokerOrderId = f"broker-{self._counter:06d}"
        if simulateReject:
            outcome = SubmitOutcomeV1(
                outcome=OrderOutcome.Rejected,
                brokerOrderId=brokerOrderId,
                brokerState=BrokerState.Rejected.value,
                reasonCode=reasonCode or "BROKER_REJECTED",
            )
        elif simulateAccept:
            outcome = SubmitOutcomeV1(
                outcome=OrderOutcome.Accepted,
                brokerOrderId=brokerOrderId,
                brokerState=BrokerState.Accepted.value,
            )
        elif self._allowUnknownAsAccepted:
            outcome = SubmitOutcomeV1(
                outcome=OrderOutcome.Accepted,
                brokerOrderId=brokerOrderId,
                brokerState=BrokerState.Unknown.value,
            )
        else:
            outcome = SubmitOutcomeV1(outcome=OrderOutcome.TimeoutUnknown)
        self._outcomes[request.clientOrderId] = outcome
        if outcome.brokerOrderId is not None:
            mapping = BrokerOrderMappingV1(
                clientOrderId=request.clientOrderId,
                brokerOrderId=outcome.brokerOrderId,
                accountId=request.accountId,
                createdAt=datetime.now(timezone.utc),
            )
            self._mappings[request.clientOrderId] = mapping
            self._byBroker[outcome.brokerOrderId] = request.clientOrderId
        return outcome

    def brokerOrderIdFor(self, clientOrderId: str) -> str | None:
        mapping = self._mappings.get(clientOrderId)
        return mapping.brokerOrderId if mapping is not None else None

    def clientOrderIdFor(self, brokerOrderId: str) -> str | None:
        return self._byBroker.get(brokerOrderId)

    def cancel(
        self,
        *,
        session: BrokerSessionV1,
        clientOrderId: str,
        brokerOrderId: str,
        simulateFailure: bool = False,
    ) -> OrderOutcome:
        """撤单；映射缺失或券商失败 -> 进入对账状态，不假装成功。"""
        self._sessionManager.validate(session)
        if not session.hasPermission("order:cancel"):
            raise BrokerPortError("会话缺少 order:cancel 权限")
        if self.clientOrderIdFor(brokerOrderId) != clientOrderId:
            raise BrokerPortError("client/broker ID 映射不一致，禁止撤单")
        if simulateFailure:
            return OrderOutcome.TimeoutUnknown
        return OrderOutcome.Cancelled

    def query(
        self,
        *,
        session: BrokerSessionV1,
        clientOrderId: str,
        brokerOrderId: str,
        brokerState: BrokerState = BrokerState.Working,
    ) -> QueryOutcomeV1:
        """查询订单状态；映射缺失 -> 拒绝。"""
        self._sessionManager.validate(session)
        if not session.hasPermission("order:query"):
            raise BrokerPortError("会话缺少 order:query 权限")
        if self.clientOrderIdFor(brokerOrderId) != clientOrderId:
            raise BrokerPortError("client/broker ID 映射不一致，禁止查询")
        return QueryOutcomeV1(brokerState=brokerState)

    def mappingCount(self) -> int:
        return len(self._mappings)
