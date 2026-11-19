# GET /API/V1/Meta/Finv/Quote/Collect/FiveDayFt/{Page,FilterOptions} — 五日分钟行情采集配置查询

> 所属：行情 → 行情采集 → 五日分钟行情采集配置
> 数据表：Supabase `public.finv_quote_secu_ft`（**本地库无副本**，经 PostgREST 只读访问）
> 连接来源：`sys.sys_config` 20013 `{PROJECT_ID}|{APIKEY}`（见 [CollectSysConfig.md](CollectSysConfig.md)）
> 关联需求：11344
> 实现：`internal/api/handler/quote_collect_fiveday_ft.go`、`internal/meta/finv/quote/secuft/`、
> 通道复用 `internal/meta/finv/quote/secukey/client.go::FetchQuery`
> 关联菜单：[`Menu/Quote/RealtimeCollect/FiveDayFt.md`](../../Menu/Quote/RealtimeCollect/FiveDayFt.md)

## 1. 概述

本组两支只读接口服务「五日分钟行情采集配置」页面：把 Supabase 上的采集配置表 `finv_quote_secu_ft`
分页展示出来，供排障与核对。

| 接口 | 作用 |
|---|---|
| `GET .../FiveDayFt/Page` | 分页查询（关键字 + 4 个维度筛选 + 白名单排序） |
| `GET .../FiveDayFt/FilterOptions` | 下拉候选（表内 DISTINCT） |

两条硬口径：

1. **只读**：本服务不提供写接口。该表是采集链路的配置源，改动在 Supabase 侧进行。
2. **列元数据由服务端下发**：前端不硬编码列名，表结构变化只需改后端常量。

## 2. 表结构（实测 21 列）

> 探明方式（2026-11-19）：对 PostgREST 逐列 `select=<col>` 探测（存在返回 200，不存在返回
> `42703 column ... does not exist`），类型用 `?<col>=eq.@@@` 的类型转换报错反推。

| 列 | 中文表头 | 类型 | 说明 |
|---|---|---|---|
| `usc` | 统一编号 | text | 与本地 `finv_security.usc` 同口径 |
| `symbol` | 证券代码 | text | — |
| `futu_symbol` | 富途代码 | text | 富途侧证券代码原始形态 |
| `name` | 证券名称 | text | — |
| `stock_id` | 富途ID | bigint | 富途证券内部 ID |
| `market` | 市场 | text | 市场缩写（HK/SH/SZ/BJ/US 等） |
| `market_label` | 行情市场 | text | HK/SH/SZ/BJ/US/FX/CC |
| `market_type` | 市场类型 | bigint | 1 港股 / 2 美股 / 4 A股 / 11 外汇 / 17 加密 |
| `market_code` | 市场代码 | bigint | 富途市场代码（**不是** `finv_market.market_code`） |
| `instrument` | 品种 | text | stock/etfs/index/currency/crypto/futures/sectors |
| `instrument_type` | 品种类型 | bigint | 3 股票 / 4 ETF / 6 指数 / 7 板块 / 9 期货 / 10 外汇 / 11 加密 |
| `sub_instrument_type` | 品种子类型 | bigint | 如 3002 / 4002 / 6001 |
| `type_secu` | 证券分类 | text | 规范证券分类 |
| `type_secu_sub` | 证券子类 | text | 规范证券子类 |
| `req_section` | 订阅段 | integer | 富途行情订阅段，0 表示未设 |
| `provider` | 提供方 | text | 数据提供方 |
| `flag_enable` | 采集开关 | smallint | 1 启用 / 0 禁用 |
| `remark` | 备注 | text | — |
| `gmt_create` | 创建时间 | timestamptz | — |
| `gmt_update` | 更新时间 | timestamptz | — |
| `dt_update` | 数据更新 | timestamptz | 表内业务时间列 |

**当前数据量（2026-11-19 实测）**：`Content-Range: */0`，即可读但 **0 行**。
同一把 APIKEY 能读到 `public.finv_quote_secu_key` 的 73,212 行，所以不是鉴权问题；

- 若 Supabase 控制台里也看不到数据 → 表本来就是空的，等采集侧写入；
- 若控制台能看到数据而这里为 0 行 → 该表开了 RLS 且策略没向该 key 开放 `select`，
  需在 Supabase 侧补 `select` 策略。

