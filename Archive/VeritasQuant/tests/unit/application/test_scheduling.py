"""P2-034 调度状态机与服务测试：迁移、幂等、重试、租约与补跑。"""

from __future__ import annotations

import pytest

from veritasquant.application.Scheduling import (
    InMemoryJobStore,
    JobRunStateError,
    JobRunStateMachineV1,
    JobRunV1,
    JobStatus,
    ScheduleDefinition,
    ScheduleService,
)


class TestJobRunStateMachine:
    def test_scheduled_to_claimed_to_running_to_succeeded(self) -> None:
        state = JobStatus.Scheduled
        state = JobRunStateMachineV1.transition(state, JobStatus.Claimed)
        state = JobRunStateMachineV1.transition(state, JobStatus.Running)
        state = JobRunStateMachineV1.transition(state, JobStatus.Succeeded)
        assert state is JobStatus.Succeeded

    def test_failure_path_to_retry_wait_and_dead_letter(self) -> None:
        state = JobStatus.Running
        state = JobRunStateMachineV1.transition(state, JobStatus.RetryWait)
        assert state is JobStatus.RetryWait
        state = JobRunStateMachineV1.transition(state, JobStatus.Claimed)
        state = JobRunStateMachineV1.transition(state, JobStatus.Running)
        state = JobRunStateMachineV1.transition(state, JobStatus.DeadLetter)
        assert state is JobStatus.DeadLetter

    def test_cancel_path(self) -> None:
        state = JobStatus.Scheduled
        state = JobRunStateMachineV1.transition(state, JobStatus.CancelRequested)
        state = JobRunStateMachineV1.transition(state, JobStatus.Cancelled)
        assert state is JobStatus.Cancelled

    def test_illegal_transition_rejected(self) -> None:
        with pytest.raises(JobRunStateError):
            JobRunStateMachineV1.transition(JobStatus.Scheduled, JobStatus.Succeeded)
        with pytest.raises(JobRunStateError):
            JobRunStateMachineV1.transition(JobStatus.Succeeded, JobStatus.Running)

    def test_terminal_states_are_final(self) -> None:
        for terminal in (JobStatus.Succeeded, JobStatus.DeadLetter, JobStatus.Cancelled):
            assert JobRunStateMachineV1.canTransition(terminal, JobStatus.Running) is False


def _schedule() -> ScheduleDefinition:
    return ScheduleDefinition(
        scheduleId="sched-1",
        scheduleVersion="1.0.0",
        jobType="DATA_INGESTION",
        command="vq-job-data-ingestion",
        parameterSchemaVersion="1",
        parameters={"source": "cn-feed"},
        scheduleExpression="0 2 * * *",
    )


