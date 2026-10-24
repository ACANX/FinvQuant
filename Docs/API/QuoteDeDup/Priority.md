# 证券优先级 / SLA（Priority）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_usc_priority`

## 1. GET /Priority/List（查询，不分页）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码精确匹配 |

响应 `data`：`{ total, list: [{ usc, priority_rank, sla_days, max_wait_days, gmt_update }] }`

## 2. PUT /Priority（新增或更新）

请求体：`{ usc, priority_rank, sla_days, max_wait_days }`

约束：`priority_rank > 0`、`sla_days >= 0`、`max_wait_days >= 0`。

## 3. DELETE /Priority?usc=xxx（删除）

`usc` 不存在时返回 `409`。
