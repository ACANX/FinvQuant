# 每日预算（Budget）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_daily_budget`

## 1. GET /Budget/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| dateFrom / dateTo | string | 否 | `yyyy-MM-dd`，按 `biz_date` 过滤 |
| status | string | 否 | `READY` / `PAUSED` 等 |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ biz_date, base_rows, base_bytes, p75_rows, backlog_ratio, sla_pressure, factor, budget_rows, budget_bytes, hard_cap_rows, consumed_rows, consumed_bytes, status, gmt_create }] }`

## 2. PUT /Budget/Override（管理员覆盖当日预算）

请求体：`{ biz_date?, budget_rows, hard_cap_rows, factor, status? }`

- `biz_date` 缺省为业务时区「今天」；
- 约束：`budget_rows > 0`、`hard_cap_rows >= budget_rows`、`factor` 取值 1.00 ~ 3.00；
- 仅影响「启动新任务」的额度判定，不会中断已启动任务；
- 成功后写审计。
