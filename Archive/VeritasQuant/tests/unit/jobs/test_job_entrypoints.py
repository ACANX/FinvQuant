"""P2-034 任务入口契约测试：执行键、参数 Schema 与幂等语义。"""

from __future__ import annotations

import argparse

from veritasquant.jobs.DataIngestionJob import DataIngestionJob
from veritasquant.jobs.JobEntrypoint import JobEntrypoint


def test_job_requires_execution_key() -> None:
    """缺 --job-run-id/--job-execution-key 返回非零（TechSpec 11.5）。"""
    job = DataIngestionJob()
    assert job.main([]) != 0
    assert job.main(["--job-run-id", "jr-1"]) != 0


def test_job_accepts_execution_key() -> None:
    job = DataIngestionJob()
    assert (
        job.main(
            [
                "--job-run-id", "jr-1",
                "--job-execution-key", "sched-1:1.0.0:2026-08-03T02:00:00Z",
                "--source", "cn-feed",
                "--instrument-id", "510300.SH",
            ]
        )
        == 0
    )


def test_job_rejects_missing_parameters() -> None:
    job = DataIngestionJob()
    assert (
        job.main(
            [
                "--job-run-id", "jr-1",
                "--job-execution-key", "key-1",
            ]
        )
        == 2
    )


def test_job_rejects_wrong_parameter_schema_version() -> None:
    job = DataIngestionJob()
    assert (
        job.main(
            [
                "--job-run-id", "jr-1",
                "--job-execution-key", "key-1",
                "--parameter-schema-version", "999",
                "--source", "cn-feed",
                "--instrument-id", "510300.SH",
            ]
        )
        == 2
    )


def test_job_help_offline() -> None:
    job = DataIngestionJob()
    assert job.main(["--help"]) == 0


def test_custom_job_entrypoint() -> None:
    """自定义任务：run 实现 + 执行键契约自动生效。"""

    class _FakeJob(JobEntrypoint):
        def __init__(self) -> None:
            super().__init__("vq-job-fake", "fake job")
            self.calls: list[str] = []

        def run(self, arguments: argparse.Namespace) -> int:  # noqa: ANN001
            self.calls.append(arguments.job_execution_key)
            return 0

    job = _FakeJob()
    assert (
        job.main(["--job-run-id", "jr-9", "--job-execution-key", "k-9"]) == 0
    )
    assert job.calls == ["k-9"]


def test_job_idempotency_by_execution_key() -> None:
    """同执行键重复调用不重复副作用：由执行键哈希与调度器去重保证。

    此处验证执行键稳定：同一调度输入产生同一执行键哈希（单元层）。
    """
    from veritasquant.application.Scheduling import JobRunV1, JobStatus

    run1 = JobRunV1(
        jobRunId="a", scheduleId="s1", scheduleVersion="v1", jobType="T",
        command="cmd", parameters={}, scheduledForIso="2026-08-03T02:00:00Z",
        status=JobStatus.Scheduled,
    )
    run2 = JobRunV1(
        jobRunId="b", scheduleId="s1", scheduleVersion="v1", jobType="T",
        command="cmd", parameters={}, scheduledForIso="2026-08-03T02:00:00Z",
        status=JobStatus.Scheduled,
    )
    assert run1.jobExecutionKey == run2.jobExecutionKey
    assert run1.executionKeyHash() == run2.executionKeyHash()
