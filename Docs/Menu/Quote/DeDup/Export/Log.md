# 归档日志与补丁（Log）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 归档日志与补丁」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Log`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/LogView.vue`
> 菜单 key：`dedup-export-log`
> 数据表：`cdeu.finv_quote_cdeu_job_run_log` / `_line`、`cdeu.finv_quote_cdeu_export_file_upload_log`、补丁任务表
> 接口：`/Meta/Finv/Quote/DeDup/RunLog/Page`、`/Meta/Finv/Quote/DeDup/UploadLog/Page`、`/Meta/Finv/Quote/DeDup/Patch/Page`、`/Meta/Finv/Quote/DeDup/Patch/Retry`
> 关联接口文档：[`Docs/API/QuoteDeDup/RunLog.md`](../../../../API/QuoteDeDup/RunLog.md)、[`Docs/API/QuoteDeDup/UploadLog.md`](../../../../API/QuoteDeDup/UploadLog.md)、[`Docs/API/QuoteDeDup/Patch.md`](../../../../API/QuoteDeDup/Patch.md)

## 1. 用途

三个 Tab：Job 运行日志、上传审计（每次尝试）、补丁任务（可重试），用于排障与追溯。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 运行日志 | 按任务号 / 时间查询逐行日志。 |
| 上传审计 | 每次上传尝试的记录与结果。 |
| 补丁任务 | 查看失败补丁并发起重试。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/RunLog/Page` | Job 运行日志 |
| `GET` | `/Meta/Finv/Quote/DeDup/UploadLog/Page` | 上传审计（每次尝试） |
| `GET` | `/Meta/Finv/Quote/DeDup/Patch/Page` | 补丁任务 |
| `POST` | `/Meta/Finv/Quote/DeDup/Patch/Retry` | 重试补丁任务 |

## 4. 说明

日志只读；重试仅对失败补丁开放。
