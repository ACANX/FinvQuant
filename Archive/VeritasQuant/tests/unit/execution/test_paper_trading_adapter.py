"""P2-006 纸上交易适配器与增量接入单元测试。

验收标准映射：
- 适配器遵循相同订单/回报契约（返回 ExecutionReportEventV1）；
- 断流进入保护状态而非继续发单。
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from decimal import Decimal

import pytest

from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.IdealExecution import IdealExecutionAdapterV1
from veritasquant.execution.IncrementalFeed import IncrementalFeedError, IncrementalMarketFeedV1
from veritasquant.execution.Orders import ExecutionReportEventV1, OrderSide, OrderState, OrderType
from veritasquant.execution.PaperTradingAdapter import PaperTradingAdapterV1


def _bar(barStart: datetime, symbol: str = "TEST", close: str = "10.00") -> MinuteBarSchemaV1:
    price = Decimal(close)
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": barStart + timedelta(minutes=1),
            "BarStart": barStart,
            "BarEnd": barStart + timedelta(minutes=1),
            "Symbol": symbol,
            "Market": "SSE",
            "Open": price,
            "High": price,
            "Low": price,
            "Close": price,
            "Volume": Decimal("1000"),
            "Currency": "CNY",
            "SessionId": "s1",
            "Source": "fixture",
            "SourceRecordId": "r1",
            "SourceSequence": 1,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "v1",
            "QualityFlags": 0,
        }
    )


def _adapter(maxGapSeconds: int = 300) -> PaperTradingAdapterV1:
    feed = IncrementalMarketFeedV1(maxGapSeconds=maxGapSeconds)
    executor = IdealExecutionAdapterV1()
    return PaperTradingAdapterV1(feed, executor)


class TestIncrementalFeed:
    def test_consecutive_bars_advance_normally(self) -> None:
        feed = IncrementalMarketFeedV1()
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        feed.ingest(_bar(base))
        feed.ingest(_bar(base + timedelta(minutes=1)))
        assert not feed.protected
        assert feed.state.lastBarStart == base + timedelta(minutes=1)

    def test_gap_enters_protection(self) -> None:
        feed = IncrementalMarketFeedV1(maxGapSeconds=300)
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        feed.ingest(_bar(base))
        feed.ingest(_bar(base + timedelta(minutes=10)))  # 600s > 300s
        assert feed.protected
        assert "断流" in (feed.lastProtectionReason or "")

    def test_out_of_order_enters_protection(self) -> None:
        feed = IncrementalMarketFeedV1()
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        feed.ingest(_bar(base + timedelta(minutes=5)))
        feed.ingest(_bar(base))  # 倒退
        assert feed.protected

    def test_protected_feed_rejects_ingest_until_recover(self) -> None:
        feed = IncrementalMarketFeedV1(maxGapSeconds=60)
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        feed.ingest(_bar(base))
        feed.ingest(_bar(base + timedelta(minutes=5)))
        assert feed.protected
        with pytest.raises(IncrementalFeedError):
            feed.ingest(_bar(base + timedelta(minutes=6)))
        feed.recover()
        feed.ingest(_bar(base + timedelta(minutes=6)))  # 恢复后可继续


class TestPaperTradingAdapter:
    def test_submit_returns_same_report_contract(self) -> None:
        adapter = _adapter()
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        adapter.ingestBar(_bar(base))
        result = adapter.submitOrder(
            clientOrderId="c1",
            accountId="a1",
            orderState=OrderState.Accepted,
            orderVersion=1,
            side=OrderSide.Buy,
            orderType=OrderType.Market,
            quantity=Decimal("100"),
            limitPrice=None,
            symbol="TEST",
            brokerOrderId=None,
            effectiveAfterEventId="evt-1",
            currentBar=_bar(base + timedelta(minutes=1), close="10.50"),
        )
        assert result.report is not None
        assert isinstance(result.report, ExecutionReportEventV1)
        assert result.mode == "PAPER_TRADING"
        assert not result.protectionTriggered

    def test_stale_feed_blocks_new_orders(self) -> None:
        """断流进入保护状态而非继续发单。"""
        adapter = _adapter(maxGapSeconds=60)
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        adapter.ingestBar(_bar(base))
        adapter.ingestBar(_bar(base + timedelta(minutes=5)))  # 断流
        assert adapter.protected
        result = adapter.submitOrder(
            clientOrderId="c2",
            accountId="a1",
            orderState=OrderState.Accepted,
            orderVersion=1,
            side=OrderSide.Buy,
            orderType=OrderType.Market,
            quantity=Decimal("100"),
            limitPrice=None,
            symbol="TEST",
            brokerOrderId=None,
            effectiveAfterEventId="evt-2",
            currentBar=_bar(base + timedelta(minutes=6)),
        )
        assert result.report is None  # 不发单、无副作用
        assert result.protectionTriggered

    def test_recover_allows_orders_again(self) -> None:
        adapter = _adapter(maxGapSeconds=60)
        base = datetime(2026, 8, 3, 9, 30, tzinfo=timezone.utc)
        adapter.ingestBar(_bar(base))
        adapter.ingestBar(_bar(base + timedelta(minutes=5)))
        assert adapter.protected
        adapter.recover()
        result = adapter.submitOrder(
            clientOrderId="c3",
            accountId="a1",
            orderState=OrderState.Accepted,
            orderVersion=1,
            side=OrderSide.Buy,
            orderType=OrderType.Market,
            quantity=Decimal("100"),
            limitPrice=None,
            symbol="TEST",
            brokerOrderId=None,
            effectiveAfterEventId="evt-3",
            currentBar=_bar(base + timedelta(minutes=6)),
        )
        assert result.report is not None
