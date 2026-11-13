# GET /API/V1/Meta/Finv/Quote/Secu/Indicator/{Columns,Page} — 证券指标表只读查询

> 所属：行情 → 实时行情 → 证券指标
> 数据表：`cdeu.finv_quote_secu_indicator`（建表 `V300144`，改名 `V300145`，列注释 `V300146`，补 `stock_id` `V300148`；全表 132 列）
> 关联需求：11249（列元数据接口）、11250（分页查询接口）、11251（前端页面）、11048–11063（表与同步链路）、11248（采集端回传）
> 实现：`internal/api/handler/secu_indicator.go`（`SecuIndicator.Columns` / `SecuIndicator.Page`）、
> `internal/meta/finv/quote/indicator/read.go`（`Reader.Columns` / `Reader.Page`）
> 机器可读契约：[`IndicatorPage.yml`](IndicatorPage.yml)（OpenAPI 3.1）

## 1. 概述

两支**只读**接口，供「行情 → 实时行情 → 证券指标」页面浏览指标表：

| 接口 | 作用 |
|---|---|
| `GET .../Secu/Indicator/Columns` | 列元数据：表里**实际存在**的列（列名 / 中文名 / 简化类型），按建表列序 |
| `GET .../Secu/Indicator/Page` | 分页查询：**可变列集**（`columns=` 指定要哪些列）、关键字 / 市场筛选、白名单列排序 |

三条硬口径：

1. **不硬编码列清单**：列来自 `pg_attribute` + `col_description`，上游补列后重新同步即自动出现在 `/Columns`，
   同一份清单兼任 SQL 标识符白名单——拼进 SQL 的列名永远来自数据库目录，不来自请求参数。
2. **值一律文本**：所有列 `::text` 取回（含 `numeric(30,10)` / `bigint` / `timestamptz`），
   不经 `float64`，保住精度；万/亿折算与小数位裁剪由前端负责（与「证券行情排行」同口径）。
3. **只读**：本组接口不写任何表、不触发触发器；写入只有「FT证券指标数据同步」（`.../SecuKey/Indicator/Sync/Run`）
   与采集端 OpenAPI `FTSecuIndicatorUpsert` 两条路径。

> 路径不带 `Cdeu`：指标表不属于 CDEU 跑批域。写入侧同步接口仍在 `/Quote/Cdeu/SecuKey/Indicator/*`，两者互不影响。

## 2. 列元数据 `GET .../Secu/Indicator/Columns`

### 2.1 请求参数

| 参数 | 位置 | 类型 | 必填 | 说明 |
|---|---|---|---|---|
| `Authorization` | header | string | 是 | `Bearer <token>` |

