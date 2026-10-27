# 证券优先级 / SLA（Priority）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：**`dedup.finv_dedup_usc`**（V200051 起，原 `dedup.finv_dedup_usc_priority` 已合并进证券主表）

## 0. 语义（V100051 变更）

优先级 / SLA 不再是一张独立的稀疏表，而是证券主表上的**可选覆盖列**：

| 列 | 未配置（NULL）时的兜底值 |
|---|---|
| `priority_rank` | `100` |
| `sla_days` | `5` |
| `max_wait_days` | `7` |

约束 `finv_dedup_usc_priority_group_check` 保证三列**同时为 NULL 或同时有值**，不会出现半配置态。

`priority_gmt_update` 独立记录优先级最后修改时间，与主数据的 `gmt_update` 分离，便于审计。

## 1. GET /Priority/List（查询已配置覆盖的证券，不分页）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码精确匹配 |

只返回 `priority_rank IS NOT NULL` 的行。未配置的证券不会出现在此列表，需查 `GET /Usc/Page`。

响应 `data`：`{ total, list: [{ usc, priority_rank, sla_days, max_wait_days, priority_gmt_update }] }`

> 注意：`gmt_update` 已改名为 `priority_gmt_update`。

## 2. PUT /Priority（设置覆盖）

请求体：`{ usc, priority_rank, sla_days, max_wait_days }`

约束：`priority_rank > 0`、`sla_days >= 0`、`max_wait_days >= 0`。

| 状态码 | 场景 |
|---|---|
| `200` | 已设置覆盖 |
| `400` | 请求体格式错误 / `usc` 为空 / 数值越界 |
| `404` | `usc` 在 `finv_dedup_usc` 中不存在（**V100051 收紧**：不能再给未登记证券单独建优先级行） |
| `500` | 其他写入失败 |

## 3. DELETE /Priority?usc=xxx（清除覆盖）

等价于把三列置 NULL，退回默认 `100 / 5 / 7`。**证券行本身保留**。

| 状态码 | 场景 |
|---|---|
| `200` | 覆盖已清除 |
| `400` | `usc` 为空 |
| `404` | `usc` 不存在 |

## 4. 前端入口（V200051 起）

归档主数据页不再有独立的「优先级 / SLA」标签页。优先级在**证券主数据列表**中内联展示与编辑：

- 列表列：`优先级` / `SLA 天数` / `最大等待`
- 未配置时显示「默认 100」/「未配置(5)」/「未配置(7)」并置灰
- 行尾「优先级」按钮打开对话框，可设置或清除覆盖
