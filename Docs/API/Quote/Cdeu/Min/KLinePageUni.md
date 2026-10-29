# GET /API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni — 条件查询 + 分页（column/row）

> 接口文档结构见 [`KLineUpsert.md`](KLineUpsert.md)（OpenAPI 3.1 样板）。
> 正文（第 1~6 节）是人类可读说明，第 8 节附录链接到机器可读契约
> [`KLinePageUni.yml`](KLinePageUni.yml)。契约变更时两者须同时更新。
>
> - 变更记录：需求 10781（迁移到 CDEU 前缀）、10782（全量统一 CDEU）、10785（按样板重写并拆分 .yml）
> - 实现：`internal/api/handler/kline.go`（`PageUni`）、`internal/meta/finv/quote/cdeu/kline/service.go`（`Page`）
> - 错误目录：`internal/meta/finv/quote/cdeu/kline/apierr.go`

## 1. 概述

按证券/来源/交易日期/时间戳过滤并分页，输出**固定 15 列**的 `column` 与等长 `row` 数组，
供表格组件直接渲染（无需前端再做字段映射）。

查询参数与 [`KLinePage.md`](KLinePage.md) 完全一致，差异仅在响应格式：

| 接口 | 响应形态 | 适用场景 |
|---|---|---|
| `Page` | 对象数组（`list`，含完整字段名） | 程序处理、逐字段读取 |
| `PageUni` | `column` + `row`（单元格为字符串/null） | 表格渲染、导出、核对工具 |

## 2. 请求参数说明

### 2.1 Header

| 参数 | 类型 | 必填 | 取值 | 说明 |
|---|---|---|---|---|
| `Authorization` | string | 是 | `Bearer <token>` | 登录接口获取的令牌 |
| `Accept` | string | 否 | `application/json` | 响应为 JSON |

### 2.2 Query 参数

| 参数 | 类型 | 必填 | 默认 | 约束 | 说明 |
|---|---|---|---|---|---|
| `usc` | string | 否 | — | 1–255 字符 | 统一证券代码，精确匹配 |
| `provider` | string | 否 | — | ≤255 字符 | 数据来源，精确匹配（如 `FTMM`） |
| `dateFrom` | integer | 否 | — | 19500101–21001231 | 交易日期下限（含）；兼容 `date_from` |
| `dateTo` | integer | 否 | — | 19500101–21001231 | 交易日期上限（含）；兼容 `date_to` |
| `tsFrom` | integer(int64) | 否 | — | ≥ 0 | 时间戳下限（含，UTC 秒）；兼容 `ts_from` |
| `tsTo` | integer(int64) | 否 | — | ≥ 0 | 时间戳上限（含，UTC 秒）；兼容 `ts_to` |
| `limit` | integer | 否 | `50` | 1–1000 | 每页条数；兼容 `pageSize` / `page_size` |
| `offset` | integer | 否 | `0` | ≥ 0 | 偏移量（从 0 开始） |

- 所有过滤条件之间为 **AND**；`dateFrom/dateTo`、`tsFrom/tsTo` 均为**闭区间**（含端点）。
- 小驼峰为规范名；下划线名（`date_from` 等）仅为兼容旧客户端，新调用方请用小驼峰。

## 3. 行为与语义

### 3.1 过滤与排序

- 过滤：`usc`、`provider` 精确匹配；日期/时间戳范围闭区间；未传的条件不参与过滤。
- 排序：固定 `ORDER BY usc, date, ts` 升序（无自定义排序参数）。
- 计数：`cnt` 为满足过滤条件的**总记录数**（与分页无关）。

### 3.2 分页

| 响应字段 | 计算方式 | 说明 |
|---|---|---|
| `ps` | 请求的 `limit`（默认 50） | 每页条数 |
| `pn` | `offset / limit + 1` | 页码从 1 开始；`limit` 非法时按 1 处理（此处 limit 已校验 1–1000） |
| `cnt` | `SELECT count(*)` | 总记录数 |

越界行为：`limit` 不在 1–1000 或 `offset < 0` → 422 / `211004`（不会自动截断）。

### 3.3 响应列定义（`column` 固定 15 项，顺序不可变）

