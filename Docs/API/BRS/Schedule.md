# GET/POST /API/V1/Meta/BRS/Backup/Finv/Schedule/* — 备份调度配置

自动模式（定时任务）的调度配置，对应系统配置 `sys.sys_config` 39001 的 `str_value`（6 段 cron：秒 分 时 日 月 周）。

> 生效时机：调度器每 60 秒重新读取 39001，**保存后最多 1 分钟生效，无需重启**。
>
> 手动模式不受本配置影响，见 [Run.md](Run.md)（两者共用同一执行链）。
>
> 前端页面：`/Meta/BRS/Backup/Finv/Schedule`（`Web/src/views/Meta/BRS/Backup/Finv/BrsScheduleView.vue`）。

## 1. 读取调度配置

- **方法**：`GET`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Schedule/Get`

无查询参数。

### 响应（成功 HTTP 200）

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "cron_expr": "0 0 3 * * *",
    "remark": "秒 分 时 日 月 周；默认每天 03:00:00",
    "gmt_modify": "2026-10-29T03:37:36.754789+08:00",
    "next_runs": [
      "2026-11-02 03:00:00",
      "2026-11-03 03:00:00",
      "2026-11-04 03:00:00",
      "2026-11-05 03:00:00",
      "2026-11-06 03:00:00"
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `cron_expr` | string | 6 段 cron 表达式（秒 分 时 日 月 周） |
| `remark` | string | 只读：39001 的说明文本（迁移内置） |
| `gmt_modify` | string | 只读：最近修改时间 |
| `next_runs` | array<string> | 只读：未来 5 次执行时间（服务器本地时区，`YYYY-MM-DD HH:mm:ss`）；表达式非法时为空数组 |

## 2. 保存调度配置

- **方法**：`POST`
- **路径**：`/API/V1/Meta/BRS/Backup/Finv/Schedule/Save`

### 请求体（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `cron_expr` | string | ✅ | 6 段 cron 表达式；服务端用 `internal/meta/base/cron` 解析校验 |

支持语法：`*`、`?`（仅「日」「周」）、范围 `a-b`、步长 `*/n` / `a-b/n` / `a/n`、列表 `a,b,c`；周日为 0。

### 响应（成功 HTTP 200）

返回保存后的配置（结构同 Get，`next_runs` 已按新表达式重算）。

### 失败

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | 表达式为空；段数不为 6；字段越界或语法非法（消息含具体原因）；39001 配置行不存在 |
| 500 | 2006 | 数据库读写失败 |

## 3. 关联

| 项 | 说明 |
|------|------|
| 配置行 | `sys.sys_config` 39001（`V300060` 初始化，`data_type='3'`，`str_value`） |
| 调度实现 | `internal/meta/finv/brs/runner.go`（`Runner.Start` 每秒匹配、每 60 秒重读配置） |
| 执行链 | 与手动触发共用 `RunOnceWithHooks`，见 [Run.md](Run.md) |
| 备份范围 | `brs.brs_bak_rdb_table_full`（`flag_enable='1'`，`priority` 倒序），见 [BackupTable.md](BackupTable.md) |
