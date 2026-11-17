"""P3-001 SignalReference、人工审核、人工成交与忽略原因契约。

阶段 3 信号参考闭环的强类型模型：

- `SignalReferenceV1`：一条不可变的信号参考记录。字段固定为 状态、版本、
  账户、策略、来源事件和操作者；信号方向/数量/冻结策略 checksum 在相同
  输入下保持一致（P3-002 生成器契约）。
- `ManualReviewActionV1`：人工审核动作（确认/忽略）登记；每个动作必须携带
  身份、理由、ts、版本和审计字段。
- `ManualExecutionV1`：人工成交登记；必须引用 SignalReference 并携带操作者
  与结构化偏差原因，禁止直接修改内核或账本（P3-004/P3-005 消费）。
- `IgnoreReasonV1`：结构化忽略原因枚举，人工执行偏差必须存在结构化原因。
"""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any

from pydantic import field_validator, model_validator

from veritasquant.core.Models import EventPayloadV1, PascalAlias
from veritasquant.core.Time import TsPrecision, validateUtcTimestamp


class SignalContractError(ValueError):
    """信号参考或人工动作不满足可追溯契约时抛出。"""


class SignalStatus(StrEnum):
    """SignalReference 生命周期状态。"""

    Pending = "PENDING"          # 已生成待人工审核
    Confirmed = "CONFIRMED"      # 人工确认
    Ignored = "IGNORED"          # 人工忽略（必须含结构化原因）
    Executed = "EXECUTED"        # 人工成交登记完成
    Expired = "EXPIRED"          # 超出有效窗口自动失效


class SignalActionType(StrEnum):
    """人工审核动作类型。"""

    Confirm = "CONFIRM"
    Ignore = "IGNORE"
    RegisterExecution = "REGISTER_EXECUTION"


class IgnoreReasonV1(EventPayloadV1):
    """结构化忽略原因：人工执行偏差必须有结构化原因覆盖（P3 策略 gate）。"""

    reasonCode: str = PascalAlias("ReasonCode", min_length=1)
    detail: str = PascalAlias("Detail", default="", max_length=500)
    source: str = PascalAlias("Source", default="manual", min_length=1)

    @model_validator(mode="after")
    def validateReason(self) -> "IgnoreReasonV1":
        if not self.reasonCode.strip():
            raise SignalContractError("忽略原因代码不能为空")
        return self

    @classmethod
    def create(cls, **values: Any) -> "IgnoreReasonV1":
        """从 Python 内部字段创建（自动 alias 转换）。"""
        wireValues: dict[str, Any] = {}
        for fieldName, fieldInfo in cls.model_fields.items():
            alias = str(fieldInfo.validation_alias)
            if fieldName in values:
                wireValues[alias] = values[fieldName]
            elif not fieldInfo.is_required():
                wireValues[alias] = fieldInfo.get_default(call_default_factory=True)
            else:
                wireValues[alias] = None
        return cls.model_validate(wireValues)

    def __hash__(self) -> int:  # frozen model 可用于集合去重
        return hash((self.reasonCode, self.detail, self.source))


