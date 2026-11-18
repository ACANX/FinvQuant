# GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Future/Page — 期货行情排行分页查询

> 所属：行情 → 实时行情 → 期货行情排行
> 数据表：`cdeu.finv_quote_realtime_future`（主键 `stock_id`，建表 `V300216`）
> 分类字典：`cdeu.finv_quote_cdeu_future_sync_config`（`futures_type` → `futures_type_name`）
> 关联需求：11322、11324（持仓量/日增仓列与排序）
> 实现：`internal/api/handler/cdeu_realtime_future.go`（`RealtimeFuturePage`）、
> `internal/meta/finv/quote/cdeu/store_realtime_future.go`（`ListRealtimeFutures`）
> 配套接口：[`GET .../Realtime/Future/Types`](#5-分类下拉)（分类下拉）

## 1. 概述

分页查询期货实时行情快照（一行一合约，主键 `stock_id`），用于「期货行情排行」页面：

- 筛选：`keyword`（模糊匹配 `contract_code` / `name` / `usc` / `stock_id`）、`futuresType`、`marketLabel`、`onlyOnline`；
- 排序：白名单列 + `asc`/`desc`，空值（NULL）一律排在最后；默认按涨跌幅 `change_ratio` 降序；
- 数值列（现价、涨跌幅）以**文本**返回，保住 `NUMERIC(20,6)` 精度，小数位裁剪由前端负责；
- 分类中文名通过 `LEFT JOIN` 配置表带出，配置缺行时返回空串（前端降级显示「分类 N」）；
- `fields_json` / `field_list_json` 原样返回，供前端「明细」弹窗按字段定义渲染。

## 2. 请求参数

### 2.1 Header 与 Query

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | `Bearer <token>` |
| `keyword` | query | string | 否 | 空 | 模糊匹配 `contract_code` / `name` / `usc` / `stock_id`（ILIKE，`%kw%`） |
| `futuresType` | query | string | 否 | 空 | 期货分类 `1`~`8`；空或 `all` 不限 |
| `marketLabel` | query | string | 否 | 空 | 市场：`HK`/`US`/`SG`/`JP`；空或 `all` 不限 |
| `onlyOnline` | query | string | 否 | 空 | `1` 或 `true` → 只列在线（`flag_online='1'`）；空 = 全部 |
| `orderBy` | query | string | 否 | `change_ratio` | 排序字段，白名单见 2.2 |
| `order` | query | string | 否 | `desc` | `desc` / `asc` |
| `page` | query | integer | 否 | `1` | 页码，从 1 开始 |
| `pageSize` | query | integer | 否 | `50` | 每页条数，1–500 |

### 2.2 排序白名单（`orderBy`）

`price_nominal`、`change_ratio`、`open_interest`、`day_increase`、`dt_update`、`stock_id`、
`contract_code`、`futures_type`、`market_label`、`name`。

`open_interest` / `day_increase` 不是主表列：它们按 `field_list_json` 里
`fieldName=持仓量 / 日增仓` 找到对应 `fieldId`，再取 `fields_json` 原文；排序时转 `NUMERIC`，
非数值（如 `--`）按 NULL 排在最后。**按字段名查编号，不硬编码 11/12**（需求 11324）。

白名单外的值 → HTTP 400（`code=400`），不做字段名拼接。

## 3. 响应

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "total": 335,
    "page": 1,
    "pageSize": 50,
    "list": [
      {
        "stock_id": "71003131",
        "usc": "",
        "futures_type": "8",
        "futures_type_name": "股票期货",
        "market_type": "0",
        "market_label": "HK",
        "contract_code": "JDHmain",
        "name": "京东健康",
        "price_nominal": "35.830000",
        "change_ratio": "-13.240000",
        "open_interest": "18.00",
        "day_increase": "1.00",
        "price_direct": "-",
        "flag_online": "1",
        "provider": "FTMM",
        "remark": "",
        "dt_update": 1794970226189,
        "fields_json": "{\"1\": \"JDHmain\", \"3\": \"35.83\", \"4\": \"-13.24%\"}",
        "field_list_json": "[{\"fieldId\": 1, \"fieldName\": \"合约代码\"}, ...]"
      }
    ]
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `stock_id` | string | 主键（富途合约编号） |
| `usc` | string | 统一证券代码，采集侧解析不到时为空串 |
| `futures_type` | string | 期货分类 `1`~`8` |
| `futures_type_name` | string | 分类中文名（配置表 `finv_quote_cdeu_future_sync_config`，缺行时为空） |
| `market_type` | string | 采集市场类型（配置表取值） |
| `market_label` | string | 市场标签 `HK` / `US` / `SG` / `JP` |
| `contract_code` | string | 合约代码（如 `XAMD0main`） |
| `name` | string | 合约名称 |
| `price_nominal` | string | 现价（NUMERIC 文本） |
| `change_ratio` | string | 涨跌幅（百分点数值，如 `-13.24` 表示 -13.24%） |
| `open_interest` | string | 持仓量（来自字段留档，按 `fieldName=持仓量` 定位编号；取不到为空串） |
| `day_increase` | string | 日增仓（同上，`fieldName=日增仓`） |
| `price_direct` | string | 涨跌方向 `+` / `-` / `=`（前端据此整行着色） |
| `flag_online` | string | `1` 在线 / `0` 离线 |
| `provider` | string | 数据提供方（`FTMM`） |
| `remark` | string | 备注 |
| `dt_update` | int64 | 更新时间（Unix 毫秒） |
| `fields_json` | string | 字段留档（编号 → 原文，jsonb 文本） |
| `field_list_json` | string | 字段定义留档（`fieldId`/`fieldName`/…，jsonb 文本） |

## 4. 错误

| HTTP | code | 场景 |
|---|---|---|
| 400 | 400 | `orderBy` 不在白名单、`order` 非 `asc`/`desc`、`page`/`pageSize` 越界 |
| 401 | — | 未登录或 token 过期 |
| 500 | 2006 | 数据库错误 |

## 5. 分类下拉

`GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Future/Types`

返回配置表中启用的分类（按 `futures_type` 升序）：

```json
{
  "code": 0,
  "message": "查询完成",
  "data": {
    "list": [
      { "value": "1", "label": "指数期货" },
      { "value": "7", "label": "加密货币期货" },
      { "value": "8", "label": "股票期货" }
    ]
  }
}
```
