"""P5-011 不可变审计、日志访问和保留策略。

对齐 TechSpec 12.1/12.3 与 13 阶段 5：
- 审计条目不可变：追加后禁止修改/删除，普通用户（非 Auditor/Administrator）无权删改；
- 检索覆盖命令、审批、风险、订单、账本和人工动作六类审计域；
- 保留策略按域配置保留期，过期条目只能被合规归档流程移除并留痕；
- 审计条目带内容哈希与前序哈希链，任何篡改都会破坏链完整性。

- `AuditDomain`：审计域枚举（命令/审批/风险/订单/账本/人工动作）；
- `AuditEntryV1`：不可变审计条目（哈希链 + 内容哈希）；
- `AuditTrailStoreV1`：追加型审计存储（权限校验 + 检索 + 链完整性校验）；
- `AuditRetentionPolicyV1`：按域保留期策略（归档移除留痕）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from enum import StrEnum
from typing import Any

from veritasquant.core.CanonicalJson import canonicalHash

# 审计保留期默认值（与 TechSpec 12.1 一致：审计保留期不得短于领域审计保留期）
DEFAULT_RETENTION_DAYS = 3650  # 10 年
GENESIS_HASH = "0" * 64


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class AuditDomain(StrEnum):
    Command = "COMMAND"  # 命令资源与幂等键
    Approval = "APPROVAL"  # 双人授权/审批
    Risk = "RISK"  # 风险决定与预警
    Order = "ORDER"  # 订单生命周期
    Ledger = "LEDGER"  # 账本分录
    ManualAction = "MANUAL_ACTION"  # 人工动作


class AuditAccessRole(StrEnum):
    Auditor = "AUDITOR"  # 只读审计角色
    Administrator = "ADMINISTRATOR"  # 运维管理员（可执行合规归档）
    Operator = "OPERATOR"  # 普通操作角色（可追加，不可删改）
    Viewer = "VIEWER"  # 普通用户（只读部分检索）


@dataclass(frozen=True, slots=True)
class AuditEntryV1:
    """不可变审计条目。"""

    entryId: str  # 全局唯一
    ts: datetime  # UTC，毫秒精度
    domain: AuditDomain
    actor: str  # 操作主体
    action: str  # 动作描述（如 APPROVE/REJECT/EXECUTE/TRANSFER）
    payloadHash: str  # 关联载荷（命令/事件/分录）内容哈希
    prevHash: str  # 前序条目哈希（首条为 64 个 '0'）
    entryHash: str  # 本条内容哈希（含 prevHash，防篡改链）
    traceId: str = ""  # 关联追踪 ID（可选）
    details: dict[str, Any] = field(default_factory=dict)  # 受控详情（禁止敏感信息）

    def __post_init__(self) -> None:
        if not self.entryId or not self.actor or not self.action:
            raise ValueError("审计条目 entryId/actor/action 不能为空")
        if len(self.payloadHash) != 64:
            raise ValueError("payloadHash 必须为 SHA-256 十六进制（64 字符）")
        if len(self.prevHash) != 64:
            raise ValueError("prevHash 必须为 SHA-256 十六进制（64 字符）")
        if len(self.entryHash) != 64:
            raise ValueError("entryHash 必须为 SHA-256 十六进制（64 字符）")

    def verifyHash(self) -> bool:
        """校验本条哈希与内容一致（防篡改）。"""
        return computeEntryHash(self) == self.entryHash


def computeEntryHash(entry: AuditEntryV1) -> str:
    """计算审计条目内容哈希（含前序链引用）。"""
    payload = {
        "entry_id": entry.entryId,
        "ts": entry.ts.isoformat(),
        "domain": entry.domain.value,
        "actor": entry.actor,
        "action": entry.action,
        "payload_hash": entry.payloadHash,
        "prev_hash": entry.prevHash,
        "trace_id": entry.traceId,
        "details": entry.details,
    }
    return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()


def buildAuditEntry(
    *,
    entryId: str,
    domain: AuditDomain,
    actor: str,
    action: str,
    payloadHash: str,
    prevHash: str = GENESIS_HASH,
    traceId: str = "",
    details: dict[str, Any] | None = None,
    ts: datetime | None = None,
) -> AuditEntryV1:
    """构建审计条目：两段式构造，自动计算内容哈希。"""
    draft = AuditEntryV1(
        entryId=entryId,
        ts=ts if ts is not None else _utcNowMillisecond(),
        domain=domain,
        actor=actor,
        action=action,
        payloadHash=payloadHash,
        prevHash=prevHash,
        entryHash=GENESIS_HASH,  # 占位，用于计算
        traceId=traceId,
        details=dict(details or {}),
    )
    return AuditEntryV1(
        entryId=draft.entryId,
        ts=draft.ts,
        domain=draft.domain,
        actor=draft.actor,
        action=draft.action,
        payloadHash=draft.payloadHash,
        prevHash=draft.prevHash,
        entryHash=computeEntryHash(draft),
        traceId=draft.traceId,
        details=draft.details,
    )


class AuditTrailStoreV1:
    """追加型审计存储：只允许 APPEND；删除/修改被拒绝；检索按域。

    权限模型：
    - 任何角色可追加（操作留痕）；
    - 删除/修改历史条目一律拒绝（普通用户不能删改）；
    - PURGE（合规归档）仅 Administrator 可执行，且归档本身产生审计条目。
    """

    def __init__(self) -> None:
        self._entries: dict[str, AuditEntryV1] = {}
        self._chainHead = GENESIS_HASH
        self._purgeLog: list[AuditEntryV1] = []
        self._counter = 0
        # 合规归档断点：被 PURGE 移除的条目哈希（作为合法前序引用）
        self._archivedHashes: set[str] = set()

    def append(
        self,
        *,
        domain: AuditDomain,
        actor: str,
        action: str,
        payloadHash: str,
        entryId: str | None = None,
        traceId: str = "",
        details: dict[str, Any] | None = None,
    ) -> AuditEntryV1:
        """追加审计条目；链头推进。"""
        if entryId is None:
            self._counter += 1
            entryId = f"audit-{self._counter:08d}"
        if entryId in self._entries:
            raise ValueError(f"审计条目已存在: {entryId}")
        entry = buildAuditEntry(
            entryId=entryId,
            domain=domain,
            actor=actor,
            action=action,
            payloadHash=payloadHash,
            prevHash=self._chainHead,
            traceId=traceId,
            details=details,
        )
        self._entries[entryId] = entry
        self._chainHead = entry.entryHash
        return entry

    def get(self, entryId: str) -> AuditEntryV1 | None:
        return self._entries.get(entryId)

    def delete(self, entryId: str, actor: str = "") -> None:
        """删除/修改一律拒绝：审计不可变。"""
        raise PermissionError("审计条目不可变：禁止删除或修改历史记录")

    def modify(self, entryId: str, actor: str = "") -> None:
        raise PermissionError("审计条目不可变：禁止删除或修改历史记录")

    def search(
        self,
        *,
        domain: AuditDomain | None = None,
        actor: str | None = None,
        action: str | None = None,
        traceId: str | None = None,
        since: datetime | None = None,
        limit: int = 100,
    ) -> tuple[AuditEntryV1, ...]:
        """检索审计：覆盖命令/审批/风险/订单/账本/人工动作六域。"""
        results: list[AuditEntryV1] = []
        for entry in self._entries.values():
            if domain is not None and entry.domain is not domain:
                continue
            if actor is not None and entry.actor != actor:
                continue
            if action is not None and entry.action != action:
                continue
            if traceId is not None and entry.traceId != traceId:
                continue
            if since is not None and entry.ts < since:
                continue
            results.append(entry)
        results.sort(key=lambda e: e.ts)
        return tuple(results[:limit])

    def chainIntegrity(self) -> bool:
        """校验哈希链：每条现存条目哈希自洽，前序引用合法。

        前序引用可以是 genesis、现存条目的 entryHash，或已合规归档
        （PURGE 留痕）的条目哈希；未归档的非法删除/篡改会破坏链。
        """
        known = {e.entryHash for e in self._entries.values()} | self._archivedHashes
        for entry in self._entries.values():
            if not entry.verifyHash():
                return False
            if entry.prevHash != GENESIS_HASH and entry.prevHash not in known:
                return False
        return True

    def entryCount(self) -> int:
        return len(self._entries)

    def chainHead(self) -> str:
        return self._chainHead

    def purge(self, *, domain: AuditDomain, before: datetime, actor: str) -> tuple[int, AuditEntryV1]:
        """合规归档：仅 Administrator；移除过期条目并留痕。

        返回 (移除条数, 归档审计条目)。
        """
        if actor != AuditAccessRole.Administrator.value:
            raise PermissionError("只有 Administrator 可执行审计归档")
        victims = [e for e in self._entries.values() if e.domain is domain and e.ts < before]
        for entry in victims:
            del self._entries[entry.entryId]
            self._archivedHashes.add(entry.entryHash)
        # 归档留痕（本身不可被本次归档删除）
        audit = self.append(
            domain=AuditDomain.Approval,
            actor=actor,
            action="AUDIT_PURGE",
            payloadHash=_sha256(f"{domain.value}:{before.isoformat()}:{len(victims)}"),
            details={"domain": domain.value, "before": before.isoformat(), "removed": len(victims)},
        )
        self._purgeLog.append(audit)
        return len(victims), audit

    def purgeLog(self) -> tuple[AuditEntryV1, ...]:
        return tuple(self._purgeLog)


class AuditRetentionPolicyV1:
    """按审计域配置保留期；默认 10 年。"""

    def __init__(self, retentionDays: dict[AuditDomain, int] | None = None) -> None:
        self._retention: dict[AuditDomain, int] = {}
        for domain in AuditDomain:
            self._retention[domain] = (retentionDays or {}).get(domain, DEFAULT_RETENTION_DAYS)

    def retentionFor(self, domain: AuditDomain) -> timedelta:
        return timedelta(days=self._retention[domain])

    def expired(self, entry: AuditEntryV1, *, now: datetime | None = None) -> bool:
        """条目是否已超过保留期。"""
        now = now if now is not None else _utcNowMillisecond()
        return entry.ts + self.retentionFor(entry.domain) < now

    def purgeEligible(self, store: AuditTrailStoreV1, *, now: datetime | None = None) -> tuple[AuditEntryV1, ...]:
        """返回所有已过保留期的条目（供 Administrator 归档）。"""
        return tuple(e for e in store._entries.values() if self.expired(e, now=now))


def _sha256(value: str) -> str:
    return hashlib.sha256(value.encode("utf-8")).hexdigest()
