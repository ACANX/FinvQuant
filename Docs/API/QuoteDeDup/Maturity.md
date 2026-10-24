# 成熟度（Maturity）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_quote_maturity_mon`（只读，由 `REFRESH_QUOTE_MATURITY` 维护）

## 1. GET /Maturity/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码 |
| status | string | 否 | `IMMATURE` / `PARTIAL` / `COMPLETE` / `FROZEN` |
| bizMonth | int | 否 | `yyyymm` |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ usc, trade_date, biz_month, expected_rows, actual_rows, maturity_status, gmt_first_seen, gmt_last_arrival, gmt_seale, gmt_update }] }`

## 2. POST /Maturity/Refresh（立即重算）

异步触发 `REFRESH_QUOTE_MATURITY`，返回 `202`；执行结果通过 `Status` 与 `RunLog/Page` 观察。
