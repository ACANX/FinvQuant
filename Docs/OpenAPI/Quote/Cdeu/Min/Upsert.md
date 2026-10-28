# POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert — 外部批量 Upsert

> 外部 OpenAPI 示例端点（需求 10787 / 10788）。业务逻辑复用内部
> `KLine.Upsert` handler；与内部接口的差异仅在**认证方式**与**响应信封**。
> 正文（第 1~6 节）是人读说明，第 8 节附录链接机器可读契约
> [`Upsert.yml`](Upsert.yml)。契约变更时两者须同时更新。
>
> - 实现：`internal/api/router.go`（`/OpenAPI/V1` 路由组）、`internal/api/middleware.go`（`WrapOpenAPIResponse`）
> - 说明：本文件是**对外独立契约**，字段/错误码/示例全部自包含，不引用内部 API 文档。

## 1. 概述

供外部 HOOK / 采集端调用的分钟行情批量写入接口：

- 主键 `(usc, ts)`；`onConflict=skip` 跳过，`onConflict=update`（默认）整行覆盖；
- 与内部 `/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` 复用一个 handler，请求体、校验规则、
  错误码完全一致；
- 成功响应统一为 `{code, data, message}` 信封，业务字段位于 `data` 节点。

## 2. 认证

> **当前版本暂不鉴权（需求 10790）**：`/OpenAPI/V1` 路由组公开访问，无需 token 或 API Key。
> 上线前请通过防火墙/内网限制来源；API Key 体系（`FINV_OPENAPI_KEY` → `sys.sys_open_api_key`）
> 落地后再恢复鉴权。

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
| `Content-Type` | string | 是 | `application/json` |

### 3.2 Query

| 参数 | 类型 | 必填 | 默认 | 取值 | 说明 |
|---|---|---|---|---|---|
| `onConflict` | string | 否 | `update` | `skip` / `update` | 冲突处理模式；兼容旧名 `on_conflict` |

### 3.3 请求体

请求体结构为 `{"items": [KLineItem, ...]}`；`items` 必填，1 ≤ 条数 ≤ 5000（由
`FINV_KLINE_MAX_BATCH_ROWS` 控制）。字段定义如下（机器可读 schema 见
[`Upsert.yml`](Upsert.yml) 的 `KLineItem`）：

| 字段 | 类型 | 必填 | 约束 | 落库 | 说明 |
|---|---|---|---|---|---|
| `usc` | string | **是** | 1–255 字符 | 是 | 统一证券代码（`GCMain` / `518880`） |
| `ts` | integer(int64) | **是** | 0 ≤ ts ≤ 100000000000 | 是 | UTC 秒级时间戳；`FINV_KLINE_TS_UNIT=ms` 时可传 13 位毫秒并自动 /1000 |
| `date` | integer \| null | 否 | 19500101–21001231 | 是 | 交易日期 `yyyyMMdd` |
| `time` | integer \| null | 否 | 0–235959 | 是 | 交易时间 `HHmmss` |
| `open` | number \| null | 否 | — | 是 | 起始价；缺失时用 `prev_close` 回填 |
| `high` | number \| null | 否 | — | 是 | 区间最高价 |
| `low` | number \| null | 否 | — | 是 | 区间最低价 |
| `close` | number \| null | 否 | — | 是 | 最新价/收盘价 |
| `volume` | number \| null | 否 | ≥ 0 | 是 | 成交量（支持小数，`numeric(20,6)`） |
| `turnover` | number \| null | 否 | ≥ 0 | 是 | 成交金额 |
| `change_price` | number \| null | 否 | — | 是 | 涨跌额；与 `change_ratio` 均未传时自动计算 |
| `change_ratio` | number \| null | 否 | — | 是 | 涨跌幅；与 `change_price` 均未传时自动计算 |
| `paocd` | number \| null | 否 | — | 是 | 当日累计均价 |
| `ext_field` | string \| null | 否 | — | 是 | 预留扩展字段 |
| `remark` | string \| null | 否 | — | 是 | 备注 |
| `provider` | string \| null | 否 | ≤ 255 字符 | 是 | 数据来源（如 `FT`） |
| `prev_close` | number \| null | 否 | — | **否** | 兼容字段：仅用于回填 `open`，自身不落库 |

数值字段只接受 **JSON 数字或 `null`**（字符串 `"3.60"` 会触发 422 / `211003`）。

