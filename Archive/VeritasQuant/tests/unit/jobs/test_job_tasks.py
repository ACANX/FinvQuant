"""P2-035 任务清单测试：四类任务执行、幂等、退出码与清单契约。"""

from __future__ import annotations

from pathlib import Path

import yaml

from veritasquant.application.JobTasks import (
    DataImportTask,
    ExecutionCalibrationTask,
    InMemoryTaskRecorder,
    ReconciliationTask,
    ReportGenerationTask,
    runTask,
)
from veritasquant.jobs.AccountReconciliationJob import AccountReconciliationJob
from veritasquant.jobs.DataIngestionJob import DataIngestionJob
from veritasquant.jobs.ExecutionCalibrationJob import ExecutionCalibrationJob
from veritasquant.jobs.ReportGenerationJob import ReportGenerationJob


class TestDataImportTask:
    def test_success(self) -> None:
        result = DataImportTask().run("jr-1", "k-1", {"source": "cn-feed", "instrument_id": "510300.SH"})
        assert result.ok
        assert result.checkpointReference == "ckpt:data_import:jr-1"

    def test_missing_parameters(self) -> None:
        result = DataImportTask().run("jr-1", "k-1", {})
        assert result.exitCode == 2


class TestReconciliationTask:
    def test_success_zero_differences(self) -> None:
        result = ReconciliationTask().run("jr-1", "k-1", {"account_group": "ag-1"})
        assert result.ok
        assert result.metrics["differences"] == 0

    def test_missing_group(self) -> None:
        result = ReconciliationTask().run("jr-1", "k-1", {})
        assert result.exitCode == 2


class TestExecutionCalibrationTask:
    def test_success(self) -> None:
        result = ExecutionCalibrationTask().run("jr-1", "k-1", {"model_version": "v1"})
        assert result.ok

    def test_missing_model_version(self) -> None:
        result = ExecutionCalibrationTask().run("jr-1", "k-1", {})
        assert result.exitCode == 2


class TestReportGenerationTask:
    def test_success_full(self) -> None:
        result = ReportGenerationTask().run("jr-1", "k-1", {"report_type": "full"})
        assert result.ok
        assert result.checkpointReference == "ckpt:report:jr-1"

    def test_invalid_report_type(self) -> None:
        result = ReportGenerationTask().run("jr-1", "k-1", {"report_type": "bogus"})
        assert result.exitCode == 2


class TestRunTaskDispatch:
    def test_unknown_job_type(self) -> None:
        result = runTask("UNKNOWN", "jr-1", "k-1", {})
        assert result.exitCode == 2
        assert "未知" in result.message

    def test_idempotent_skip(self) -> None:
        recorder = InMemoryTaskRecorder()
        result1 = runTask("DATA_IMPORT", "jr-1", "key-1", {"source": "s", "instrument_id": "i"}, recorder)
        assert result1.ok
        result2 = runTask("DATA_IMPORT", "jr-2", "key-1", {"source": "s", "instrument_id": "i"}, recorder)
        assert result2.exitCode == 4  # 幂等跳过
        assert "已执行" in result2.message

    def test_different_key_executes(self) -> None:
        recorder = InMemoryTaskRecorder()
        result1 = runTask("DATA_IMPORT", "jr-1", "key-1", {"source": "s", "instrument_id": "i"}, recorder)
        result2 = runTask("DATA_IMPORT", "jr-2", "key-2", {"source": "s", "instrument_id": "i"}, recorder)
        assert result1.ok and result2.ok


class TestJobEntrypoints:
    def test_data_ingestion_job_flow(self) -> None:
        job = DataIngestionJob()
        assert (
            job.main(
                [
                    "--job-run-id", "jr-1",
                    "--job-execution-key", "k-1",
                    "--source", "cn-feed",
                    "--instrument-id", "510300.SH",
                ]
            )
            == 0
        )

    def test_reconciliation_job_flow(self) -> None:
        job = AccountReconciliationJob()
        assert (
            job.main(
                [
                    "--job-run-id", "jr-1",
                    "--job-execution-key", "k-1",
                    "--account-group", "ag-1",
                ]
            )
            == 0
        )

    def test_calibration_job_flow(self) -> None:
        job = ExecutionCalibrationJob()
        assert (
            job.main(
                [
                    "--job-run-id", "jr-1",
                    "--job-execution-key", "k-1",
                    "--model-version", "v1",
                ]
            )
            == 0
        )

    def test_report_job_flow(self) -> None:
        job = ReportGenerationJob()
        assert (
            job.main(
                [
                    "--job-run-id", "jr-1",
                    "--job-execution-key", "k-1",
                    "--report-type", "full",
                ]
            )
            == 0
        )

    def test_all_jobs_offline_validation(self) -> None:
        for job in (DataIngestionJob(), AccountReconciliationJob(), ExecutionCalibrationJob(), ReportGenerationJob()):
            assert job.main(["--help"]) == 0

    def test_all_jobs_reject_missing_execution_key(self) -> None:
        for job in (DataIngestionJob(), AccountReconciliationJob(), ExecutionCalibrationJob(), ReportGenerationJob()):
            assert job.main([]) != 0


class TestJobManifestContract:
    def test_manifest_is_pascal_case_yaml(self) -> None:
        """验收标准：根级 Jobs 仅含 PascalCase Yml 清单。"""
        manifestPath = Path("Jobs/JobManifests.yml")
        assert manifestPath.exists()
        with open(manifestPath, encoding="utf-8") as file:
            document = yaml.safe_load(file)
        assert document["ManifestVersion"] == "1.0"
        schedules = document["Schedules"]
        assert len(schedules) == 4

        # 必填 PascalCase 字段（TechSpec 11.5）
        required = {
            "ScheduleId", "ScheduleVersion", "JobType", "Command",
            "ParameterSchemaVersion", "Parameters", "ScheduleExpression",
            "TimeZone", "MisfirePolicy", "ConcurrencyPolicy",
            "LockTtlSeconds", "TimeoutSeconds", "MaxAttempts",
            "BackoffPolicy", "Enabled",
        }
        for schedule in schedules:
            missing = required - set(schedule.keys())
            assert not missing, f"{schedule.get('ScheduleId')} 缺少: {missing}"
            assert schedule["TimeZone"] == "UTC"
            assert schedule["JobType"] in {
                "DATA_IMPORT", "RECONCILIATION", "EXECUTION_CALIBRATION", "REPORT_GENERATION",
            }

    def test_manifest_commands_match_entrypoints(self) -> None:
        """清单命令必须是已安装 console script。"""
        manifestPath = Path("Jobs/JobManifests.yml")
        with open(manifestPath, encoding="utf-8") as file:
            document = yaml.safe_load(file)
        known = {
            "vq-job-data-ingestion",
            "vq-job-account-reconciliation",
            "vq-job-execution-calibration",
            "vq-job-report-generation",
        }
        for schedule in document["Schedules"]:
            assert schedule["Command"] in known
