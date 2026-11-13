# 日K线存量统计 API（Stat）

> 所属：行情 → 收剔导传ETL → 日K线数据同步
> 路径：`GET /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Stat`
> 数据表：`cdeu.finv_quote_secu_kline_day`
> 关联需求：10952（同步能力 10951，建表 10950）

## 0. 用途

查询单个 `usc` 在 `cdeu.finv_quote_secu_kline_day` 里的存量：行数、日期范围、来源（`provider`）。
供同步前核对基线、同步后确认落库结果。

## 1. 请求

`GET /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Stat?usc=GCMain`

查询参数（小驼峰）：

| 参数 | 类型 | 必填 | 说明 |
|------|------|:---:|------|
| `usc` | string | 是 | 证券唯一编码；为空返回 400 |

## 2. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "usc": "GCMain",
    "rows": 12789,
    "date_from": 20000104,
    "date_to": 20261102,
    "providers": "FTMM"
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `rows` | int | 该 usc 的日K行数 |
| `date_from` | int | 最小交易日（`yyyyMMdd`；无数据为 0） |
| `date_to` | int | 最大交易日（`yyyyMMdd`；无数据为 0） |
| `providers` | string | 涉及的数据来源，逗号分隔（如 `FTMM`；多来源时为 `FTMM,其他`） |

## 3. 错误码

| `code` | HTTP | 触发场景 |
|--------|------|----------|
| `400` | 400 | `usc` 为空 |
| `2006` | 500 | 查询失败（数据库错误） |

## 4. 已使用位置登记

| 业务菜单 | 菜单文档 |
|----------|----------|
| 日K线数据同步（`/Meta/Finv/Quote/Cdeu/DayKline/Sync`，视图 `DayKlineSyncView.vue`） | [Docs/Menu/Quote/Cdeu/DayKlineSync.md](../../Menu/Quote/Cdeu/DayKlineSync.md) |
