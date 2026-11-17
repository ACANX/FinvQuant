"""RiskEngine 分层审批与唯一发布权（技术方案 8.2/8.3 节）。

RiskEngine 是唯一交易风险决策点：全局、组合、策略三层检查，最严格控制
优先；只有 RiskEngine 能分配决定/控制 ID、持久化结果并写入 outbox。
AlertPolicyEngine 仅返回候选建议，无事件发布权。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.Orders import OrderIntentV1
from veritasquant.risk.AlertModels import AlertEventV1
from veritasquant.risk.AlertPolicyEngine import AlertPolicyEngineV1, PolicyContextV1


class RiskEngineError(ValueError):
    """风险决定违反分层审批或唯一发布权契约时抛出。"""


class RiskDecision(StrEnum):
    Approved = "APPROVED"
    Rejected = "REJECTED"
    Reduced = "REDUCED"


class ControlAction(StrEnum):
    """控制强度偏序：数值越大越严格。"""

    RejectNewOrders = "REJECT_NEW_ORDERS"
    ReduceOnly = "REDUCE_ONLY"
    PauseScope = "PAUSE_SCOPE"
    StopTrading = "STOP_TRADING"


@dataclass(frozen=True, slots=True)
class RiskDecisionEventV1:
    """仅 RiskEngine 发布的不可变风险决定。"""

    decisionId: str
    requestEventId: str
    accountId: str
    decision: RiskDecision
    approvedQuantity: Decimal
    ruleIds: tuple[str, ...]
    riskPolicyVersion: str
    accountSnapshotVersion: int
    orderSnapshotVersion: int
    positionSnapshotVersion: int
    reasonCodes: tuple[str, ...]
    decisionHash: str


@dataclass(frozen=True, slots=True)
class TradingControlEventV1:
    """仅 RiskEngine 发布的交易控制；版本从 1 单调递增。"""

    controlId: str
    controlVersion: int
    controlRequestId: str
    idempotencyKey: str
    scope: str
    action: ControlAction
    strength: int
    parameters: dict[str, object]
    effectiveFrom: str
    expiresAt: str | None
    sourceDecisionId: str
    riskPolicyVersion: str
    status: str
    controlHash: str


@dataclass(frozen=True, slots=True)
class ApprovalContextV1:
    """审批所需的只读快照版本与状态。"""

    accountId: str
    accountSnapshotVersion: int
    orderSnapshotVersion: int
    positionSnapshotVersion: int
    cashAvailable: Decimal
    exposure: Decimal
    equity: Decimal
    openOrderQuantity: Decimal


@dataclass(slots=True)
class RiskEngineV1:
    """三层审批引擎：全局/组合/策略最严格控制优先，唯一发布权。"""

    policyEngine: AlertPolicyEngineV1
    activeControls: dict[str, TradingControlEventV1] = field(default_factory=dict)
    _decisions: dict[str, RiskDecisionEventV1] = field(default_factory=dict)
    _controls: dict[str, TradingControlEventV1] = field(default_factory=dict)
    _decisionCounter: int = 0
    _controlCounter: int = 0

    def approveIntent(
        self,
        intent: OrderIntentV1,
        context: ApprovalContextV1,
        alert: AlertEventV1 | None = None,
    ) -> RiskDecisionEventV1:
        """审批一条订单意图：三层检查后给出唯一最终决定。"""
        if intent.accountId != context.accountId:
            raise RiskEngineError("意图账户与审批上下文账户不一致")

        # 1) 全局层：活动 STOP_TRADING / PAUSE_SCOPE 控制
        globalBlocks = self._strongestControlFor("account", context.accountId)
        if globalBlocks is not None and globalBlocks.strength >= 40:
            return self._decide(
                intent=intent,
                context=context,
                decision=RiskDecision.Rejected,
                approvedQuantity=Decimal("0"),
                ruleIds=(globalBlocks.action.value,),
                reasonCodes=("GLOBAL_CONTROL_BLOCK",),
            )

        # 2) 组合层：纯函数策略引擎候选
        if alert is not None:
            evaluation = self.policyEngine.evaluate(alert, self._policyContext(context))
            for candidate in evaluation.actions:
                if candidate.strength >= 40:
                    return self._decide(
                        intent=intent,
                        context=context,
                        decision=RiskDecision.Rejected,
                        approvedQuantity=Decimal("0"),
                        ruleIds=evaluation.matchedRules,
                        reasonCodes=("POLICY_CANDIDATE_BLOCK",),
                    )
                if candidate.strength >= 30:
                    return self._decide(
                        intent=intent,
                        context=context,
                        decision=RiskDecision.Reduced,
                        approvedQuantity=intent.quantity // 2,
                        ruleIds=evaluation.matchedRules,
                        reasonCodes=("POLICY_CANDIDATE_REDUCE",),
                    )

        # 3) 策略级：现金与敞口边界
        if context.cashAvailable < intent.quantity:
            return self._decide(
                intent=intent,
                context=context,
                decision=RiskDecision.Rejected,
                approvedQuantity=Decimal("0"),
                ruleIds=("rule.insufficient_cash",),
                reasonCodes=("INSUFFICIENT_CASH",),
            )
        if context.equity > 0 and context.exposure + intent.quantity > context.equity * Decimal("2"):
            return self._decide(
                intent=intent,
                context=context,
                decision=RiskDecision.Reduced,
                approvedQuantity=intent.quantity // 2,
                ruleIds=("rule.exposure_limit",),
                reasonCodes=("EXPOSURE_LIMIT",),
            )

        return self._decide(
            intent=intent,
            context=context,
            decision=RiskDecision.Approved,
            approvedQuantity=intent.quantity,
            ruleIds=(),
            reasonCodes=(),
        )

    def publishControl(self, control: TradingControlEventV1) -> TradingControlEventV1:
        """发布或更新控制；版本必须单调递增，禁止原地放宽。"""
        existing = self._controls.get(control.controlId)
        if existing is not None:
            if control.controlVersion <= existing.controlVersion:
                raise RiskEngineError("控制版本必须单调递增")
            if control.strength < existing.strength:
                raise RiskEngineError("禁止原地放宽控制强度")
        self._controls[control.controlId] = control
        self.activeControls[control.controlId] = control
        return control

    def releaseControl(self, controlId: str, newVersion: int) -> TradingControlEventV1:
        """解除控制使用新版本；禁止删除历史。"""
        existing = self._controls.get(controlId)
        if existing is None:
            raise RiskEngineError("未知控制 ID")
        if newVersion <= existing.controlVersion:
            raise RiskEngineError("解除版本必须高于现有版本")
        released = TradingControlEventV1(
            controlId=existing.controlId,
            controlVersion=newVersion,
            controlRequestId=existing.controlRequestId,
            idempotencyKey=existing.idempotencyKey,
            scope=existing.scope,
            action=existing.action,
            strength=existing.strength,
            parameters=existing.parameters,
            effectiveFrom=existing.effectiveFrom,
            expiresAt=None,
            sourceDecisionId=existing.sourceDecisionId,
            riskPolicyVersion=existing.riskPolicyVersion,
            status="RELEASED",
            controlHash=canonicalHash(
                {
                    "control_id": existing.controlId,
                    "control_version": newVersion,
                    "status": "RELEASED",
                }
            ),
        )
        self._controls[controlId] = released
        self.activeControls.pop(controlId, None)
        return released

    def decisions(self) -> tuple[RiskDecisionEventV1, ...]:
        """已发布决定（只读）。"""
        return tuple(self._decisions.values())

    def controls(self) -> tuple[TradingControlEventV1, ...]:
        """全部控制版本历史（只读）。"""
        return tuple(self._controls.values())

    def _strongestControlFor(self, scope: str, accountId: str) -> TradingControlEventV1 | None:
        candidates = [
            item
            for item in self.activeControls.values()
            if item.scope == scope and item.status != "RELEASED"
        ]
        if not candidates:
            return None
        return max(candidates, key=lambda item: item.strength)

    def _policyContext(self, context: ApprovalContextV1) -> PolicyContextV1:
        return PolicyContextV1(
            accountId=context.accountId,
            cashAvailable=context.cashAvailable,
            exposure=context.exposure,
            equity=context.equity,
            openOrderQuantity=context.openOrderQuantity,
        )

    def _decide(
        self,
        intent: OrderIntentV1,
        context: ApprovalContextV1,
        decision: RiskDecision,
        approvedQuantity: Decimal,
        ruleIds: tuple[str, ...],
        reasonCodes: tuple[str, ...],
    ) -> RiskDecisionEventV1:
        self._decisionCounter += 1
        decisionId = f"decision-{self._decisionCounter}"
        event = RiskDecisionEventV1(
            decisionId=decisionId,
            requestEventId=intent.intentId,
            accountId=context.accountId,
            decision=decision,
            approvedQuantity=approvedQuantity,
            ruleIds=ruleIds,
            riskPolicyVersion=self.policyEngine.policyVersion,
            accountSnapshotVersion=context.accountSnapshotVersion,
            orderSnapshotVersion=context.orderSnapshotVersion,
            positionSnapshotVersion=context.positionSnapshotVersion,
            reasonCodes=reasonCodes,
            decisionHash=canonicalHash(
                {
                    "decision_id": decisionId,
                    "request_event_id": intent.intentId,
                    "account_id": context.accountId,
                    "decision": decision.value,
                    "approved_quantity": approvedQuantity,
                    "rule_ids": list(ruleIds),
                }
            ),
        )
        self._decisions[decisionId] = event
        return event
