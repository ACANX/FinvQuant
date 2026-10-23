"""P5-007 实盘适配器、幂等下单和权威对账。

对齐 TechSpec 7.1/13 阶段 5：
- 发送结果未知不生成新 ID（幂等下单：同 clientOrderId 重试返回原 brokerOrderId）；
- 每日现金/持仓/订单/成交差异为 0（权威对账：以券商侧为权威）。

- `LiveOrderSubmissionV1`：实盘发单结果（未知结果复用原 ID）；
- `LiveBrokerAdapterV1`：实盘适配器（幂等下单 + 状态映射）；
- `AuthorityReconcilerV1`：权威对账（券商为权威，差异门禁）。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from veritasquant.broker.BrokerOrderGateway import OrderOutcome, SimBrokerOrderGatewayV1
from veritasquant.broker.BrokerPort import OrderRequestV1
from veritasquant.broker.BrokerSession import BrokerSessionV1, SessionManagerV1


class LiveOrderError(ValueError):
    """实盘订单或对账不满足契约时抛出。"""


class UnknownResultPolicy(StrEnum):
    """发送结果未知时的策略：复用原 ID（不生成新 ID）。"""

    ReuseOriginalId = "REUSE_ORIGINAL_ID"


@dataclass(frozen=True, slots=True)
class LiveOrderSubmissionV1:
    """实盘发单结果。"""

    clientOrderId: str
    brokerOrderId: str | None
    outcome: OrderOutcome
    reusedId: bool = False  # True = 未知结果重试复用原 ID


@dataclass(frozen=True, slots=True)
class AuthorityReconciliationItemV1:
    """权威对账单项（券商为权威）。"""

    itemType: str  # ORDER / FILL / POSITION / CASH
    referenceId: str
    matched: bool
    localValue: str
    authorityValue: str
    detail: str = ""


@dataclass(frozen=True, slots=True)
class AuthorityReconciliationReportV1:
    """权威对账报告：差异阻止交易。"""

    accountId: str
    items: tuple[AuthorityReconciliationItemV1, ...]
    differences: int
    blocking: bool

    @property
    def clean(self) -> bool:
        return self.differences == 0 and not self.blocking


class LiveBrokerAdapterV1:
    """实盘适配器：幂等下单（未知结果复用原 ID）+ 状态映射。"""

    def __init__(
        self,
        gateway: SimBrokerOrderGatewayV1,
        sessionManager: SessionManagerV1,
    ) -> None:
        if gateway is None or sessionManager is None:
            raise LiveOrderError("订单网关与会话管理不能为空")
        self._gateway = gateway
        self._sessionManager = sessionManager
        self._submissions: dict[str, LiveOrderSubmissionV1] = {}

    def submitOrder(
        self,
        *,
        session: BrokerSessionV1,
        request: OrderRequestV1,
        simulateAccept: bool = True,
        simulateReject: bool = False,
        simulateUnknown: bool = False,
    ) -> LiveOrderSubmissionV1:
        """幂等下单：未知结果不生成新 ID；同 clientOrderId 重试复用原 ID。"""
        # 幂等：同 clientOrderId 已有提交记录 -> 复用原结果（不生成新 ID）
        existing = self._submissions.get(request.clientOrderId)
        if existing is not None:
            return LiveOrderSubmissionV1(
                clientOrderId=request.clientOrderId,
                brokerOrderId=existing.brokerOrderId,
                outcome=existing.outcome,
                reusedId=True,
            )
        if simulateUnknown:
            # 未知结果：不生成新 ID，记录待查询状态
            submission = LiveOrderSubmissionV1(
                clientOrderId=request.clientOrderId,
                brokerOrderId=None,
                outcome=OrderOutcome.TimeoutUnknown,
                reusedId=False,
            )
            self._submissions[request.clientOrderId] = submission
            return submission
        outcome = self._gateway.submit(
            session=session,
            request=request,
            simulateAccept=simulateAccept,
            simulateReject=simulateReject,
        )
        submission = LiveOrderSubmissionV1(
            clientOrderId=request.clientOrderId,
            brokerOrderId=outcome.brokerOrderId,
            outcome=outcome.outcome,
            reusedId=False,
        )
        self._submissions[request.clientOrderId] = submission
        return submission

    def submissionFor(self, clientOrderId: str) -> LiveOrderSubmissionV1 | None:
        return self._submissions.get(clientOrderId)


class AuthorityReconcilerV1:
    """权威对账：以券商侧数据为权威，逐项核对；差异阻止交易。"""

    def __init__(self) -> None:
        self._reports: list[AuthorityReconciliationReportV1] = []

    def reconcile(
        self,
        *,
        accountId: str,
        localOrders: dict[str, str],      # clientOrderId -> localState
        authorityOrders: dict[str, str],  # clientOrderId -> brokerState（权威）
        localPositions: dict[str, str] | None = None,  # symbol -> qty
        authorityPositions: dict[str, str] | None = None,  # symbol -> qty
        localCash: str = "",
        authorityCash: str = "",
    ) -> AuthorityReconciliationReportV1:
        """逐项核对订单/持仓/现金；任一差异 -> blocking。"""
        items: list[AuthorityReconciliationItemV1] = []
        differences = 0

        # 订单核对
        allOrderIds = set(localOrders) | set(authorityOrders)
        for orderId in sorted(allOrderIds):
            localState = localOrders.get(orderId, "UNKNOWN")
            authorityState = authorityOrders.get(orderId, "UNKNOWN")
            matched = localState == authorityState
            if not matched:
                differences += 1
            items.append(
                AuthorityReconciliationItemV1(
                    itemType="ORDER",
                    referenceId=orderId,
                    matched=matched,
                    localValue=localState,
                    authorityValue=authorityState,
                    detail="" if matched else "本地与券商订单状态不一致",
                )
            )

        # 持仓核对
        localPositions = localPositions or {}
        authorityPositions = authorityPositions or {}
        allSymbols = set(localPositions) | set(authorityPositions)
        for symbol in sorted(allSymbols):
            localQty = localPositions.get(symbol, "0")
            authorityQty = authorityPositions.get(symbol, "0")
            matched = localQty == authorityQty
            if not matched:
                differences += 1
            items.append(
                AuthorityReconciliationItemV1(
                    itemType="POSITION",
                    referenceId=symbol,
                    matched=matched,
                    localValue=localQty,
                    authorityValue=authorityQty,
                    detail="" if matched else "持仓数量不一致",
                )
            )

        # 现金核对
        if localCash or authorityCash:
            matched = localCash == authorityCash
            if not matched:
                differences += 1
            items.append(
                AuthorityReconciliationItemV1(
                    itemType="CASH",
                    referenceId=accountId,
                    matched=matched,
                    localValue=localCash,
                    authorityValue=authorityCash,
                    detail="" if matched else "现金余额不一致",
                )
            )

        report = AuthorityReconciliationReportV1(
            accountId=accountId,
            items=tuple(items),
            differences=differences,
            blocking=differences > 0,
        )
        self._reports.append(report)
        return report

    def reports(self) -> tuple[AuthorityReconciliationReportV1, ...]:
        return tuple(self._reports)