## 3. 分页查询 `GET .../FiveDayFt/Page`

### 3.1 请求参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `page` | query | integer | 否 | `1` | 页码（≥1） |
| `pageSize` | query | integer | 否 | `15` | 每页条数，1–200（超出截断） |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `usc` / `symbol` / `futu_symbol` / `name` |
| `marketLabel` | query | string | 否 | 空 | 精确匹配 `market_label`；空或 `all` 不限 |
| `instrument` | query | string | 否 | 空 | 精确匹配 `instrument` |
| `typeSecu` | query | string | 否 | 空 | 精确匹配 `type_secu` |
| `provider` | query | string | 否 | 空 | 精确匹配 `provider` |
| `flagEnable` | query | string | 否 | 空 | `1` 启用 / `0` 禁用；其它值视为不限 |
| `sortBy` | query | string | 否 | `usc` | 白名单：21 列全部可排序，非法值回退 `usc` |
| `sortOrder` | query | string | 否 | `asc` | `asc` / `desc` |

### 3.2 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "list": [],
    "total": 0,
    "page": 1,
    "pageSize": 15,
    "table": "finv_quote_secu_ft",
    "elapsed_ms": 839,
    "columns": [
      { "name": "usc", "title": "统一编号", "kind": "text", "sortable": true, "note": "证券统一编码，与本地 finv_security.usc 同口径" }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.list` | array | 本页行，键为原表列名；`null` 原样保留 |
| `data.total` | integer | 满足条件的总行数（`Content-Range` 尾段；取不到为 0） |
| `data.page` | integer | 当前页（服务端收敛后的值） |
| `data.pageSize` | integer | 每页条数（服务端收敛后的值） |
| `data.table` | string | 源表名 |
| `data.elapsed_ms` | integer | 请求 Supabase 耗时 |
| `data.columns` | array | 列元数据（`name` / `title` / `kind` / `sortable` / `note`） |

### 3.3 错误

| `code` | HTTP | 触发场景 |
|---|---|---|
| `2007` | 502 | Supabase 不可达 / 20013 未配置 / 鉴权失败 / 表不存在 |
| `401` | 401 | 未登录 |

**特殊处理：offset 越界不是错误。** PostgREST 在 `offset` 超过总行数时返回
`416` + `{"code":"PGRST103","message":"Requested range not satisfiable"}`；
服务端把它当作「本页为空」，`total` 取 `Content-Range` 尾段原样返回，
因此翻到越界页得到的是 `list=[]` 而不是 502。

## 4. 下拉候选 `GET .../FiveDayFt/FilterOptions`

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "options": { "marketLabel": [], "instrument": [], "typeSecu": [], "provider": [] },
    "elapsed_ms": 793
  }
}
```

按 `market_label` / `instrument` / `type_secu` / `provider` 各拉一次 DISTINCT（每维上限 500），
表为空时各维度为空数组，页面下拉只有「全部」选项。

## 5. 排序与分页稳定性

- 排序列追加 `usc` 作为兜底键（`sortBy=usc` 时不重复追加），避免同值行翻页时重复或丢失。
- 前端每页条数选项 `15 / 20 / 50 / 100 / 200`，默认 15（需求指定）。

## 6. 实测记录（2026-11-19）

| 场景 | 结果 |
|---|---|
| `GET .../FiveDayFt/Page` | `code=0`，`pageSize=15`，`total=0`，`columns=21`，`elapsed_ms=839` |
| `Page?page=2`（越界） | 修复前 502（416 透传）；修复后 `code=0`、`list=[]`、`total=0` |
| `Page?pageSize=1000` | `pageSize=200`（截断生效） |
| `Page?sortBy=evil` | 回退 `usc`，`code=0` |
| `Page?sortOrder=desc&sortBy=dt_update&page=2&pageSize=20` | `code=0` |
| `FilterOptions` | 4 个维度均为空数组 |
| 同一套 PostgREST 语法在 `finv_quote_secu_key`（有数据）上验证 | `or=(...)` + `ilike` 关键字命中 5 行；`limit=15&offset=15` 返回 15 行、`Content-Range: 15-29/70690` |
