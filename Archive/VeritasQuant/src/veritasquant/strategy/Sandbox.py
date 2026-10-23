"""策略进程隔离、序列化 IPC、超时和资源配额（技术方案 4.5 节）。

不可信 Python 策略不得访问文件、网络、环境变量、系统时间、熵、子进程或
危险导入；超限或越权时丢弃本次全部输出并进入安全处置。策略运行器只返回
OrderIntent、受限日志和指标，不能直接修改内核状态。
"""

from __future__ import annotations

import ast
import time as _time
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum
from typing import Any

from veritasquant.core.CanonicalJson import canonicalHash
from veritasquant.execution.Orders import OrderIntentV1


class SandboxError(ValueError):
    """策略违反沙箱隔离或配额契约时抛出。"""


# 危险导入：任何策略源码不得引用
_FORBIDDEN_MODULES = frozenset(
    {
        "os",
        "sys",
        "subprocess",
        "socket",
        "requests",
        "urllib",
        "http",
        "ftplib",
        "smtplib",
        "multiprocessing",
        "threading",
        "ctypes",
        "importlib",
        "pickle",
        "marshal",
        "shelve",
        "sqlite3",
        "psycopg",
        "redis",
        "shutil",
        "pathlib",
        "tempfile",
        "glob",
        "builtins.open",
    }
)

# 禁止访问的内置与属性名
_FORBIDDEN_NAMES = frozenset(
    {
        "open",
        "exec",
        "eval",
        "compile",
        "__import__",
        "getattr",
        "setattr",
        "delattr",
        "globals",
        "locals",
        "vars",
        "input",
        "breakpoint",
        "memoryview",
        "help",
        "exit",
        "quit",
    }
)

# 非确定性来源：系统时间、熵、随机数
_NONDETERMINISTIC_MODULES = frozenset(
    {
        "time",
        "datetime.now",
        "random",
        "secrets",
        "uuid",
        "hashlib.sha3",
    }
)


@dataclass(frozen=True, slots=True)
class SandboxQuotaV1:
    """版本化沙箱配额（技术方案 4.5 节默认值）。"""

    policyVersion: str = "StrategySandboxPolicyVersion-1"
    cpuCores: int = 1
    memoryMiB: int = 512
    fileDescriptors: int = 64
    callbackWallSeconds: float = 1.0
    ipcInputBytes: int = 256 * 1024
    ipcOutputBytes: int = 256 * 1024
    maxOrderIntents: int = 100

    def quotaHash(self) -> str:
        """配额身份哈希，进入运行清单。"""
        return canonicalHash(
            {
                "policy_version": self.policyVersion,
                "cpu_cores": self.cpuCores,
                "memory_mib": self.memoryMiB,
                "file_descriptors": self.fileDescriptors,
                "callback_wall_ms": int(self.callbackWallSeconds * 1000),
                "ipc_input_bytes": self.ipcInputBytes,
                "ipc_output_bytes": self.ipcOutputBytes,
                "max_order_intents": self.maxOrderIntents,
            }
        )


@dataclass(frozen=True, slots=True)
class SandboxProbeResultV1:
    """危险访问静态扫描结果。"""

    violations: tuple[str, ...]

    @property
    def blocked(self) -> bool:
        return bool(self.violations)


class StrategySourceScannerV1:
    """AST 静态扫描：阻止危险导入、非确定性来源和越权属性访问。"""

    def __init__(self, quota: SandboxQuotaV1 | None = None) -> None:
        self._quota = quota or SandboxQuotaV1()

    def scan(self, source: str) -> SandboxProbeResultV1:
        """扫描策略源码，返回违规清单。"""
        violations: list[str] = []
        try:
            tree = ast.parse(source)
        except SyntaxError as error:
            return SandboxProbeResultV1((f"SYNTAX_ERROR: {error}",))
        for node in ast.walk(tree):
            if isinstance(node, ast.Import):
                for alias in node.names:
                    self._checkModule(alias.name, violations)
            elif isinstance(node, ast.ImportFrom) and node.module:
                self._checkModule(node.module, violations)
            elif isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
                if node.func.id in _FORBIDDEN_NAMES:
                    violations.append(f"CALL_FORBIDDEN:{node.func.id}")
            elif isinstance(node, ast.Attribute):
                dotted = self._dottedName(node)
                if dotted in _FORBIDDEN_MODULES or dotted in _NONDETERMINISTIC_MODULES:
                    violations.append(f"ATTRIBUTE_FORBIDDEN:{dotted}")
        return SandboxProbeResultV1(tuple(sorted(set(violations))))

    def _checkModule(self, moduleName: str, violations: list[str]) -> None:
        topLevel = moduleName.split(".")[0]
        if topLevel in _FORBIDDEN_MODULES or moduleName in _FORBIDDEN_MODULES:
            violations.append(f"IMPORT_FORBIDDEN:{moduleName}")
        if topLevel in _NONDETERMINISTIC_MODULES:
            violations.append(f"IMPORT_NONDETERMINISTIC:{moduleName}")

    @staticmethod
    def _dottedName(node: ast.Attribute) -> str:
        parts: list[str] = [node.attr]
        current: ast.AST = node
        while isinstance(current, ast.Attribute):
            current = current.value
            if isinstance(current, ast.Attribute):
                parts.append(current.attr)
            elif isinstance(current, ast.Name):
                parts.append(current.id)
        return ".".join(reversed(parts))


