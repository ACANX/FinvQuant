"""P2-034 调度计划、JobRun 状态机与调度服务（应用层）。

TechSpec 11.5：
- JobRun 状态机固定为 SCHEDULED -> CLAIMED -> RUNNING -> SUCCEEDED；
  失败进入 RETRY_WAIT -> CLAIMED；超过最大次数 DEAD_LETTER；
  取消 CANCEL_REQUESTED -> CANCELLED；
- worker 使用带 fencing token 的租约；续租失败必须停止；
- 任务入口接收 job_run_id + job_execution_key，业务幂等由 command_id/
  inbox/outbox/checkpoint 保证；
- 重复触发、misfire、租约丢失、重试和补跑不得重复业务副作用。
"""

from __future__ import annotations

import hashlib
from collections.abc import Callable
from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Protocol

UTC = timezone.utc


def _utcNowIso() -> str:
    return datetime.now(UTC).isoformat()


class JobStatus(Enum):
    """JobRun 状态机状态。"""

    Scheduled = "SCHEDULED"
    Claimed = "CLAIMED"
    Running = "RUNNING"
    Succeeded = "SUCCEEDED"
    RetryWait = "RETRY_WAIT"
    DeadLetter = "DEAD_LETTER"
    CancelRequested = "CANCEL_REQUESTED"
    Cancelled = "CANCELLED"


_TERMINAL = frozenset({JobStatus.Succeeded, JobStatus.DeadLetter, JobStatus.Cancelled})


class JobRunStateError(ValueError):
    """非法状态迁移。"""


class JobRunStateMachineV1:
    """JobRun 状态机（TechSpec 11.5 固定迁移）。"""

    _TRANSITIONS: dict[JobStatus, frozenset[JobStatus]] = {
        JobStatus.Scheduled: frozenset({JobStatus.Claimed, JobStatus.CancelRequested}),
        JobStatus.Claimed: frozenset({JobStatus.Running, JobStatus.RetryWait, JobStatus.DeadLetter}),
        JobStatus.Running: frozenset({JobStatus.Succeeded, JobStatus.RetryWait, JobStatus.DeadLetter, JobStatus.CancelRequested}),
        JobStatus.RetryWait: frozenset({JobStatus.Claimed, JobStatus.CancelRequested, JobStatus.DeadLetter}),
        JobStatus.CancelRequested: frozenset({JobStatus.Cancelled, JobStatus.DeadLetter}),
        JobStatus.Succeeded: frozenset(),
        JobStatus.DeadLetter: frozenset(),
        JobStatus.Cancelled: frozenset(),
    }

    @classmethod
    def canTransition(cls, current: JobStatus, target: JobStatus) -> bool:
        return target in cls._TRANSITIONS.get(current, frozenset())

    @classmethod
    def transition(cls, current: JobStatus, target: JobStatus) -> JobStatus:
        if not cls.canTransition(current, target):
            raise JobRunStateError(f"非法迁移 {current.value} -> {target.value}")
        return target


@dataclass(frozen=True, slots=True)
class ScheduleDefinition:
    """版本化调度计划（TechSpec 11.5 必填字段）。"""

    scheduleId: str
    scheduleVersion: str
    jobType: str
    command: str  # 已安装的 vq-job-* console script
    parameterSchemaVersion: str
    parameters: dict[str, object]
    scheduleExpression: str  # cron 五字段（UTC）或间隔描述
    timeZone: str = "UTC"  # 固定 UTC
    misfirePolicy: str = "Skip"  # Skip / FireImmediately
    concurrencyPolicy: str = "Forbid"  # Forbid / Allow
    lockTtlSeconds: int = 60
    timeoutSeconds: int = 3600
    maxAttempts: int = 3
    backoffPolicy: str = "Exponential"  # Exponential / Fixed
    enabled: bool = True

    @property
    def executionKey(self) -> str:
        """稳定 job_execution_key：schedule_id + version + scheduled_for 由调度器组装。"""
        return f"{self.scheduleId}:{self.scheduleVersion}"


