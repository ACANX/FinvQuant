# POST /API/V1/Meta/Finv/Quote/DeDup/Job/Run — 手动触发归档 Job

手动触发一个归档链路 Job（与进程内调度器使用同一实现，同类 Job 在进程内互斥）。**异步执行**：接口立即返回 `202`，执行结果通过 `GET .../Status`、`GET .../Job/List` 与 `dedup.finv_dedup_job_run_log_export_file` 观察。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Job/Run`
- **Query 参数**：无
- **请求体**（snake_case）：

```json
{"job_kind": "CALC_DAILY_EXPORT_BUDGET"}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `job_kind` | string | 是 | Job 标识（大小写不敏感，内部转大写），取值见下表 |

可用 `job_kind`：

| 标识 | 说明 |
|---|---|
| `CALC_DAILY_INGEST_STATS` | 统计近 30 天入口流量 |
| `CALC_DAILY_EXPORT_BUDGET` | 计算当日导出预算 |
| `REFRESH_QUOTE_MATURITY` | 刷新成熟度并自动发现 `usc` |
| `MARK_SEALED_JOBS_FROZEN` | 收口冻结并输出差异报告 |
| `RUN_DUE_EXPORT_JOBS` | 生成候选并执行 LOAD→EXPORT→CLEANUP（仅在低峰窗口内实际执行） |
| `UPLOAD_PENDING_EXPORT_FILES` | 上传待传文件（`UPLOAD_MODE=none` 时直接跳过） |
| `SCAN_POST_SEAL_PATCHES` | 生成分级补丁 |
| `CLEAN_EXPIRED_EXPORT_ARTIFACTS` | 清理残留 / 本地文件 / 主表旧数据 |
| `RECOVER_TIMEOUT_EXPORT_JOBS` | 回收超时任务与上传 |
| `CHECK_EXPORT_JOB_HEALTH` | 汇总指标、抽检 MVsv 结构并输出告警 |

### 成功（HTTP 202）

```json
{"code": 202, "message": "已接受，任务异步执行", "data": {"job_kind": "CALC_DAILY_EXPORT_BUDGET"}}
```

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `400` | 400 | 请求体格式错误、`job_kind` 为空或不是已知 Job |

## 已使用位置登记

- 运维排查与灰度验证（暂无前端菜单）
