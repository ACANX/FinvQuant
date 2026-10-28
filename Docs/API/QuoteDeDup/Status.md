# GET /API/V1/Meta/Finv/Quote/DeDup/Status — 归档链路状态与指标

返回证券行情月归档链路（导出 / 上传 / 补丁 / 清理）的实时指标、运行参数与调度表。

## 请求

- **方法**：`GET`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Status`
- **Query 参数**：无
- **请求体**：无

### 成功（HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "metrics": {
      "generated_at": "2026-10-24T16:05:12Z",
      "job_status": {"WAITING": 12, "EXPORTED": 340, "FROZEN": 2},
      "upload_status": {"PENDING_UPLOAD": 8, "UPLOADED": 332, "UPLOAD_FAILED": 1},
      "pending_rows": 28560,
      "staging_rows": 0,
      "staging_uscs": 0,
      "backlog_days": 1.42,
      "budget_factor": 1.28,
      "budget_rows": 26000,
      "budget_consumed_rows": 12000,
      "budget_usage_ratio": 0.4615,
      "frozen_missing_rows": 0,
      "frozen_expected_rows": 4820,
      "frozen_missing_ratio": 0,
      "heartbeat_timeouts": 0,
      "upload_retry_exhausted": 0,
      "patch_last_7_days": 3,
      "high_frequency_patch_groups": 0,
      "local_files_pending_cleanup": 0,
      "alerts": []
    },
    "options": {
      "enabled": true,
      "worker_id": "finvquant-1",
      "time_zone": "Asia/Shanghai",
      "output_dir": "/data/App/FinvQuant/Export",
      "upload_mode": "dir",
      "compression": "none",
      "window_start": "00:30:00",
      "window_end": "06:30:00",
      "page_size": 10000,
      "maturity_threshold": 0.95,
      "cdeu_mode": "Manual",
      "cdeu_schedule_enabled": false
    },
    "jobs": [
      {"job_kind": "CALC_DAILY_INGEST_STATS", "daily_at": "00:05", "windowed": false, "schedule": "每日 00:05"},
      {"job_kind": "RUN_DUE_EXPORT_JOBS", "interval_seconds": 60, "windowed": true, "schedule": "低峰窗口内每 60 秒一轮"}
    ]
  }
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `job_status` | `finv_dedup_export_job` 按状态聚合 |
| `upload_status` | `finv_dedup_export_file` 按上传状态聚合 |
| `pending_rows` | 未完成导出的 COMPLETE/FROZEN 证券月行数（积压量） |
| `staging_rows` | 中转表 `export.finv_quote_secu_kline_min` 残留行数 |
| `staging_uscs` | 中转表残留证券数 |
| `backlog_days` | 积压天数（积压量 / 近 30 天日均入口条数） |
| `budget_factor` | 当日预算因子 |
| `budget_rows` | 当日预算条数 |
| `budget_consumed_rows` | 当日已消耗条数 |
| `frozen_missing_*` | FROZEN 证券月缺失行数、预期行数与缺失比例 |
| `alerts[].code` | `BACKLOG_TOO_HIGH`、`BUDGET_FACTOR_HIGH`、`HEARTBEAT_TIMEOUT`、`UPLOAD_RETRY_EXHAUSTED`、`PATCH_FREQUENCY_HIGH`、`STAGING_RESIDUAL`、`FROZEN_MISSING_HIGH`、`MVSV_STRUCTURE_INVALID` |
| `options` | 当前生效的运行参数（不包含任何凭据） |
| `options.upload_mode` | 当前生效上传模式：`none` / `dir` / `http` / `github`。`sys_config` 20002='1' 时为 `github`（Contents API），否则取 `FINV_QUOTE_DEDUB_UPLOAD_MODE` |
| `options.window_start` / `options.window_end` | 当前低峰窗口，`HH:MM:SS`（来自 `sys.sys_config` 20006，秒级） |
| `options.dynamic_job_retention_days` | 历史运行动态 JOB 实例保留天数（来自 `sys.sys_config` 20008，默认 30） |
| `options.cdeu_mode` | CDEU 工作流运行模式（`sys_config` 20005，字符串）：`Manual`（默认）——进程内调度器**不自动触发任何 Job**，只能通过 `POST .../Job/Run` 手动执行；`Auto`——每天按定时任务时间（`cdeu_job.schedule_value`）自动跑。取值大小写不敏感，空值/未知值按 `Manual` 处理 |
| `options.cdeu_schedule_enabled` | `cdeu_mode == "Auto"` 的便捷布尔值，供前端直接判断是否会自动运行 |
| `options.cdeu_mode_error` | 仅当读取 `sys_config` 20005 失败时出现，说明服务已按 `Manual` 兜底 |
| `job_progress` | 当前月导出批次进度（成功 / 失败 / 总数 / 进行中 / 待执行 / 暂停 / 冻结，装载中 / 已装载 / 导出中，以及 ⑫ 补丁任务计数 `patch_*`） |
| `jobs[]` | 10 个 Job 的调度方式与中文说明（`job_kind`、`name`、`summary`、`flow`、`details`、`cautions`、`schedule` 等） |

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `2006` | 500 | 查询归档状态失败（数据库不可用或统计 SQL 失败） |

## 已使用位置登记

- 运维排查与监控采集（暂无前端菜单；后续可接入「数据采集 → 行情归档」看板）
