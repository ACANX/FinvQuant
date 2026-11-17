"""P5-009 生产 trading-readiness 门禁。

对齐 TechSpec 12.3/13 阶段 5：
- 行情、时钟、券商、账本、控制、队列、磁盘、沙箱任一不合格即禁止发单。

- `ReadinessCheckName`：八类门禁检查；
- `ReadinessCheckResultV1`：单项结果（PASS/FAIL）；
- `TradingReadinessGateV1`：聚合门禁（任一 FAIL 禁止发单）。
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone
from enum import StrEnum


class ReadinessError(ValueError):
    """门禁检查不满足契约时抛出。"""


class ReadinessCheckName(StrEnum):
    MarketData = "MARKET_DATA"
    Clock = "CLOCK"
    Broker = "BROKER"
    Ledger = "LEDGER"
    Control = "CONTROL"
    Queue = "QUEUE"
    Disk = "DISK"
    Sandbox = "SANDBOX"


class CheckStatus(StrEnum):
    Pass = "PASS"
    Fail = "FAIL"


@dataclass(frozen=True, slots=True)
class ReadinessCheckResultV1:
    """单项门禁检查结果。"""

    checkName: ReadinessCheckName
    status: CheckStatus
    detail: str = ""
    checkedAt: datetime | None = None

    def __post_init__(self) -> None:
        if self.checkedAt is None:
            object.__setattr__(self, "checkedAt", datetime.now(timezone.utc))


@dataclass(frozen=True, slots=True)
class ReadinessVerdictV1:
    """门禁结论。"""

    ready: bool
    results: tuple[ReadinessCheckResultV1, ...]

    @property
    def failedChecks(self) -> tuple[ReadinessCheckResultV1, ...]:
        return tuple(r for r in self.results if r.status is CheckStatus.Fail)


class TradingReadinessGateV1:
    """生产 trading-readiness 门禁：任一不合格即禁止发单。"""

    _REQUIRED_CHECKS: frozenset[ReadinessCheckName] = frozenset(ReadinessCheckName)

    def __init__(self) -> None:
        self._results: dict[ReadinessCheckName, ReadinessCheckResultV1] = {}
        self._verdicts: list[ReadinessVerdictV1] = []

    def record(
        self, checkName: ReadinessCheckName, status: CheckStatus, detail: str = ""
    ) -> None:
        """记录单项检查结果；重复覆盖。"""
        self._results[checkName] = ReadinessCheckResultV1(
            checkName=checkName, status=status, detail=detail
        )

    def evaluate(self) -> ReadinessVerdictV1:
        """评估门禁：全部必检项存在且 PASS 才 ready。"""
        missing = self._REQUIRED_CHECKS - set(self._results)
        results = list(self._results.values())
        if missing:
            for name in sorted(missing, key=lambda n: n.value):
                results.append(
                    ReadinessCheckResultV1(
                        checkName=name,
                        status=CheckStatus.Fail,
                        detail="检查未执行",
                    )
                )
        verdict = ReadinessVerdictV1(
            ready=all(r.status is CheckStatus.Pass for r in results),
            results=tuple(sorted(results, key=lambda r: r.checkName.value)),
        )
        self._verdicts.append(verdict)
        return verdict

    def canSubmitOrder(self) -> bool:
        """发单门禁：最新评估必须 ready。"""
        if not self._verdicts:
            return False
        return self._verdicts[-1].ready

    def verdicts(self) -> tuple[ReadinessVerdictV1, ...]:
        return tuple(self._verdicts)
