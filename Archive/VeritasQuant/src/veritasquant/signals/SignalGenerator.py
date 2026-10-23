"""P3-002 近实时信号生成与幂等发布。

- `SignalGeneratorV1`：从策略信号意图（固定输入：方向/数量/价格/策略
  checksum）确定性生成 `SignalReferenceV1`；相同输入产生相同信号内容
  （策略 checksum 一致），不依赖服务器当前时间。
- `SignalPublisherV1`：幂等发布端口 —— 以 (account, strategy, source
  event) 为幂等键；重复事件不重复信号（同键同内容 = 重复投递，返回既有
  信号；同键不同内容 = 冲突，拒绝并记录）。
- `InMemorySignalStoreV1`：内存实现，供模拟盘/测试使用。
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Protocol

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.signals.SignalReference import SignalContractError, SignalReferenceV1, SignalStatus


class SignalGenerationError(ValueError):
    """信号生成或发布不满足契约时抛出。"""


@dataclass(frozen=True, slots=True)
class SignalIntentV1:
    """策略信号意图：生成器的确定性输入。"""

    accountId: str
    strategyId: str
    strategyChecksum: str
    sourceEventId: str
    sourceEventType: str
    direction: str  # BUY / SELL / HOLD
    quantity: str  # Decimal 字符串
    priceLimit: str | None
    availableTs: datetime  # 事件可用时间（即 ts），用于确定信号生成时间

    def __post_init__(self) -> None:
        if not self.accountId or not self.strategyId or not self.sourceEventId:
            raise SignalGenerationError("信号意图标识字段不能为空")
        if len(self.strategyChecksum) != 64:
            raise SignalGenerationError("strategyChecksum 必须为 SHA-256")
        if self.direction not in ("BUY", "SELL", "HOLD"):
            raise SignalGenerationError(f"未知信号方向: {self.direction}")
        if not self.quantity:
            raise SignalGenerationError("信号数量不能为空")


@dataclass(frozen=True, slots=True)
class PublishResultV1:
    """幂等发布结果。"""

    signal: SignalReferenceV1
    duplicate: bool  # True=重复投递（返回既有信号）


@dataclass(frozen=True, slots=True)
class PublishConflictV1:
    """同幂等键不同内容冲突。"""

    idempotencyKey: str
    existingSignalId: str
    incomingHash: str
    existingHash: str


class SignalStore(Protocol):
    """信号持久化端口；实现可为内存或 PostgreSQL。"""

    def get(self, signalReferenceId: str) -> SignalReferenceV1 | None: ...

    def getByIdempotencyKey(self, idempotencyKey: str) -> SignalReferenceV1 | None: ...

    def save(self, signal: SignalReferenceV1) -> None: ...


class SignalGeneratorV1:
    """确定性信号生成器：相同输入 -> 相同信号内容与策略 checksum。"""

    def __init__(self, generatorVersion: str = "V1") -> None:
        if not generatorVersion:
            raise SignalGenerationError("生成器版本不能为空")
        self._version = generatorVersion

    @property
    def version(self) -> str:
        return self._version

    def generate(self, intent: SignalIntentV1, signalReferenceId: str, expirySeconds: int = 900) -> SignalReferenceV1:
        """生成一条 PENDING 信号；generatedTs 取意图的 availableTs（不取服务器时间）。"""
        if expirySeconds <= 0:
            raise SignalGenerationError("信号有效窗口必须为正")
        expiresAt = intent.availableTs.replace(tzinfo=timezone.utc)
        from datetime import timedelta

        expiresAt = expiresAt + timedelta(seconds=expirySeconds)
        return SignalReferenceV1.create(
            signalReferenceId=signalReferenceId,
            version=1,
            status=SignalStatus.Pending,
            accountId=intent.accountId,
            strategyId=intent.strategyId,
            strategyChecksum=intent.strategyChecksum,
            sourceEventId=intent.sourceEventId,
            sourceEventType=intent.sourceEventType,
            direction=intent.direction,
            quantity=intent.quantity,
            priceLimit=intent.priceLimit,
            operatorId=None,
            generatedTs=intent.availableTs,
            expiresAt=expiresAt,
            previousSignalReferenceId=None,
        )

    def contentChecksum(self, intent: SignalIntentV1) -> str:
        """信号内容 checksum：方向/数量/价格/策略/来源确定，与生成时间无关。"""
        return canonicalHash(
            {
                "account_id": intent.accountId,
                "strategy_id": intent.strategyId,
                "strategy_checksum": intent.strategyChecksum,
                "source_event_id": intent.sourceEventId,
                "direction": intent.direction,
                "quantity": intent.quantity,
                "price_limit": intent.priceLimit,
            }
        )


class SignalPublisherV1:
    """幂等信号发布：重复事件不重复信号。"""

    def __init__(self, store: SignalStore, generator: SignalGeneratorV1 | None = None) -> None:
        if store is None:
            raise SignalGenerationError("信号存储不能为空")
        self._store = store
        self._generator = generator or SignalGeneratorV1()
        self._conflicts: dict[str, PublishConflictV1] = {}

    @staticmethod
    def idempotencyKey(intent: SignalIntentV1) -> str:
        """幂等键 = 账户 + 策略 + 来源事件。"""
        return f"{intent.accountId}|{intent.strategyId}|{intent.sourceEventId}"

    def publish(self, intent: SignalIntentV1, signalReferenceId: str, expirySeconds: int = 900) -> PublishResultV1:
        """发布信号；重复投递返回既有信号，冲突抛异常并留档。"""
        key = self.idempotencyKey(intent)
        existing = self._store.getByIdempotencyKey(key)
        if existing is not None:
            incomingHash = self._generator.contentChecksum(intent)
            existingHash = self._storeContentHash(existing)
            if incomingHash == existingHash:
                return PublishResultV1(existing, duplicate=True)
            self._conflicts[key] = PublishConflictV1(
                idempotencyKey=key,
                existingSignalId=existing.signalReferenceId,
                incomingHash=incomingHash,
                existingHash=existingHash,
            )
            raise SignalGenerationError(
                f"幂等冲突：同键不同内容 {key}（既有 {existing.signalReferenceId}）"
            )
        signal = self._generator.generate(intent, signalReferenceId, expirySeconds)
        self._store.save(signal)
        return PublishResultV1(signal, duplicate=False)

    def conflicts(self) -> tuple[PublishConflictV1, ...]:
        return tuple(self._conflicts.values())

    @staticmethod
    def _storeContentHash(signal: SignalReferenceV1) -> str:
        """从已存信号重建内容 checksum（排除生成时间与过期时间）。"""
        return canonicalHash(
            {
                "account_id": signal.accountId,
                "strategy_id": signal.strategyId,
                "strategy_checksum": signal.strategyChecksum,
                "source_event_id": signal.sourceEventId,
                "direction": signal.direction,
                "quantity": signal.quantity,
                "price_limit": signal.priceLimit,
            }
        )


@dataclass(slots=True)
class InMemorySignalStoreV1:
    """内存信号存储（模拟盘/测试）。"""

    _signals: dict[str, SignalReferenceV1] = field(default_factory=dict)
    _byIdempotency: dict[str, str] = field(default_factory=dict)  # key -> signalReferenceId

    def get(self, signalReferenceId: str) -> SignalReferenceV1 | None:
        return self._signals.get(signalReferenceId)

    def getByIdempotencyKey(self, idempotencyKey: str) -> SignalReferenceV1 | None:
        signalId = self._byIdempotency.get(idempotencyKey)
        if signalId is None:
            return None
        return self._signals.get(signalId)

    def save(self, signal: SignalReferenceV1) -> None:
        if signal.signalReferenceId in self._signals:
            raise SignalContractError(f"信号已存在: {signal.signalReferenceId}")
        self._signals[signal.signalReferenceId] = signal
        key = f"{signal.accountId}|{signal.strategyId}|{signal.sourceEventId}"
        self._byIdempotency[key] = signal.signalReferenceId

    def all(self) -> tuple[SignalReferenceV1, ...]:
        return tuple(self._signals.values())
