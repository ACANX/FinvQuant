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

响应 `data`：`{ total, list: [{ run_id, job_kind, job_id, usc, biz_month, gmt_start, gmt_finish, status, processed_rows, processed_bytes, error }] }`
