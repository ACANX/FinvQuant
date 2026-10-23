"""P1-021 质量规则、隔离记录与 dry-run 摘要。

导入提交前执行时间顺序、重复、缺口、OHLC、会话、标的映射与来源质量规则；
失败记录写入隔离 manifest，生成可审阅的 dry-run 摘要，严禁静默跳过或修正。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import StrEnum

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.core.Models import PascalAlias, StrictModel
from veritasquant.core.Time import TsPrecision, serializeUtcTimestamp
from veritasquant.data.MinuteBar import MinuteBarContractError, MinuteBarSchemaV1
from veritasquant.instruments.Registry import InstrumentContractError, InstrumentV1

from pydantic import field_validator


class QualityRuleKind(StrEnum):
    """阶段 1 固定的质量规则目录。"""

    TimeOrder = "TimeOrder"
    Duplicate = "Duplicate"
    Gap = "Gap"
    Ohlc = "Ohlc"
    Session = "Session"
    InstrumentMapping = "InstrumentMapping"
    SourceQuality = "SourceQuality"


class QualityRuleSeverity(StrEnum):
    """规则失败的可处置级别；失败记录不得静默跳过。"""

    Reject = "Reject"
    Isolate = "Isolate"


class QualityCheckError(ValueError):
    """质量检查配置或运行不满足契约。"""


class IsolationRecordV1(StrictModel):
    """单条进入隔离 manifest 的失败记录指纹。"""

    ruleKind: QualityRuleKind = PascalAlias("RuleKind")
    severity: QualityRuleSeverity = PascalAlias("Severity")
    barPrimaryKey: str = PascalAlias("BarPrimaryKey", min_length=1)
    sourceRecordId: str = PascalAlias("SourceRecordId", min_length=1)
    reason: str = PascalAlias("Reason", min_length=1)

    @field_validator("ruleKind", "severity", mode="before")
    @classmethod
    def parseEnums(cls, value: object) -> object:
        if isinstance(value, (QualityRuleKind, QualityRuleSeverity)):
            return value
        if isinstance(value, str):
            try:
                if value in {item.value for item in QualityRuleKind}:
                    return QualityRuleKind(value)
                return QualityRuleSeverity(value)
            except ValueError:
                pass
        raise QualityCheckError(f"未知质量枚举值: {value!r}")


class QualityRuleConfigV1(StrictModel):
    """质量规则的版本化参数；变更必须产生新 QualityRuleVersion。"""

    qualityRuleVersion: str = PascalAlias("QualityRuleVersion", min_length=1)
    maxGapSeconds: int = PascalAlias("MaxGapSeconds", ge=0)
    allowUnknownTurnoverScale: bool = PascalAlias("AllowUnknownTurnoverScale")

    @field_validator("qualityRuleVersion")
    @classmethod
    def validateVersion(cls, value: str) -> str:
        if not value.strip():
            raise QualityCheckError("质量规则版本不得为空")
        return value


@dataclass(frozen=True, slots=True)
class DryRunSummaryV1:
    """提交前可审阅的 dry-run 摘要（哈希与计数，供人工/CI 审阅）。"""

    configHash: str
    inputFileHash: str
    contractHash: str
    acceptedCount: int
    isolatedCount: int
    isolationRecordHash: str
    isolationRecords: tuple[IsolationRecordV1, ...]

    def toDict(self) -> dict[str, object]:
        return {
            "config_hash": self.configHash,
            "input_file_hash": self.inputFileHash,
            "contract_hash": self.contractHash,
            "accepted_count": self.acceptedCount,
            "isolated_count": self.isolatedCount,
            "isolation_record_hash": self.isolationRecordHash,
            "isolation_records": [
                {
                    "rule_kind": record.ruleKind.value,
                    "severity": record.severity.value,
                    "bar_primary_key": record.barPrimaryKey,
                    "source_record_id": record.sourceRecordId,
                    "reason": record.reason,
                }
                for record in self.isolationRecords
            ],
        }


class QualityRuleEngineV1:
    """按固定顺序执行质量规则，失败记录进入隔离集合而非静默修正。"""

    def __init__(
        self,
        config: QualityRuleConfigV1,
        instrument: InstrumentV1,
        tsPrecision: TsPrecision,
        sessionIds: set[str] | None = None,
    ) -> None:
        self._config = config
        self._instrument = instrument
        self._tsPrecision = tsPrecision
        self._sessionIds = sessionIds if sessionIds is not None else set()
        self._seenPrimaryKeys: set[str] = set()
        self._lastBarEnd: datetime | None = None
        self._isolated: list[IsolationRecordV1] = []

    def check(self, bar: MinuteBarSchemaV1, inputFileHash: str | None = None) -> bool:
        """校验单条 Bar；返回是否通过（失败记录自动进入隔离集合）。"""
        failures: list[str] = []
        primaryKey = self._primaryKey(bar)
        if not self._checkInstrumentMapping(bar):
            failures.append(self._record(QualityRuleKind.InstrumentMapping, primaryKey, bar, "标的映射不匹配"))
        if not self._checkOhlc(bar):
            failures.append(self._record(QualityRuleKind.Ohlc, primaryKey, bar, "OHLC 或数量契约失败"))
        if not self._checkTimeOrder(bar):
            failures.append(self._record(QualityRuleKind.TimeOrder, primaryKey, bar, "时间顺序倒退"))
        if not self._checkDuplicate(bar):
            failures.append(self._record(QualityRuleKind.Duplicate, primaryKey, bar, "主键重复"))
        if not self._checkGap(bar):
            failures.append(self._record(QualityRuleKind.Gap, primaryKey, bar, "时间缺口超过配置上限"))
        if not self._checkSession(bar):
            failures.append(self._record(QualityRuleKind.Session, primaryKey, bar, "交易时段不匹配"))
        if not self._checkSourceQuality(bar):
            failures.append(self._record(QualityRuleKind.SourceQuality, primaryKey, bar, "来源质量标志异常"))
        if self._lastBarEnd is None or bar.barEnd > self._lastBarEnd:
            self._lastBarEnd = bar.barEnd
        return not failures

    def dryRun(self, bars: list[MinuteBarSchemaV1], inputFileHash: str) -> DryRunSummaryV1:
        """执行全部规则并生成提交前可审阅摘要（不修改任何持久状态）。

        每次调用从全新状态开始，保证幂等且不依赖调用顺序。
        """
        fresh = self._freshEngine()
        accepted: list[MinuteBarSchemaV1] = []
        for bar in bars:
            if fresh.check(bar, inputFileHash):
                accepted.append(bar)
        isolationHash = canonicalHash(
            [
                record.model_dump(mode="python", by_alias=False, exclude_none=False)
                for record in fresh._isolated
            ],
            self._tsPrecision,
        )
        return DryRunSummaryV1(
            configHash=self._configHash(),
            inputFileHash=inputFileHash,
            contractHash=self._contractHash(),
            acceptedCount=len(accepted),
            isolatedCount=len(fresh._isolated),
            isolationRecordHash=isolationHash,
            isolationRecords=tuple(fresh._isolated),
        )

    def _freshEngine(self) -> "QualityRuleEngineV1":
        """返回共享配置的全新状态引擎。"""
        return QualityRuleEngineV1(
            self._config,
            self._instrument,
            self._tsPrecision,
            sessionIds=self._sessionIds,
        )

    def acceptedBars(self, bars: list[MinuteBarSchemaV1]) -> list[MinuteBarSchemaV1]:
        """返回通过全部规则的 Bar（与 dryRun 的判定完全一致）。"""
        return [bar for bar in bars if self.check(bar)]

    # -- 规则实现（每个规则只读，不修正数据） ----------------------------------

    def _checkInstrumentMapping(self, bar: MinuteBarSchemaV1) -> bool:
        try:
            bar.validateAgainstInstrument(self._instrument, self._tsPrecision)
        except (InstrumentContractError, MinuteBarContractError):
            return False
        return True

    def _checkOhlc(self, bar: MinuteBarSchemaV1) -> bool:
        return bar.low <= bar.open <= bar.high and bar.low <= bar.close <= bar.high

    def _checkTimeOrder(self, bar: MinuteBarSchemaV1) -> bool:
        if self._lastBarEnd is None:
            return True
        return bar.barStart >= self._lastBarEnd

    def _checkDuplicate(self, bar: MinuteBarSchemaV1) -> bool:
        key = self._primaryKey(bar)
        if key in self._seenPrimaryKeys:
            return False
        self._seenPrimaryKeys.add(key)
        return True

    def _checkGap(self, bar: MinuteBarSchemaV1) -> bool:
        if self._lastBarEnd is None:
            return True
        gap = bar.barStart - self._lastBarEnd
        if gap <= timedelta(0):
            return True  # 顺序由 TimeOrder 规则负责
        return gap.total_seconds() <= self._config.maxGapSeconds

    def _checkSession(self, bar: MinuteBarSchemaV1) -> bool:
        if not self._sessionIds:
            return True  # 未注入会话白名单时不启用会话规则
        if bar.sessionId == "source-unverified":
            return False
        return bar.sessionId in self._sessionIds

    def _checkSourceQuality(self, bar: MinuteBarSchemaV1) -> bool:
        if bar.amount is None and not self._config.allowUnknownTurnoverScale:
            return False
        return True

    # -- 辅助 ------------------------------------------------------------------

    def _primaryKey(self, bar: MinuteBarSchemaV1) -> str:
        return "|".join((
            bar.market.value,
            bar.symbol,
            serializeUtcTimestamp(bar.barStart, self._tsPrecision),
            serializeUtcTimestamp(bar.barEnd, self._tsPrecision),
            bar.source,
        ))

    def _record(
        self,
        kind: QualityRuleKind,
        primaryKey: str,
        bar: MinuteBarSchemaV1,
        reason: str,
    ) -> str:
        self._isolated.append(IsolationRecordV1.model_validate({
            "RuleKind": kind.value,
            "Severity": QualityRuleSeverity.Isolate.value,
            "BarPrimaryKey": primaryKey,
            "SourceRecordId": bar.sourceRecordId,
            "Reason": reason,
        }))
        return reason

    def _configHash(self) -> str:
        return canonicalHash(
            self._config.model_dump(mode="python", by_alias=False, exclude_none=False),
            self._tsPrecision,
        )

    def _contractHash(self) -> str:
        return canonicalHash(
            {
                "schema": "MinuteBarSchemaV1",
                "rules": [kind.value for kind in QualityRuleKind],
            },
            self._tsPrecision,
        )
