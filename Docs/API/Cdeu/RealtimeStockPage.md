# GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page — 实时行情分页查询

> 所属：行情 → 实时行情 → 实时行情
> 数据表：`cdeu.finv_cdeu_secu_realtime_stock`（建表 `V300115`，数值列 `V300117`/`V300118`/`V300119`）
> 关联需求：10962（菜单）、10956–10961（表与写入）、11023（表增 `rmb_*` 列）、11024（补全接口）、11025（人民币口径展示与排行）
> 实现：`internal/api/handler/cdeu_realtime_stock.go`（`RealtimeStockPage`）、
> `internal/meta/finv/quote/cdeu/store_realtime_stock.go`（`ListRealtimeStocks`）
> 机器可读契约：[`RealtimeStockPage.yml`](RealtimeStockPage.yml)（OpenAPI 3.1）

## 1. 概述

分页查询证券实时行情快照（一行一证券，主键 `stock_id`），用于「行情 → 实时行情」页面：

- 筛选：`keyword`（模糊匹配 `usc` / `stock_code` / `name`）、`marketLabel`、`currency`；
- 排序：白名单列 + `asc`/`desc`，空值（NULL）一律排在最后；**默认按人民币总市值 `rmb_market_value` 降序**（需求 11025，未补全的行排最后）；
  白名单只含「指标」列——`rmb_exchange_rate` 是换算参数而非指标，**不参与排序**（仍随行返回用于展示，需求 11027）；
- 数值列（市值、股本、成交、AUM、涨跌幅等）以**文本**返回，保住 `NUMERIC(24,6)` 精度，
  万/亿折算与小数位裁剪由前端负责。

## 2. 请求参数

### 2.1 Header 与 Query

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `usc` / `stock_code` / `name`（ILIKE，`%kw%`） |
| `marketLabel` | query | string | 否 | 空 | 市场：`HK`/`SH`/`SZ`/`BJ`/`US`；空或 `all` 不限 |
| `currency` | query | string | 否 | 空 | 币种：`CNY`/`HKD`/`USD`；空或 `all` 不限 |
| `orderBy` | query | string | 否 | `rmb_market_value` | 排序字段，白名单见 2.2 |
| `order` | query | string | 否 | `desc` | `desc`（降序）/ `asc`（升序） |
| `page` | query | integer | 否 | `1` | 页码，从 1 开始 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1–500 |

### 2.2 排序字段白名单

| 取值 | 说明 |
|---|---|
| `rmb_market_value` | 人民币总市值（**默认**，需求 11025） |
| `rmb_circulation_market_value` | 人民币流通市值 |
| `rmb_trade_trunover` | 人民币成交额 |
| `rmb_price_nominal` | 人民币单价 |
| `market_value` | 总市值（本币计价） |
| `circulation_market_value` | 流通市值 |
| `trade_trunover` | 成交额 |
| `trade_volumn` | 成交量 |
| `change_ratio` | 涨跌幅 |
| `change_price` | 涨跌额 |
| `trust_aum` | 信托/基金资产规模（元） |
| `volumn_ratio` | 量比 |
| `dt_update` | 快照更新时间 |
| `stock_id` | 证券编号 |

`orderBy` 传白名单之外的值 → HTTP 400（不允许拼接任意列名）。

## 3. 响应

成功（HTTP 200）：

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 13045,
    "page": 1,
    "pageSize": 2,
    "list": [
      {
        "stock_id": 5,
        "usc": "00005",
        "name": "汇丰控股",
        "stock_code": "00005",
        "market_type": "1",
        "market_label": "HK",
        "instrument_type": "3",
        "currency": "HKD",
        "price_nominal": "73.700000",
        "change_price": "0.750000",
        "change_ratio": "1.030000",
        "price_direct": "+",
        "market_value": "2510000000000.000000",
        "total_shares": "17184000000.000000",
        "circulation_total_shares": "17184000000.000000",
        "trade_trunover": "1580000000.000000",
        "trade_volumn": "10893400.000000",
        "circulation_market_value": "2510000000000.000000",
        "trust_aum": "0.000000",
        "trade_change_ratio": "0.120000",
        "price_amplitude": "1.300000",
        "volumn_ratio": "1.150000",
        "buysell_ratio": "0.000000",
        "leverage": "",
        "rmb_exchange_rate": "0.8516000000",
        "rmb_market_value": "2137600000000.000000",
        "rmb_circulation_market_value": "2137600000000.000000",
        "rmb_trade_trunover": "1345528000.000000",
        "rmb_price_nominal": "62.763920",
        "dt_update": 1781619897000
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.total` | integer | 满足筛选条件的总行数 |
| `data.page` | integer | 当前页码 |
| `data.pageSize` | integer | 每页条数 |
| `data.list[]` | array | 当前页行，字段与表列同名（snake_case） |
| `data.list[].price_direct` | string | 涨跌方向：`+` / `-` / `0` |
| `data.list[].*`（数值列） | string | 十进制文本；空列返回空串 `""` |
| `data.list[].rmb_exchange_rate` | string | 人民币汇率（1 单位本币 = N 人民币，需求 11023/11025）；**空串表示尚未补全**（库里 NULL），不是 0 |
| `data.list[].rmb_market_value` | string | 人民币计价总市值（元）；同上，空串=未补全 |
| `data.list[].rmb_circulation_market_value` | string | 人民币计价流通市值（元） |
| `data.list[].rmb_trade_trunover` | string | 人民币计价成交额（元） |
| `data.list[].rmb_price_nominal` | string | 人民币计价单价（元/股） |
| `data.list[].remark` | string | 备注（需求 11028）：人工标注/采集端说明；空串表示无备注，不参与换算与排序 |
| `data.list[].gmt_create` | string | 记录创建时间（RFC 3339，需求 11029）：入库时刻，此后不变 |
| `data.list[].gmt_update` | string | 记录更新时间（RFC 3339，需求 11029）：每次 UPDATE 由触发器自动刷新 |
| `data.list[].dt_update` | integer | Unix 毫秒；无值时 `0` |

## 4. 错误目录

| `code` | HTTP | retryable | 触发场景 |
|---|---|---|---|
| `400` | 400 | 否 | `page` / `pageSize` 非整数或越界（`page` ≥ 1，`pageSize` 1–500） |
| `400` | 400 | 否 | `order` 不是 `asc` / `desc` |
| `400` | 400 | 否 | `orderBy` 不在白名单内 |
| `2006` | 500 | 是 | 数据库查询失败 |

## 5. 调用示例

```bash
curl -sS -G 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page' \
  -H "Authorization: Bearer $TOKEN" \
  --data-urlencode 'keyword=汇丰' \
  --data-urlencode 'marketLabel=HK' \
  --data-urlencode 'orderBy=rmb_market_value' \
  --data-urlencode 'order=desc' \
  --data-urlencode 'page=1' \
  --data-urlencode 'pageSize=20'
```

## 6. 已使用位置

| 使用方 | 说明 |
|---|---|
| 行情 → 实时行情 → 证券行情排行（`/Meta/Finv/Quote/Realtime/FTStockRank`） | `Web/src/views/Meta/Finv/Quote/Realtime/RealtimeStockView.vue`（见 [Menu/Quote/Realtime/RealtimeStock.md](../../Menu/Quote/Realtime/RealtimeStock.md)） |

## 7. 附录：OpenAPI 3.1 契约

机器可读契约见同目录 [`RealtimeStockPage.yml`](RealtimeStockPage.yml)（自包含）。

```bash
npx --yes @stoplight/spectral-cli lint Docs/API/Cdeu/RealtimeStockPage.yml
```