```json
{
  "items": [
    {
      "usc": "GCMain",
      "ts": 1700000000,
      "date": 20231114,
      "time": 93000,
      "open": 3.51,
      "high": 3.62,
      "low": 3.50,
      "close": 3.60,
      "volume": 1000,
      "turnover": 3560.00,
      "provider": "FT"
    }
  ]
}
```

## 4. 响应

### 4.1 成功（HTTP 200）

```json
{
  "code": 0,
  "data": {"affected": 1, "mode": "update"},
  "message": "OK"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | integer | 成功码，**默认 `0`**；`1` / `200` / `202` 同样视为正常响应（兼容），服务端默认输出 `0` |
| `data.affected` | integer(int64) | `skip` 为实际插入行数；`update` 为插入 + 覆盖影响行数 |
| `data.mode` | string | 本次实际使用的模式（`skip` / `update`） |
| `message` | string | 成功固定为 `OK` |

### 4.2 失败（非 2xx）

沿用内部错误信封（不再套一层 `data`）：

```json
{
  "code": 211102,
  "message": "请求内存在重复主键",
  "detail": "请求内存在重复主键",
  "request_id": "6f1c0f7a9d2e4b0c",
  "error": {
    "code": 211102,
    "catalog_version": "finvquant-1",
    "retryable": false
  }
}
```

## 5. 错误码

| 错误码 | HTTP | 名称 | retryable | 触发场景 | 白名单 `debug` 键 |
|---|---|---|---|---|---|
| `211001` | 400 | `KLineMissingParam` | 否 | 缺少 `usc`/`ts`/`items` | `field` |
| `211002` | 400 | `KLineMalformedJSON` | 否 | 请求体不是合法 JSON | `err_class` |
| `211003` | 422 | `KLineTypeMismatch` | 否 | 字段类型不匹配 | `field`、`want`、`got` |
| `211004` | 422 | `KLineOutOfRange` | 否 | `usc` 超长、`ts`/`date`/`time` 越界、负 `volume`/`turnover` | `field`、`min`、`max` |
| `211005` | 422 | `KLineUnknownField` | 否 | 请求含未定义字段 | `field` |
| `211006` | 400 | `KLineInvalidEnum` | 否 | `onConflict` 不是 `skip`/`update` | `field`、`allowed` |
| `211101` | 409 | `KLineDuplicateKey` | 否 | 数据库主键/唯一键冲突 | `pg_code`、`sqlstate`、`constraint` |
| `211102` | 409 | `KLineDuplicateInRequest` | 否 | `update` 模式请求内重复 `(usc, ts)` | `first_dup`、`count` |
| `211103` | 404 | `KLineNotFound` | 否 | 记录不存在（本接口不产生） | `usc`、`ts` |
| `211104` | 409 | `KLineStateConflict` | 否 | 当前状态不允许该操作 | `state`、`reason_class` |
| `211201` | 413 | `KLineBatchTooLarge` | 否 | `items` 超过上限（默认 5000） | `limit`、`got` |
| `211202` | 413 | `KLineListTooLarge` | 否 | 列表长度超过上限 | `limit`、`got` |
| `211301` | 500 | `KLineDBError` | **是** | 数据库执行错误 | `pg_code`、`sqlstate`、`err_class` |
| `211302` | 503 | `KLineDBUnavailable` | **是** | 连接池不可用/数据库不可达 | `pool_state` |
| `211303` | 422 | `KLineDBConstraint` | 否 | 违反业务约束 | `pg_code`、`constraint` |
| — | `401` | — | 否 | 仅未来启用鉴权后出现 | — |

## 6. 示例

```bash
# 当前无需认证
curl -X POST 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert?onConflict=update' \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"usc":"GCMain","ts":1700000000,"close":3.60}]}'
```

## 7. 已使用位置登记

- 外部数据采集 HOOK（示例接入，端到端联调）

## 8. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`Upsert.yml`](Upsert.yml)（`openapi: 3.1.0`，**自包含**；
`KLineItem` schema 已内联，不依赖内部契约）。

```bash
npx @redocly/cli lint Docs/OpenAPI/Quote/Cdeu/Min/Upsert.yml
npx @redocly/cli preview-docs Docs/OpenAPI/Quote/Cdeu/Min/Upsert.yml
```
