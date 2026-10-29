# GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni — 外部条件查询 + 分页（column/row）

> 外部 OpenAPI 端点（需求 10789）。业务逻辑复用内部
> `KLine.PageUni` handler；与内部接口的差异仅在**认证方式**与**响应信封**。
> 正文（第 1~6 节）是人读说明，第 8 节附录链接机器可读契约
> [`PageUni.yml`](PageUni.yml)。
>
> - 实现：`internal/api/router.go`（`/OpenAPI/V1` 路由组）、`internal/api/middleware.go`（`WrapOpenAPIResponse`）
> - 说明：本文件是**对外独立契约**，参数/列定义/示例全部自包含，不引用内部 API 文档。

## 1. 概述

供外部 HOOK / 核对工具查询分钟行情：

- 查询参数与内部 `/API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni` 完全一致；
- 返回固定 15 列的 `column` 与等长 `row` 数组（单元格为字符串或 `null`）；
- 成功响应统一为 `{code, data, message}`，业务字段位于 `data` 节点。

## 2. 认证

> **当前版本暂不鉴权（需求 10790）**：`/OpenAPI/V1` 路由组公开访问，无需 token 或 API Key。
> 上线前请通过防火墙/内网限制来源；API Key 体系落地后再恢复鉴权。

| 项 | 当前状态 | 后续（API Key 落地后） |
|---|---|---|
| 认证 | 无（公开） | `Authorization: Bearer <key>` 或 `X-API-Key: <key>` |
| 401 | 不会返回（除未来启用鉴权） | 未携带/错误密钥 → `401` |

## 3. 请求

### 3.1 Header

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `Authorization` | string | 二选一 | `Bearer <FINV_OPENAPI_KEY>` |
| `X-API-Key` | string | 二选一 | 直接传 `<FINV_OPENAPI_KEY>` |

### 3.2 Query

| 参数 | 类型 | 必填 | 默认 | 约束 | 说明 |
|---|---|---|---|---|---|
| `usc` | string | 否 | — | 1–255 字符 | 统一证券代码，精确匹配 |
| `provider` | string | 否 | — | ≤255 字符 | 数据来源，精确匹配 |
| `dateFrom` | integer | 否 | — | 19500101–21001231 | 交易日期下限（含）；兼容 `date_from` |
| `dateTo` | integer | 否 | — | 19500101–21001231 | 交易日期上限（含）；兼容 `date_to` |
| `tsFrom` | integer(int64) | 否 | — | ≥ 0 | 时间戳下限（含）；兼容 `ts_from` |
| `tsTo` | integer(int64) | 否 | — | ≥ 0 | 时间戳上限（含）；兼容 `ts_to` |
| `limit` | integer | 否 | `50` | 1–1000 | 每页条数；兼容 `pageSize` / `page_size` |
| `offset` | integer | 否 | `0` | ≥ 0 | 偏移量 |

过滤条件之间为 **AND**；日期/时间戳范围均为闭区间。排序固定 `usc, date, ts` 升序。

## 4. 响应

### 4.1 成功（HTTP 200）

```json
{
  "code": 0,
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
| `code` | integer | 成功码，**默认 `0`**；`1` / `200` / `202` 兼容，服务端默认输出 `0` |
| `data.cnt` | integer(int64) | 满足条件的总记录数 |
| `data.ps` | integer | 每页条数（回显 `limit`） |
| `data.pn` | integer | 页码（`offset / limit + 1`，从 1 开始） |
| `data.column` | array\<string\> | 固定 15 列，顺序不可变 |
| `data.row` | array\<array\<string\>\> | 行数据；单元格可为 `null`，每行与 `column` 等长 |
| `data.request_id` | string | 请求标识 |
| `data.ts` | integer(int64) | 服务端响应时间（Unix 秒） |
| `message` | string | 成功固定 `OK` |

### 4.2 列定义

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

### 4.3 单元格取值规则

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

### 4.4 失败（非 2xx）

沿用内部错误信封（不再套 `data`）：

```json
{
  "code": 211004,
  "message": "字段取值超出允许范围",
  "detail": "字段取值超出允许范围",
  "request_id": "0b0d1f2e3a4c5d6e",
  "error": {"code": 211004, "catalog_version": "finvquant-1", "retryable": false}
}
```

## 5. 错误码

| 错误码 | HTTP | 说明 |
|---|---|---|
| — | `401` | 未携带密钥或密钥错误 |
| `211003` | 422 | 参数类型不匹配（如 `limit=abc`） |
| `211004` | 422 | 参数越界（`limit` 不在 1–1000、`offset < 0`、日期/时间戳越界） |
| `211301` | 500 | 数据库错误（可重试） |
| `211302` | 503 | 数据库不可用（可重试） |

## 6. 示例

```bash
curl -G 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni' \
  --data-urlencode 'usc=000001' \
  --data-urlencode 'dateFrom=20261001' \
  --data-urlencode 'dateTo=20261019' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'offset=0'
```

## 7. 已使用位置登记

- 外部核对工具 / 数据消费方（示例接入）

## 8. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`PageUni.yml`](PageUni.yml)（`openapi: 3.1.0`）。

```bash
npx @redocly/cli lint Docs/OpenAPI/Quote/Cdeu/Min/PageUni.yml
npx @redocly/cli preview-docs Docs/OpenAPI/Quote/Cdeu/Min/PageUni.yml
```
