# 待录入证券分页查询（RealtimeStockPendingPage）

> 接口：`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Pending`
> 需求：10995（待录入清单）、11145（按派生键先补 usc 再判断）
> 权限：需登录（内部接口）
> 关联菜单：[`Menu/Quote/Realtime/PendingSecu.md`](../../../Menu/Quote/Realtime/PendingSecu.md)

## 1. 用途

查询 `cdeu.finv_cdeu_secu_realtime_stock` 中 **补全后 `usc` 仍为空** 的快照行（需求 11145 起的口径）：

1. 由快照行派生键 `ftmm_symbol = upper(market_label || '.' || stock_code)`（英文字母统一大写，与
   `futu.finv_futu_secu_key.ftmm_symbol` 同口径）；
2. `LEFT JOIN futu.finv_futu_secu_key k ON k.ftmm_symbol = <派生键>`，取出 `k.usc` 作为补全值；
3. 判定 `COALESCE(NULLIF(btrim(t.usc),''), NULLIF(btrim(k.usc),'')) IS NULL` —— 即快照与键表都没有 usc 才算待录入；
   键表里已登记的证券**不进这份清单**。

响应里 `usc` 是补全后的值（清单内恒为空串），`ftmm_symbol` 是派生键，便于核对「本该对应哪个键」。

采集会把所有在售证券写入快照表，`usc` 为空表示该证券尚未录入证券主数据
（新上市 / 未纳入采集范围）。这份清单是后续补录两张主数据表的依据：

| 表 | 职责 |
|----|------|
| `public.finv_security` | 证券主数据（含 `currency_type`、`exchange_code`） |
| `futu.finv_futu_quote_collect` | futu 采集配置（`stock_id ↔ usc`） |

> `cdeu.finv_quote_cdeu_usc` **不在补录范围**（需求 11147）：该表数据由运行时采集链路动态生成，
> 人工录入没有意义，页面也不再展示。

> **`usc` 为空是刻意保留的观测信号**：不得用 `stock_code` 回填——两者并非等价，
> 直接挪用会把不同市场的同码证券映射错（如 `000001` 在 SZ 与 SH 都存在）。

本接口**只提供查询**，不改库。要把 `usc` 真正写回快照表，用配套的补录接口
[`POST /Realtime/Stock/USCBackfill`](RealtimeStockUSCBackfill.md)（需求 11146，页面「手动补录USC」按钮）；
「一键录入（写三张主数据表）」仍为规划功能，尚未实现。

## 2. 请求

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|------|
| `page` | query | integer | 否 | `1` | 页码，从 1 开始 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1..500 |
| `keyword` | query | string | 否 | 空 | 匹配 `stock_code` / `name` / **派生 `ftmm_symbol`**（ILIKE） |
| `marketLabel` | query | string | 否 | 空 | `HK` / `SH` / `SZ` / `BJ` / `US`；空或 `all` 表示不限 |
| `orderBy` | query | string | 否 | `market_value` | 排序字段白名单：`stock_id` / `market_value` / `circulation_market_value` / `trade_trunover` / `trade_volumn` / `change_ratio` / `change_price` / `trust_aum` / `volumn_ratio` / `dt_update` / `page` |
| `order` | query | string | 否 | `desc` | `desc` / `asc` |

## 3. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 1151,
    "page": 1,
    "pageSize": 50,
    "list": [
      {
        "stock_id": 86002425217944,
        "usc": "",
        "ftmm_symbol": "HK.06288",
        "name": "FAST RETAIL-DRS",
        "stock_code": "06288",
        "market_type": "1",
        "market_label": "HK",
        "instrument_type": "3",
        "currency": "HKD",
        "price_nominal": "34500.000000",
        "change_price": "300.000000",
        "change_ratio": "0.88",
        "price_direct": "+",
        "market_value": "1240000000000.000000",
        "trade_trunover": "1200000000.000000",
        "trade_volumn": "35000.000000",
        "page": 1,
        "dt_update": 1793826253136
      }
    ]
  }
}
```

| 字段 | 说明 |
|------|------|
| `data.total` | 待录入证券总数（应用同样筛选条件后统计） |
| `data.list[].usc` | 恒为空串/`null`（本接口按 `usc IS NULL` 过滤） |
| `data.list[].page` | 来源页码（采集时的接口页码，从 1 开始） |
| `data.list[].dt_update` | 快照写入时间（Unix 毫秒） |

## 4. 错误

| HTTP | code | 场景 |
|------|------|------|
| 400 | 400 | `page` / `pageSize` / `order` 非法，或 `orderBy` 不在白名单 |
| 401 | 401 | 未登录 |
| 500 | 2006 | 数据库查询失败 |
