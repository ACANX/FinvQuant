"""执行校准任务 console script 入口。

- 接收 --job-run-id / --job-execution-key（调度幂等契约）；
- 复用 JobTasks.ExecutionCalibrationTask（P2-035 任务清单）；
- 退出码：0 成功 / 2 参数无效 / 3 业务失败 / 4 幂等跳过。
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence

from veritasquant.application.JobTasks import runTask
from veritasquant.jobs.JobEntrypoint import JobEntrypoint

_PROG = "vq-job-execution-calibration"


class ExecutionCalibrationJob(JobEntrypoint):
    def __init__(self) -> None:
        super().__init__(_PROG, "运行 VeritasQuant 执行校准任务")

    def buildParser(self) -> argparse.ArgumentParser:
        parser = super().buildParser()
        parser.add_argument("--model-version", default="", help="校准模型版本")
        return parser

    def run(self, arguments: argparse.Namespace) -> int:
        result = runTask(
            "EXECUTION_CALIBRATION",
            arguments.job_run_id,
            arguments.job_execution_key,
            {"model_version": arguments.model_version},
        )
        print(result.message)
        return result.exitCode


def main(argv: Sequence[str] | None = None) -> int:
    """返回执行校准任务离线参数校验的明确退出码。"""
    return ExecutionCalibrationJob().main(argv)
