# 缺省汇率（FxCer）

> 接口：
> - `POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync` —— 手动更新（同步执行）
> - `GET /API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer` —— 整表查询
> 需求：10999（接口与菜单）、11000（双模式/中文名称/路由）、11001（双来源互补）、11002（表 v2：usc/name 与列改名）；表结构 10998（v1）/11002（v2）
> 权限：需登录（内部接口，暂不对外暴露 OpenAPI）
> 关联表：[`public.finv_fxcer`](../../../DataDictMapping/FinvFxcer.md)（迁移 `V300124`；`public.finv_fxcer` 为 v1，已停写保留）
> 关联菜单：[`Menu/Quote/Realtime/FxCer.md`](../../../Menu/Quote/Realtime/FxCer.md)（路由 `/Meta/Finv/Quote/Realtime/FXCEROverview`）

## 1. 用途

把 `public.finv_fxcer`（缺省汇率表）维护成「多货币环境下金额统一换算」的唯一权威来源。

- 数据源：MetaExt `quoteapiv2.GetFxOverview`（moomoo `/quote-api/quote-v2/get-fx-overview`），
  一次返回 **72 个货币对** = 9 币种（`CNH`/`USD`/`HKD`/`EUR`/`GBP`/`AUD`/`SGD`/`CAD`/`JPY`）两两组合；
- 口径：`rate` = 1 单位 `currency_src` 折合的 `currency_dest` 数量（`USDCNH=6.67333` 即 1 USD = 6.67333 CNH）；
- 人民币：`get-fx-overview` 只提供离岸 `CNH`；`CNY` 相关货币对由 `get-fx-rate-list` **直采**，
  **不再按 `CNH` 同价派生**（需求 11020：派生值等于离岸价、不等于在岸真实价，会污染换算与回测口径），
  因此概览来源一轮只落 72 行，`derive` 恒为 0；
- 更新节奏：当前**仅支持本接口手动触发**，定期更新后续实现；表只保最新、不建历史表。

## 2. 手动更新

### 2.1 请求

`POST /API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync?source=overview|rateList`，无请求体。

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|------|------|------|------|------|------|
| `source` | query | string | 否 | `overview` | 采集来源：`overview`=get-fx-overview（9 币种两两完整矩阵，72 对，只落直采）；`rateList`=get-fx-rate-list（`listId=1`、`page=0`、`pageSize=500`，币种更多，补齐概览缺失的货币对） |

两个来源写同一张表（`public.finv_fxcer`）、互不删除对方数据：同一来源内按「新 `ts_quote` 覆盖旧」、跨来源以最后一次采集为准，
因此可以先用概览铺满矩阵、再用列表补长尾币种。前端矩阵模式触发 `overview`、表格模式触发 `rateList`。

> **`rateList` 的涨跌字段（需求 11016）**：上游条目另带 `changeAmount`（涨跌额，带符号）与
> `changePercent`（涨跌幅，含 `%`），采集时去掉百分号、去掉千分位与空白后落库；
> 解析失败或缺失时该行两列留空（不因单条异常丢弃整条汇率）。`overview` 接口**不提供**这两个字段，
> 写入时用 `COALESCE` 保留已有值，因此「先用列表采集、再用矩阵采集」不会把涨跌数据清空。
>
> **`rateList` 的条目过滤**：该列表混有非货币对条目（美元指数 `USDindex`、欧元指数 `EURindex` 等）
> 与可能的缺价条目，这些逐条**跳过并计入 `data.skipped`**，不会让整批失败；只有全部条目都不可用时才返回 500。
>
> **`rateList` 的降级分页**：先按 `pageSize=500` 单页请求；实测上游对大 `pageSize`（300/500）会返回
> `code=500`「Cannot read properties of undefined (reading 'spreadTableCode')」，且 `data` 是数组，
> MetaExt v0.1.2 在解析阶段就报错。此时服务端自动降级为 `pageSize=50` 逐页拉取（最多 20 页，页间 300 ms），
> `data.pages` 记录实际请求页数；若中途某页失败则使用已获取的部分。

```bash
# 矩阵模式：完整矩阵
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync?source=overview' \
  -H 'Authorization: Bearer <token>'

# 表格模式：全量汇率列表（该请求不需要 `_` 时间戳参数）
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Fx/CerSync?source=rateList' \
  -H 'Authorization: Bearer <token>'
```

### 2.2 响应

