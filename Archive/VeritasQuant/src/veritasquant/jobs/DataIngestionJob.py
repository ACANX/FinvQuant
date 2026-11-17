"""数据导入任务 console script 入口。

- 接收 --job-run-id / --job-execution-key（调度幂等契约，TechSpec 11.5）；
- 复用 JobTasks.DataImportTask（P2-035 任务清单，业务幂等由执行键
  哈希 + command_id/inbox/outbox/checkpoint 保证）；
- 退出码：0 成功 / 2 参数无效 / 3 业务失败 / 4 幂等跳过。
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence

from veritasquant.application.JobTasks import runTask
from veritasquant.jobs.JobEntrypoint import JobEntrypoint

_PROG = "vq-job-data-ingestion"


class DataIngestionJob(JobEntrypoint):
    def __init__(self) -> None:
        super().__init__(_PROG, "运行 VeritasQuant 数据导入任务")

    def buildParser(self) -> argparse.ArgumentParser:
        parser = super().buildParser()
        parser.add_argument("--source", default="", help="数据源名称")
        parser.add_argument("--instrument-id", default="", help="目标标的 ID")
        return parser

    def run(self, arguments: argparse.Namespace) -> int:
        result = runTask(
            "DATA_IMPORT",
            arguments.job_run_id,
            arguments.job_execution_key,
            {"source": arguments.source, "instrument_id": arguments.instrument_id},
        )
        print(result.message)
        return result.exitCode


def main(argv: Sequence[str] | None = None) -> int:
    """返回数据导入任务离线参数校验的明确退出码。"""
    return DataIngestionJob().main(argv)
