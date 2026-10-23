"""控制强度偏序、作用域展开与版本合并（技术方案 8.2 节）。

最严格控制优先；确认、到期或解除只移除该来源贡献，不能覆盖其他活动
控制；乱序旧版本不覆盖新控制；不同作用域先展开到具体账户、策略和标的，
再对每个目标取最大强度。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.risk.RiskEngine import ControlAction, TradingControlEventV1


class ControlMergeError(ValueError):
    """控制合并违反强度偏序或版本契约时抛出。"""


# 控制强度偏序：数值越大越严格（技术方案 8.2 节表）
_CONTROL_STRENGTH: dict[ControlAction, int] = {
    ControlAction.RejectNewOrders: 20,
    ControlAction.ReduceOnly: 30,
    ControlAction.PauseScope: 40,
    ControlAction.StopTrading: 50,
}

# 正交控制参数合并语义：布尔取或、集合取交、数值上限取最小
_BOOLEAN_PARAMS = frozenset({"cancel_active_orders", "notify_escalation"})
_SET_PARAMS = frozenset({"allowed_symbols", "allowed_strategies"})
_MIN_PARAMS = frozenset({"quantity_cap", "exposure_cap"})


@dataclass(frozen=True, slots=True)
class EffectiveControlV1:
    """作用域目标上的合并后生效控制。"""

    scopeTarget: str
    action: ControlAction
    strength: int
    parameters: dict[str, object]
    contributingControlIds: tuple[str, ...]
    controlHash: str


@dataclass(frozen=True, slots=True)
class ScopeExpansionV1:
    """控制作用域的确定性展开结果。"""

    controlId: str
    targets: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class ControlBookV1:
    """活动控制账本：维护每个控制的最新版本与作用域展开。"""

    controls: dict[str, TradingControlEventV1] = field(default_factory=dict)
    _expansions: dict[str, ScopeExpansionV1] = field(default_factory=dict)

    def publish(self, control: TradingControlEventV1) -> None:
        """发布或更新控制；乱序旧版本拒绝，同版本同哈希幂等。"""
        existing = self.controls.get(control.controlId)
        if existing is not None:
            if control.controlVersion < existing.controlVersion:
                raise ControlMergeError("乱序旧版本不覆盖新控制")
            if control.controlVersion == existing.controlVersion:
                if control.controlHash == existing.controlHash:
                    return  # 幂等重放
                raise ControlMergeError("同版本不同哈希，协议冲突")
        self.controls[control.controlId] = control
        self._expansions[control.controlId] = self._expand(control)

    def release(self, controlId: str, newVersion: int) -> TradingControlEventV1:
        """解除控制：只移除该来源贡献，不影响其他控制。"""
        existing = self.controls.get(controlId)
        if existing is None:
            raise ControlMergeError("未知控制 ID")
        if newVersion <= existing.controlVersion:
            raise ControlMergeError("解除版本必须高于现有版本")
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
            controlHash=canonicalHash({"control_id": controlId, "control_version": newVersion, "status": "RELEASED"}),
        )
        self.controls[controlId] = released
        self._expansions.pop(controlId, None)
        return released

    def effectiveFor(self, accountId: str, strategyId: str | None = None, symbol: str | None = None) -> EffectiveControlV1 | None:
        """对作用域目标取所有活动控制的最大强度与合并参数。"""
        contributions = [
            control
            for control in self.controls.values()
            if control.status != "RELEASED" and self._matches(control, accountId, strategyId, symbol)
        ]
        if not contributions:
            return None
        strongest = max(contributions, key=lambda item: _CONTROL_STRENGTH[item.action])
        merged = self._mergeParameters(contributions)
        return EffectiveControlV1(
            scopeTarget=f"{accountId}|{strategyId or '*'}|{symbol or '*'}",
            action=strongest.action,
            strength=_CONTROL_STRENGTH[strongest.action],
            parameters=merged,
            contributingControlIds=tuple(sorted(item.controlId for item in contributions)),
            controlHash=canonicalHash(
                {
                    "scope_target": f"{accountId}|{strategyId or '*'}|{symbol or '*'}",
                    "action": strongest.action.value,
                    "strength": _CONTROL_STRENGTH[strongest.action],
                    "parameters": merged,
                    "contributors": sorted(item.controlId for item in contributions),
                }
            ),
        )

    def expansions(self) -> tuple[ScopeExpansionV1, ...]:
        """全部控制的作用域展开。"""
        return tuple(self._expansions.values())

    def _matches(self, control: TradingControlEventV1, accountId: str, strategyId: str | None, symbol: str | None) -> bool:
        """控制作用域是否覆盖目标（展开后判断）。"""
        expansion = self._expansions.get(control.controlId)
        if expansion is None:
            return False
        targets = set(expansion.targets)
        if f"account:{accountId}" not in targets and "account:*" not in targets:
            return False
        if strategyId is not None and f"strategy:{strategyId}" not in targets and "strategy:*" not in targets:
            return False
        if symbol is not None and f"symbol:{symbol}" not in targets and "symbol:*" not in targets:
            return False
        return True

    def _expand(self, control: TradingControlEventV1) -> ScopeExpansionV1:
        """按控制作用域参数展开为具体目标集合。"""
        scope = control.scope
        targets: list[str] = []
        if scope == "account":
            accounts = control.parameters.get("account_ids")
            if isinstance(accounts, (list, tuple)):
                targets.extend(f"account:{item}" for item in accounts)
            else:
                targets.append("account:*")
        elif scope == "strategy":
            strategies = control.parameters.get("strategy_ids")
            if isinstance(strategies, (list, tuple)):
                targets.extend(f"strategy:{item}" for item in strategies)
            else:
                targets.append("strategy:*")
        elif scope == "symbol":
            symbols = control.parameters.get("symbols")
            if isinstance(symbols, (list, tuple)):
                targets.extend(f"symbol:{item}" for item in symbols)
            else:
                targets.append("symbol:*")
        elif scope == "global":
            targets.extend(("account:*", "strategy:*", "symbol:*"))
        else:
            raise ControlMergeError(f"未知控制作用域: {scope}")
        if not targets:
            raise ControlMergeError("控制作用域展开结果为空")
        return ScopeExpansionV1(control.controlId, tuple(sorted(set(targets))))

    def _mergeParameters(self, controls: list[TradingControlEventV1]) -> dict[str, object]:
        """正交参数合并：布尔取或、集合取交、数值取最小。"""
        merged: dict[str, object] = {}
        booleans: dict[str, bool] = {}
        sets: dict[str, set[object]] = {}
        minimums: dict[str, Decimal] = {}
        for control in controls:
            for key, value in control.parameters.items():
                if key in _BOOLEAN_PARAMS:
                    booleans[key] = booleans.get(key, False) or bool(value)
                elif key in _SET_PARAMS:
                    sets.setdefault(key, set()).update(value if isinstance(value, (list, tuple, set)) else (value,))
                elif key in _MIN_PARAMS:
                    if isinstance(value, Decimal):
                        minimums[key] = min(minimums.get(key, value), value)
                else:
                    merged[key] = value
        merged.update(booleans)
        merged.update({key: tuple(sorted(item.value if hasattr(item, "value") else str(item)) for item in values) for key, values in sets.items()})
        merged.update(minimums)
        return merged


def controlStrength(action: ControlAction) -> int:
    """返回控制的强度偏序值。"""
    return _CONTROL_STRENGTH[action]