```json
{
  "code": 0,
  "message": "汇率已更新",
  "data": {
    "source": "overview",
    "pages": 1,
    "rows_receive": 88,
    "skipped": 0,
    "derive": 16,
    "insert": 0,
    "update": 88,
    "ts_quote_max": 1794002398409,
    "elapsed_ms": 1453
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `data.source` | string | 本次采集来源：`overview` / `rateList` |
| `data.pages` | integer | 本次实际请求页数：`overview` 固定 1；`rateList` 单页成功为 1，降级分页时 > 1 |
| `data.skipped` | integer | 上游返回但被跳过的条目数（非货币对如 `USDindex`、缺价条目；`overview` 恒为 0） |
| `data.rows_receive` | integer | 本次组装出的行数（overview 固定 72；rateList 为可用条目数，如 280） |
| `data.derive` | integer | 派生行数；需求 11020 起恒为 `0`（字段保留以向后兼容） |
| `data.insert` | integer | 本次实际新增行数 |
| `data.update` | integer | 本次实际覆盖行数 |
| `data.ts_quote_max` | integer | 本次最大报价时间（Unix 毫秒） |
| `data.elapsed_ms` | integer | 耗时（毫秒） |

> `rateList` 来源的上游条目**不含报价时间**，`data.ts_quote_max` 与写入行的 `ts_quote` 均取本次采集时刻；
> `overview` 来源则使用接口返回的逐对 `time`。
>
> `insert + update` 可能小于 `rows_receive`：upsert 有两个跳过条件 ——
> ① **同一来源内**新报价 `ts_quote` 小于库中已有值（旧报价不覆盖新报价）；跨来源则以最后一次采集为准
>   （`overview` 用上游时间、`rateList` 用采集时刻，两个时钟不可比，只比时间会让先跑的 `rateList` 永久压住 `overview`）；
> ② 新行是派生行而库中已有同对直采行（历史兼容，派生不覆盖直采）。被跳过的行不计入统计。

### 2.3 错误

| 场景 | HTTP | code | message |
|------|------|------|---------|
| 上游请求失败（网络/超时/token 被拒/`code≠0`） | 500 | 2006 | `汇率更新失败: 拉取汇率概览失败: ...` |
| `source` 非 `overview`/`rateList` | 400 | 400 | `source 非法（仅支持 overview / rateList）: ...` |
| 上游数据口径异常（键非法、价格非正数、报价时间缺失） | 500 | 2006 | `汇率更新失败: 汇率数据非法: ...` |
| 未登录 | 401 | 401 | 统一鉴权错误 |

> 上游数据异常不写半批数据：72 对全部校验通过后才发一条 upsert 语句，单语句失败即整批回滚。

## 3. 整表查询

### 3.1 请求

`GET /API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer`，无参数（88 行量级，不分页）。

```bash
curl -sS 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Fx/Cer' \
  -H 'Authorization: Bearer <token>'
```

### 3.2 响应

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "total": 88,
    "list": [
      {
        "usc": "AUDCAD",
        "name": "澳元/加元",
        "currency_src": "AUD",
        "currency_dest": "CAD",
        "rate": "0.9120000000",
        "change_price": "-0.0025000000",
        "change_ratio": "-0.274000",
        "ts_quote": 1794002398409,
        "source": "moomoo.get-fx-overview",
        "flag_derive": "0",
        "gmt_update": "2026-11-08T14:31:02.123456+08:00"
      },
      {
        "usc": "CNYUSD",
        "name": "人民币/美元",
        "currency_src": "CNY",
        "currency_dest": "USD",
        "rate": "0.1396000000",
        "ts_quote": 1794153633897,
        "source": "moomoo.get-fx-rate-list",
        "flag_derive": "0",
        "remark": "",
        "gmt_update": "2026-11-08T14:31:02.123456+08:00"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `data.total` | integer | 行数；`0` 表示从未手动更新过 |
| `data.currency_names` | object | 币种简体中文名（展示用，需求 11000）：键为币种代码、值为名称，如 `{"CNY":"人民币","CNH":"离岸人民币","USD":"美元",...}` |
| `data.list[].usc` | string | 上游标识（`stockCode` / 概览 map key） |
| `data.list[].name` | string | 展示名称（简体中文，如 `美元/日元`） |
| `data.list[].currency_src` | string | 源币种（ISO 4217 alpha-3；离岸人民币 `CNH`） |
| `data.list[].currency_dest` | string | 目标币种 |
| `data.list[].rate` | string | 缺省汇率原文（`NUMERIC(24,10)::text`，保精度故不作 JSON 数值） |
| `data.list[].change_price` | string | 涨跌额（带符号，`NUMERIC(24,10)::text`）：仅 `rateList` 来源有值，`overview` 来源与从未用列表采集过的行为空、字段省略（需求 11016） |
| `data.list[].change_ratio` | string | 涨跌幅（百分数数值，`-0.274` 表示 `-0.274%`，`NUMERIC(12,6)::text`）：同上，`overview` 来源为空、字段省略 |
| `data.list[].ts_quote` | integer | 报价时间（Unix 毫秒）：`overview` 为上游逐对时间，`rateList` 为采集时刻 |
| `data.list[].source` | string | `moomoo.get-fx-overview` / `moomoo.get-fx-rate-list` / `derive:CNH` |
| `data.list[].flag_derive` | string | `0`=直采、`1`=派生（仅历史数据；需求 11020 起只会是 `0`） |
| `data.list[].remark` | string | 口径说明（可空） |
| `data.list[].gmt_update` | string | 最近更新时间（RFC 3339，可作批次时间） |

> `currency_names` 取服务端 `cdeu.FxCERCurrencyNames`（**不用** `public.finv_currency.currency_name`）：
> 该字典只有 7 条静态种子、缺 `CNH`/`EUR`/`GBP`/`AUD`/`CAD`，且不区分在岸/离岸人民币。
> `CNY`（人民币，在岸）与 `CNH`（离岸人民币）价格存在基差，展示时靠币种代码区分；未登记的币种由前端回落为代码。

排序：`usc` 升序。

## 4. 消费口径（换算建议）

查询 `A → B` 的三级回退（详见 [`DataDictMapping/FinvFxcer.md`](../../../DataDictMapping/FinvFxcer.md) §4）：

1. 直采行 `(A, B)`；
2. 反向行 `(B, A)` 取倒数；
3. 经 `USD` 交叉（`(A, USD) × (USD, B)`，两条都应是直采行）；
4. 都不命中时按 `public.finv_currency.exchange_rate_cny` 兜底并告警（该字典为静态种子，不可作为常规来源）。