class TestScheduleService:
    def _service(self, store: InMemoryJobStore | None = None) -> tuple[ScheduleService, InMemoryJobStore]:
        store = store or InMemoryJobStore()
        return ScheduleService(store, nowProvider=lambda: "2026-08-03T00:00:00Z"), store

    def test_schedule_run_creates_jobrun(self) -> None:
        service, store = self._service()
        run = service.scheduleRun(_schedule(), "2026-08-03T02:00:00Z")
        assert run.status is JobStatus.Scheduled
        assert run.jobExecutionKey.startswith("sched-1:1.0.0:")

    def test_same_execution_key_idempotent(self) -> None:
        """重复触发不创建新运行（验收标准：重复触发不重复副作用）。"""
        service, store = self._service()
        run1 = service.scheduleRun(_schedule(), "2026-08-03T02:00:00Z")
        run2 = service.scheduleRun(_schedule(), "2026-08-03T02:00:00Z")
        assert run1.jobRunId == run2.jobRunId
        assert len(store._runs) == 1  # type: ignore[attr-defined]

    def test_claim_next_with_fence_token(self) -> None:
        service, _ = self._service()
        service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        claimed = service.claimNext("worker-1")
        assert len(claimed) == 1
        assert claimed[0].status is JobStatus.Claimed
        assert claimed[0].claimedBy == "worker-1"
        assert claimed[0].fenceToken is not None

    def test_full_success_lifecycle(self) -> None:
        service, _ = self._service()
        service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        claimed = service.claimNext("worker-1")[0]
        started = service.start(claimed.jobRunId, "worker-1")
        assert started is not None and started.status is JobStatus.Running
        succeeded = service.succeed(claimed.jobRunId, "worker-1", "ckpt-1")
        assert succeeded is not None and succeeded.status is JobStatus.Succeeded
        assert succeeded.checkpointReference == "ckpt-1"

    def test_failure_retries_then_dead_letter(self) -> None:
        service, _ = self._service()
        run = service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        jobRunId = run.jobRunId
        # 第 1 次失败 -> RETRY_WAIT（attempts=1 < maxAttempts=3）
        service.claimNext("worker-1")
        failed = service.fail(jobRunId, "worker-1", "boom", maxAttempts=3)
        assert failed is not None and failed.status is JobStatus.RetryWait
        assert failed.attempts == 1
        # 重试 -> CLAIMED
        retried = service.retry(jobRunId, "worker-1")
        assert retried is not None and retried.status is JobStatus.Claimed
        # 第 2 次失败 -> RETRY_WAIT
        failed2 = service.fail(jobRunId, "worker-1", "boom2", maxAttempts=3)
        assert failed2 is not None and failed2.status is JobStatus.RetryWait
        assert failed2.attempts == 2
        # 第 3 次重试后失败 -> DEAD_LETTER
        service.retry(jobRunId, "worker-1")
        service.start(jobRunId, "worker-1")
        dead = service.fail(jobRunId, "worker-1", "boom3", maxAttempts=3)
        assert dead is not None and dead.status is JobStatus.DeadLetter
        assert dead.attempts == 3

    def test_cancel_scheduled(self) -> None:
        service, _ = self._service()
        run = service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        cancelled = service.cancel(run.jobRunId)
        assert cancelled is not None and cancelled.status is JobStatus.Cancelled

    def test_stale_worker_cannot_update(self) -> None:
        """租约丢失：旧 worker 更新被拒（fencing token 校验）。"""
        service, store = self._service()
        run = service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        service.claimNext("worker-1")
        claimed = store.get(run.jobRunId)
        assert claimed is not None and claimed.fenceToken is not None
        # 重置原运行状态为 SCHEDULED 供继任者领取
        store._runs[run.jobRunId] = JobRunV1(  # type: ignore[attr-defined]
            jobRunId=run.jobRunId,
            scheduleId=run.scheduleId,
            scheduleVersion=run.scheduleVersion,
            jobType=run.jobType,
            command=run.command,
            parameters=run.parameters,
            scheduledForIso=run.scheduledForIso,
            status=JobStatus.Scheduled,
            fenceToken=None,
            claimedBy=None,
            createdTsIso=run.createdTsIso,
            updatedTsIso=run.updatedTsIso,
        )
        store.claim(run.jobRunId, "worker-2", JobStatus.Scheduled, "fence_new")
        # 旧 worker 用旧 token 更新 -> 拒绝
        updated = claimed.withStatus(JobStatus.Running)
        with pytest.raises(JobRunStateError):
            store.update(updated, claimed.fenceToken)

    def test_find_due_only_scheduled(self) -> None:
        service, store = self._service()
        service.scheduleRun(_schedule(), "2026-08-02T23:00:00Z")
        due = store.findDue("2026-08-02T23:30:00Z")
        assert len(due) == 1
        # 已领取的不再 due
        service.claimNext("worker-1")
        due2 = store.findDue("2026-08-02T23:30:00Z")
        assert len(due2) == 0


class TestScheduleDefinition:
    def test_execution_key(self) -> None:
        assert _schedule().executionKey == "sched-1:1.0.0"

    def test_misfire_policy_defaults(self) -> None:
        schedule = _schedule()
        assert schedule.misfirePolicy == "Skip"
        assert schedule.timeZone == "UTC"

    def test_jobrun_execution_key_hash_stable(self) -> None:
        run = JobRunV1(
            jobRunId="r1",
            scheduleId="s1",
            scheduleVersion="v1",
            jobType="T",
            command="cmd",
            parameters={},
            scheduledForIso="2026-08-03T02:00:00Z",
            status=JobStatus.Scheduled,
        )
        assert run.executionKeyHash() == run.executionKeyHash()
        assert len(run.executionKeyHash()) == 64
