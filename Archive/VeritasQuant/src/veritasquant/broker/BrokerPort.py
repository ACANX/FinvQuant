"""P4-002 统一 BrokerPort 与能力协商。

对齐 TechSpec 7.1 与 13 阶段 4：
- 不支持能力在发单前拒绝；
- 第三方字段只停留在适配边界，不扩散到领域模型。

- `BrokerCapabilityV1`：券商能力清单（订单类型、时间生效、最小手数、
  支持的市场/标的、会话能力、查询能力）；
- `BrokerPort`：统一端口（能力查询 + 发单 + 撤单 + 查询 + 回报订阅）；
- `CapabilityNegotiatorV1`：发单前能力协商 —— 请求的能力不在清单内
  直接拒绝，避免到达券商侧才发现不支持。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol

from veritasquant.execution.Orders import OrderSide, OrderType, TimeInForce


class BrokerPortError(ValueError):
    """BrokerPort 能力协商或调用不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class BrokerCapabilityV1:
    """券商能力清单（P4-001 冻结清单的结构化表达）。"""

    brokerId: str
    capabilityVersion: str
    orderTypes: frozenset[OrderType]
    timeInForces: frozenset[TimeInForce]
    orderSides: frozenset[OrderSide]
    symbols: frozenset[str]
    markets: frozenset[str]
    supportsCancel: bool
    supportsOrderQuery: bool
    supportsPositionQuery: bool
    supportsCashQuery: bool
    minQuantity: str  # Decimal 字符串，禁止 float
    maxOrderRatePerSecond: int
    sessionOpenSupported: bool
    sessionIntradaySupported: bool
    sessionCloseSupported: bool

    def __post_init__(self) -> None:
        if not self.brokerId or not self.capabilityVersion:
            raise BrokerPortError("券商 ID 与能力版本不能为空")
        if not self.orderTypes:
            raise BrokerPortError("能力清单必须至少声明一种订单类型")
        if self.maxOrderRatePerSecond <= 0:
            raise BrokerPortError("限频必须为正")
        if not self.minQuantity:
            raise BrokerPortError("最小手数不能为空")

    def supportsOrderType(self, orderType: OrderType) -> bool:
        return orderType in self.orderTypes

    def supportsTimeInForce(self, timeInForce: TimeInForce) -> bool:
        return timeInForce in self.timeInForces

    def supportsSymbol(self, symbol: str) -> bool:
        return symbol in self.symbols


@dataclass(frozen=True, slots=True)
class OrderRequestV1:
    """统一发单请求（BrokerPort 输入；第三方字段不进入领域模型）。"""

    clientOrderId: str
    accountId: str
    symbol: str
    side: OrderSide
    orderType: OrderType
    timeInForce: TimeInForce
    quantity: str  # Decimal 字符串
    limitPrice: str | None = None
    stopPrice: str | None = None

    def __post_init__(self) -> None:
        if not self.clientOrderId or not self.accountId or not self.symbol:
            raise BrokerPortError("发单请求标识字段不能为空")


@dataclass(frozen=True, slots=True)
class OrderSubmissionResultV1:
    """发单结果：broker 侧受理凭证。"""

    clientOrderId: str
    brokerOrderId: str
    brokerState: str  # 受控枚举值（见 Orders.BrokerState）
    submittedAt: str  # UTC ISO 诊断时间
    diagnosticFields: dict[str, str] = None  # type: ignore[assignment]

    def __post_init__(self) -> None:
        if not self.clientOrderId or not self.brokerOrderId:
            raise BrokerPortError("发单结果标识字段不能为空")
        object.__setattr__(self, "diagnosticFields", dict(self.diagnosticFields or {}))


class BrokerPort(Protocol):
    """统一券商端口：能力查询 + 发单 + 撤单 + 查询 + 回报订阅。"""

    def capability(self) -> BrokerCapabilityV1: ...

    def submitOrder(self, request: OrderRequestV1) -> OrderSubmissionResultV1: ...

    def cancelOrder(self, clientOrderId: str, brokerOrderId: str) -> str: ...

    def queryOrder(self, clientOrderId: str, brokerOrderId: str) -> dict[str, object]: ...

    def queryPositions(self, accountId: str) -> dict[str, str]: ...

    def queryCash(self, accountId: str) -> str: ...


class CapabilityNegotiatorV1:
    """发单前能力协商：不支持的能力在发单前拒绝。

    请求订单类型/时间生效/标的/方向任一不在能力清单内 -> 抛 BrokerPortError，
    不会向券商发送任何请求。
    """

    def __init__(self, capability: BrokerCapabilityV1) -> None:
        if capability is None:
            raise BrokerPortError("能力清单不能为空")
        self._capability = capability

    @property
    def capability(self) -> BrokerCapabilityV1:
        return self._capability

    def negotiate(self, request: OrderRequestV1) -> None:
        """校验发单请求全部能力受支持；不满足直接拒绝。"""
        if not self._capability.supportsOrderType(request.orderType):
            raise BrokerPortError(
                f"券商 {self._capability.brokerId} 不支持订单类型 {request.orderType.value}"
            )
        if not self._capability.supportsTimeInForce(request.timeInForce):
            raise BrokerPortError(
                f"券商 {self._capability.brokerId} 不支持有效期 {request.timeInForce.value}"
            )
        if not self._capability.supportsSymbol(request.symbol):
            raise BrokerPortError(
                f"券商 {self._capability.brokerId} 不支持标的 {request.symbol}"
            )
        if request.side not in self._capability.orderSides:
            raise BrokerPortError(
                f"券商 {self._capability.brokerId} 不支持方向 {request.side.value}"
            )
        if request.orderType in (OrderType.Limit, OrderType.StopLimit) and request.limitPrice is None:
            raise BrokerPortError("LIMIT/STOP_LIMIT 必须携带 limitPrice")
        if request.orderType in (OrderType.Stop, OrderType.StopLimit) and request.stopPrice is None:
            raise BrokerPortError("STOP/STOP_LIMIT 必须携带 stopPrice")
        if request.orderType is OrderType.Market and (request.limitPrice is not None or request.stopPrice is not None):
            raise BrokerPortError("MARKET 订单不得携带限价或止损价格")
