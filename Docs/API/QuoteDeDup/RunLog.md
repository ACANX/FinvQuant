# Job 运行日志（RunLog）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_job_run_log_export_file`（只读）

## GET /RunLog/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| jobKind | string | 否 | Job 标识；管理员操作记为 `MANUAL_ADMIN_EDIT` |
| status | string | 否 | `SUCCESS` / `FAILED` / `PAUSED` |
| usc | string | 否 | 证券代码 |
| keyword | string | 否 | `error` 字段模糊匹配 |
| dateFrom / dateTo | string | 否 | `yyyy-MM-dd`，按 `gmt_start` 过滤 |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ run_id, job_kind, job_id, usc, biz_month, gmt_start, gmt_finish, status, processed_rows, processed_bytes, error, stage, progress_percent, progress_detail, gmt_heartbeat }] }`

- `stage`：当前 / 最近执行阶段（`RUNNING`、`LOAD`、`EXPORT`、`CLEANUP`、`UPLOAD`、`PATCH`、`DONE` 等）。
- `progress_percent`：进度百分比；负值表示不确定进度，`null` 表示不适用。
- `gmt_heartbeat`：最近心跳时间；运行中记录超过心跳阈值未更新即视为「疑似卡住」。


## GET /RunLog/Detail（单次运行详细日志）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| runId | int | 是 | `RunLog/Page` 返回的 `run_id` |

响应 `data`：

```json
{
  "run_id": 123,
  "job_kind": "RUN_DUE_EXPORT_JOBS",
  "job_id": 88,
  "usc": "GCMain",
  "biz_month": 201912,
  "gmt_start": "2026-10-26T12:00:00+08:00",
  "gmt_finish": "2026-10-26T12:00:05+08:00",
  "status": "SUCCESS",
  "stage": "DONE",
  "progress_percent": 100,
  "progress_detail": "任务完成",
  "error": null,
  "lines": [
    {
      "line_id": 1,
      "run_id": 123,
      "level": "INFO",
      "stage": "RUNNING",
      "message": "JOB 开始 · RUN_DUE_EXPORT_JOBS",
      "progress_percent": null,
      "gmt_create": "2026-10-26T12:00:00+08:00"
    }
  ]
}
```

- 运行开始后 `status` 即为 `RUNNING`，历史运行完成后再变为终态；前端可在手动触发拿到 `run_log_id` 后立即打开本接口。
- `lines` 来自 `dedup.finv_dedup_job_run_log_line`，按 `line_id` 升序返回。
- 历史运行若没有详细日志行，`lines` 为空，但仍返回运行汇总。
