"""P2-024 API 服务端口与应用组装。

领域/应用层只定义端口（Protocol）与健康检查结果模型，不依赖
FastAPI；FastAPI 接线全部放在 apps.server，保证领域模块可离线
测试且入口导入无副作用（import 不连接数据库、不启动线程）。
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol


class ApiVersionProvider(Protocol):
    """提供 API 版本信息。"""

    @property
    def apiVersion(self) -> str: ...

    @property
    def catalogVersion(self) -> str: ...


class ReadinessProbe(Protocol):
    """单项 readiness 自检；返回 (通过, 说明)。"""

    def check(self) -> tuple[bool, str]: ...


@dataclass(frozen=True, slots=True)
class ReadinessResultV1:
    """readiness 汇总结果。"""

    ready: bool
    checks: tuple[tuple[str, bool, str], ...]  # (名称, 通过, 说明)


@dataclass(frozen=True, slots=True)
class ApiVersionInfoV1:
    """版本路由返回的固定信息。"""

    apiVersion: str
    catalogVersion: str
    service: str


class HealthService:
    """健康检查用例：liveness 恒真，readiness 汇总全部探针。"""

    def __init__(self, probes: tuple[ReadinessProbe, ...] = ()) -> None:
        self._probes = probes

    def liveness(self) -> bool:
        """liveness 只判断进程能否响应；恒真由路由层返回。"""
        return True

    def readiness(self) -> ReadinessResultV1:
        """readiness 要求全部探针通过。"""
        checks = tuple((probe.__class__.__name__, *probe.check()) for probe in self._probes)
        return ReadinessResultV1(all(passed for _, passed, _ in checks), checks)
