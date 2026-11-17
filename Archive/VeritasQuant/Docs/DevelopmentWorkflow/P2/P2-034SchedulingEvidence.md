# P2-034 调度计划、JobRun 状态机和 console job 入口证据

## 任务信息
- **PlanTaskId:** P2-034
- **标题:** 实现调度计划、JobRun 状态机和 console job 入口
- **阶段/里程碑:** M2A 模拟盘与基金能力建设
- **状态:** IN_REVIEW（等待第五批 PR 合并后验收）

## 实现内容

### 调度模型（`application/Scheduling.py`）
- **JobRunStateMachineV1**（TechSpec 11.5 固定）：
  SCHEDULED → CLAIMED → RUNNING → SUCCEEDED；失败 RETRY_WAIT → CLAIMED；
  超最大次数 DEAD_LETTER；取消 CANCEL_REQUESTED → CANCELLED；终态不可回退
- **ScheduleDefinition**：版本化计划（11.5 必填字段全含，TimeZone 固定 UTC）
- **InMemoryJobStore**：fencing token 租约（旧 token 更新拒绝）、
  (schedule_id+version+scheduled_for) 执行键唯一
- **ScheduleService**：scheduleRun（执行键幂等）、claimNext（fence token）、
  start/succeed/fail（重试计数 → DEAD_LETTER）、retry、cancel

### 任务入口契约（`jobs/JobEntrypoint.py`）
- 必收 `--job-run-id` + `--job-execution-key`；参数 Schema 版本校验
- 退出码语义：0 成功 / 2 参数无效 / 3 业务失败 / 4 幂等跳过

### 调度服务（`apps/server/SchedulerService.py`）
- Jobs/*.yml 清单解析（PascalCase）、cron 五字段匹配（UTC）、
  到期派发创建 JobRun（分钟对齐保证幂等）

## 验收标准映射
| 验收标准 | 实现 | 证据 |
| --- | --- | --- |
| 重复触发不重复副作用 | 执行键幂等 scheduleRun | test_same_execution_key_idempotent |
| misfire 处理 | Skip/FireImmediately 策略字段 | test_misfire_policy_defaults |
| 租约丢失 | fencing token 校验 | test_stale_worker_cannot_update |
| 重试和补跑不重复 | RETRY_WAIT 计数 + 幂等 | test_failure_retries_then_dead_letter |

## 测试结果
- `tests/unit/application/test_scheduling.py`：16 个测试通过
- `tests/unit/jobs/test_job_entrypoints.py`：7 个测试通过
- `tests/unit/apps/test_scheduler_service.py`：9 个测试通过
- 全量 918 测试通过（2026-08-03，本地）
- ruff/mypy: 通过

## 关键决策
- 调度器只唤醒任务，不包含任务业务逻辑（TechSpec 11.5）
- 执行键分钟对齐保证同分钟重复派发幂等
