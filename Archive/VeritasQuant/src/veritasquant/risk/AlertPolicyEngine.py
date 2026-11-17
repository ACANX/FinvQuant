"""纯函数 AlertPolicyEngine：输入告警、上下文与策略版本，输出候选动作。

禁止访问总线、数据库、OMS、网络或产生最终交易决定；相同输入输出哈希
相同；任何失败默认保持或收紧保护，不得隐式放宽交易权限。
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertSeverity,
    AlertStatus,
)


class PolicyEngineError(ValueError):
    """策略输入或版本不满足纯函数契约时抛出。"""


@dataclass(frozen=True, slots=True)
class PolicyContextV1:
    """求值的只读上下文：账户余额、持仓、敞口与现有控制。"""

    accountId: str
    cashAvailable: Decimal
    exposure: Decimal
    activeControls: tuple[str, ...] = ()
    openOrderQuantity: Decimal = Decimal("0")
    equity: Decimal = Decimal("0")


@dataclass(frozen=True, slots=True)
class CandidateActionV1:
    """候选动作：仅供 RiskEngine 决策，本引擎无权发布。"""

    action: str
    scope: str
    strength: int
    reasonCodes: tuple[str, ...]
    evidence: tuple[dict[str, object], ...]


@dataclass(frozen=True, slots=True)
class PolicyEvaluationV1:
    """一次求值的完整结果。"""

    policyVersion: str
    alertId: str
    actions: tuple[CandidateActionV1, ...]
    matchedRules: tuple[str, ...]
    outputHash: str


class AlertPolicyEngineV1:
    """纯函数规则求值器：无副作用，相同输入产生相同输出哈希。"""

    def __init__(self, policyVersion: str = "P1-RISK-POLICY-V1") -> None:
        if not policyVersion:
            raise PolicyEngineError("策略版本不能为空")
        self._policyVersion = policyVersion

    @property
    def policyVersion(self) -> str:
        return self._policyVersion

    def evaluate(self, alert: AlertEventV1, context: PolicyContextV1) -> PolicyEvaluationV1:
        """对单条预警执行纯规则求值，返回候选动作与命中规则。"""
        actions: list[CandidateActionV1] = []
        matched: list[str] = []

        # 规则 1：P0 严重度且 ACTIVE → 暂停新订单（保护收紧）
        if alert.severity is AlertSeverity.P0 and alert.status is AlertStatus.Active:
            actions.append(
                self._candidate(
                    action="PAUSE_SCOPE",
                    scope="account",
                    strength=40,
                    reasonCodes=("P0_ACTIVE_ALERT",),
                    evidence=[{"alert_id": alert.alertId, "severity": alert.severity.value}],
                )
            )
            matched.append("rule.p0_active_pause")

        # 规则 2：现金不足 → 拒绝新买入意图（数量相关）
        if context.cashAvailable < context.openOrderQuantity * Decimal("1"):
            actions.append(
                self._candidate(
                    action="REJECT_NEW_ORDERS",
                    scope="account",
                    strength=30,
                    reasonCodes=("INSUFFICIENT_CASH",),
                    evidence=[{"cash_available": context.cashAvailable}],
                )
            )
            matched.append("rule.insufficient_cash")

        # 规则 3：敞口超过权益阈值 → 限制新开仓
        if context.equity > 0 and context.exposure > context.equity * Decimal("2"):
            actions.append(
                self._candidate(
                    action="REDUCE_ONLY",
                    scope="account",
                    strength=30,
                    reasonCodes=("EXPOSURE_LIMIT",),
                    evidence=[{"exposure": context.exposure, "equity": context.equity}],
                )
            )
            matched.append("rule.exposure_limit")

        # 规则 4：SUPPRESSED 预警不产生新动作（已有控制维持）
        if alert.status is AlertStatus.Suppressed:
            matched.append("rule.suppressed_no_action")

        outputHash = self._hash(alert, context, actions)
        return PolicyEvaluationV1(
            policyVersion=self._policyVersion,
            alertId=alert.alertId,
            actions=tuple(actions),
            matchedRules=tuple(matched),
            outputHash=outputHash,
        )

    def _candidate(
        self, action: str, scope: str, strength: int, reasonCodes: tuple[str, ...], evidence: list[dict[str, object]]
    ) -> CandidateActionV1:
        return CandidateActionV1(action, scope, strength, reasonCodes, tuple(evidence))

    def _hash(
        self, alert: AlertEventV1, context: PolicyContextV1, actions: list[CandidateActionV1]
    ) -> str:
        """输出哈希：相同输入必有相同输出哈希。"""
        return canonicalHash(
            {
                "policy_version": self._policyVersion,
                "alert_id": alert.alertId,
                "alert_version": alert.alertVersion,
                "alert_status": alert.status.value,
                "alert_severity": alert.severity.value,
                "cash_available": context.cashAvailable,
                "exposure": context.exposure,
                "equity": context.equity,
                "open_order_quantity": context.openOrderQuantity,
                "actions": [
                    {
                        "action": item.action,
                        "scope": item.scope,
                        "strength": item.strength,
                        "reason_codes": list(item.reasonCodes),
                    }
                    for item in actions
                ],
            }
        )
