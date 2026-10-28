# POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert — 外部批量 Upsert

> 外部 OpenAPI 示例端点（需求 10787 / 10788）。业务逻辑复用内部
> `KLine.Upsert` handler；与内部接口的差异仅在**认证方式**与**响应信封**。
> 正文（第 1~6 节）是人读说明，第 8 节附录链接机器可读契约
> [`Upsert.yml`](Upsert.yml)。契约变更时两者须同时更新。
>
> - 实现：`internal/api/router.go`（`/OpenAPI/V1` 路由组）、`internal/api/middleware.go`（`RequireOpenAPIKey`、`WrapOpenAPIResponse`）
> - 内部同业务接口：[`KLineUpsert.md`](../../../../API/Quote/Cdeu/Min/KLineUpsert.md)

## 1. 概述

供外部 HOOK / 采集端调用的分钟行情批量写入接口：

- 主键 `(usc, ts)`；`onConflict=skip` 跳过，`onConflict=update`（默认）整行覆盖；
- 与内部 `/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` 复用一个 handler，请求体、校验规则、
  错误码完全一致；
- 成功响应统一为 `{code, data, message}` 信封，业务字段位于 `data` 节点。

## 2. 认证

| 项 | 说明 |
|---|---|
| 凭据 | 服务端配置项 `FINV_OPENAPI_KEY`（静态密钥，过渡方案） |
| 传递方式 | `Authorization: Bearer <FINV_OPENAPI_KEY>` 或 `X-API-Key: <FINV_OPENAPI_KEY>` |
| 校验 | 常量时间比较；错误/缺失 → `401` |
| 未配置 | 服务端**不注册**该路由（返回 404），不会无认证暴露 |
| 后续 | 由 `sys.sys_open_api_key`（API Key）体系替换，见 `Docs/Refactor/2610-009-ExternalHookApiKey.md` |

> 该密钥与登录会话令牌（`Authorization: Bearer <session token>`）**不通用**。

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

与内部接口完全一致，字段定义见
[`KLineUpsert.md` §2.3](../../../../API/Quote/Cdeu/Min/KLineUpsert.md#23-条目-klineitem)；
机器可读 schema 复用
[`KLineUpsert.yml#/components/schemas/KLineItem`](../../../../API/Quote/Cdeu/Min/KLineUpsert.yml)。

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

与内部接口一致（详见 [`KLineUpsert.md` §4](../../../../API/Quote/Cdeu/Min/KLineUpsert.md#4-错误目录)），
外部入口额外可能返回：

| 错误码 | HTTP | 说明 |
|---|---|---|
| — | `401` | 未携带密钥或密钥错误（`{"code":401,"message":"未登录或登录已过期"}`） |
| `211001` | 400 | 缺少 `usc`/`ts`/`items` |
| `211003` | 422 | 字段类型不匹配 |
| `211004` | 422 | 字段取值越界 |
| `211005` | 422 | 未知字段 |
| `211006` | 400 | `onConflict` 非法 |
| `211102` | 409 | `update` 模式请求内重复键 |
| `211201` | 413 | `items` 超过上限（默认 5000） |
| `211301` | 500 | 数据库错误（可重试） |
| `211302` | 503 | 数据库不可用（可重试） |

## 6. 示例

```bash
# Bearer 方式
curl -X POST 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert?onConflict=update' \
  -H "Authorization: Bearer ${FINV_OPENAPI_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"usc":"GCMain","ts":1700000000,"close":3.60}]}'

# X-API-Key 方式
curl -X POST 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert?onConflict=skip' \
  -H "X-API-Key: ${FINV_OPENAPI_KEY}" \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"usc":"GCMain","ts":1700000000,"close":3.60}]}'
```

## 7. 已使用位置登记

- 外部数据采集 HOOK（示例接入，端到端联调）

## 8. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`Upsert.yml`](Upsert.yml)（`openapi: 3.1.0`），请求体 schema
通过相对 `$ref` 复用内部 [`KLineUpsert.yml`](../../../../API/Quote/Cdeu/Min/KLineUpsert.yml)。

```bash
npx @redocly/cli lint Docs/API/OpenAPI/Quote/Cdeu/Min/Upsert.yml
npx @redocly/cli preview-docs Docs/API/OpenAPI/Quote/Cdeu/Min/Upsert.yml
```
