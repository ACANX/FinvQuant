"""P2-034 调度服务入口：读取计划清单、创建 JobRun、触发任务。

- 默认离线参数校验（无副作用，供 packaging 契约测试）；
- `--serve` 时扫描 Jobs/*.yml 清单，按 cron 表达式创建 JobRun 并触发
  vq-job-* console script；
- 调度器只负责唤醒任务，不包含任务业务逻辑（TechSpec 11.5）；
- 导入模块不连接数据库、不启动线程。
"""

from __future__ import annotations

import argparse
import time
from collections.abc import Sequence

from veritasquant.application.Entrypoints import configureStandardStreams


def main(argv: Sequence[str] | None = None) -> int:
    """解析调度服务参数；--serve 时启动调度循环。"""
    configureStandardStreams()
    parser = argparse.ArgumentParser(prog="vq-scheduler-service", description="启动 VeritasQuant 调度服务")
    parser.add_argument("--jobs-dir", default=None, help="显式绝对 Jobs 部署清单目录")
    parser.add_argument("--poll-seconds", type=float, default=60.0, help="调度轮询间隔（秒）")
    parser.add_argument("--once", action="store_true", help="只执行一轮调度后退出（测试/冒烟）")
    parser.add_argument(
        "--serve",
        action="store_true",
        help="实际启动调度；缺省时仅做离线参数校验",
    )
    try:
        arguments = parser.parse_args(argv)
    except SystemExit as error:
        return error.code if isinstance(error.code, int) else 1
    if not arguments.serve:
        return 0
    return _serve(arguments)


def _serve(arguments: argparse.Namespace) -> int:
    """调度主循环：扫描清单 -> 校验计划 -> 创建/触发 JobRun。"""
    from veritasquant.application.Scheduling import InMemoryJobStore, ScheduleService

    store = InMemoryJobStore()
    service = ScheduleService(store)
    pollSeconds = max(0.5, arguments.poll_seconds)

    while True:
        try:
            manifestPath = _resolveJobsDir(arguments.jobs_dir)
            schedules = _loadSchedules(manifestPath)
            dueCount = _dispatchDue(service, schedules)
            if dueCount:
                print(f"调度: 触发 {dueCount} 个到期 JobRun")
        except Exception as error:  # noqa: BLE001 - 调度循环必须容错
            print(f"调度错误: {error}", file=__import__("sys").stderr)
        if arguments.once:
            return 0
        time.sleep(pollSeconds)


def _resolveJobsDir(explicit: str | None) -> str:
    """解析 Jobs 清单目录；显式参数优先，缺省用包外部署目录。"""
    if explicit:
        return explicit
    # 包内无 Jobs 清单（TechSpec：根级 Jobs/ 不进入 wheel），
    # 缺省时回退到空清单（模拟盘默认无计划），不猜测仓库根。
    return ""


def _loadSchedules(jobsDir: str) -> tuple[object, ...]:
    """加载并校验 Jobs 清单中的调度计划。"""
    import os

    if not jobsDir or not os.path.isdir(jobsDir):
        return ()
    import glob

    schedules: list[object] = []
    for manifestPath in sorted(glob.glob(os.path.join(jobsDir, "*.yml"))):
        schedules.extend(_parseManifest(manifestPath))
    return tuple(schedules)


def _parseManifest(path: str) -> tuple[object, ...]:
    """解析单个清单文件为 ScheduleDefinition 列表。"""
    import yaml

    from veritasquant.application.Scheduling import ScheduleDefinition

    with open(path, encoding="utf-8") as file:
        document = yaml.safe_load(file)
    if not isinstance(document, dict):
        return ()
    result: list[object] = []
    for raw in document.get("Schedules", []):
        if not isinstance(raw, dict):
            continue
        try:
            schedule = ScheduleDefinition(
                scheduleId=str(raw.get("ScheduleId", "")),
                scheduleVersion=str(raw.get("ScheduleVersion", "1.0.0")),
                jobType=str(raw.get("JobType", "")),
                command=str(raw.get("Command", "")),
                parameterSchemaVersion=str(raw.get("ParameterSchemaVersion", "1")),
                parameters=dict(raw.get("Parameters", {}) or {}),
                scheduleExpression=str(raw.get("ScheduleExpression", "")),
                timeZone=str(raw.get("TimeZone", "UTC")),
                misfirePolicy=str(raw.get("MisfirePolicy", "Skip")),
                concurrencyPolicy=str(raw.get("ConcurrencyPolicy", "Forbid")),
                lockTtlSeconds=int(raw.get("LockTtlSeconds", 60)),
                timeoutSeconds=int(raw.get("TimeoutSeconds", 3600)),
                maxAttempts=int(raw.get("MaxAttempts", 3)),
                backoffPolicy=str(raw.get("BackoffPolicy", "Exponential")),
                enabled=bool(raw.get("Enabled", True)),
            )
            result.append(schedule)
        except (TypeError, ValueError):
            continue
    return tuple(result)


def _dispatchDue(service: object, schedules: tuple[object, ...]) -> int:
    """为到期计划创建 JobRun（幂等）；返回触发数量。"""
    from datetime import datetime, timezone

    from veritasquant.application.Scheduling import ScheduleDefinition

    now = datetime.now(timezone.utc)
    # cron 语义：按分钟对齐（秒归零），保证同一分钟重复派发执行键稳定
    dueMinute = now.replace(second=0, microsecond=0).isoformat()
    dueCount = 0
    for schedule in schedules:
        if not isinstance(schedule, ScheduleDefinition) or not schedule.enabled:
            continue
        if _cronMatches(schedule.scheduleExpression, now):
            service.scheduleRun(schedule, dueMinute)  # type: ignore[attr-defined]
            dueCount += 1
    return dueCount


def _cronMatches(expression: str, now: object) -> bool:
    """极简 cron 匹配：仅支持分钟字段 `m h * * *`（UTC）。"""

    if not expression or expression.count(" ") != 4:
        return False
    minuteField, hourField, _, _, _ = expression.split(" ")
    minute = now.minute  # type: ignore[attr-defined]
    hour = now.hour  # type: ignore[attr-defined]
    return _fieldMatches(minuteField, minute) and _fieldMatches(hourField, hour)


def _fieldMatches(field: str, value: int) -> bool:
    if field == "*":
        return True
    try:
        return int(field) == value
    except ValueError:
        return False