@dataclass(frozen=True, slots=True)
class JobRunV1:
    """一次 JobRun 实例。"""

    jobRunId: str
    scheduleId: str
    scheduleVersion: str
    jobType: str
    command: str
    parameters: dict[str, object]
    scheduledForIso: str
    status: JobStatus
    attempts: int = 0
    fenceToken: str | None = None
    claimedBy: str | None = None
    createdTsIso: str = field(default_factory=_utcNowIso)
    updatedTsIso: str = field(default_factory=_utcNowIso)
    lastError: str | None = None
    checkpointReference: str | None = None

    @property
    def jobExecutionKey(self) -> str:
        """稳定执行键：调度器保证 (schedule_id+version+scheduled_for) 唯一。"""
        return f"{self.scheduleId}:{self.scheduleVersion}:{self.scheduledForIso}"

    def executionKeyHash(self) -> str:
        return hashlib.sha256(self.jobExecutionKey.encode("utf-8")).hexdigest()

    def withStatus(self, target: JobStatus, **updates: Any) -> "JobRunV1":
        newStatus = JobRunStateMachineV1.transition(self.status, target)
        result = JobRunV1(
            jobRunId=updates.get("jobRunId", self.jobRunId),
            scheduleId=updates.get("scheduleId", self.scheduleId),
            scheduleVersion=updates.get("scheduleVersion", self.scheduleVersion),
            jobType=updates.get("jobType", self.jobType),
            command=updates.get("command", self.command),
            parameters=updates.get("parameters", self.parameters),
            scheduledForIso=updates.get("scheduledForIso", self.scheduledForIso),
            status=newStatus,
            attempts=updates.get("attempts", self.attempts),
            fenceToken=updates.get("fenceToken", self.fenceToken),
            claimedBy=updates.get("claimedBy", self.claimedBy),
            createdTsIso=updates.get("createdTsIso", self.createdTsIso),
            updatedTsIso=_utcNowIso(),
            lastError=updates.get("lastError", self.lastError),
            checkpointReference=updates.get("checkpointReference", self.checkpointReference),
        )
        # 显式覆盖 status 字段（updates 可能传 status 时以 target 为准）
        return result


class JobStore(Protocol):
    """JobRun 持久化端口（Postgres 实现见 infrastructure）。"""

    def create(self, run: JobRunV1) -> JobRunV1: ...

    def get(self, jobRunId: str) -> JobRunV1 | None: ...

    def claim(
        self, jobRunId: str, workerId: str, expectedStatus: JobStatus, fenceToken: str
    ) -> JobRunV1 | None: ...

    def update(self, run: JobRunV1, expectedFenceToken: str) -> JobRunV1: ...

    def findDue(
        self, nowIso: str, limit: int = 20
    ) -> tuple[JobRunV1, ...]: ...

    def findByExecutionKey(self, executionKey: str) -> JobRunV1 | None: ...


