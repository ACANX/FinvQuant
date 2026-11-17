"""P2-005 AccountRiskSnapshot 屏障与组合只读评估。

跨账户组组合风险只读取不可变 `AccountRiskSnapshot`（TechSpec 3.3）：

- 每份快照包含 `account_group_id`、`account_id`、共享 `barrier_event_id`、
  逻辑 `ts`、账本序号、订单版本、控制版本和内容哈希；
- 组合评估器只有收齐目标集合在同一 `barrier_event_id` 的快照后才能形成
  `PortfolioSnapshotSet` 并发布组合风险请求；
- 缺失、版本不一致或超时不得用新旧快照拼接，应维持上一条更严格控制
  并禁止相关范围新增风险（返回 None 即维持更严格状态）；
- 组合评估器只读，不直接改账户。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime

from veritasquant.core.CanonicalJson import canonicalHash


class PortfolioSnapshotError(ValueError):
    """快照版本、屏障或内容哈希不满足契约。"""


@dataclass(frozen=True, slots=True)
class AccountRiskSnapshotV1:
    """某账户在共享屏障事件处的不可变风险快照。"""

    accountGroupId: str
    accountId: str
    barrierEventId: str
    logicalTs: datetime
    ledgerSequence: int
    orderVersion: int
    controlVersion: int

    @property
    def contentHash(self) -> str:
        """内容哈希：屏障与全部版本字段，用于一致性校验。"""
        content = {
            "AccountGroupId": self.accountGroupId,
            "AccountId": self.accountId,
            "BarrierEventId": self.barrierEventId,
            "LogicalTs": self.logicalTs,
            "LedgerSequence": self.ledgerSequence,
            "OrderVersion": self.orderVersion,
            "ControlVersion": self.controlVersion,
        }
        return canonicalHash(content)

    def matchesBarrier(self, barrierEventId: str) -> bool:
        return self.barrierEventId == barrierEventId


@dataclass(frozen=True, slots=True)
class PortfolioSnapshotSetV1:
    """同一屏障下收齐的全部目标账户快照（不可变）。"""

    barrierEventId: str
    snapshots: tuple[AccountRiskSnapshotV1, ...]

    def __post_init__(self) -> None:
        if not self.snapshots:
            raise PortfolioSnapshotError("组合快照集不能为空")
        if len({snapshot.accountId for snapshot in self.snapshots}) != len(self.snapshots):
            raise PortfolioSnapshotError("同一账户不得重复出现在组合快照集")
        if any(not snapshot.matchesBarrier(self.barrierEventId) for snapshot in self.snapshots):
            raise PortfolioSnapshotError("组合快照集内存在不同屏障事件")

    def snapshotFor(self, accountId: str) -> AccountRiskSnapshotV1:
        for snapshot in self.snapshots:
            if snapshot.accountId == accountId:
                return snapshot
        raise PortfolioSnapshotError(f"组合快照集缺少账户 {accountId}")


class PortfolioSnapshotRegistryV1:
    """按账户登记最新不可变快照；只有收齐同一屏障才允许组装。"""

    def __init__(self) -> None:
        self._snapshots: dict[str, AccountRiskSnapshotV1] = {}

    def register(self, snapshot: AccountRiskSnapshotV1) -> None:
        """登记快照；同一账户同一屏障的重复登记被接受，旧屏障快照被替换。"""
        if not snapshot.accountId or not snapshot.barrierEventId:
            raise PortfolioSnapshotError("快照必须包含账户与屏障事件")
        current = self._snapshots.get(snapshot.accountId)
        if current is not None and current.barrierEventId == snapshot.barrierEventId:
            if current.contentHash != snapshot.contentHash:
                raise PortfolioSnapshotError("同一账户同一屏障的快照内容冲突")
            return
        self._snapshots[snapshot.accountId] = snapshot

    def snapshotFor(self, accountId: str) -> AccountRiskSnapshotV1 | None:
        return self._snapshots.get(accountId)

    def tryAssemble(
        self,
        targetAccounts: tuple[str, ...],
        barrierEventId: str,
    ) -> PortfolioSnapshotSetV1 | None:
        """收齐同一屏障快照才返回集合；缺失/不同步返回 None（维持更严格控制）。"""
        if not targetAccounts:
            raise PortfolioSnapshotError("目标账户集合不能为空")
        collected: list[AccountRiskSnapshotV1] = []
        for accountId in targetAccounts:
            snapshot = self._snapshots.get(accountId)
            if snapshot is None or not snapshot.matchesBarrier(barrierEventId):
                return None
            collected.append(snapshot)
        return PortfolioSnapshotSetV1(barrierEventId, tuple(collected))

    def clear(self) -> None:
        """清除全部登记（屏障推进时由调用方决定）。"""
        self._snapshots.clear()
