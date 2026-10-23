"""AlertCorrelator：去重、抑制、升级与生命周期管理（技术方案第 9 章）。

同一 (alert_id, alert_version) 内容哈希相同视为重复投递；版本更低时保留
审计不更新投影；版本缺口进入等待队列并请求重放；同版本不同哈希属于协议
冲突。RESOLVED/EXPIRED 是终态，后续同类风险必须创建新 alert_id。
"""

from __future__ import annotations

from dataclasses import dataclass

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.risk.AlertModels import (
    AlertEventV1,
    AlertSeverity,
    AlertStatus,
)


class CorrelationError(ValueError):
    """预警关联违反去重、缺口或生命周期契约时抛出。"""


@dataclass(frozen=True, slots=True)
class CorrelationResultV1:
    """单条预警事件的处理结果。"""

    alertId: str
    alertVersion: int
    disposition: str
    message: str = ""


@dataclass(frozen=True, slots=True)
class LifecycleStateV1:
    """预警生命周期的当前投影状态。"""

    alertId: str
    alertVersion: int
    status: AlertStatus
    severity: AlertSeverity
    dedupeKey: str
    suppressionKey: str | None = None

    @property
    def isTerminal(self) -> bool:
        return self.status in (AlertStatus.Resolved, AlertStatus.Expired)


class AlertCorrelatorV1:
    """维护预警生命周期投影并处理重复、缺口与协议冲突。"""

    def __init__(self) -> None:
        self._lifecycle: dict[str, LifecycleStateV1] = {}
        self._eventHashes: dict[str, str] = {}
        self._pendingGaps: dict[str, dict[int, AlertEventV1]] = {}
        self._gapPaused: set[str] = set()
        self._audit: list[CorrelationResultV1] = []

    @property
    def audit(self) -> tuple[CorrelationResultV1, ...]:
        return tuple(self._audit)

    def process(self, event: AlertEventV1) -> CorrelationResultV1:
        """处理一条预警事件；重复/缺口/冲突各归其位。"""
        eventKey = f"{event.alertId}:{event.alertVersion}"
        eventHash = self._eventHash(event)

        if eventKey in self._eventHashes:
            if self._eventHashes[eventKey] != eventHash:
                return self._record("CONFLICT", event, "同版本不同哈希，协议冲突")
            return self._record("DUPLICATE", event, "重复投递返回已提交结果")

        current = self._lifecycle.get(event.alertId)
        if current is not None and event.alertVersion < current.alertVersion:
            return self._record("STALE_VERSION", event, "低版本只保留审计，不更新投影")
        if current is not None and event.alertVersion > current.alertVersion + 1:
            self._pendingGaps.setdefault(event.alertId, {})[event.alertVersion] = event
            self._gapPaused.add(event.alertId)
            return self._record("GAP_PAUSED", event, f"版本缺口: 期望 {current.alertVersion + 1}")
        if current is not None and current.isTerminal and event.alertVersion > current.alertVersion:
            # 终态后同类风险必须创建新的 alert_id；同一 alertId 不允许复活
            return self._record("TERMINAL_REVIVAL_REJECTED", event, "终态预警不得复活")

        self._apply(event)
        self._eventHashes[eventKey] = eventHash
        return self._record("APPLIED", event, "已应用")

    def applyVerifiedSnapshot(self, alertId: str, verifiedVersion: int) -> CorrelationResultV1 | None:
        """权威重放核验后补齐版本缺口。"""
        pending = self._pendingGaps.get(alertId, {})
        current = self._lifecycle.get(alertId)
        floor = current.alertVersion if current is not None else 0
        if verifiedVersion < floor:
            raise CorrelationError("权威快照版本不得低于已应用版本")
        for version in sorted(pending):
            if version <= verifiedVersion:
                self._apply(pending.pop(version))
        if not pending:
            self._gapPaused.discard(alertId)
            return CorrelationResultV1(alertId, verifiedVersion, "GAP_FILLED", "缺口已由权威快照补齐")
        return None

    def lifecycle(self, alertId: str) -> LifecycleStateV1:
        """返回预警生命周期投影。"""
        state = self._lifecycle.get(alertId)
        if state is None:
            raise CorrelationError("未知预警 ID")
        return state

    def isGapPaused(self, alertId: str) -> bool:
        return alertId in self._gapPaused

    def pendingGapCount(self, alertId: str) -> int:
        return len(self._pendingGaps.get(alertId, {}))

    def _apply(self, event: AlertEventV1) -> None:
        """应用预警事件到生命周期投影。"""
        suppressionKey = self._suppressionKey(event) if event.status is AlertStatus.Suppressed else None
        self._lifecycle[event.alertId] = LifecycleStateV1(
            alertId=event.alertId,
            alertVersion=event.alertVersion,
            status=event.status,
            severity=event.severity,
            dedupeKey=event.dedupeKey,
            suppressionKey=suppressionKey,
        )

    def _suppressionKey(self, event: AlertEventV1) -> str:
        return canonicalHash(
            {
                "alert_type": event.alertType,
                "dedupe_key": event.dedupeKey,
            }
        )

    def _eventHash(self, event: AlertEventV1) -> str:
        return canonicalHash(
            {
                "alert_id": event.alertId,
                "alert_version": event.alertVersion,
                "status": event.status.value,
                "severity": event.severity.value,
                "dedupe_key": event.dedupeKey,
            }
        )

    def _record(self, disposition: str, event: AlertEventV1, message: str) -> CorrelationResultV1:
        result = CorrelationResultV1(event.alertId, event.alertVersion, disposition, message)
        self._audit.append(result)
        return result