class InMemoryJobStore:
    """进程内 JobRun 存储（模拟盘默认；测试可注入）。"""

    def __init__(self) -> None:
        self._runs: dict[str, JobRunV1] = {}
        self._byKey: dict[str, str] = {}

    def create(self, run: JobRunV1) -> JobRunV1:
        self._runs[run.jobRunId] = run
        self._byKey[run.jobExecutionKey] = run.jobRunId
        return run

    def get(self, jobRunId: str) -> JobRunV1 | None:
        return self._runs.get(jobRunId)

    def claim(
        self, jobRunId: str, workerId: str, expectedStatus: JobStatus, fenceToken: str
    ) -> JobRunV1 | None:
        run = self._runs.get(jobRunId)
        if run is None or run.status is not expectedStatus:
            return None
        if run.status is JobStatus.Claimed and run.claimedBy == workerId:
            # 同 worker 续租：直接轮换 token（不经状态机）
            renewed = JobRunV1(
                jobRunId=run.jobRunId,
                scheduleId=run.scheduleId,
                scheduleVersion=run.scheduleVersion,
                jobType=run.jobType,
                command=run.command,
                parameters=run.parameters,
                scheduledForIso=run.scheduledForIso,
                status=JobStatus.Claimed,
                attempts=run.attempts,
                fenceToken=fenceToken,
                claimedBy=workerId,
                createdTsIso=run.createdTsIso,
                updatedTsIso=run.updatedTsIso,
                lastError=run.lastError,
                checkpointReference=run.checkpointReference,
            )
            self._runs[jobRunId] = renewed
            return renewed
        claimed = run.withStatus(
            JobStatus.Claimed, fenceToken=fenceToken, claimedBy=workerId
        )
        self._runs[jobRunId] = claimed
        return claimed

    def update(self, run: JobRunV1, expectedFenceToken: str) -> JobRunV1:
        existing = self._runs.get(run.jobRunId)
        if existing is None:
            raise JobRunStateError("运行不存在")
        if expectedFenceToken and existing.fenceToken != expectedFenceToken:
            raise JobRunStateError("租约丢失或 fencing token 不匹配")
        if existing.status in _TERMINAL and run.status is not existing.status:
            raise JobRunStateError("终态不可回退")
        self._runs[run.jobRunId] = run
        return run

    def findDue(self, nowIso: str, limit: int = 20) -> tuple[JobRunV1, ...]:
        return tuple(
            run
            for run in self._runs.values()
            if run.status is JobStatus.Scheduled and run.scheduledForIso <= nowIso
        )[:limit]

    def findByExecutionKey(self, executionKey: str) -> JobRunV1 | None:
        jobRunId = self._byKey.get(executionKey)
        return self._runs.get(jobRunId) if jobRunId else None


class JobRunner(Protocol):
    """任务执行端口：触发已安装 console script。"""

    def run(self, jobRun: JobRunV1) -> str | None: ...  # 返回 checkpoint 引用


