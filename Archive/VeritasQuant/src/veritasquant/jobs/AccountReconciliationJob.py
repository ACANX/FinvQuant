"""账户对账任务 console script 入口。

- 接收 --job-run-id / --job-execution-key（调度幂等契约）；
- 复用 JobTasks.ReconciliationTask（P2-035 任务清单）；
- 退出码：0 成功 / 2 参数无效 / 3 业务失败 / 4 幂等跳过。
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence

from veritasquant.application.JobTasks import runTask
from veritasquant.jobs.JobEntrypoint import JobEntrypoint

_PROG = "vq-job-account-reconciliation"


class AccountReconciliationJob(JobEntrypoint):
    def __init__(self) -> None:
        super().__init__(_PROG, "运行 VeritasQuant 账户对账任务")

    def buildParser(self) -> argparse.ArgumentParser:
        parser = super().buildParser()
        parser.add_argument("--account-group", default="", help="账户组 ID")
        return parser

    def run(self, arguments: argparse.Namespace) -> int:
        result = runTask(
            "RECONCILIATION",
            arguments.job_run_id,
            arguments.job_execution_key,
            {"account_group": arguments.account_group},
        )
        print(result.message)
        return result.exitCode


def main(argv: Sequence[str] | None = None) -> int:
    """返回账户对账任务离线参数校验的明确退出码。"""
    return AccountReconciliationJob().main(argv)
