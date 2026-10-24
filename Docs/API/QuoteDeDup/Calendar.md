# 交易日历（Calendar）

> 前缀：`/API/V1/Meta/Finv/Quote/DeDup`；数据源：`dedup.finv_dedup_trade_calendar`

## 1. GET /Calendar/Page（查询）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| usc | string | 否 | 证券代码 |
| bizMonth | int | 否 | `yyyymm`，按月过滤 |
| page / pageSize | int | 否 | 默认 1 / 20 |

响应 `data`：`{ total, list: [{ usc, trade_date, flag_trade_day }] }`（`trade_date` 为 `yyyymmdd` 整数）

## 2. PUT /Calendar（新增或更新）

请求体：`{ usc, trade_date, flag_trade_day }`；`trade_date` 需为合法 `yyyymmdd`。

## 3. DELETE /Calendar?usc=xxx&tradeDate=yyyymmdd（删除）
