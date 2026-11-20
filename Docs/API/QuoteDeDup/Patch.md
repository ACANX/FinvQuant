# 补丁任务（Patch）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_export_file_patch`

## 1. GET /Patch/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码 |
| status | string | 否 | `PENDING` / `RUNNING` / `DONE` / `FAILED` |
| bizMonth | int | 否 | `yyyymm` |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ patch_id, job_id, usc, biz_month, patch_level, patch_version, source_rows, patch_rows, reason, status, patch_file_id, gmt_create, gmt_finish, last_error }] }`

## 2. POST /Patch/Retry（重试）

请求体：`{ patch_id }`；把补偿任务重置为 `PENDING` 并清空 `last_error`，成功后写审计。
