"""理想执行适配器：只撮合此前已生效的订单，结果显式标记理想模式。

技术方案 7.2 节：市价单按下一根允许成交 Bar 的开盘价完全成交；限价单在
价格触及时按不劣于限价的价格完全成交；除显式配置的手续费外不引入延迟、
滑点和部分成交。结果必须显式标识理想模式，避免混同实盘预期，并走与
真实路径相同的订单、回报、账本和审计契约。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.Orders import (
    BrokerState,
    ExecutionReportEventV1,
    ExecutionType,
    OrderSide,
    OrderState,
    OrderType,
)


class IdealExecutionError(ValueError):
    """理想执行违反生效边界或价格保护契约时抛出。"""


@dataclass(frozen=True, slots=True)
class IdealFillResultV1:
    """理想模式单笔成交结果。"""

    report: ExecutionReportEventV1
    fillPrice: Decimal
    fillQuantity: Decimal
    mode: str = "IDEAL"


class IdealExecutionAdapterV1:
    """确定性理想撮合：仅撮合已生效订单，按开盘价/触发价完全成交。"""

    def __init__(self, feePerUnit: Decimal = Decimal("0")) -> None:
        """feePerUnit 为显式配置的单位手续费；缺省零摩擦。"""
        if not isinstance(feePerUnit, Decimal) or feePerUnit < 0:
            raise IdealExecutionError("单位手续费必须为非负 Decimal")
        self._feePerUnit = feePerUnit
        self._nextReportSequence = 1

    def matchOrder(
        self,
        *,
        clientOrderId: str,
        accountId: str,
        orderState: OrderState,
        orderVersion: int,
        side: OrderSide,
        orderType: OrderType,
        quantity: Decimal,
        limitPrice: Decimal | None,
        symbol: str,
        brokerOrderId: str | None,
        effectiveAfterEventId: str,
        currentBar: MinuteBarSchemaV1,
        previouslyMatchedQuantity: Decimal = Decimal("0"),
    ) -> IdealFillResultV1 | None:
        """对当前 Bar 撮合一笔此前已生效订单；返回成交或 None（未触发）。"""
        if not clientOrderId or not accountId:
            raise IdealExecutionError("订单必须包含账户和订单 ID")
        if orderState not in (OrderState.Accepted, OrderState.PartiallyFilled):
            raise IdealExecutionError("理想执行只接受已生效订单")
        if orderType not in (OrderType.Market, OrderType.Limit):
            raise IdealExecutionError("理想模式阶段 1 只支持市价与限价单")
        if not isinstance(quantity, Decimal) or quantity <= 0:
            raise IdealExecutionError("订单数量必须为正 Decimal")
        if not isinstance(previouslyMatchedQuantity, Decimal) or previouslyMatchedQuantity < 0:
            raise IdealExecutionError("已成交数量必须为非负 Decimal")
        if previouslyMatchedQuantity > quantity:
            raise IdealExecutionError("已成交数量不得超过订单数量")
        _validateBar(currentBar)

        remaining = quantity - previouslyMatchedQuantity
        if remaining <= 0:
            return None

        if orderType is OrderType.Market:
            # 市价单：下一允许 Bar 开盘价完全成交（理想模式无滑点）。
            fillPrice = currentBar.open
            triggered = True
        else:
            if limitPrice is None:
                raise IdealExecutionError("限价单必须携带限价")
            fillPrice, triggered = _limitTrigger(side, limitPrice, currentBar)
            if not triggered:
                return None

        fillQuantity = remaining
        report = ExecutionReportEventV1(
            BrokerReportId=f"ideal-{self._nextReportSequence}",
            ClientOrderId=clientOrderId,
            BrokerOrderId=brokerOrderId,
            ReportSequence=self._nextReportSequence,
            ExecutionType=ExecutionType.Fill,
            ExecutionId=f"ideal-exec-{self._nextReportSequence}",
            LastQuantity=fillQuantity,
            LastPrice=fillPrice,
            CumulativeQuantity=previouslyMatchedQuantity + fillQuantity,
            RemainingQuantity=Decimal("0"),
            BrokerState=BrokerState.Filled,
            DiagnosticTs=currentBar.ts,
            AccountId=accountId,
            Ts=_utcNow(),
        )  # type: ignore[call-arg]
        self._nextReportSequence += 1
        return IdealFillResultV1(report=report, fillPrice=fillPrice, fillQuantity=fillQuantity)


def _limitTrigger(side: OrderSide, limitPrice: Decimal, bar: MinuteBarSchemaV1) -> tuple[Decimal, bool]:
    """DIRECTIONAL_OHLC_V1 路径首次触发：买入价格 <= 限价，卖出价格 >= 限价。"""
    if side is OrderSide.Buy:
        if bar.open <= limitPrice:
            # 开盘即满足：按不劣于限价的价格（开盘价）成交
            return bar.open, True
        if bar.low <= limitPrice:
            return limitPrice, True
        return Decimal("0"), False
    if side is OrderSide.Sell:
        if bar.open >= limitPrice:
            return bar.open, True
        if bar.high >= limitPrice:
            return limitPrice, True
        return Decimal("0"), False
    raise IdealExecutionError("未知订单方向")


def _validateBar(bar: MinuteBarSchemaV1) -> None:
    if bar.high < bar.low:
        raise IdealExecutionError("Bar high 不得低于 low")
    if bar.high < bar.open or bar.high < bar.close:
        raise IdealExecutionError("Bar high 必须覆盖开盘与收盘")
    if bar.low > bar.open or bar.low > bar.close:
        raise IdealExecutionError("Bar low 必须覆盖开盘与收盘")


def _utcNow() -> datetime:
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1000) * 1000)