| # | `column` | 对应字段 | 类型（单元格） | 可空 | 说明 |
| --- | --- | --- | --- | --- | --- |
| 1 | `usc` | `usc` | string | 否 | 统一证券代码 |
| 2 | `ts` | `ts` | string（整数） | 否 | UTC 秒级时间戳 |
| 3 | `date` | `date` | string（整数） | 是 | 交易日期 `yyyyMMdd` |
| 4 | `time` | `time` | string（整数） | 是 | 交易时间 `HHmmss` |
| 5 | `o` | `open` | string（数值） | 是 | 起始价 |
| 6 | `h` | `high` | string（数值） | 是 | 区间最高价 |
| 7 | `l` | `low` | string（数值） | 是 | 区间最低价 |
| 8 | `c` | `close` | string（数值） | 是 | 最新价/收盘价 |
| 9 | `v` | `volume` | string（数值，支持小数） | 是 | 成交量（`numeric(20,6)`，输出时去尾零） |
| 10 | `t` | `turnover` | string（数值） | 是 | 成交金额 |
| 11 | `cr` | `change_ratio` | string（两位小数） | 是 | 涨跌幅 |
| 12 | `cp` | `change_price` | string（数值） | 是 | 涨跌额 |
| 13 | `pa` | `pacd`（库列 `paocd`） | string（数值） | 是 | 当日累计均价 |
| 14 | `pv` | `provider` | string | 是 | 数据来源 |
| 15 | `ext` | `ext_field` | string | 是 | 预留扩展字段 |

### 3.4 单元格取值规则

| 场景 | 规则 | 示例 |
|---|---|---|
| 数值去尾零 | 去掉无意义尾随零与小数点 | `11.480000` → `"11.48"`、`3.600000` → `"3.6"` |
| 负零归一 | `-0` / `-0.000000` → `"0"` | `-0.000000` → `"0"` |
| 科学计数法 | 转为普通十进制文本 | `1.2e3` → `"1200"` |
| `cr`（涨跌幅） | 固定两位小数，`ROUND_HALF_UP`（远离零） | `-0.004` → `"-0.00"`、`-0.005` → `"-0.01"` |
| 整数列 | `ts`、`date`、`time` 输出整数字符串 | `1792373400` → `"1792373400"` |
| 空值 | 输出 JSON `null`（不是 `""`） | `null` |
| 字符串列 | `usc`、`provider`、`ext_field` 原样输出 | `FTMM` |

> 所有数值均以**字符串**返回，避免 JSON number 的精度损失；调用方按需自行解析。

### 3.5 响应信封

