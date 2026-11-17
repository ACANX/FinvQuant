"""P2-006 纸上交易适配器。

纸上交易使用增量行情回放（非全量历史），并遵循与理想/仿真/实盘相同的
订单、回报、账本和审计契约（TechSpec 7.x）：订单意图 -> 订单状态迁移
-> `ExecutionReportEventV1` 回报。增量行情断流时进入保护状态，禁止
继续发单，直到显式恢复确认。

阶段 1 纸上撮合采用保守参数：复用 `IdealExecutionAdapterV1` 的确定性
撮合（市价按下一 Bar 开盘价、限价触发成交），由增量 feed 负责断流保护。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.ExecutionModel import ExecutionModelError
from veritasquant.execution.IdealExecution import IdealExecutionAdapterV1
from veritasquant.execution.IncrementalFeed import IncrementalMarketFeedV1
from veritasquant.execution.Orders import (
    ExecutionReportEventV1,
    OrderSide,
    OrderState,
    OrderType,
)


class PaperTradingError(ValueError):
    """纸上交易保护状态或订单契约不满足。"""


@dataclass(frozen=True, slots=True)
class PaperOrderSubmissionV1:
    """纸上交易发单结果：与外部适配器相同的回报契约。"""

    report: ExecutionReportEventV1 | None
    protectionTriggered: bool
    mode: str = "PAPER_TRADING"


class PaperTradingAdapterV1:
    """增量行情驱动的纸上交易适配器。"""

    def __init__(
        self,
        feed: IncrementalMarketFeedV1,
        executor: IdealExecutionAdapterV1,
    ) -> None:
        if feed is None or executor is None:
            raise PaperTradingError("增量 feed 与撮合执行器不能为空")
        self._feed = feed
        self._executor = executor

    @property
    def protected(self) -> bool:
        """断流保护状态：True 时禁止新发单。"""
        return self._feed.protected

    def ingestBar(self, bar: MinuteBarSchemaV1, ingestedAt=None) -> None:  # noqa: ANN001
        """增量接入一根新 Bar；断流进入保护状态。"""
        self._feed.ingest(bar, ingestedAt)

    def recover(self) -> None:
        """显式恢复发单（数据连续性确认后）。"""
        self._feed.recover()

    def submitOrder(
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
    ) -> PaperOrderSubmissionV1:
        """提交订单；断流保护状态下拒绝发单（不产生任何订单副作用）。"""
        if self._feed.protected:
            return PaperOrderSubmissionV1(None, protectionTriggered=True)
        try:
            result = self._executor.matchOrder(
                clientOrderId=clientOrderId,
                accountId=accountId,
                orderState=orderState,
                orderVersion=orderVersion,
                side=side,
                orderType=orderType,
                quantity=quantity,
                limitPrice=limitPrice,
                symbol=symbol,
                brokerOrderId=brokerOrderId,
                effectiveAfterEventId=effectiveAfterEventId,
                currentBar=currentBar,
                previouslyMatchedQuantity=previouslyMatchedQuantity,
            )
        except ExecutionModelError as error:
            raise PaperTradingError(f"纸上交易撮合失败: {error}") from error
        return PaperOrderSubmissionV1(result.report if result is not None else None, protectionTriggered=False)
