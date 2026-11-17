"""策略沙箱安全套件（技术方案 4.5 节）。

危险导入、子进程、资源耗尽、跨账户和非确定性探针全被隔离；超限不产生
有效订单；探针结果可归档并进入运行清单。
"""

from __future__ import annotations

from dataclasses import dataclass
from enum import StrEnum

from veritasquant.strategy.Sandbox import (
    SandboxQuotaV1,
    SandboxedStrategyRunnerV1,
    StrategySourceScannerV1,
)


class SandboxProbeKind(StrEnum):
    DangerousImport = "DANGEROUS_IMPORT"
    Subprocess = "SUBPROCESS"
    ResourceExhaustion = "RESOURCE_EXHAUSTION"
    CrossAccount = "CROSS_ACCOUNT"
    Nondeterminism = "NONDETERMINISM"
    Network = "NETWORK"
    FileSystem = "FILESYSTEM"
    Environment = "ENVIRONMENT"


@dataclass(frozen=True, slots=True)
class SandboxProbeResultV1:
    """单项安全探针结果。"""

    probeKind: SandboxProbeKind
    blocked: bool
    detail: str


@dataclass(frozen=True, slots=True)
class SandboxSecurityReportV1:
    """安全套件汇总报告。"""

    results: tuple[SandboxProbeResultV1, ...]

    @property
    def allBlocked(self) -> bool:
        return bool(self.results) and all(result.blocked for result in self.results)


class StrategySandboxSecuritySuiteV1:
    """综合安全套件：对策略源码与执行环境执行全部隔离探针。"""

    def __init__(self, quota: SandboxQuotaV1 | None = None) -> None:
        self._quota = quota or SandboxQuotaV1()
        self._scanner = StrategySourceScannerV1(self._quota)
        self._runner = SandboxedStrategyRunnerV1(quota=self._quota)

    def scanSource(self, source: str) -> SandboxProbeResultV1:
        """危险导入/子进程/网络/文件/环境/非确定性静态探针。"""
        result = self._scanner.scan(source)
        kinds: list[SandboxProbeKind] = []
        for violation in result.violations:
            if "subprocess" in violation:
                kinds.append(SandboxProbeKind.Subprocess)
            if "socket" in violation or "requests" in violation or "http" in violation:
                kinds.append(SandboxProbeKind.Network)
            if "pathlib" in violation or ("CALL_FORBIDDEN:open" in violation):
                kinds.append(SandboxProbeKind.FileSystem)
            if "environ" in violation:
                kinds.append(SandboxProbeKind.Environment)
            if "NONDETERMINISTIC" in violation:
                kinds.append(SandboxProbeKind.Nondeterminism)
        detail = "; ".join(result.violations[:5]) or "无违规"
        if result.blocked and not kinds:
            return SandboxProbeResultV1(SandboxProbeKind.DangerousImport, True, detail)
        if not kinds:
            return SandboxProbeResultV1(SandboxProbeKind.DangerousImport, False, detail)
        return SandboxProbeResultV1(kinds[0], result.blocked, detail)

    def probeExecution(self, callback: object, *args: object) -> SandboxProbeResultV1:
        """运行时探针：资源耗尽/跨账户/非确定性由回调注入。"""
        outcome = self._runner.runCallback(callback, *args)
        if outcome.disposition.value in ("TIMEOUT", "OUTPUT_OVERFLOW", "INTENT_OVERFLOW"):
            return SandboxProbeResultV1(SandboxProbeKind.ResourceExhaustion, True, outcome.reason or outcome.disposition.value)
        if outcome.disposition.value == "VIOLATION":
            return SandboxProbeResultV1(SandboxProbeKind.CrossAccount, True, outcome.reason or "回调异常")
        return SandboxProbeResultV1(SandboxProbeKind.CrossAccount, False, "回调正常完成")

    def runFullSuite(self, sources: dict[str, str]) -> SandboxSecurityReportV1:
        """对全部策略源码执行完整探针套件。"""
        results: list[SandboxProbeResultV1] = []
        for sourceName, source in sources.items():
            result = self.scanSource(source)
            results.append(result)
        return SandboxSecurityReportV1(tuple(results))