```json
{
  "code": 1,
  "data": {
    "cnt": 43756,
    "ps": 50,
    "pn": 1,
    "column": [
      "usc",
      "ts",
      "date",
      "time",
      "o",
      "h",
      "l",
      "c",
      "v",
      "t",
      "cr",
      "cp",
      "pa",
      "pv",
      "ext"
    ],
    "row": [
      [
        "000001",
        "1792373400",
        "20261019",
        "93000",
        null,
        null,
        null,
        "11.48",
        "392600",
        "4507048",
        "-0.17",
        "-0.02",
        null,
        "FTMM",
        null
      ]
    ],
    "request_id": "B7C63349-EB36-49AB-B16A-E8E1BE475D3B",
    "ts": 1792695045
  },
  "message": "OK"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | integer | 成功固定为 `1` |
| `data.cnt` | integer(int64) | 满足条件的总记录数 |
| `data.ps` | integer | 每页条数（回显 `limit`） |
| `data.pn` | integer | 页码（从 1 开始） |
| `data.column` | array\<string\> | 固定 15 列，顺序不可变 |
| `data.row` | array\<array\<string\>\> | 行数据；单元格可为 `null`，每行与 `column` 等长 |
| `data.request_id` | string | 请求标识，便于与服务端日志关联 |
| `data.ts` | integer(int64) | 服务端响应时间（Unix 秒） |
| `message` | string | 成功固定为 `OK` |

## 4. 错误目录

| 错误码 | HTTP | 名称 | retryable | 本接口触发场景 | 白名单 `debug` 键 |
|---|---|---|---|---|---|
| `211001` | 400 | `KLineMissingParam` | 否 | 不适用（本接口无必填参数） | `field` |
| `211002` | 400 | `KLineMalformedJSON` | 否 | 不适用（无请求体） | `err_class` |
| `211003` | 422 | `KLineTypeMismatch` | 否 | `dateFrom`/`limit` 等不是整数 | `field`、`want`、`got` |
| `211004` | 422 | `KLineOutOfRange` | 否 | `dateFrom/dateTo`、`tsFrom/tsTo`、`limit`、`offset` 越界 | `field`、`min`、`max` |
| `211005` | 422 | `KLineUnknownField` | 否 | 不适用（无请求体） | `field` |
| `211006` | 400 | `KLineInvalidEnum` | 否 | 不适用（无枚举参数） | `field`、`allowed` |
| `211101` | 409 | `KLineDuplicateKey` | 否 | 不适用（只读接口） | `pg_code`、`sqlstate`、`constraint` |
| `211102` | 409 | `KLineDuplicateInRequest` | 否 | 不适用（只读接口） | `first_dup`、`count` |
| `211103` | 404 | `KLineNotFound` | 否 | 不适用（无单条查询） | `usc`、`ts` |
| `211104` | 409 | `KLineStateConflict` | 否 | 不适用 | `state`、`reason_class` |
| `211201` | 413 | `KLineBatchTooLarge` | 否 | 不适用（无批量写入） | `limit`、`got` |
| `211202` | 413 | `KLineListTooLarge` | 否 | 不适用（无列表入参） | `limit`、`got` |
| `211301` | 500 | `KLineDBError` | **是** | 数据库执行错误 | `pg_code`、`sqlstate`、`err_class` |
| `211302` | 503 | `KLineDBUnavailable` | **是** | 连接池不可用/数据库不可达 | `pool_state` |
| `211303` | 422 | `KLineDBConstraint` | 否 | 不适用（只读接口） | `pg_code`、`constraint` |

错误响应示例（`limit=0`）：

```json
{
  "code": 211004,
  "message": "字段取值超出允许范围",
  "detail": "字段取值超出允许范围",
  "request_id": "0b0d1f2e3a4c5d6e",
  "error": {
    "code": 211004,
    "catalog_version": "finvquant-1",
    "retryable": false
  },
  "debug": {
    "field": "limit",
    "min": "1",
    "max": "1000"
  }
}
```

> `debug` 仅在 `FINV_ERROR_DETAIL_MODE=admin` 且来自内网 CIDR 或携带有效
> `X-Debug-Token` 时出现；默认安全模式不返回。

## 5. 示例

### 5.1 curl

```bash
curl -G 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni' \
  -H 'Authorization: Bearer <token>' \
  --data-urlencode 'usc=000001' \
  --data-urlencode 'dateFrom=20261001' \
  --data-urlencode 'dateTo=20261019' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'offset=0'
```

### 5.2 成功响应（HTTP 200）

见 §3.5。

### 5.3 常见错误

| 场景 | HTTP | 响应 `code` |
|---|---|---|
| `dateFrom=2026`（非 8 位日期） | 422 | `211004` |
| `limit=0` 或 `limit=2000` | 422 | `211004` |
| `offset=-1` | 422 | `211004` |
| `tsFrom=abc` | 422 | `211003` |
| 未携带令牌 | 401 | 认证中间件返回 |

## 6. 已使用位置登记

- 表格类前端/核对工具
- 去重分钟行情查询/统计页的数据导出与核对

## 7. 编写约定

字段表/行为/错误/示例的写作要求见 [`KLineUpsert.md`](KLineUpsert.md) §7 与
[`Docs/DevSpec/ApiSpec.md`](../../../../DevSpec/ApiSpec.md) §7.7；本文件不再重复。

## 8. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`KLinePageUni.yml`](KLinePageUni.yml)（`openapi: 3.1.0`，含
`paths` / `parameters` / `components` / `securitySchemes`）。本文正文与该文件逐项对应，
**契约变更时两者须同时更新**。

```bash
# 规范校验
npx @redocly/cli lint Docs/API/Quote/Cdeu/Min/KLinePageUni.yml

# 生成交互式文档
npx @redocly/cli preview-docs Docs/API/Quote/Cdeu/Min/KLinePageUni.yml
```
