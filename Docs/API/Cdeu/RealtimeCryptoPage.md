# GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page — 加密货币行情排行分页查询

> 所属：行情 → 实时行情 → 加密货币行情排行
> 数据表：`cdeu.finv_quote_secu_realtime_crypto`（主键 `usc`，建表 `V300225`，列注释 `V300226`）
> 关联需求：11323
> 实现：`internal/api/handler/cdeu_realtime_crypto.go`（`RealtimeCryptoPage`）、
> `internal/meta/finv/quote/cdeu/store_realtime_crypto.go`（`ListRealtimeCryptos`）
> 配套接口：[`GET .../Realtime/Crypto/QuoteCurrencies`](#5-计价币种下拉)

## 1. 概述

分页查询加密货币行情快照（一行一币种，主键 `usc`），用于「加密货币行情排行」页面：

- 筛选：`keyword`（模糊匹配 `usc` / `code` / `name`）、`quoteCurrency`、`secStatus`、`suspension`；
- 排序：白名单列 + `asc`/`desc`，空值（NULL）一律排在最后；默认按涨跌幅 `change_ratio` 降序；
- 数值列一律以**文本**返回（NUMERIC 精度不丢），万/亿折算与小数位裁剪由前端负责；
- `suspension` 以 `'true'` / `'false'` 文本返回，前端渲染为「停牌 / 正常」。

## 2. 请求参数

### 2.1 Header 与 Query

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `usc` / `code` / `name`（ILIKE，`%kw%`） |
| `quoteCurrency` | query | string | 否 | 空 | 计价币种（如 `USD`/`USDC`/`USDT`）；空或 `all` 不限 |
| `secStatus` | query | string | 否 | 空 | 证券状态原文（如 `NORMAL`）；空或 `all` 不限 |
| `suspension` | query | string | 否 | 空 | `1` = 停牌、`0` = 未停牌；空或 `all` 不限；其它值 → HTTP 400 |
| `orderBy` | query | string | 否 | `change_ratio` | 排序字段，白名单见 2.2 |
| `order` | query | string | 否 | `desc` | `desc` / `asc` |
| `page` | query | integer | 否 | `1` | 页码，从 1 开始 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1–500 |

### 2.2 排序白名单（`orderBy`）

`price_nominal`、`change_price`、`change_ratio`、`price_amplitude`、`trade_volumn`、
`trade_trunover`、`volumn_ratio`、`buysell_ratio`、`bid_price`、`ask_price`、
`highest52weeks`、`lowest52weeks`、`dt_update`、`code`、`usc`。

白名单外的值 → HTTP 400（`code=400`）。

## 3. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 8,
    "page": 1,
    "pageSize": 50,
    "list": [
      {
        "usc": "PAXG",
        "code": "CC.PAXG",
        "name": "PAX Gold",
        "quote_currency": "",
        "price_nominal": "3781.540000",
        "price_open": "3837.500000",
        "price_highest": "3837.500000",
        "price_lowest": "3712.010000",
        "price_last_close": "3837.500000",
        "price_avg": "",
        "change_price": "-55.960000",
        "change_ratio": "-1.458241",
        "price_amplitude": "3.269985",
        "price_direct": "-",
        "trade_volumn": "",
        "trade_trunover": "",
        "volumn_ratio": "",
        "bid_price": "",
        "ask_price": "",
        "bid_vol": "",
        "ask_vol": "",
        "buysell_ratio": "",
        "highest52weeks": "",
        "lowest52weeks": "",
        "highest_history": "",
        "lowest_history": "",
        "sec_status": "NORMAL",
        "suspension": "false",
        "data_date": "2026-11-17",
        "provider": "MMAPI",
        "dt_update": 1794970226189
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `usc` | string | 主键（统一证券代码，如 `BTCUSD`、`PAXG`） |
| `code` | string | moomoo 代码（`CC.` 前缀，如 `CC.BTCUSD`） |
| `name` | string | 币种名称 |
| `quote_currency` | string | 计价币种（可能为空） |
| `price_nominal` | string | 现价 |
| `price_open` | string | 今开 |
| `price_highest` | string | 最高 |
| `price_lowest` | string | 最低 |
| `price_last_close` | string | 昨收 |
| `price_avg` | string | 均价（上游未提供时为空） |
| `change_price` | string | 涨跌额（`last - prev_close`） |
| `change_ratio` | string | 涨跌幅（百分点数值） |
| `price_amplitude` | string | 振幅（不是涨跌幅） |
| `price_direct` | string | 涨跌方向 `+` / `-` |
| `trade_volumn` | string | 成交量 |
| `trade_trunover` | string | 成交额 |
| `volumn_ratio` | string | 量比 |
| `buysell_ratio` | string | 买卖比 |
| `bid_price` | string | 买价 |
| `ask_price` | string | 卖价 |
| `bid_vol` | string | 买量 |
| `ask_vol` | string | 卖量 |
| `highest52weeks` | string | 52 周最高 |
| `lowest52weeks` | string | 52 周最低 |
| `highest_history` | string | 历史最高 |
| `lowest_history` | string | 历史最低 |
| `sec_status` | string | 证券状态（实测 `NORMAL`） |
| `suspension` | string | `true` 停牌 / `false` 正常 |
| `data_date` | string | 数据日期（`yyyy-MM-dd`） |
| `provider` | string | 数据提供方（`MMAPI`） |
| `dt_update` | int64 | 更新时间（Unix 毫秒） |

## 4. 错误

| HTTP | code | 场景 |
|---|---|---|
| 400 | 400 | `orderBy` 不在白名单、`order` 非 `asc`/`desc`、`suspension` 非 `0`/`1`、分页参数越界 |
| 401 | — | 未登录或 token 过期 |
| 500 | 2006 | 数据库错误 |

## 5. 计价币种下拉

`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Crypto/QuoteCurrencies`

返回表内 `DISTINCT quote_currency`（去空、按字母序）：

```json
{ "code": 0, "message": "查询完成", "data": { "list": ["USD", "USDC", "USDT"] } }
```
