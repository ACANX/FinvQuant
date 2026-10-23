"""P2-034 调度服务测试：清单解析、cron 匹配与到期派发。"""

from __future__ import annotations

from pathlib import Path

from veritasquant.apps.server.SchedulerService import (
    _cronMatches,
    _fieldMatches,
    _loadSchedules,
    _parseManifest,
    main as schedulerMain,
)
from veritasquant.application.Scheduling import InMemoryJobStore, ScheduleService


def test_scheduler_offline_validation() -> None:
    assert schedulerMain(["--help"]) == 0
    assert schedulerMain(["--poll-seconds", "1"]) == 0
    assert schedulerMain(["--unknown"]) != 0


def test_parse_manifest(tmp_path: Path) -> None:
    manifest = tmp_path / "Jobs.yml"
    manifest.write_text(
        """
Schedules:
  - ScheduleId: daily-reconcile
    ScheduleVersion: "1.0.0"
    JobType: RECONCILIATION
    Command: vq-job-account-reconciliation
    ParameterSchemaVersion: "1"
    Parameters:
      account_group: ag-1
    ScheduleExpression: "0 2 * * *"
    TimeZone: UTC
    MisfirePolicy: Skip
    ConcurrencyPolicy: Forbid
    LockTtlSeconds: 60
    TimeoutSeconds: 3600
    MaxAttempts: 3
    BackoffPolicy: Exponential
    Enabled: true
""",
        encoding="utf-8",
    )
    schedules = _parseManifest(str(manifest))
    assert len(schedules) == 1
    schedule = schedules[0]
    assert schedule.scheduleId == "daily-reconcile"  # type: ignore[attr-defined]
    assert schedule.command == "vq-job-account-reconciliation"  # type: ignore[attr-defined]
    assert schedule.timeZone == "UTC"  # type: ignore[attr-defined]


def test_load_schedules_empty_dir(tmp_path: Path) -> None:
    assert _loadSchedules(str(tmp_path)) == ()


def test_load_schedules_nonexistent_dir() -> None:
    assert _loadSchedules("/nonexistent") == ()


def test_cron_matches_minute_hour() -> None:
    from datetime import datetime, timezone

    now = datetime(2026, 8, 3, 2, 0, tzinfo=timezone.utc)
    assert _cronMatches("0 2 * * *", now) is True
    assert _cronMatches("30 2 * * *", now) is False
    assert _cronMatches("0 3 * * *", now) is False


def test_cron_matches_star() -> None:
    from datetime import datetime, timezone

    now = datetime(2026, 8, 3, 15, 42, tzinfo=timezone.utc)
    assert _cronMatches("* * * * *", now) is True
    assert _cronMatches("42 * * * *", now) is True


def test_cron_invalid_expression() -> None:
    from datetime import datetime, timezone

    now = datetime(2026, 8, 3, 2, 0, tzinfo=timezone.utc)
    assert _cronMatches("", now) is False
    assert _cronMatches("0 2 * *", now) is False  # 缺字段


def test_field_matches() -> None:
    assert _fieldMatches("*", 5) is True
    assert _fieldMatches("5", 5) is True
    assert _fieldMatches("6", 5) is False
    assert _fieldMatches("abc", 5) is False


def test_dispatch_due_creates_jobrun(tmp_path: Path) -> None:
    """到期计划派发创建 JobRun；调度器不执行任务业务逻辑。"""
    manifest = tmp_path / "Jobs.yml"
    manifest.write_text(
        """
Schedules:
  - ScheduleId: sched-a
    ScheduleVersion: "1.0.0"
    JobType: DATA_INGESTION
    Command: vq-job-data-ingestion
    ParameterSchemaVersion: "1"
    Parameters: {}
    ScheduleExpression: "* * * * *"
    Enabled: true
""",
        encoding="utf-8",
    )
    from veritasquant.apps.server.SchedulerService import _dispatchDue

    store = InMemoryJobStore()
    service = ScheduleService(store)
    schedules = _parseManifest(str(manifest))
    count = _dispatchDue(service, schedules)
    assert count == 1
    runs = list(store._runs.values())  # type: ignore[attr-defined]
    assert len(runs) == 1
    assert runs[0].command == "vq-job-data-ingestion"

    # 幂等：再次派发不创建新运行
    count2 = _dispatchDue(service, schedules)
    assert count2 == 1
    assert len(list(store._runs.values())) == 1  # type: ignore[attr-defined]
