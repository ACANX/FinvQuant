# 任务暂停 / 取消（JobControl）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_export_job`

## 1. POST /Job/Pause（暂停）

请求体：`{ job_id }`；任务状态置为 `PAUSED`，`last_error` 记录「管理员暂停」，并清空租约字段。

## 2. POST /Job/Cancel（取消）

请求体：`{ job_id }`；任务状态置为 `FAILED`，`last_error` 记录「管理员取消」。

两者成功后均写审计（`MANUAL_ADMIN_EDIT`）；恢复执行请使用 `POST /Job/Retry`（重置为 `WAITING`）。
