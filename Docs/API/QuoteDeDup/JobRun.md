# POST /API/V1/Meta/Finv/Quote/DeDup/Job/Run — 手动触发归档 Job

手动触发一个归档链路 Job（与进程内调度器使用同一实现，同类 Job 在进程内互斥）。

## 手动触发与调度触发的约束差异（重要）

| 约束 | 调度触发（scheduler） | 手动触发（本接口） |
|---|---|---|
| `sys_config` 20005 CDEU 运行模式 | 必须为 `Auto` 才会触发 | 不检查（`Manual` 只用于关闭自动调度） |
| 低峰窗口（`sys_config` 20006） | 窗口外不触发 | **不限制**，整轮跑完 |
| 当日导出预算（`finv_dedup_daily_budget`） | 预算耗尽即停 | **不限制**（消耗仍记账，供审计与 Auto 模式基线） |
| `max_jobs_per_run` | 生效 | 生效 |

设计缘由：日预算是按「近 30 天入口流量 × 因子」算出的**增量保护**，历史全量重导（数百万行、上百个月）不可能塞进这个预算里；手动触发由运维显式发起，因此不受预算与窗口限制。`RUN_DUE_EXPORT_JOBS` 的进度详情会标注「手动触发：不限制」。
**异步执行**：接口立即返回 `202` 与 `run_log_id`，可立刻通过 `GET .../RunLog/Detail?runId=` 查看实时日志；执行结果同时写入 `dedup.finv_dedup_job_run_log_export_file`。

`auto_continue` 默认为 `false`，只执行当前 JOB；设为 `true` 时，当前 JOB 完成后按 CDEU 工作流定义顺序继续执行后续 JOB（同一 `job_kind` 只执行一次）。

## 请求

- **方法**：`POST`
- **路径**：`/API/V1/Meta/Finv/Quote/DeDup/Job/Run`
- **Query 参数**：无
- **请求体**（snake_case）：

```json
{"job_kind": "CALC_DAILY_EXPORT_BUDGET", "auto_continue": false}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `job_kind` | string | 是 | Job 标识（大小写不敏感，内部转大写），取值见下表 |
| `auto_continue` | boolean | 否 | 是否自动续跑同流程后续 JOB，默认 `false`（仅执行当前 JOB） |

可用 `job_kind`：

| 标识 | 说明 |
|---|---|
| `CALC_DAILY_INGEST_STATS` | 统计近 30 天入口流量 |
| `CALC_DAILY_EXPORT_BUDGET` | 计算当日导出预算 |
| `REFRESH_QUOTE_MATURITY` | 刷新成熟度并自动发现 `usc` |
| `MARK_SEALED_JOBS_FROZEN` | 收口冻结并输出差异报告 |
| `RUN_DUE_EXPORT_JOBS` | 生成候选并执行 LOAD→EXPORT→CLEANUP。调度触发需在低峰窗口内且受预算约束；**手动触发不受窗口与预算限制** |
| `UPLOAD_PENDING_EXPORT_FILES` | 上传待传文件（生效模式为 `none` 时直接跳过；`sys_config` 20002='1' 时为 `github`） |
| `SCAN_POST_SEAL_PATCHES` | 生成分级补丁 |
| `CLEAN_EXPIRED_EXPORT_ARTIFACTS` | 清理残留 / 本地文件 / 历史动态 JOB 实例 / 已沉淀证券月的主表数据（20007=1） |
| `RECOVER_TIMEOUT_EXPORT_JOBS` | 回收超时任务与上传 |
| `CHECK_EXPORT_JOB_HEALTH` | 汇总指标、抽检 MVsv 结构并输出告警 |

### 成功（HTTP 202）

```json
{"code": 202, "message": "已接受，任务异步执行", "data": {"job_kind": "CALC_DAILY_EXPORT_BUDGET", "run_log_id": 3098, "auto_continue": false}}
```

### 错误

| `code` | HTTP | 含义 |
|---|---|---|
| `400` | 400 | 请求体格式错误、`job_kind` 为空或不是已知 Job |

## 已使用位置登记

- CDEU 流程控制与监控页 JOB 清单 / 阶段详情「立即运行」「手动执行」按钮（先弹出确认框选择是否续跑后续 JOB，确认后立即打开执行日志弹窗）。

### `CLEAN_EXPIRED_EXPORT_ARTIFACTS` 的 `details`

| 字段 | 含义 |
|---|---|
| `staging_deleted` | 中转表残留删除行数 |
| `staging_truncated` | 中转表是否已 TRUNCATE |
| `local_deleted` | 过期本地文件删除数 |
| `purge_after_export` | 系统配置 20007 当前取值（`true`=`1` 删除 / `false`=`0` 保留） |
| `stale_dynamic_deleted` | 超过 20008 保留期的历史运行动态 JOB 实例删除数 |
| `dynamic_retention_days` | 系统配置 20008 的生效值（天，默认 30） |
| `settled_months_purged` | 「已沉淀」证券月清理数量（仅 20007=1 时非 0） |
| `settled_rows_purged` | 「已沉淀」证券月删除的主表行数（仅 20007=1 时非 0） |

「已沉淀」判定：BASE 文件 `export_status='EXPORTED'` 且 `upload_status='UPLOADED'`，主表该月仍有数据，
且 BASE 生成之后该月主表没有任何 `gmt_create`/`gmt_update` 晚于 `BASE.gmt_create` 的行。
满足即说明远端 BASE 已完整包含该月数据，本地 `dedup` 主表副本可按 20007 释放（`quote` 副本不受影响）。
