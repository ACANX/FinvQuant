# POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/RmbSync — 人民币计价指标手动补全

> 所属：行情 → 实时行情 → 证券行情排行（`/Meta/Finv/Quote/Realtime/FTStockRank`）
> 数据表：`cdeu.finv_cdeu_secu_realtime_stock`（人民币字段：`V300126` / 需求 11023）
> 关联需求：11023（加列）、11024（本接口与前端按钮）
> 实现：`internal/api/handler/cdeu_realtime_stock.go`（`RealtimeStockRmbSync`）、
> `internal/meta/finv/quote/cdeu/store_rmb_metrics.go`（`SyncRMBMetrics`）
> 机器可读契约：[`RealtimeStockRmbSync.yml`](RealtimeStockRmbSync.yml)（OpenAPI 3.1）

## 1. 概述

给快照表逐行补上**人民币计价**字段，供后续按人民币口径做证券排行。分两步，可一次执行也可分开执行：

1. **rate**：按行内 `currency` 求「1 单位本币 = N 人民币」的汇率，写入 `rmb_exchange_rate`；
2. **amount**：用该汇率把 `market_value` / `circulation_market_value` / `trade_trunover` / `price_nominal`
   换算成人民币，写入 `rmb_market_value` / `rmb_circulation_market_value` / `rmb_trade_trunover` / `rmb_price_nominal`。

汇率来源是缺省汇率表 `public.finv_fxcer`（`GET .../Fx/Cer`），按下列顺序回退：

| 顺序 | 规则 |
|---|---|
| 1 | `CNY` 自身恒为 `1`（不查表，避免汇率表暂缺 CNY 行时把人民币证券算成空） |
| 2 | 直采 `(currency, CNY)` |
| 3 | 反向 `(CNY, currency)` 取倒数 |
| 4 | 经 `USD` 交叉：`currency→USD` × `USD→CNY`（每一腿各自允许反向取倒数） |

汇率乘除用有理数（`math/big.Rat`）计算、按 10 位小数落库，不经过 `float64`。
查不到汇率的币种**不写任何值**，在响应的 `missing_currencies` 里列出；
`currency` 为空的行无法换算，计入 `rows_currency_missing`。

## 2. 请求

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `step` | query | string | 否 | `all` | `all`=先 rate 后 amount；`rate`=只补 `rmb_exchange_rate`；`amount`=只补 `rmb_` 金额/价格字段（需已有汇率） |

无请求体（`POST` 仅表示写操作，Body 可为空对象 `{}`）。

`step` 取值非法（非 `all`/`rate`/`amount`）返回 `400`。

## 3. 响应

```json
{
  "code": 0,
  "message": "人民币计价指标已更新",
  "data": {
    "step": "all",
    "rows_total": 13906,
    "rows_currency_missing": 5235,
    "rate_rows": 8671,
    "rate_update": 8671,
    "amount_rows": 8671,
    "amount_update": 8671,
    "rates": { "CNY": "1.0000000000", "HKD": "0.8516000000", "USD": "6.6760000000" },
    "missing_currencies": [],
    "elapsed_ms": 252
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.step` | string | 实际执行的步骤：`all` / `rate` / `amount` |
| `data.rows_total` | integer | 快照表总行数 |
| `data.rows_currency_missing` | integer | `currency` 为空、无法换算的行数（这些行的 `rmb_*` 保持 NULL） |
| `data.rate_rows` | integer | 补全后已有 `rmb_exchange_rate` 的行数 |
| `data.rate_update` | integer | 本次实际写入 `rmb_exchange_rate` 的行数（值未变不计） |
| `data.amount_rows` | integer | 补全后已有任一 `rmb_` 金额/价格字段的行数 |
| `data.amount_update` | integer | 本次实际写入 `rmb_` 金额/价格字段的行数（值未变不计） |
| `data.rates` | object | 本次使用的汇率（币种 → 10 位小数文本），便于前端提示 |
| `data.missing_currencies` | string[] | 查不到汇率的币种；为空表示全部币种都有汇率 |
| `data.elapsed_ms` | integer | 服务端耗时（毫秒） |

## 4. 语义与幂等

- **幂等**：两个步骤都是「差异更新」，重复点击第二次起 `rate_update` / `amount_update` 均为 `0`；
- **只补不伪**：本币值（`market_value` 等）为 NULL 时，对应的 `rmb_` 字段保持 NULL（不写 0）；
- **两步可分离**：先 `step=rate` 补汇率并人工核对（响应里的 `rates` 可直接看），确认后再 `step=amount`；
- **不删行、不改本币列**：接口只写 `rmb_*` 五个字段。

## 5. 错误

| 场景 | HTTP | `code` | 说明 |
|---|---|---|---|
| 未登录 / token 失效 | 401 | 401 | 与全站一致 |
| `step` 非法 | 400 | 400 | `step 非法（仅支持 all / rate / amount）: ...` |
| 数据库写入/查询失败 | 500 | 2006 | `人民币计价指标更新失败: ...` |

## 6. 前端

「行情 → 实时行情 → 证券行情排行」筛选行右侧的 **「人民币计价指标更新」** 按钮
（`mdi-currency-cny`，需求 11024）点击后调用本接口（`step=all`），
toast 提示 `汇率 N 行（累计 X/总）`、`金额 N 行（累计 Y）`、耗时，
以及「无币种行数」「缺汇率币种」等异常信息，随后自动刷新列表。

## 7. 核对命令

```bash
# 补全覆盖率
psql -U finv -d finv -c "SELECT currency, count(*),
       count(rmb_exchange_rate) AS with_rate, count(rmb_market_value) AS with_mv
  FROM cdeu.finv_cdeu_secu_realtime_stock GROUP BY 1 ORDER BY 1"
# 换算正确性（应为 t）
psql -U finv -d finv -c "SELECT stock_code, currency, market_value, rmb_exchange_rate, rmb_market_value,
       round(market_value * rmb_exchange_rate, 6) = rmb_market_value AS ok
  FROM cdeu.finv_cdeu_secu_realtime_stock
 WHERE market_value IS NOT NULL LIMIT 5"
```
