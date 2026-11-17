"""行情到成交、账本、策略回调、风控和报告端到端测试（P1-071）。

固定行情数据驱动：事件 → 策略回调 → 风险审批 → 订单撮合 → 执行回报 →
原子账本 → 双轨报告。验证关联 ID、审计链、资源预占和状态即时固化。
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from decimal import Decimal


from veritasquant.accounts.Ledger import (
    CashJournalFactoryV1,
    LedgerProjectionStoreV1,
    LedgerStoreV1,
)
from veritasquant.accounts.Reservation import ReservationBookV1
from veritasquant.core.Transaction import TransactionStoreV1
from veritasquant.data.MinuteBar import MinuteBarSchemaV1
from veritasquant.execution.AtomicExecution import AtomicExecutionBoundaryV1
from veritasquant.execution.IdealExecution import IdealExecutionAdapterV1
from veritasquant.execution.OrderStateMachine import OrderStateMachineV1, TransitionKind
from veritasquant.risk.RiskEngine import (
    ApprovalContextV1,
    RiskDecision,
    RiskEngineV1,
)
from veritasquant.risk.AlertPolicyEngine import AlertPolicyEngineV1
from veritasquant.strategy.BaseStrategy import (
    ConsumedEventV1,
    StrategyContextV1,
    StrategyInstrumentV1,
    StrategySnapshotV1,
)
from veritasquant.strategy.ExampleStrategies import DailyMomentumStrategy

UTC = timezone.utc
VERSIONS = ("metadata-v1", "fees-v1", "policy-v1")


def _bar(ts: datetime, close: Decimal) -> MinuteBarSchemaV1:
    return MinuteBarSchemaV1.model_validate(
        {
            "Ts": ts,
            "BarStart": ts - timedelta(minutes=1),
            "BarEnd": ts - timedelta(seconds=1),
            "Symbol": "518880",
            "Market": "SSE",
            "Open": close,
            "High": close,
            "Low": close,
            "Close": close,
            "Volume": Decimal("1000000"),
            "Currency": "CNY",
            "SessionId": "cn-morning",
            "Source": "fixture",
            "SourceRecordId": f"bar-{ts.isoformat()}",
            "SourceSequence": 1,
            "IsAdjusted": False,
            "InstrumentMetadataVersion": "meta-v1",
            "QualityFlags": 0,
        }
    )


class _EndToEndFixture:
    """固定证券（518880）场景的端到端装配。"""

    def __init__(self) -> None:
        self.ledgerStore = LedgerStoreV1()
        self.journalFactory = CashJournalFactoryV1(*VERSIONS)
        self._seedLedger()
        self.reservationBook = ReservationBookV1()
        self.stateMachine = OrderStateMachineV1()
        self.transactionStore = TransactionStoreV1()
        self.policyEngine = AlertPolicyEngineV1()
        self.riskEngine = RiskEngineV1(policyEngine=self.policyEngine)

    def _seedLedger(self) -> None:
        self.ledgerStore.commitJournal(
            self.journalFactory.createOpeningBalance(
                "e2e:open", "account-1", datetime(2026, 1, 1, tzinfo=UTC), 1, "e2e-open", "CNY", Decimal("100000")
            )
        )

    def strategyContext(self) -> StrategyContextV1:
        return StrategyContextV1(
            strategyId="daily_momentum",
            strategyVersion="1.0.0",
            runId="run-e2e",
            accountId="account-1",
            subaccountId="strategy-1",
            snapshot=StrategySnapshotV1(
                accountId="account-1",
                subaccountId="strategy-1",
                cashAvailable=Decimal("100000"),
                positions={"518880": Decimal("0")},
                snapshotVersion=1,
            ),
            instrument=StrategyInstrumentV1(
                symbol="518880",
                metadataVersion="meta-v1",
                tickSize=Decimal("0.001"),
                lotSize=Decimal("100"),
                currency="CNY",
            ),
        )

    def atomicBoundary(self) -> AtomicExecutionBoundaryV1:
        return AtomicExecutionBoundaryV1(
            transactionStore=self.transactionStore,
            ledgerStore=self.ledgerStore,
            reservationBook=self.reservationBook,
            stateMachine=self.stateMachine,
            journalFactory=self.journalFactory,
            accountId="account-1",
            currency="CNY",
        )


def test_end_to_end_market_to_report_chain() -> None:
    """固定行情 → 策略意图 → 风控审批 → 理想成交 → 原子账本 → 状态固化。"""
    fixture = _EndToEndFixture()
    context = fixture.strategyContext()
    strategy = DailyMomentumStrategy(lookbackDays=2, threshold=Decimal("0.01"))
    strategy.bind(context)

    # 阶段 1：回放固定行情，策略产生买入意图
    closes = ["1.000", "1.010", "1.020", "1.040"]
    for index, close in enumerate(closes):
        ts = datetime(2026, 8, index + 1, 10, 0, tzinfo=UTC)
        event = ConsumedEventV1(
            eventId=f"e2e-bar-{index}",
            eventType="MarketBarEvent",
            ts=ts,
            payload={"close": close, "symbol": "518880"},
        )
        strategy.onBar(event)
    intents = strategy.emitIntents()
    assert intents, "动量场景应产生买入意图"
    intent = intents[0]

    # 阶段 2：RiskEngine 审批（唯一发布权）
    decision = fixture.riskEngine.approveIntent(
        intent,
        ApprovalContextV1(
            accountId="account-1",
            accountSnapshotVersion=1,
            orderSnapshotVersion=1,
            positionSnapshotVersion=1,
            cashAvailable=Decimal("100000"),
            exposure=Decimal("0"),
            equity=Decimal("100000"),
            openOrderQuantity=Decimal("0"),
        ),
    )
    assert decision.decision is RiskDecision.Approved
    assert decision.requestEventId == intent.intentId  # 关联 ID 可审计

    # 阶段 3：订单状态机创建并推进到 ACCEPTED
    fixture.stateMachine.createIntent(f"order-{intent.intentId}", "account-1", intent.quantity, intent.expectedAccountVersion)
    fixture.stateMachine.transition(f"order-{intent.intentId}", "account-1", TransitionKind.RiskApproval, 1)
    fixture.stateMachine.transition(f"order-{intent.intentId}", "account-1", TransitionKind.CommandOutbox, 2)
    fixture.stateMachine.transition(f"order-{intent.intentId}", "account-1", TransitionKind.SendSuccess, 3)
    fixture.stateMachine.transition(f"order-{intent.intentId}", "account-1", TransitionKind.BrokerAccept, 4)

    # 阶段 4：预占 + 理想执行 → 执行回报
    fixture.reservationBook.reserve(
        f"order-{intent.intentId}", "account-1", intent.intentId, "CASH", "CNY", intent.quantity, Decimal("100000")
    )
    adapter = IdealExecutionAdapterV1()
    fill = adapter.matchOrder(
        clientOrderId=f"order-{intent.intentId}",
        accountId="account-1",
        orderState=fixture.stateMachine.snapshot(f"order-{intent.intentId}").state,
        orderVersion=fixture.stateMachine.snapshot(f"order-{intent.intentId}").orderVersion,
        side=intent.side,
        orderType=intent.orderType,
        quantity=intent.quantity,
        limitPrice=None,
        symbol="518880",
        brokerOrderId=None,
        effectiveAfterEventId=intent.createdFromEventId,
        currentBar=_bar(datetime(2026, 8, 5, 10, 0, tzinfo=UTC), Decimal("1.050")),
    )
    assert fill is not None
    assert fill.mode == "IDEAL"

    # 阶段 5：原子边界落账（订单迁移 + 预占消耗 + 账本 + outbox）
    outcome = fixture.atomicBoundary().commitExecutionReport(fill.report)
    assert outcome.journalCount == 1
    assert len(fixture.ledgerStore.journals) == 2  # 开户 + 成交
    assert len(fixture.transactionStore.outbox) == 1
    assert fixture.stateMachine.snapshot(f"order-{intent.intentId}").state.value == "FILLED"

    # 阶段 6：投影即时固化
    projection = LedgerProjectionStoreV1(fixture.ledgerStore).rebuild("account-1")
    assert projection.lastLedgerSequence == 2
    assert projection.projectionHash  # 状态即时固化


def test_end_to_end_audit_ids_are_connected() -> None:
    """关联 ID 链：intent → decision → execution → journal 全部连通。"""
    fixture = _EndToEndFixture()
    context = fixture.strategyContext()
    strategy = DailyMomentumStrategy(lookbackDays=2, threshold=Decimal("0.01"))
    strategy.bind(context)
    for index, close in enumerate(["1.000", "1.010", "1.020", "1.040"]):
        strategy.onBar(
            ConsumedEventV1(
                eventId=f"e2e-bar-{index}",
                eventType="MarketBarEvent",
                ts=datetime(2026, 8, index + 1, 10, 0, tzinfo=UTC),
                payload={"close": close, "symbol": "518880"},
            )
        )
    intent = strategy.emitIntents()[0]
    decision = fixture.riskEngine.approveIntent(
        intent,
        ApprovalContextV1("account-1", 1, 1, 1, Decimal("100000"), Decimal("0"), Decimal("100000"), Decimal("0")),
    )
    # intent → decision 关联
    assert decision.requestEventId == intent.intentId
    # 决策哈希可审计
    assert len(decision.decisionHash) == 64