class SignalReferenceV1(EventPayloadV1):
    """不可变的信号参考记录（P3-001 契约）。

    固定字段集：状态、版本、账户、策略、来源事件和操作者。字段完整且不可变，
    生成后不允许原地修改；生命周期推进通过派生新的不可变记录实现。
    """

    signalReferenceId: str = PascalAlias("SignalReferenceId", min_length=1)
    version: int = PascalAlias("Version", ge=1)
    status: SignalStatus = PascalAlias("Status")
    accountId: str = PascalAlias("AccountId", min_length=1)
    strategyId: str = PascalAlias("StrategyId", min_length=1)
    strategyChecksum: str = PascalAlias("StrategyChecksum", min_length=64, max_length=64)
    sourceEventId: str = PascalAlias("SourceEventId", min_length=1)
    sourceEventType: str = PascalAlias("SourceEventType", min_length=1)
    direction: str = PascalAlias("Direction", pattern=r"^(BUY|SELL|HOLD)$")
    quantity: str = PascalAlias("Quantity", min_length=1)  # Decimal 字符串，禁止 float
    priceLimit: str | None = PascalAlias("PriceLimit", default=None, min_length=1)
    operatorId: str | None = PascalAlias("OperatorId", default=None, min_length=1)
    generatedTs: datetime = PascalAlias("GeneratedTs")
    expiresAt: datetime | None = PascalAlias("ExpiresAt", default=None)
    previousSignalReferenceId: str | None = PascalAlias("PreviousSignalReferenceId", default=None, min_length=1)

    @field_validator("status", mode="before")
    @classmethod
    def parseStatus(cls, value: object) -> Any:
        if isinstance(value, SignalStatus):
            return value
        if not isinstance(value, str):
            raise SignalContractError("status 必须是受控字符串")
        try:
            return SignalStatus(value)
        except ValueError as error:
            raise SignalContractError(f"未知 SignalStatus: {value}") from error

    @field_validator("generatedTs", "expiresAt", mode="before")
    @classmethod
    def validateTimes(cls, value: object) -> Any:
        if value is None:
            return None
        if not isinstance(value, datetime):
            raise SignalContractError("时间必须是 datetime")
        validateUtcTimestamp(value, TsPrecision.Millisecond)
        return value

    @model_validator(mode="after")
    def validateSignal(self) -> "SignalReferenceV1":
        if self.version == 1 and self.previousSignalReferenceId is not None:
            raise SignalContractError("创建版本不得引用 previousSignalReferenceId")
        if self.version > 1 and self.previousSignalReferenceId is None:
            raise SignalContractError("生命周期更新必须引用 previousSignalReferenceId")
        if self.expiresAt is not None and self.expiresAt < self.generatedTs:
            raise SignalContractError("过期时间不得早于生成时间")
        return self

    @classmethod
    def create(cls, **values: Any) -> "SignalReferenceV1":
        """从 Python 内部字段创建，默认 version=1。"""
        values["version"] = values.get("version", 1)
        values["previousSignalReferenceId"] = values.get("previousSignalReferenceId")
        wireValues: dict[str, Any] = {}
        for fieldName, fieldInfo in cls.model_fields.items():
            alias = str(fieldInfo.validation_alias)
            if fieldName in values:
                wireValues[alias] = values[fieldName]
            elif not fieldInfo.is_required():
                wireValues[alias] = fieldInfo.get_default(call_default_factory=True)
            else:
                wireValues[alias] = None
        return cls.model_validate(wireValues)

    def transition(self, *, status: SignalStatus, operatorId: str, newId: str) -> "SignalReferenceV1":
        """派生下一版本，返回新的不可变记录。"""
        if self.status == status:
            raise SignalContractError(f"状态已是 {status.value}，无需更新")
        return SignalReferenceV1.create(
            signalReferenceId=newId,
            version=self.version + 1,
            status=status,
            accountId=self.accountId,
            strategyId=self.strategyId,
            strategyChecksum=self.strategyChecksum,
            sourceEventId=self.sourceEventId,
            sourceEventType=self.sourceEventType,
            direction=self.direction,
            quantity=self.quantity,
            priceLimit=self.priceLimit,
            operatorId=operatorId,
            generatedTs=self.generatedTs,
            expiresAt=self.expiresAt,
            previousSignalReferenceId=self.signalReferenceId,
        )


