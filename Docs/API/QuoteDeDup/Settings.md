# 运行时设置（Settings）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：**`sys.sys_config` 配置项 `20006`**

低峰窗口保存在系统配置表 `sys.sys_config`（`config_no = 20006`），**精确到秒**，格式 `HH:MM:SS|HH:MM:SS`（半角竖线分隔起止）。读取带 30 秒 TTL 缓存，保存后立即失效缓存，**无需重启服务**。

> 历史沿革：窗口曾保存在 `dedup.finv_dedup_runtime_setting`（键值表，`V43` 创建）。该表仅为承载两个时间而建，已于 `V100041` 删除，配置迁入 `sys.sys_config 20006`；环境变量 `FINV_QUOTE_DEDUB_WINDOW_START/_END` 同时废弃、不再读取。

## GET /Settings（查询）

响应 `data`：

| 字段 | 类型 | 可空 | 说明 |
| --- | --- | --- | --- |
| window_start | string | 否 | 当前生效的低峰窗口开始，`HH:MM:SS` |
| dynamic_job_retention_days | int | 否 | 历史运行动态 JOB 实例保留天数（`sys.sys_config` 20008，默认 30，越界回退 30） |
| window_end | string | 否 | 当前生效的低峰窗口结束，`HH:MM:SS` |
| source | string | 否 | `sysconfig`（来自 20006 且格式合法）/ `default`（20006 缺失或非法，已回退内置默认 `00:30:00\|06:30:00`） |
| updated_at | string | 是 | 配置快照时间；读配置失败时为 null |
| time_zone | string | 否 | 业务时区 |
| in_window | bool | 否 | 当前时刻是否处于窗口内 |

响应示例：

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "window_start": "00:00:00",
    "window_end": "23:59:59",
    "source": "sysconfig",
    "updated_at": "2026-10-27T02:55:00.154917305Z",
    "time_zone": "Asia/Shanghai",
    "in_window": true
  }
}
```

## PUT /Settings/Window（修改低峰窗口）

请求体：

```json
{ "window_start": "01:00:00", "window_end": "06:00:00" }
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| window_start | string | 是 | **`HH:MM:SS`**（24 小时制，精确到秒） |
| window_end | string | 是 | **`HH:MM:SS`**（24 小时制，精确到秒） |

规则：

- **必须带秒**：`HH:MM` 形式会被拒绝（400），避免不同调用方对秒的默认值理解不一致；
- 支持跨零点（如 `22:00:00` ~ `06:00:00`）；
- 开始与结束相同表示全天；
- `00:00:00` ~ `23:59:59` 表示全天候；
- 保存后写入 `sys.sys_config` 20006（`str_value = "HH:MM:SS|HH:MM:SS"`），并立即失效配置缓存，**调度器与导出 Job 立即使用新窗口**；
- 成功时写一条管理员审计（`job_kind=MANUAL_ADMIN_EDIT`）；
- 响应 `data` 与 `GET /Settings` 一致。

错误：

| HTTP | 场景 |
|---|---|
| 400 | 请求体缺少字段；时间不是 `HH:MM:SS`；小时/分钟/秒越界 |
| 500 | 写 `sys.sys_config` 失败（如 20006 行缺失、数据库不可用） |

## 降级策略

读到 20006 但解析失败、或该行缺失时：

1. 服务**不阻断**，按内置默认 `00:30:00|06:30:00` 继续运行；
2. `GET /Settings` 的 `source` 返回 `default`，`GET /Status` 的 `options.window_start/window_end` 显示默认值；
3. 调度器启动日志会打印当前生效窗口与来源（`来源 sysconfig` / `来源 default`）。

## 多实例部署说明

配置在数据库中，各实例按 30 秒 TTL 自然刷新，**修改后最多 30 秒内在所有实例生效**（单容器 All-in-One 部署即时生效）。
