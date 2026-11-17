"""阶段 1 不可变复式账本 journal 与分录契约。"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from decimal import Decimal
from enum import StrEnum
from typing import TypeVar

from pydantic import field_validator, model_validator

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import PascalAlias, StrictModel


class LedgerContractError(ValueError):
    """账本事实不满足逐单位平衡或来源追溯契约。"""


class JournalType(StrEnum):
    """技术方案定义的不可变账本事实类别。"""

    OpeningBalance = "OPENING_BALANCE"
    OrderReservation = "ORDER_RESERVATION"
    OrderRelease = "ORDER_RELEASE"
    Trade = "TRADE"
    TradeSettlement = "TRADE_SETTLEMENT"
    FundSubscription = "FUND_SUBSCRIPTION"
    FundRedemption = "FUND_REDEMPTION"
    FundDistribution = "FUND_DISTRIBUTION"
    Fee = "FEE"
    Tax = "TAX"
    Deposit = "DEPOSIT"
    Withdrawal = "WITHDRAWAL"
    Interest = "INTEREST"
    Dividend = "DIVIDEND"
    CorporateAction = "CORPORATE_ACTION"
    MarkToMarket = "MARK_TO_MARKET"
    Margin = "MARGIN"
    Settlement = "SETTLEMENT"
    Delivery = "DELIVERY"
    FxConversion = "FX_CONVERSION"
    BrokerCorrection = "BROKER_CORRECTION"
    Reversal = "REVERSAL"
    ManualAdjustment = "MANUAL_ADJUSTMENT"


class LedgerAccount(StrEnum):
    """阶段 1 显式科目，禁止用无约束字符串猜测科目。"""

    CashAvailable = "CASH_AVAILABLE"
    CashFrozen = "CASH_FROZEN"
    CashReceivable = "CASH_RECEIVABLE"
    CashPayable = "CASH_PAYABLE"
    SecuritiesAvailable = "SECURITIES_AVAILABLE"
    SecuritiesFrozen = "SECURITIES_FROZEN"
    SecuritiesReceivable = "SECURITIES_RECEIVABLE"
    MarginAvailable = "MARGIN_AVAILABLE"
    MarginFrozen = "MARGIN_FROZEN"
    TradingClearing = "TRADING_CLEARING"
    FeeExpense = "FEE_EXPENSE"
    TaxExpense = "TAX_EXPENSE"
    RoundingResidual = "ROUNDING_RESIDUAL"
    ExternalCapital = "EXTERNAL_CAPITAL"
    RealizedProfitLoss = "REALIZED_PROFIT_LOSS"
    UnrealizedProfitLoss = "UNREALIZED_PROFIT_LOSS"
    DividendIncome = "DIVIDEND_INCOME"


class EntryDirection(StrEnum):
    """复式记账方向。"""

    Debit = "DEBIT"
    Credit = "CREDIT"


class CostBasisMethod(StrEnum):
    """运行开始时固定的成本法。"""

    MovingAverage = "MOVING_AVERAGE"
    Fifo = "FIFO"


class AccountingPolicyV1(StrictModel):
    """运行级会计策略，不允许在同一运行内切换。"""

    policyVersion: str = PascalAlias("PolicyVersion", min_length=1)
    costBasisMethod: CostBasisMethod = PascalAlias("CostBasisMethod")
    monetaryRounding: str = PascalAlias("MonetaryRounding", pattern="^ROUND_HALF_EVEN$")
    policyHash: str = PascalAlias("PolicyHash", pattern=r"^[0-9a-f]{64}$")

    @field_validator("costBasisMethod", mode="before")
    @classmethod
    def parseCostBasisMethod(cls, value: object) -> CostBasisMethod:
        return _parseEnum(value, CostBasisMethod, "成本法")


class AssetUnitV1(StrictModel):
    """用于独立借贷平衡的资产或币种计量单位。"""

    unitId: str = PascalAlias("UnitId", min_length=1)
    assetId: str = PascalAlias("AssetId", min_length=1)
    currency: str | None = PascalAlias("Currency", default=None, pattern=r"^[A-Z]{3}$")

    @model_validator(mode="after")
    def validateUnit(self) -> "AssetUnitV1":
        if self.currency is None and not self.unitId.startswith("INSTRUMENT:"):
            raise LedgerContractError("非币种计量单位必须使用 INSTRUMENT: 前缀")
        if self.currency is not None and self.unitId != self.currency:
            raise LedgerContractError("币种计量单位 ID 必须等于币种代码")
        return self


class LedgerEntryV1(StrictModel):
    """journal 内不可修改的单条借方或贷方分录。"""

    entryId: str = PascalAlias("EntryId", min_length=1)
    ledgerAccount: LedgerAccount = PascalAlias("LedgerAccount")
    direction: EntryDirection = PascalAlias("Direction")
    unit: AssetUnitV1 = PascalAlias("Unit")
    quantity: Decimal = PascalAlias("Quantity", gt=Decimal("0"))
    bookCurrency: str = PascalAlias("BookCurrency", pattern=r"^[A-Z]{3}$")
    bookAmount: Decimal = PascalAlias("BookAmount", ge=Decimal("0"))
    costAmount: Decimal = PascalAlias("CostAmount", ge=Decimal("0"))

    @field_validator("ledgerAccount", mode="before")
    @classmethod
    def parseLedgerAccount(cls, value: object) -> LedgerAccount:
        return _parseEnum(value, LedgerAccount, "账本科目")

    @field_validator("direction", mode="before")
    @classmethod
    def parseDirection(cls, value: object) -> EntryDirection:
        return _parseEnum(value, EntryDirection, "分录方向")


class JournalV1(StrictModel):
    """账户范围内按计量单位严格平衡的不可变复式 journal。"""

    journalId: str = PascalAlias("JournalId", min_length=1)
    journalType: JournalType = PascalAlias("JournalType")
    accountId: str = PascalAlias("AccountId", min_length=1)
    subaccountId: str | None = PascalAlias("SubaccountId", default=None, min_length=1)
    ts: datetime = PascalAlias("Ts")
    commitSequence: int = PascalAlias("CommitSequence", ge=1)
    sourceEventId: str = PascalAlias("SourceEventId", min_length=1)
    reversalOfJournalId: str | None = PascalAlias("ReversalOfJournalId", default=None, min_length=1)
    instrumentMetadataVersion: str = PascalAlias("InstrumentMetadataVersion", min_length=1)
    feeScheduleVersion: str = PascalAlias("FeeScheduleVersion", min_length=1)
    accountingPolicyVersion: str = PascalAlias("AccountingPolicyVersion", min_length=1)
    entries: tuple[LedgerEntryV1, ...] = PascalAlias("Entries", min_length=2)

    @field_validator("journalType", mode="before")
    @classmethod
    def parseJournalType(cls, value: object) -> JournalType:
        return _parseEnum(value, JournalType, "journal 类型")

    @field_validator("ts")
    @classmethod
    def validateUtc(cls, value: datetime) -> datetime:
        if value.tzinfo is None or value.utcoffset() != timezone.utc.utcoffset(value):
            raise LedgerContractError("账本 ts 必须为 UTC 时区时间")
        return value

    @model_validator(mode="after")
    def validateJournal(self) -> "JournalV1":
        if self.journalType is JournalType.Reversal and self.reversalOfJournalId is None:
            raise LedgerContractError("REVERSAL 必须引用原 journal")
        if self.journalType is not JournalType.Reversal and self.reversalOfJournalId is not None:
            raise LedgerContractError("仅 REVERSAL 可以声明原 journal 引用")
        if self.reversalOfJournalId == self.journalId:
            raise LedgerContractError("journal 不得引用自身为冲正对象")
        entryIds = [entry.entryId for entry in self.entries]
        if len(entryIds) != len(set(entryIds)):
            raise LedgerContractError("同一 journal 的 entryId 不得重复")
        balances: dict[str, Decimal] = {}
        bookBalances: dict[str, Decimal] = {}
        for entry in self.entries:
            signedQuantity = entry.quantity if entry.direction is EntryDirection.Debit else -entry.quantity
            balances[entry.unit.unitId] = balances.get(entry.unit.unitId, Decimal("0")) + signedQuantity
            signedBookAmount = entry.bookAmount if entry.direction is EntryDirection.Debit else -entry.bookAmount
            bookBalances[entry.bookCurrency] = bookBalances.get(entry.bookCurrency, Decimal("0")) + signedBookAmount
        unbalancedUnits = [unitId for unitId, balance in balances.items() if balance != 0]
        unbalancedUnits.extend(
            f"BOOK:{currency}" for currency, balance in bookBalances.items() if balance != 0
        )
        if unbalancedUnits:
            raise LedgerContractError(f"journal 按计量单位不平衡: {', '.join(sorted(unbalancedUnits))}")
        return self


class LedgerStoreV1:
    """阶段 1 的只追加账本事实存储；P2 替换为同语义数据库实现。"""

    def __init__(self) -> None:
        self._journals: list[JournalV1] = []
        self._journalIds: set[str] = set()

    @property
    def journals(self) -> tuple[JournalV1, ...]:
        """返回不可修改的 journal 顺序快照。"""
        return tuple(self._journals)

    @property
    def entries(self) -> tuple[LedgerEntryV1, ...]:
        """按提交顺序返回不可修改的分录快照。"""
        return tuple(entry for journal in self._journals for entry in journal.entries)

    def commitJournal(self, journal: JournalV1) -> JournalV1:
        """校验完整 journal 后一次性追加，失败时不改变已有历史。"""
        # model_copy 可绕过 Pydantic 校验，提交边界必须从 wire 形式重新验证。
        validatedJournal = JournalV1.model_validate(journal.model_dump(by_alias=True))
        if validatedJournal.journalId in self._journalIds:
            raise LedgerContractError("journalId 已提交，禁止覆盖或重复记账")
        expectedSequence = len(self._journals) + 1
        if validatedJournal.commitSequence != expectedSequence:
            raise LedgerContractError(f"账本提交序号必须为 {expectedSequence}")
        self._journals.append(validatedJournal)
        self._journalIds.add(validatedJournal.journalId)
        return validatedJournal


@dataclass(frozen=True, slots=True)
class LedgerBalanceV1:
    """某科目、资产单位和记账币种的可重放余额。"""

    ledgerAccount: LedgerAccount
    unitId: str
    bookCurrency: str
    quantity: Decimal
    bookAmount: Decimal


@dataclass(frozen=True, slots=True)
class LedgerProjectionSnapshotV1:
    """账户账本上界对应的只读投影快照。"""

    accountId: str
    lastLedgerSequence: int
    balances: tuple[LedgerBalanceV1, ...]
    projectionHash: str

    def balanceFor(
        self,
        ledgerAccount: LedgerAccount,
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
            LedgerBalanceV1(ledgerAccount, unitId, bookCurrency, Decimal("0"), Decimal("0")),
        )


class LedgerProjectionStoreV1:
    """从只追加 journal 重建现金、冻结、持仓、成本和盈亏投影。"""

    def __init__(self, ledgerStore: LedgerStoreV1) -> None:
        self._ledgerStore = ledgerStore

    def rebuild(self, accountId: str) -> LedgerProjectionSnapshotV1:
        """从空投影顺序应用指定账户的所有已提交 journal。"""
        if not accountId:
            raise LedgerContractError("账户 ID 不能为空")
        values: dict[tuple[LedgerAccount, str, str], tuple[Decimal, Decimal]] = {}
        lastSequence = 0
        for journal in self._ledgerStore.journals:
            if journal.accountId != accountId:
                continue
            lastSequence = journal.commitSequence
            for entry in journal.entries:
                direction = Decimal("1") if entry.direction is EntryDirection.Debit else Decimal("-1")
                key = (entry.ledgerAccount, entry.unit.unitId, entry.bookCurrency)
                quantity, bookAmount = values.get(key, (Decimal("0"), Decimal("0")))
                values[key] = (quantity + direction * entry.quantity, bookAmount + direction * entry.bookAmount)
        balances = tuple(
            LedgerBalanceV1(ledgerAccount, unitId, bookCurrency, quantity, bookAmount)
            for (ledgerAccount, unitId, bookCurrency), (quantity, bookAmount) in sorted(
                values.items(), key=lambda item: (item[0][0].value, item[0][1], item[0][2])
            )
        )
        projectionHash = canonicalHash(
            [
                {
                    "ledger_account": item.ledgerAccount.value,
                    "unit_id": item.unitId,
                    "book_currency": item.bookCurrency,
                    "quantity": item.quantity,
                    "book_amount": item.bookAmount,
                }
                for item in balances
            ]
        )
        return LedgerProjectionSnapshotV1(accountId, lastSequence, balances, projectionHash)


class CashJournalFactoryV1:
    """生成资金流、费用税款和冲正的严格平衡 journal。"""

    def __init__(
        self,
        instrumentMetadataVersion: str,
        feeScheduleVersion: str,
        accountingPolicyVersion: str,
    ) -> None:
        if not all((instrumentMetadataVersion, feeScheduleVersion, accountingPolicyVersion)):
            raise LedgerContractError("journal 工厂必须绑定全部版本")
        self._instrumentMetadataVersion = instrumentMetadataVersion
        self._feeScheduleVersion = feeScheduleVersion
        self._accountingPolicyVersion = accountingPolicyVersion

    def createOpeningBalance(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal
    ) -> JournalV1:
        return self._createCashTransfer(
            journalId, JournalType.OpeningBalance, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.CashAvailable, EntryDirection.Debit, LedgerAccount.ExternalCapital, EntryDirection.Credit,
        )

    def createDeposit(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal
    ) -> JournalV1:
        return self._createCashTransfer(
            journalId, JournalType.Deposit, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.CashAvailable, EntryDirection.Debit, LedgerAccount.ExternalCapital, EntryDirection.Credit,
        )

    def createWithdrawal(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal,
        availableCash: Decimal,
    ) -> JournalV1:
        if amount > availableCash:
            raise LedgerContractError("出金金额不得超过可用资金")
        return self._createCashTransfer(
            journalId, JournalType.Withdrawal, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.ExternalCapital, EntryDirection.Debit, LedgerAccount.CashAvailable, EntryDirection.Credit,
        )

    def createFee(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal
    ) -> JournalV1:
        return self._createCashTransfer(
            journalId, JournalType.Fee, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.FeeExpense, EntryDirection.Debit, LedgerAccount.CashAvailable, EntryDirection.Credit,
        )

    def createTax(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal
    ) -> JournalV1:
        return self._createCashTransfer(
            journalId, JournalType.Tax, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.TaxExpense, EntryDirection.Debit, LedgerAccount.CashAvailable, EntryDirection.Credit,
        )

    def createDividend(
        self, journalId: str, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str, currency: str, amount: Decimal
    ) -> JournalV1:
        return self._createCashTransfer(
            journalId, JournalType.Dividend, accountId, ts, commitSequence, sourceEventId, currency, amount,
            LedgerAccount.CashAvailable, EntryDirection.Debit, LedgerAccount.DividendIncome, EntryDirection.Credit,
        )

    def createReversal(self, journalId: str, sourceEventId: str, original: JournalV1, commitSequence: int) -> JournalV1:
        entries = tuple(
            LedgerEntryV1(  # type: ignore[call-arg]
                EntryId=f"{journalId}:{index}",
                LedgerAccount=entry.ledgerAccount,
                Direction=EntryDirection.Credit if entry.direction is EntryDirection.Debit else EntryDirection.Debit,
                Unit=entry.unit,
                Quantity=entry.quantity,
                BookCurrency=entry.bookCurrency,
                BookAmount=entry.bookAmount,
                CostAmount=entry.costAmount,
            )
            for index, entry in enumerate(original.entries, start=1)
        )
        return JournalV1(  # type: ignore[call-arg]
            JournalId=journalId,
            JournalType=JournalType.Reversal,
            AccountId=original.accountId,
            SubaccountId=original.subaccountId,
            Ts=original.ts,
            CommitSequence=commitSequence,
            SourceEventId=sourceEventId,
            ReversalOfJournalId=original.journalId,
            InstrumentMetadataVersion=original.instrumentMetadataVersion,
            FeeScheduleVersion=original.feeScheduleVersion,
            AccountingPolicyVersion=original.accountingPolicyVersion,
            Entries=entries,
        )

    def _createCashTransfer(
        self, journalId: str, journalType: JournalType, accountId: str, ts: datetime, commitSequence: int, sourceEventId: str,
        currency: str, amount: Decimal, debitAccount: LedgerAccount, debitDirection: EntryDirection,
        creditAccount: LedgerAccount, creditDirection: EntryDirection,
    ) -> JournalV1:
        unit = AssetUnitV1(UnitId=currency, AssetId=currency, Currency=currency)  # type: ignore[call-arg]
        entries = (
            LedgerEntryV1(EntryId=f"{journalId}:1", LedgerAccount=debitAccount, Direction=debitDirection, Unit=unit, Quantity=amount, BookCurrency=currency, BookAmount=amount, CostAmount=Decimal("0")),  # type: ignore[call-arg]
            LedgerEntryV1(EntryId=f"{journalId}:2", LedgerAccount=creditAccount, Direction=creditDirection, Unit=unit, Quantity=amount, BookCurrency=currency, BookAmount=amount, CostAmount=Decimal("0")),  # type: ignore[call-arg]
        )
        return JournalV1(  # type: ignore[call-arg]
            JournalId=journalId, JournalType=journalType, AccountId=accountId, Ts=ts, CommitSequence=commitSequence,
            SourceEventId=sourceEventId, InstrumentMetadataVersion=self._instrumentMetadataVersion,
            FeeScheduleVersion=self._feeScheduleVersion, AccountingPolicyVersion=self._accountingPolicyVersion, Entries=entries,
        )


EnumType = TypeVar("EnumType", bound=StrEnum)


def _parseEnum(value: object, enumType: type[EnumType], label: str) -> EnumType:
    """仅接受模型枚举或协议中完全匹配的字符串。"""
    if isinstance(value, enumType):
        return value
    if not isinstance(value, str):
        raise LedgerContractError(f"{label}必须是受控字符串")
    try:
        return enumType(value)
    except ValueError as error:
        raise LedgerContractError(f"未知{label}: {value}") from error
