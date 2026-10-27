# CDEU 工作流控制视图（CDEU/Overview）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.cdeu_*`

返回 CDEU（历史行情收集 → 历史行情去重 → 导出决策 → 月度行情导出 → 月度行情上传 → 归档复核）的**工作流定义树 + 指定运行状态 + 最近运行历史**，供「工作流控制与监控」页面绘制 `Workflow → Phase → Process → JOB → Step` 树。

## GET /CDEU/Overview

查询参数：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| runId | int64 | 否 | 工作流运行 ID；缺省取最近一次运行 |
| historyLimit | int | 否 | 历史运行条数，默认 20，最大 100 |

响应 `data`：

| 字段 | 类型 | 说明 |
|---|---|---|
| generated_at | string | 查询时间（RFC3339） |
| cdeu_mode | string | 系统配置 20005 的 CDEU 工作流运行模式：`Manual`（仅手动触发，调度器不自动跑）/ `Auto`（按定时任务自动触发）；读取失败或未配置时按 `Manual` 兜底 |
| cdeu_schedule_enabled | bool | 是否允许调度器自动触发，仅 `cdeu_mode=Auto` 时为 `true` |
| workflow | object | 工作流定义（`workflow_code`、名称、版本、运行模式、调度时间、时区、完成条件） |
| run | object \| null | 当前 / 指定运行摘要；库中尚无运行时为 null |
| phases | array | 阶段定义树；每项含 `processes`，每个 Process 含 `jobs`，每个 JOB 含 `steps` |
| history | array | 最近运行摘要（`run_id`、`biz_date`、`trigger_type`、`status`、开始/结束时间等） |

状态取值：

- `NOT_STARTED`：工作流尚无运行实例；
- `PENDING` / `READY` / `RUNNING`；
- `SUCCESS` / `PARTIAL` / `FAILED` / `PAUSED` / `CANCELLED` / `SKIPPED`。

字段说明：

- `phases[].status`、`processes[].status`、`jobs[].status`、`jobs[].steps[].status` 均为对应层级最近一次运行状态；
- `processes[].error` 为 Process 运行异常信息；
- `jobs[]` 的 `job_run_id`、`started_at`、`finished_at`、`processed_rows`、`processed_bytes`、`run_log_id`、`error` 为**最近一次** JOB 运行快照；`instances` 表示该 JOB 在当前运行中出现的次数；
- `jobs[].steps[]` 为 JOB 内 Step 定义及最近一次 Step 运行快照；历史运行没有 Step 记录时按 JOB 状态回填。
- `jobs[].is_dynamic=true` 表示运行时动态生成的 JOB：
  - 导出：`job_category=EXPORT`，`dynamic_job_id`、`usc`、`biz_month`、`export_type` 有值，`steps` 为 LOAD/EXPORT/REGISTER/CLEANUP；
  - 上传：`job_category=UPLOAD`，`file_id`、`file_type`、`file_version` 有值，`steps` 为 STEP_UPLOAD；
  - 固定执行壳 JOB（LOAD/EXPORT/REGISTER/CLEANUP、补丁扫描、上传）不再返回给前端。

示例：

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "generated_at": "2026-10-25T19:30:00+08:00",
    "cdeu_mode": "Manual",
    "cdeu_schedule_enabled": false,
    "workflow": { "workflow_code": "CDEU_DAILY", "name": "CDEU 每日全链路", "completion_rule": "{\"mode\":\"ALL\"}" },
    "run": { "run_id": 1, "biz_date": "2026-10-25", "status": "RUNNING", "trigger_type": "SCHEDULE", "workflow_code": "CDEU_DAILY" },
    "phases": [
      {
        "phase_code": "PHASE_DECISION",
        "name": "导出决策",
        "status": "SUCCESS",
        "processes": [
          {
            "process_code": "PROCESS_INGEST_STATS",
            "name": "每日入口流量统计",
            "status": "SUCCESS",
            "jobs": [
              {
                "job_code": "JOB_CALC_INGEST_STATS",
                "job_kind": "CALC_DAILY_INGEST_STATS",
                "status": "SUCCESS",
                "steps": [
                  { "step_code": "STEP_CALC_INGEST_STATS", "name": "每日入口流量统计", "status": "SUCCESS" }
                ]
              }
            ]
          }
        ]
      }
    ],
    "history": []
  }
}
```

错误：

- `404`：指定 `runId` 不存在；
- `500`（业务码 `2006`）：查询数据库失败。

> 页面按钮复用既有 `POST /Job/Run`（`{"job_kind":"...","auto_continue":false}`）手动触发 JOB；默认只执行当前 JOB，可在确认框勾选「自动续跑同流程下的后续 JOB」。导出内嵌阶段仍只能整体触发。动态 JOB 使用 `POST /Job/Dynamic/Run`，同样支持 `auto_continue` 并立即返回 `run_log_id`；勾选续跑时先执行同 Process 中 `dynamic_job_id` 更大的后续动态导出 JOB，再执行导出阶段之后的静态 JOB。