class ManualReviewActionV1(EventPayloadV1):
    """人工审核动作登记（P3-004）。

    每个动作有身份、理由、ts、版本和审计；动作不得直接修改内核或账本，
    只能登记待执行意图（P3-005 通过授权命令消费）。
    """

    actionId: str = PascalAlias("ActionId", min_length=1)
    signalReferenceId: str = PascalAlias("SignalReferenceId", min_length=1)
    actionType: SignalActionType = PascalAlias("ActionType")
    operatorId: str = PascalAlias("OperatorId", min_length=1)
    reason: str = PascalAlias("Reason", default="", max_length=500)
    ignoreReason: IgnoreReasonV1 | None = PascalAlias("IgnoreReason", default=None)
    actedAt: datetime = PascalAlias("ActedAt")
    version: int = PascalAlias("Version", ge=1)
    auditTrail: tuple[str, ...] = PascalAlias("AuditTrail", default=())

    @field_validator("actionType", mode="before")
    @classmethod
    def parseActionType(cls, value: object) -> Any:
        if isinstance(value, SignalActionType):
            return value
        if not isinstance(value, str):
            raise SignalContractError("actionType 必须是受控字符串")
        try:
            return SignalActionType(value)
        except ValueError as error:
            raise SignalContractError(f"未知 SignalActionType: {value}") from error

    @field_validator("actedAt", mode="before")
    @classmethod
    def validateActedAt(cls, value: object) -> Any:
        if not isinstance(value, datetime):
            raise SignalContractError("动作时间必须是 datetime")
        validateUtcTimestamp(value, TsPrecision.Millisecond)
        return value

    @model_validator(mode="after")
    def validateAction(self) -> "ManualReviewActionV1":
        if self.actionType is SignalActionType.Ignore and self.ignoreReason is None:
            raise SignalContractError("忽略动作必须提供结构化忽略原因")
        if self.actionType is not SignalActionType.Ignore and self.ignoreReason is not None:
            raise SignalContractError("非忽略动作不得携带忽略原因")
        if not self.reason.strip() and self.actionType is not SignalActionType.Confirm:
            raise SignalContractError("动作必须提供理由")
        return self

    @classmethod
    def create(cls, **values: Any) -> "ManualReviewActionV1":
        """从 Python 内部字段创建（自动 alias 转换）。"""
        wireValues: dict[str, Any] = {}
        for fieldName, fieldInfo in cls.model_fields.items():
            alias = str(fieldInfo.validation_alias)
            if fieldName in values:
                wireValues[alias] = values[fieldName]
            elif not fieldInfo.is_required():
                wireValues[alias] = fieldInfo.get_default(call_default_factory=True)
            else:
                wireValues[alias] = None
        return cls.model_validate(wireValues)


class ManualExecutionV1(EventPayloadV1):
    """人工成交登记（P3-004/P3-005）。

    记录人工实际成交；偏差必须存在结构化原因（P3 策略 gate：人工执行偏差
    结构化原因覆盖率 100%）。禁止直接修改内核或账本——由授权命令写入。
    """

    executionId: str = PascalAlias("ExecutionId", min_length=1)
    signalReferenceId: str = PascalAlias("SignalReferenceId", min_length=1)
    actionId: str = PascalAlias("ActionId", min_length=1)
    operatorId: str = PascalAlias("OperatorId", min_length=1)
    executedAt: datetime = PascalAlias("ExecutedAt")
    direction: str = PascalAlias("Direction", pattern=r"^(BUY|SELL)$")
    quantity: str = PascalAlias("Quantity", min_length=1)  # Decimal 字符串
    price: str = PascalAlias("Price", min_length=1)        # Decimal 字符串
    deviationReason: IgnoreReasonV1 | None = PascalAlias("DeviationReason", default=None)
    note: str = PascalAlias("Note", default="", max_length=500)

    @field_validator("executedAt", mode="before")
    @classmethod
    def validateExecutedAt(cls, value: object) -> Any:
        if not isinstance(value, datetime):
            raise SignalContractError("成交时间必须是 datetime")
        validateUtcTimestamp(value, TsPrecision.Millisecond)
        return value

    @classmethod
    def create(cls, **values: Any) -> "ManualExecutionV1":
        wireValues: dict[str, Any] = {}
        for fieldName, fieldInfo in cls.model_fields.items():
            alias = str(fieldInfo.validation_alias)
            if fieldName in values:
                wireValues[alias] = values[fieldName]
            elif not fieldInfo.is_required():
                wireValues[alias] = fieldInfo.get_default(call_default_factory=True)
            else:
                wireValues[alias] = None
        return cls.model_validate(wireValues)
