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
- `lines` 来自 `cdeu.finv_quote_cdeu_job_run_log_line`，按 `line_id` 升序返回。
- 历史运行若没有详细日志行，`lines` 为空，但仍返回运行汇总。
- `lines[].level`：`INFO`（正常节点）/ `ERROR`（失败项）；前端建议按 level 标红与过滤。

### 上传类 JOB 的日志口径（需求 10824）

`UPLOAD_PENDING_EXPORT_FILES` 每处理一个文件依次写以下日志行（`stage=UPLOAD`），确保不看数据库也能定位环节：

| 日志行 | 内容 |
|---|---|
| 上传开始 | `file_id` / `usc` / `biz_month` / `type` / `attempt` / 开始时间 |
| 本地文件 | 本地绝对路径、登记大小与登记 sha256 |
| 本地校验 | 实际大小 / 实际 sha256，与登记一致才继续 |
| 上传目标 | `mode` + **网站**（API Base）+ **组织/用户** + **仓库** + **分支** + **目标路径** + 完整 URL（PAT 不入日志） |
| 远端 HEAD | 不存在 / 已存在同内容（幂等跳过）/ 已存在但内容不同（覆盖）三类；GitHub 模式的远端校验值是 **git blob sha1**（与本地 SHA256 不可比），这类日志会写明「交由上传实现按自身口径判定是否覆盖」（需求 10825） |
| PUT 完成 | 字节数、耗时、开始与结束时间 |
| 远端校验 | 远端大小与 sha256 |
| 上传成功 | 字节数、总耗时、结束时间、目标描述 |
| 上传失败（`level=ERROR`） | 文件名、本地路径、目标路径与目标描述、`attempt`、失败原因（带环节前缀：本地校验 / HEAD / PUT / 远端校验）、下次重试时间或重试耗尽 |

本轮结束时：

- `progress_detail` 与末条日志为「本轮上传完成 成功 N 个 / 失败 N 个 / 跳过 MANIFEST N 个 共 N 字节 结束时间=… 耗时=…」；
- 若有失败，运行汇总的 `error` 字段与末条日志同时带上「首个失败的 `file_id` 与原因」，因此页面「异常」列与「执行日志」内容一致，不再出现「异常字段有错、日志只有 SUCCESS」的情况；
- `RESULT.details` 附 `uploaded` / `failed` / `manifests_skipped` / `uploaded_bytes` / `elapsed_seconds` / `first_failed_id` / `first_failed_err`。