### 2.2 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "table": "cdeu.finv_quote_secu_indicator",
    "total": 132,
    "columns": [
      { "name": "usc", "title": "证券统一编码（主键；上游为 NULL/空时由同步用 ftmm_symbol 占位）", "dataType": "text", "sortable": true },
      { "name": "pe_ttm", "title": "市盈率 TTM", "dataType": "numeric", "sortable": true }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.table` | string | 表名（`schema.table`） |
| `data.total` | integer | 列数（当前 132） |
| `data.columns[].name` | string | 列名（snake_case，与 `Page` 响应行里的键一致） |
| `data.columns[].title` | string | 列中文名（取自表列注释；注释缺失时回退列名） |
| `data.columns[].dataType` | string | 简化类型：`numeric` / `integer` / `text` / `timestamp` / `boolean` / `other` |
| `data.columns[].sortable` | boolean | 是否允许作为 `orderBy`（当前全部为 `true`） |

## 3. 分页查询 `GET .../Secu/Indicator/Page`

### 3.1 请求参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `usc` / `ftmm_symbol` / `name`（ILIKE，`%kw%`） |
| `market` | query | string | 否 | 空 | `ftmm_symbol` 第一段（`HK`/`SH`/`SZ`/`BJ`/`US`/`CC`/`FX`）；空或 `all` 不限 |
| `columns` | query | string | 否 | 见 3.2 | 逗号分隔的列名，如 `usc,name,price,pe_ttm` |
| `orderBy` | query | string | 否 | `dt_update` | 排序字段，必须是表里真实存在的列 |
| `order` | query | string | 否 | `desc` | `desc` / `asc` |
| `page` | query | integer | 否 | `1` | 页码，从 1 开始 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1–500 |

### 3.2 列集合口径

- `columns` 省略或为空 → 默认**概览 23 列**：`usc`、`ftmm_symbol`、`name`、`price`、`change_price`、`change_ratio`、`price_direct`、
  `price_open`、`price_highest`、`price_lowest`、`price_last_close`、`volume`、`turnover`、`ratio_volume`、`ratio_turnover`、
  `total_market_cap`、`outstanding_market_cap`、`pe_ttm`、`pb_ratio`、`dividend_ratio`、`market_status_text`、`ts`、`dt_update`。
- `usc` 与 `ftmm_symbol` 是**行身份列**，请求任何列集都会被服务端补齐并前置。
- 列名重复会被去重（保留首次出现的顺序）；**任一列名不在表内 → HTTP 400**，不会拼进 SQL。
- 排序：`NULLS LAST` 固定（空值一律排最后），并以 `usc ASC` 作稳定次序。
- 排序字段不要求在 `columns` 里（可以先按某列排序、只显示另几列）。

### 3.3 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 10910,
    "page": 1,
    "pageSize": 2,
    "list": [
      {
        "usc": "00700",
        "ftmm_symbol": "HK.00700",
        "name": "腾讯控股",
        "price": "385.4000000000",
        "change_ratio": "1.2300000000",
        "volume": "18400000.0000000000",
        "pe_ttm": "19.5000000000",
        "ts": "1781619897000",
        "dt_update": "2026-11-18 09:31:02.345+08"
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
| `data.list[]` | array | 当前页行；键 = 请求的列名（snake_case） |
| `data.list[].*` | string | **所有值都是文本**；空列返回空串 `""`（不是 `null`、不是 0） |
| `data.list[].dt_update` | string | `timestamptz` 的 PostgreSQL 文本形式（如 `2026-11-18 09:31:02.345+08`），前端按本地时区展示 |
| `data.list[].dt_collect` | string | 同上（上游采集时间） |
| `data.list[].dt_create` | string | 同上（上游首次采集时间） |
| `data.list[].ts` | string | 上游 `bigint`（多为 epoch 毫秒）的十进制文本 |
| `data.list[].ts_exchange_dt` | string | 同上（交易所数据时间） |
| `data.list[].ts_server_send_to_client` | string | 同上（服务端下发时间） |
| `data.list[].ts_server_recv_from_exchange` | string | 同上（服务端收到交易所数据时间） |
| `data.list[].before_open_exchange_time` | string | 同上（盘前行情时间） |
| `data.list[].json_unmapped` | string | 上游未映射字段的原始 JSON 文本（前端以 ⓘ 浮层展示） |

### 3.4 错误

| 场景 | HTTP | `code` | 说明 |
|---|---|---|---|
| `page` / `pageSize` 非法或越界 | 400 | 400 | `pageSize` 上限 500 |
| `order` 非 `asc`/`desc` | 400 | 400 | — |
| `columns` 含不存在的列 | 400 | 400 | 消息点名该列 |
| `orderBy` 不是表里的列 | 400 | 400 | `orderBy 非法（xxx）` |
| 表不存在 / 查询失败 | 500 | 2006 | `指标查询参数非法` 之外的错误一律 500 |

## 4. 已使用位置

`Web/src/views/Meta/Finv/Quote/Realtime/IndicatorView.vue`（行情 → 实时行情 → 证券指标，路由 `/Meta/Finv/Quote/Realtime/SecuIndicator`）：
`/Columns` 生成列选择器与列标题、`/Page` 取主表数据；点行看详情时用同一 `/Page` 携带全列请求该证券。
