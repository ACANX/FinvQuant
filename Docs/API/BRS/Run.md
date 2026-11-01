# POST /API/V1/Meta/BRS/Backup/Finv/Run/Manual · GET .../Run/Status — 手动备份与实时日志

手动模式：随时触发一轮备份，并实时查看进度与运行日志。与自动模式（`sys.sys_config` 39001 定时）**共用同一执行链**，页面看到的行为一致。

> 并发限制：同一时刻只允许一个运行；已有任务在跑时再次触发返回 `409`。
>
> 状态存储：运行状态与日志**只保存在服务内存**，服务重启后清空（备份产物仍落磁盘与对象存储）。
>
> 上传目标：`<39002.prefix>/<1002.str_value>/<yyyyMMdd>/<db>.<schema>.<table>.dump`；1002 缺失或非法时跳过上传（本地备份保留）。
>
> 前端页面：`/Meta/BRS/Backup/Finv/Run`（`Web/src/views/Meta/BRS/Backup/Finv/BrsRunView.vue`）。

## 1. 手动触发一轮备份

- **方法**：`POST`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Run/Manual`
- **请求体**：无（`{}`）

异步执行：立即返回本次运行的状态快照（`status=running`），随后用 `Status` 轮询进度与日志。

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "已开始备份",
  "data": {
    "run_id": "20261101-204512-manual",
    "trigger": "manual",
    "status": "running",
    "started_at": "2026-11-01T20:45:12.345678+08:00",
    "finished_at": null,
    "elapsed_ms": 0,
    "total": 0,
    "done": 0,
    "failed": 0,
    "current_table": "",
    "backup_dir": "/data/App/FinvQuant/BRS",
    "error": "",
    "logs": [{ "seq": 1, "time": "2026-11-01T20:45:12.345678+08:00", "level": "info", "message": "开始执行 手动 备份（run_id=20261101-204512-manual）" }],
    "last_seq": 1
  }
}
```

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 409 | 409 | 已有备份任务在运行（消息含运行来源与开始时间） |
| 500 | 2006 | 内部错误 |

## 2. 查询运行状态与日志

- **方法**：`GET`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Run/Status`

### Query 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sinceSeq` | int | 可选 | 增量起点：只返回 `seq` 大于该值的日志；省略或 `0` 返回全部（最多 1000 条） |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `run_id` | string | 运行标识（`yyyyMMdd-HHmmss-<trigger>`），变化表示新的一轮 |
| `trigger` | string | `manual`（手动）/ `auto`（定时） |
| `status` | string | `idle` / `running` / `succeeded` / `failed` |
| `started_at` / `finished_at` | string | 开始 / 结束时间 |
| `elapsed_ms` | int | 已耗时（运行中实时增长） |
| `total` / `done` / `failed` | int | 总表数 / 已完成 / 失败张数（`done` 为已处理张数） |
| `current_table` | string | 当前正在备份的表（`schema.table`） |
| `backup_dir` | string | 本轮备份目录（`<BackupDir>/<yyyyMMdd>`，`BackupDir` 取 `FINV_BRS_BACKUP_DIR`；Native 与容器统一为 `/data/App/FinvQuant/BRS`） |
| `error` | string | 失败原因（`status=failed` 时） |
| `logs` | array | 日志条目：`{seq, time, level, message}`，`level` 取 `info` / `warn` / `error` |
| `last_seq` | int | 当前最大日志序号，下一次增量拉取用 |

### 轮询建议

- 运行中（`status=running`）：每 1 秒带 `sinceSeq=last_seq` 拉取，日志追加显示；
- 结束或空闲：每 5 秒一次即可；
- `run_id` 变化时清空本地日志重新累积。

## 3. 日志内容示例

```text
[info]  开始执行 手动 备份（run_id=20261101-204512-manual）
[info]  未启用对象存储上传（39002 enabled=false），仅本地备份
[info]  开始备份：12 张表 → /data/App/FinvQuant/BRS/20261101
[info]  上传环境目录：prod
[info]  备份成功 sys.sys_user（…/finv.sys.sys_user.dump，1.2 MiB，耗时 210ms）
[info]  已上传 prod/20261101/finv.sys.sys_user.dump
[info]  本轮备份完成：12 张表
[info]  运行结束（成功）
```

失败时会输出 `[error] 备份失败 <schema>.<table>（priority=N）: <原因>`，并在结束行给出 `x/y 张表失败`。

## 4. 关联

| 项 | 说明 |
|------|------|
| 触发源 | 页面按钮（`manual`）与 39001 cron（`auto`），见 [Schedule.md](Schedule.md) |
| 备份范围 | `brs.brs_bak_rdb_table` 中 `flag_enable='1'` 的登记，`priority` 倒序，见 [BackupTable.md](BackupTable.md) |
| 上传目标 | 39002 对象存储配置（`flag_upload='1'` 的表才上传），见 [OSSConfig.md](OSSConfig.md) |
| 服务实现 | `internal/meta/finv/brs/runner.go`（Runner 状态机与日志缓冲）、`brs.go`（`RunOnceWithHooks` 逐表 pg_dump） |
