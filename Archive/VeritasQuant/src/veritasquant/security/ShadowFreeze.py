"""P5-017 影子运行账户、策略、额度和验收政策冻结。

对齐 TechSpec 13 阶段 5 与 ISSUE #213 验收标准：
- 冻结影子运行账户、策略、额度（资金/订单上限）和验收政策（StrategyAcceptancePolicy）；
- 版本与上限经双人签署（非作者评审/双人授权），冻结后不可变；
- 观察前不得修改阈值解释结果：冻结后任何阈值/额度变更需重新冻结并留痕；
- 初始资金与订单上限不超过批准值（冻结额度即批准值）。

- `ShadowFreezeKind`：冻结对象类型（账户/策略/额度/验收政策）；
- `ShadowFreezeEntryV1`：单项冻结条目（版本 + 上限 + 双人签署）；
- `ShadowFreezeRecordV1`：冻结清单（哈希链 + 状态）；
- `ShadowFreezeServiceV1`：冻结编排（双人签署校验、不可变、变更需重冻结）。
"""

from __future__ import annotations

import hashlib
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum
from decimal import Decimal

from veritasquant.core.CanonicalJson import canonicalHash


def _utcNowMillisecond() -> datetime:
    """UTC 当前时间，截断到毫秒（对齐 TsPrecision.Millisecond）。"""
    now = datetime.now(timezone.utc)
    return now.replace(microsecond=(now.microsecond // 1_000) * 1_000)


class ShadowFreezeKind(StrEnum):
    Account = "ACCOUNT"  # 影子运行账户
    Strategy = "STRATEGY"  # 冻结策略版本
    Limit = "LIMIT"  # 资金/订单上限额度
    AcceptancePolicy = "ACCEPTANCE_POLICY"  # StrategyAcceptancePolicy 验收政策


class ShadowFreezeStatus(StrEnum):
    Frozen = "FROZEN"
    Superseded = "SUPERSEDED"  # 被新冻结取代（历史保留）


@dataclass(frozen=True, slots=True)
class ShadowFreezeEntryV1:
    """单项冻结条目：对象 + 版本 + 上限 + 双人签署。"""

    kind: ShadowFreezeKind
    objectId: str  # 账户 ID / 策略 ID / 额度名 / 政策版本名
    version: str  # 冻结版本标识
    capValue: Decimal  # 上限值（金额/订单数/阈值；字符串语义防 float）
    capUnit: str  # 上限单位（CNY/USD/ORDER/PCT）
    signedBy: tuple[str, ...]  # 双人签署（至少两人，互不相同）
    signedAt: datetime = field(default_factory=_utcNowMillisecond)

    def __post_init__(self) -> None:
        if not self.objectId or not self.version or not self.capUnit:
            raise ValueError("冻结条目 objectId/version/capUnit 不能为空")
        if self.capValue < 0:
            raise ValueError("上限值不得为负")
        if len(self.signedBy) < 2:
            raise ValueError("冻结必须经至少两名签署人（双人签署）")
        if len(set(self.signedBy)) != len(self.signedBy):
            raise ValueError("签署人必须互不相同（禁止同一人双签）")

    def entryHash(self) -> str:
        payload = {
            "kind": self.kind.value,
            "object_id": self.objectId,
            "version": self.version,
            "cap_value": str(self.capValue),
            "cap_unit": self.capUnit,
            "signed_by": list(self.signedBy),
            "signed_at": self.signedAt.isoformat(),
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()


@dataclass(frozen=True, slots=True)
class ShadowFreezeRecordV1:
    """冻结记录：条目集合 + 清单哈希 + 状态。"""

    recordId: str
    entries: tuple[ShadowFreezeEntryV1, ...]
    frozenAt: datetime
    frozenBy: str  # 提交人（非签署人之一亦可，签署人以 entries 为准）
    recordHash: str
    status: ShadowFreezeStatus = ShadowFreezeStatus.Frozen

    def computeHash(self) -> str:
        payload = {
            "record_id": self.recordId,
            "entries": [e.entryHash() for e in self.entries],
            "frozen_at": self.frozenAt.isoformat(),
            "frozen_by": self.frozenBy,
        }
        return hashlib.sha256(canonicalHash(payload).encode("utf-8")).hexdigest()

    def verify(self) -> bool:
        """校验清单哈希未被篡改。"""
        return self.computeHash() == self.recordHash


class ShadowFreezeServiceV1:
    """影子运行冻结编排：双人签署、不可变、观察前阈值不可修改。"""

    def __init__(self) -> None:
        self._records: dict[str, ShadowFreezeRecordV1] = {}
        self._counter = 0

    def freeze(
        self,
        *,
        entries: tuple[ShadowFreezeEntryV1, ...],
        frozenBy: str,
        recordId: str | None = None,
    ) -> ShadowFreezeRecordV1:
        """冻结清单：全部条目双人签署 + 关键对象覆盖校验。"""
        if not entries:
            raise ValueError("冻结清单不能为空")
        # 关键对象类型必须全覆盖（账户/策略/额度/验收政策）
        kinds = {e.kind for e in entries}
        missing = set(ShadowFreezeKind) - kinds
        if missing:
            raise ValueError(f"冻结清单缺少关键对象类型: {sorted(k.value for k in missing)}")
        # 每项都必须是双人签署
        for entry in entries:
            if len(entry.signedBy) < 2:
                raise ValueError(f"{entry.objectId} 未完成双人签署")
        if recordId is None:
            self._counter += 1
            recordId = f"shadow-freeze-{self._counter:04d}"
        if recordId in self._records:
            raise ValueError(f"冻结记录已存在: {recordId}")
        record = ShadowFreezeRecordV1(
            recordId=recordId,
            entries=entries,
            frozenAt=_utcNowMillisecond(),
            frozenBy=frozenBy,
            recordHash="",
        )
        record = ShadowFreezeRecordV1(
            recordId=recordId,
            entries=entries,
            frozenAt=record.frozenAt,
            frozenBy=frozenBy,
            recordHash=record.computeHash(),
        )
        self._records[recordId] = record
        return record

    def current(self) -> ShadowFreezeRecordV1 | None:
        """当前生效的冻结（最新 FROZEN 记录）。"""
        frozen = [r for r in self._records.values() if r.status is ShadowFreezeStatus.Frozen]
        if not frozen:
            return None
        return max(frozen, key=lambda r: r.frozenAt)

    def all(self) -> tuple[ShadowFreezeRecordV1, ...]:
        return tuple(self._records.values())

    def get(self, recordId: str) -> ShadowFreezeRecordV1 | None:
        return self._records.get(recordId)

    def supersede(self, recordId: str) -> ShadowFreezeRecordV1:
        """标记旧冻结为 SUPERSEDED（历史保留，审计可追溯）。"""
        record = self._records.get(recordId)
        if record is None:
            raise ValueError(f"冻结记录不存在: {recordId}")
        superseded = ShadowFreezeRecordV1(
            recordId=record.recordId,
            entries=record.entries,
            frozenAt=record.frozenAt,
            frozenBy=record.frozenBy,
            recordHash=record.recordHash,
            status=ShadowFreezeStatus.Superseded,
        )
        self._records[recordId] = superseded
        return superseded

    def capFor(self, kind: ShadowFreezeKind, objectId: str) -> Decimal | None:
        """查询当前生效冻结中的上限值（批准值）。"""
        current = self.current()
        if current is None:
            return None
        for entry in current.entries:
            if entry.kind is kind and entry.objectId == objectId:
                return entry.capValue
        return None

    def verifyIntegrity(self, record: ShadowFreezeRecordV1) -> bool:
        """校验冻结清单哈希未被篡改（接收外部记录对象直接验证）。"""
        return record.verify()

    def thresholdModified(self, record: ShadowFreezeRecordV1) -> bool:
        """校验冻结清单是否被篡改（阈值解释结果不可修改）。"""
        return not record.verify()


def buildShadowFreezeEntries(
    *,
    accountId: str,
    strategyVersion: str,
    strategyChecksum: str,
    initialFundCap: Decimal,
    orderCap: Decimal,
    acceptancePolicyVersion: str,
    signerA: str,
    signerB: str,
) -> tuple[ShadowFreezeEntryV1, ...]:
    """便捷构造：生成账户/策略/额度/验收政策四类冻结条目。

    signerA/signerB 必须为两名不同签署人（双人签署）。
    """
    if signerA == signerB:
        raise ValueError("双人签署必须由两名不同人员完成")
    return (
        ShadowFreezeEntryV1(
            kind=ShadowFreezeKind.Account, objectId=accountId, version="V1",
            capValue=initialFundCap, capUnit="CNY",
            signedBy=(signerA, signerB),
        ),
        ShadowFreezeEntryV1(
            kind=ShadowFreezeKind.Strategy, objectId=strategyChecksum, version=strategyVersion,
            capValue=Decimal("1"), capUnit="STRATEGY",
            signedBy=(signerA, signerB),
        ),
        ShadowFreezeEntryV1(
            kind=ShadowFreezeKind.Limit, objectId=f"{accountId}.order-cap", version="V1",
            capValue=orderCap, capUnit="ORDER",
            signedBy=(signerA, signerB),
        ),
        ShadowFreezeEntryV1(
            kind=ShadowFreezeKind.AcceptancePolicy, objectId=acceptancePolicyVersion, version="V1",
            capValue=Decimal("1"), capUnit="POLICY",
            signedBy=(signerA, signerB),
        ),
    )
