"""P2-034 任务入口共享基类：job_run_id + job_execution_key 幂等契约。

TechSpec 11.5：任务入口接收 `job_run_id` 和该执行键，校验参数
Schema，实际业务调用继续使用领域 command_id、inbox/outbox 和
checkpoint 保证幂等。任务不得依赖常驻 API 进程内存状态。
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence
from typing import Callable


class JobEntrypoint:
    """可安装任务入口骨架：参数解析 + 执行键校验 + 退出码。"""

    def __init__(
        self,
        prog: str,
        description: str,
        parameterSchemaVersion: str = "1",
    ) -> None:
        self._prog = prog
        self._description = description
        self._parameterSchemaVersion = parameterSchemaVersion

    def buildParser(self) -> argparse.ArgumentParser:
        parser = argparse.ArgumentParser(prog=self._prog, description=self._description)
        parser.add_argument("--job-run-id", required=True, help="调度器分配的 JobRun ID")
        parser.add_argument(
            "--job-execution-key",
            required=True,
            help="稳定执行键 schedule_id:version:scheduled_for（幂等依据）",
        )
        parser.add_argument(
            "--parameter-schema-version",
            default=self._parameterSchemaVersion,
            help="参数 Schema 版本（默认与任务声明一致）",
        )
        return parser

    def run(self, arguments: argparse.Namespace) -> int:
        """执行任务主体；返回退出码。子类必须实现。"""
        raise NotImplementedError

    def main(self, argv: Sequence[str] | None = None) -> int:
        """解析参数并执行；离线校验失败返回非零。"""
        from veritasquant.application.Entrypoints import configureStandardStreams

        configureStandardStreams()
        parser = self.buildParser()
        try:
            arguments = parser.parse_args(argv)
        except SystemExit as error:
            return error.code if isinstance(error.code, int) else 1
        if arguments.parameter_schema_version != self._parameterSchemaVersion:
            print(
                f"参数 Schema 版本不匹配: {arguments.parameter_schema_version} != {self._parameterSchemaVersion}",
                file=__import__("sys").stderr,
            )
            return 2
        return self.run(arguments)


def jobEntrypoint(
    prog: str,
    description: str,
    parameterSchemaVersion: str = "1",
) -> Callable[[Callable[[argparse.Namespace], int]], JobEntrypoint]:
    """装饰器：把 run(arguments) -> int 函数包装为 JobEntrypoint。"""

    def decorator(impl: Callable[[argparse.Namespace], int]) -> JobEntrypoint:
        class _Bound(JobEntrypoint):
            def run(self, arguments: argparse.Namespace) -> int:
                return impl(arguments)

        return _Bound(prog, description, parameterSchemaVersion)

    return decorator
