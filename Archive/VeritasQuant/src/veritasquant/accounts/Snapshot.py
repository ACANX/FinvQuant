"""账户快照、版本控制和只读组合汇总契约。

快照携带账本上界（lastLedgerSequence）、账户版本和内容哈希；
版本存储拒绝旧版本覆盖；组合汇总为纯只读聚合，禁止跨账户调拨。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal

from veritasquant.accounts.Ledger import (
    LedgerBalanceV1,
    LedgerProjectionSnapshotV1,
    LedgerProjectionStoreV1,
)
from veritasquant.core.CanonicalJson import canonicalHash


class AccountSnapshotError(ValueError):
    """账户快照版本、上界或内容哈希不满足契约。"""


@dataclass(frozen=True, slots=True)
class AccountSnapshotV1:
    """某账户在账本上界处的版本化只读状态快照。"""

    accountId: str
    snapshotVersion: int
    lastLedgerSequence: int
    snapshotTs: datetime
    balances: tuple[LedgerBalanceV1, ...]
    contentHash: str

    def balanceFor(
        self,
        ledgerAccount: object,
        unitId: str,
        bookCurrency: str,
    ) -> LedgerBalanceV1:
        """返回精确维度余额；缺失维度按零余额返回。"""
        return next(
            (
                item
                for item in self.balances
                if item.ledgerAccount is ledgerAccount
                and item.unitId == unitId
                and item.bookCurrency == bookCurrency
            ),
            LedgerBalanceV1(ledgerAccount, unitId, bookCurrency, Decimal("0"), Decimal("0")),  # type: ignore[arg-type]
        )


class AccountSnapshotStoreV1:
    """按账户保存最新快照；版本必须严格递增，旧版本写入被拒绝。"""

    def __init__(self) -> None:
        self._snapshots: dict[str, AccountSnapshotV1] = {}

    def saveSnapshot(self, snapshot: AccountSnapshotV1) -> AccountSnapshotV1:
        """写入快照；版本必须大于当前版本，相同快照的幂等重放被接受。"""
        _validateSnapshot(snapshot)
        current = self._snapshots.get(snapshot.accountId)
        if current is not None:
            if snapshot.snapshotVersion < current.snapshotVersion:
                raise AccountSnapshotError("旧版本快照写入被拒绝")
            if snapshot.snapshotVersion == current.snapshotVersion:
                if snapshot.contentHash != current.contentHash:
                    raise AccountSnapshotError("同版本快照内容冲突，禁止覆盖")
                return current
        self._snapshots[snapshot.accountId] = snapshot
        return snapshot

    def snapshotFor(self, accountId: str) -> AccountSnapshotV1:
        """返回最新快照；未知账户抛出契约错误。"""
        if not accountId:
            raise AccountSnapshotError("账户 ID 不能为空")
        snapshot = self._snapshots.get(accountId)
        if snapshot is None:
            raise AccountSnapshotError("账户尚无快照")
        return snapshot

    def currentVersion(self, accountId: str) -> int:
        """返回账户当前快照版本；未知账户返回零。"""
        snapshot = self._snapshots.get(accountId)
        return snapshot.snapshotVersion if snapshot is not None else 0

    @property
    def accounts(self) -> tuple[str, ...]:
        """返回已登记快照的账户列表。"""
        return tuple(sorted(self._snapshots))


class AccountSnapshotBuilderV1:
    """从账本投影构建携带账本上界与内容哈希的账户快照。"""

    def __init__(self, projectionStore: LedgerProjectionStoreV1) -> None:
        self._projectionStore = projectionStore

    def build(
        self,
        accountId: str,
        snapshotVersion: int,
        snapshotTs: datetime,
    ) -> AccountSnapshotV1:
        """基于账本当前上界重建投影并封装为版本化快照。"""
        if not isinstance(snapshotVersion, int) or snapshotVersion < 1:
            raise AccountSnapshotError("快照版本必须为正整数")
        if snapshotTs.tzinfo is None or snapshotTs.utcoffset() != timezone.utc.utcoffset(snapshotTs):
            raise AccountSnapshotError("快照时间必须为 UTC 时区时间")
        projection = self._projectionStore.rebuild(accountId)
        contentHash = _snapshotHash(accountId, snapshotVersion, projection)
        return AccountSnapshotV1(
            accountId=accountId,
            snapshotVersion=snapshotVersion,
            lastLedgerSequence=projection.lastLedgerSequence,
            snapshotTs=snapshotTs,
            balances=projection.balances,
            contentHash=contentHash,
        )


@dataclass(frozen=True, slots=True)
class PortfolioSummaryRowV1:
    """组合汇总单行：账户、币种、现金净额与资产净值。"""

    accountId: str
    currency: str
    cashNet: Decimal
    assetValue: Decimal


@dataclass(frozen=True, slots=True)
class PortfolioSummaryV1:
    """跨账户只读组合汇总；不提供任何资金调拨或写入方法。"""

    summaryId: str
    generatedAt: datetime
    rows: tuple[PortfolioSummaryRowV1, ...]
    summaryHash: str

    def rowFor(self, accountId: str, currency: str) -> PortfolioSummaryRowV1:
        """返回指定账户与币种的汇总行；缺失维度按零返回。"""
        return next(
            (
                item
                for item in self.rows
                if item.accountId == accountId and item.currency == currency
            ),
            PortfolioSummaryRowV1(accountId, currency, Decimal("0"), Decimal("0")),
        )

    @property
    def totalCashNet(self) -> Decimal:
        """全部账户现金净额合计（只读聚合，不改变任何账户状态）。"""
        return sum((row.cashNet for row in self.rows), Decimal("0"))

    @property
    def totalAssetValue(self) -> Decimal:
        """全部账户资产净值合计（只读聚合）。"""
        return sum((row.assetValue for row in self.rows), Decimal("0"))


class PortfolioSummaryServiceV1:
    """纯只读组合汇总：只读取快照，不预占、不调拨、不产生副作用。"""

    def __init__(self, snapshotStore: AccountSnapshotStoreV1) -> None:
        self._snapshotStore = snapshotStore

    def summarize(
        self,
        summaryId: str,
        accountIds: tuple[str, ...],
        generatedAt: datetime,
    ) -> PortfolioSummaryV1:
        """聚合指定账户最新快照；未知账户或非法输入直接拒绝。"""
        if not summaryId or not accountIds:
            raise AccountSnapshotError("汇总必须包含 ID 和非空账户列表")
        if generatedAt.tzinfo is None or generatedAt.utcoffset() != timezone.utc.utcoffset(generatedAt):
            raise AccountSnapshotError("汇总时间必须为 UTC 时区时间")
        rows: list[PortfolioSummaryRowV1] = []
        for accountId in accountIds:
            snapshot = self._snapshotStore.snapshotFor(accountId)
            cashByCurrency: dict[str, Decimal] = {}
            assetValueByCurrency: dict[str, Decimal] = {}
            for balance in snapshot.balances:
                currency = balance.bookCurrency
                account = balance.ledgerAccount.value
                # 现金净额只含真实现金与保证金科目；EXTERNAL_CAPITAL 是权益来源，
                # 不参与净额也不计入净值，避免借贷双方互相抵消。
                if account in {
                    "CASH_AVAILABLE",
                    "CASH_FROZEN",
                    "CASH_RECEIVABLE",
                    "MARGIN_AVAILABLE",
                    "MARGIN_FROZEN",
                }:
                    cashByCurrency[currency] = cashByCurrency.get(currency, Decimal("0")) + balance.bookAmount
                elif account == "CASH_PAYABLE":
                    cashByCurrency[currency] = cashByCurrency.get(currency, Decimal("0")) - balance.bookAmount
                elif account != "EXTERNAL_CAPITAL":
                    assetValueByCurrency[currency] = (
                        assetValueByCurrency.get(currency, Decimal("0")) + balance.bookAmount
                    )
            for currency in sorted(set(cashByCurrency) | set(assetValueByCurrency)):
                rows.append(
                    PortfolioSummaryRowV1(
                        accountId=accountId,
                        currency=currency,
                        cashNet=cashByCurrency.get(currency, Decimal("0")),
                        assetValue=assetValueByCurrency.get(currency, Decimal("0")),
                    )
                )
        ordered = tuple(sorted(rows, key=lambda row: (row.accountId, row.currency)))
        summaryHash = canonicalHash(
            [
                {
                    "account_id": row.accountId,
                    "currency": row.currency,
                    "cash_net": row.cashNet,
                    "asset_value": row.assetValue,
                }
                for row in ordered
            ]
        )
        return PortfolioSummaryV1(summaryId, generatedAt, ordered, summaryHash)


def _snapshotHash(accountId: str, snapshotVersion: int, projection: LedgerProjectionSnapshotV1) -> str:
    """快照身份 = 账户 + 版本 + 账本上界 + 投影哈希，任何变动都会改变哈希。"""
    return canonicalHash(
        {
            "account_id": accountId,
            "snapshot_version": snapshotVersion,
            "last_ledger_sequence": projection.lastLedgerSequence,
            "projection_hash": projection.projectionHash,
        }
    )


def _validateSnapshot(snapshot: AccountSnapshotV1) -> None:
    if not snapshot.accountId:
        raise AccountSnapshotError("账户 ID 不能为空")
    if not isinstance(snapshot.snapshotVersion, int) or snapshot.snapshotVersion < 1:
        raise AccountSnapshotError("快照版本必须为正整数")
    if not isinstance(snapshot.lastLedgerSequence, int) or snapshot.lastLedgerSequence < 0:
        raise AccountSnapshotError("账本上界必须为非负整数")
    if snapshot.snapshotTs.tzinfo is None or snapshot.snapshotTs.utcoffset() != timezone.utc.utcoffset(snapshot.snapshotTs):
        raise AccountSnapshotError("快照时间必须为 UTC 时区时间")
    if not snapshot.contentHash or len(snapshot.contentHash) != 64:
        raise AccountSnapshotError("快照必须携带 64 位十六进制内容哈希")
