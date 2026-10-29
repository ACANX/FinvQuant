# Job 运行快照（JobRuntime）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_job_run_log_export_file` + 进程内运行状态

## GET /Job/Runtime（查询）

返回 10 个定时 Job 的运行快照，用于总览页展示「当前是否在跑、跑到哪一阶段、是否卡住、最近一次结果」。

### 请求参数

无。

### 响应 `data`

| 字段 | 类型 | 说明 |
|---|---|---|
| generated_at | string | 快照生成时间（RFC3339） |
| stale_after_seconds | int | 心跳超时阈值（租约 2 倍且不少于 2 分钟），超过即视为「疑似卡住」 |
| jobs | array | 每个 Job 一项，顺序与 `GET /Status` 的 `jobs` 一致 |

`jobs[]` 字段：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| job_kind | string | 否 | Job 标识 |
| name | string | 否 | Job 中文名称 |
| summary / flow / details / cautions | string | 否 | 任务功能 / 流程 / 工作细节 / 注意事项（供前端悬浮展示） |
| schedule | string | 否 | 调度描述 |
| daily_at / interval_seconds / windowed | string / int / bool | 否 | 调度配置（同 `GET /Status`） |
| in_process | bool | 否 | 是否由当前进程执行中 |
| elapsed_seconds | int | 否 | 已运行秒数（未运行时为 0） |
| stale | bool | 否 | 存在未结束记录但心跳超时（进程崩溃 / 卡住），需人工关注 |
| heartbeat_age_seconds | int | 是 | 最近心跳距今秒数（未运行时为 null） |
| running | object | 是 | 当前未结束的运行记录（字段同 RunLog：`run_id`、`gmt_start`、`stage`、`progress_percent`、`progress_detail`、`gmt_heartbeat` 等） |
| last | object | 是 | 最近一次已结束的运行记录（含 `status`、`gmt_finish`、`processed_rows`、`error`、`stage`） |
| last_success_at / last_failure_at | string | 是 | 最近成功 / 失败时间（不受 24 小时窗口限制） |
| success_24h / failed_24h / paused_24h / total_24h | int | 否 | 最近 24 小时运行结果统计 |

### 说明

- `progress_percent` 为 0~100；负值表示无法估算总量的不确定进度（前端显示为不确定进度条）；`null` 表示无进度语义。
- 阶段 `stage` 取值：`RUNNING`、`STATS`、`BUDGET`、`MATURITY`、`SEAL`、`LOAD`、`EXPORT`、`CLEANUP`、`UPLOAD`、`PATCH`、`RECOVER`、`HEALTH`、`DONE`、`FAILED`、`PAUSED`。
- 每次进度上报同时刷新 `gmt_heartbeat`；导出任务的装载 / 写文件 / 清理阶段均会周期上报心跳。
- 手动触发后建议轮询本接口（5 秒）直到 `in_process=false` 且 `last.gmt_start` 晚于触发时刻，即可拿到本次执行结果。
- 历史执行记录使用 `GET /RunLog/Page?jobKind=<Job 标识>`。
