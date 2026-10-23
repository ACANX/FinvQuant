"""基础风控规则：资金、数量、集中度、保证金和陈旧数据（技术方案 8.2 节）。

每条规则有版本、原因码、快照引用和边界测试；硬限制不能配置放宽。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.Orders import OrderIntentV1


class RiskRuleError(ValueError):
    """规则配置或输入违反硬限制契约时抛出。"""


@dataclass(frozen=True, slots=True)
class RuleResultV1:
    """单条规则检查结果。"""

    ruleId: str
    ruleVersion: str
    passed: bool
    reasonCode: str | None
    message: str
    snapshotReference: str

    @property
    def blocked(self) -> bool:
        return not self.passed


@dataclass(frozen=True, slots=True)
class CashRuleConfigV1:
    """资金规则：硬限制不能配置放宽。"""

    ruleId: str = "rule.cash_sufficient"
    ruleVersion: str = "V1"
    minCashReserve: Decimal = Decimal("0")

    def configHash(self) -> str:
        return canonicalHash({"rule_id": self.ruleId, "rule_version": self.ruleVersion, "min_cash_reserve": self.minCashReserve})


@dataclass(frozen=True, slots=True)
class QuantityRuleConfigV1:
    """数量规则：单订单上限与最小手数。"""

    ruleId: str = "rule.quantity_limit"
    ruleVersion: str = "V1"
    maxOrderQuantity: Decimal = Decimal("1000000")
    lotSize: Decimal = Decimal("100")

    def configHash(self) -> str:
        return canonicalHash(
            {"rule_id": self.ruleId, "rule_version": self.ruleVersion, "max_order_quantity": self.maxOrderQuantity, "lot_size": self.lotSize}
        )


@dataclass(frozen=True, slots=True)
class ConcentrationRuleConfigV1:
    """集中度规则：单标的持仓占比上限（硬限制）。"""

    ruleId: str = "rule.concentration_limit"
    ruleVersion: str = "V1"
    maxSingleSymbolExposure: Decimal = Decimal("0.25")

    def configHash(self) -> str:
        return canonicalHash(
            {"rule_id": self.ruleId, "rule_version": self.ruleVersion, "max_single_symbol_exposure": self.maxSingleSymbolExposure}
        )


@dataclass(frozen=True, slots=True)
class MarginRuleConfigV1:
    """保证金规则：维持保证金率与占用上限。"""

    ruleId: str = "rule.margin_sufficient"
    ruleVersion: str = "V1"
    maintenanceMarginRate: Decimal = Decimal("0.10")
    maxMarginUtilization: Decimal = Decimal("0.90")

    def configHash(self) -> str:
        return canonicalHash(
            {
                "rule_id": self.ruleId,
                "rule_version": self.ruleVersion,
                "maintenance_margin_rate": self.maintenanceMarginRate,
                "max_margin_utilization": self.maxMarginUtilization,
            }
        )


@dataclass(frozen=True, slots=True)
class StalenessRuleConfigV1:
    """陈旧数据规则：行情最大允许延迟。"""

    ruleId: str = "rule.data_freshness"
    ruleVersion: str = "V1"
    maxStaleness: timedelta = timedelta(minutes=5)

    def configHash(self) -> str:
        return canonicalHash(
            {
                "rule_id": self.ruleId,
                "rule_version": self.ruleVersion,
                "max_staleness_seconds": int(self.maxStaleness.total_seconds()),
            }
        )


class RiskRuleEngineV1:
    """基础规则引擎：纯函数检查，每条规则带版本与快照引用。"""

    def __init__(
        self,
        *,
        cashConfig: CashRuleConfigV1 | None = None,
        quantityConfig: QuantityRuleConfigV1 | None = None,
        concentrationConfig: ConcentrationRuleConfigV1 | None = None,
        marginConfig: MarginRuleConfigV1 | None = None,
        stalenessConfig: StalenessRuleConfigV1 | None = None,
    ) -> None:
        self._cash = cashConfig or CashRuleConfigV1()
        self._quantity = quantityConfig or QuantityRuleConfigV1()
        self._concentration = concentrationConfig or ConcentrationRuleConfigV1()
        self._margin = marginConfig or MarginRuleConfigV1()
        self._staleness = stalenessConfig or StalenessRuleConfigV1()

    def checkCash(self, intent: OrderIntentV1, cashAvailable: Decimal, snapshotRef: str) -> RuleResultV1:
        if cashAvailable < self._cash.minCashReserve + intent.quantity:
            return self._fail(self._cash.ruleId, self._cash.ruleVersion, "INSUFFICIENT_CASH", "可用资金不足以支持订单", snapshotRef)
        return self._pass(self._cash.ruleId, self._cash.ruleVersion, snapshotRef)

    def checkQuantity(self, intent: OrderIntentV1, snapshotRef: str) -> RuleResultV1:
        if intent.quantity > self._quantity.maxOrderQuantity:
            return self._fail(self._quantity.ruleId, self._quantity.ruleVersion, "QUANTITY_LIMIT", "单订单数量超过硬限制", snapshotRef)
        if intent.quantity % self._quantity.lotSize != 0:
            return self._fail(self._quantity.ruleId, self._quantity.ruleVersion, "LOT_SIZE", "数量不符合最小手数", snapshotRef)
        return self._pass(self._quantity.ruleId, self._quantity.ruleVersion, snapshotRef)

    def checkConcentration(
        self, intent: OrderIntentV1, symbolExposure: Decimal, equity: Decimal, snapshotRef: str
    ) -> RuleResultV1:
        if equity <= 0:
            return self._fail(self._concentration.ruleId, self._concentration.ruleVersion, "ZERO_EQUITY", "权益非正无法评估集中度", snapshotRef)
        projected = (symbolExposure + intent.quantity) / equity
        if projected > self._concentration.maxSingleSymbolExposure:
            return self._fail(
                self._concentration.ruleId,
                self._concentration.ruleVersion,
                "CONCENTRATION_LIMIT",
                f"单标的敞口 {projected} 超过上限 {self._concentration.maxSingleSymbolExposure}",
                snapshotRef,
            )
        return self._pass(self._concentration.ruleId, self._concentration.ruleVersion, snapshotRef)

    def checkMargin(self, marginUsed: Decimal, equity: Decimal, snapshotRef: str) -> RuleResultV1:
        if equity <= 0:
            return self._fail(self._margin.ruleId, self._margin.ruleVersion, "ZERO_EQUITY", "权益非正无法评估保证金", snapshotRef)
        utilization = marginUsed / equity
        if utilization > self._margin.maxMarginUtilization:
            return self._fail(
                self._margin.ruleId,
                self._margin.ruleVersion,
                "MARGIN_LIMIT",
                f"保证金占用 {utilization} 超过上限 {self._margin.maxMarginUtilization}",
                snapshotRef,
            )
        return self._pass(self._margin.ruleId, self._margin.ruleVersion, snapshotRef)

    def checkStaleness(self, now: datetime, lastBarAt: datetime, snapshotRef: str) -> RuleResultV1:
        if now - lastBarAt > self._staleness.maxStaleness:
            return self._fail(
                self._staleness.ruleId,
                self._staleness.ruleVersion,
                "STALE_DATA",
                f"行情延迟 {now - lastBarAt} 超过上限 {self._staleness.maxStaleness}",
                snapshotRef,
            )
        return self._pass(self._staleness.ruleId, self._staleness.ruleVersion, snapshotRef)

    def checkAll(
        self,
        intent: OrderIntentV1,
        *,
        cashAvailable: Decimal,
        symbolExposure: Decimal,
        equity: Decimal,
        marginUsed: Decimal,
        now: datetime,
        lastBarAt: datetime,
        snapshotRef: str,
    ) -> tuple[RuleResultV1, ...]:
        """批量执行全部规则；任一硬限制失败即阻断。"""
        return (
            self.checkCash(intent, cashAvailable, snapshotRef),
            self.checkQuantity(intent, snapshotRef),
            self.checkConcentration(intent, symbolExposure, equity, snapshotRef),
            self.checkMargin(marginUsed, equity, snapshotRef),
            self.checkStaleness(now, lastBarAt, snapshotRef),
        )

    def _pass(self, ruleId: str, ruleVersion: str, snapshotRef: str) -> RuleResultV1:
        return RuleResultV1(ruleId, ruleVersion, True, None, "通过", snapshotRef)

    def _fail(self, ruleId: str, ruleVersion: str, reasonCode: str, message: str, snapshotRef: str) -> RuleResultV1:
        return RuleResultV1(ruleId, ruleVersion, False, reasonCode, message, snapshotRef)