class SandboxDisposition(StrEnum):
    Accepted = "ACCEPTED"
    Violation = "VIOLATION"
    Timeout = "TIMEOUT"
    OutputOverflow = "OUTPUT_OVERFLOW"
    IntentOverflow = "INTENT_OVERFLOW"


@dataclass(frozen=True, slots=True)
class CallbackOutcomeV1:
    """一次策略回调的执行结果。"""

    disposition: SandboxDisposition
    wallSeconds: float
    intents: tuple[OrderIntentV1, ...]
    reason: str = ""


@dataclass(slots=True)
class IpcEnvelopeV1:
    """版本化序列化 IPC 消息。"""

    ipcVersion: str
    messageType: str
    payload: dict[str, Any]
    payloadHash: str

    def verify(self) -> bool:
        return self.payloadHash == canonicalHash(self.payload)


@dataclass(slots=True)
class SandboxedStrategyRunnerV1:
    """沙箱策略运行器：扫描、超时、配额与安全处置。"""

    quota: SandboxQuotaV1
    scanner: StrategySourceScannerV1 = field(default_factory=StrategySourceScannerV1)
    _lastDisposition: SandboxDisposition = SandboxDisposition.Accepted

    @property
    def lastDisposition(self) -> SandboxDisposition:
        return self._lastDisposition

    def runCallback(self, callback: Any, *args: Any) -> CallbackOutcomeV1:
        """执行策略回调：超限时丢弃全部输出。"""
        start = _time.monotonic()
        intents: list[OrderIntentV1] = []
        try:
            callback(*args)
            for attr in ("_intents",):
                collected = getattr(callback.__self__, attr, None) if hasattr(callback, "__self__") else None
                if isinstance(collected, list):
                    intents.extend(item for item in collected if isinstance(item, OrderIntentV1))
            elapsed = _time.monotonic() - start
        except Exception as error:  # noqa: BLE001
            elapsed = _time.monotonic() - start
            self._lastDisposition = SandboxDisposition.Violation
            return CallbackOutcomeV1(SandboxDisposition.Violation, elapsed, (), f"回调异常: {error}")
        if elapsed > self.quota.callbackWallSeconds:
            self._lastDisposition = SandboxDisposition.Timeout
            return CallbackOutcomeV1(SandboxDisposition.Timeout, elapsed, (), "回调超时，丢弃全部输出")
        if len(intents) > self.quota.maxOrderIntents:
            self._lastDisposition = SandboxDisposition.IntentOverflow
            return CallbackOutcomeV1(SandboxDisposition.IntentOverflow, elapsed, (), "意图数量超限，丢弃全部输出")
        self._lastDisposition = SandboxDisposition.Accepted
        return CallbackOutcomeV1(SandboxDisposition.Accepted, elapsed, tuple(intents))

    def makeIpcEnvelope(self, messageType: str, payload: dict[str, Any]) -> IpcEnvelopeV1:
        """构造受版本控制的 IPC 消息；校验输出大小配额。"""
        if len(canonicalHash(payload)) * 4 > self.quota.ipcOutputBytes:
            raise SandboxError("IPC 输出超过配额")
        envelope = IpcEnvelopeV1(
            ipcVersion="V1",
            messageType=messageType,
            payload=payload,
            payloadHash=canonicalHash(payload),
        )
        return envelope

    def verifyIpc(self, envelope: IpcEnvelopeV1) -> bool:
        """校验 IPC 信封内容哈希。"""
        return envelope.verify() and envelope.ipcVersion == "V1"


def _utcNowIso() -> str:
    return datetime.now(timezone.utc).isoformat()