class ScheduleService:
    """调度用例：创建运行、claim、推进状态机、重试与补跑。"""

    def __init__(
        self,
        store: JobStore,
        runner: JobRunner | None = None,
        nowProvider: Callable[[], str] | None = None,
    ) -> None:
        self._store = store
        self._runner = runner
        self._nowProvider = nowProvider

    def _nowIso(self) -> str:
        if self._nowProvider is not None:
            return self._nowProvider()
        return _utcNowIso()

    def scheduleRun(
        self, schedule: ScheduleDefinition, scheduledForIso: str
    ) -> JobRunV1:
        """创建 JobRun；同执行键已存在则返回既有（幂等，不重复触发）。"""
        executionKey = f"{schedule.executionKey}:{scheduledForIso}"
        existing = self._store.findByExecutionKey(executionKey)
        if existing is not None:
            return existing
        run = JobRunV1(
            jobRunId=f"jobrun_{self._jobRunId()}",
            scheduleId=schedule.scheduleId,
            scheduleVersion=schedule.scheduleVersion,
            jobType=schedule.jobType,
            command=schedule.command,
            parameters=dict(schedule.parameters),
            scheduledForIso=scheduledForIso,
            status=JobStatus.Scheduled,
        )
        return self._store.create(run)

    def _jobRunId(self) -> str:
        import uuid

        return uuid.uuid4().hex[:16]

    def claimNext(self, workerId: str, limit: int = 1) -> tuple[JobRunV1, ...]:
        """worker 领取到期运行；带新 fencing token。"""
        claimed: list[JobRunV1] = []
        for run in self._store.findDue(self._nowIso(), limit):
            fenceToken = f"fence_{self._jobRunId()}"
            result = self._store.claim(run.jobRunId, workerId, JobStatus.Scheduled, fenceToken)
            if result is not None:
                claimed.append(result)
        return tuple(claimed)

    def start(self, jobRunId: str, workerId: str) -> JobRunV1 | None:
        """CLAIMED -> RUNNING（worker 开始执行）。"""
        run = self._store.get(jobRunId)
        if run is None or run.claimedBy != workerId:
            return None
        assert run.fenceToken is not None
        updated = run.withStatus(JobStatus.Running)
        return self._store.update(updated, run.fenceToken)

    def succeed(self, jobRunId: str, workerId: str, checkpointReference: str | None = None) -> JobRunV1 | None:
        """RUNNING -> SUCCEEDED（终态不可回退）。"""
        run = self._store.get(jobRunId)
        if run is None or run.claimedBy != workerId:
            return None
        assert run.fenceToken is not None
        updated = run.withStatus(
            JobStatus.Succeeded, checkpointReference=checkpointReference, lastError=None
        )
        return self._store.update(updated, run.fenceToken)

    def fail(self, jobRunId: str, workerId: str, error: str, maxAttempts: int = 3) -> JobRunV1 | None:
        """RUNNING -> RETRY_WAIT 或 DEAD_LETTER（超过最大次数）。"""
        run = self._store.get(jobRunId)
        if run is None or run.claimedBy != workerId:
            return None
        assert run.fenceToken is not None
        nextAttempts = run.attempts + 1
        if nextAttempts >= maxAttempts:
            updated = run.withStatus(
                JobStatus.DeadLetter, attempts=nextAttempts, lastError=error
            )
        else:
            updated = run.withStatus(
                JobStatus.RetryWait, attempts=nextAttempts, lastError=error
            )
        return self._store.update(updated, run.fenceToken)

    def retry(self, jobRunId: str, workerId: str) -> JobRunV1 | None:
        """RETRY_WAIT -> CLAIMED（重试；业务幂等由 command/inbox 保证）。"""
        run = self._store.get(jobRunId)
        if run is None:
            return None
        fenceToken = f"fence_{self._jobRunId()}"
        result = self._store.claim(jobRunId, workerId, JobStatus.RetryWait, fenceToken)
        if result is None:
            return None
        return result

    def cancel(self, jobRunId: str) -> JobRunV1 | None:
        """SCHEDULED/RETRY_WAIT/RUNNING -> CANCEL_REQUESTED -> CANCELLED。"""
        run = self._store.get(jobRunId)
        if run is None:
            return None
        if run.status in _TERMINAL:
            return run
        if run.status is JobStatus.Scheduled:
            # 未执行：直接 CANCELLED（无需 worker 确认）
            cancelled = JobRunV1(
                jobRunId=run.jobRunId,
                scheduleId=run.scheduleId,
                scheduleVersion=run.scheduleVersion,
                jobType=run.jobType,
                command=run.command,
                parameters=run.parameters,
                scheduledForIso=run.scheduledForIso,
                status=JobStatus.Cancelled,
                attempts=run.attempts,
                fenceToken=run.fenceToken,
                claimedBy=run.claimedBy,
                createdTsIso=run.createdTsIso,
                updatedTsIso=_utcNowIso(),
                lastError=run.lastError,
                checkpointReference=run.checkpointReference,
            )
            return self._store.update(cancelled, run.fenceToken or "")
        if run.status is JobStatus.CancelRequested:
            assert run.fenceToken is not None
            updated = run.withStatus(JobStatus.Cancelled)
            return self._store.update(updated, run.fenceToken)
        requested = run.withStatus(JobStatus.CancelRequested)
        return self._store.update(requested, run.fenceToken or "")

    def query(self, jobRunId: str) -> JobRunV1 | None:
        return self._store.get(jobRunId)

    def queryAll(self) -> tuple[JobRunV1, ...]:
        runs = getattr(self._store, "_runs", None)
        if isinstance(runs, dict):
            return tuple(runs.values())
        return ()
