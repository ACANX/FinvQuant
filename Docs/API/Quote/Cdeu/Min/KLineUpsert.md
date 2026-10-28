# POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert — 批量 Upsert

> **本文是接口文档的 OpenAPI 3.1 样板**：第 2 节的 `openapi` 代码块是机器可读契约，
> 第 3~6 节是与之逐项对应的人类可读说明。后续新增/修改接口文档请按本结构编写
> （约定见 [`Docs/DevSpec/ApiSpec.md`](../../../../DevSpec/ApiSpec.md) §7.7）。
>
> - 变更记录：需求 10777（迁移到 CDEU 前缀）、10782（全量统一 CDEU）、10783（本文按 OpenAPI 3.1 重写）
> - 实现：`internal/api/handler/kline.go`（`Upsert`）、`internal/kline/service.go`（`Upsert`）
> - 错误目录：`internal/kline/apierr.go`

## 1. 概述

一次请求批量写入分钟行情 K 线，主键为 `(usc, ts)`：

- `onConflict=skip`：已存在的 `(usc, ts)` 直接跳过（`ON CONFLICT DO NOTHING`），天然幂等；
- `onConflict=update`（默认）：已存在的行按**整行覆盖**更新（未提供的业务列写 `NULL`）。

整个请求在**单个数据库事务**内完成：任一条目校验失败或写库失败，全部回滚。

## 2. OpenAPI 3.1 描述

```yaml
openapi: 3.1.0
info:
  title: FinvQuant — 分钟行情批量 Upsert
  version: 1.0.0
  description: |
    批量写入分钟行情 K 线（主键 usc + ts）。
    onConflict=skip 时已存在键跳过；onConflict=update 时整行覆盖。
    请求在单事务内执行，任一条目失败整体回滚。
servers:
  - url: http://127.0.0.1:16001
    description: 本机直连（生产经 Nginx 反代时替换为 https://<域名>）
tags:
  - name: Quote/Cdeu/Min
    description: 分钟行情 K 线（CDEU）
security:
  - bearerAuth: []
paths:
  /API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert:
    post:
      operationId: upsertCdeuKLineMin
      tags: [Quote/Cdeu/Min]
      summary: 批量写入分钟行情 K 线（upsert）
      description: |
        批量写入；主键冲突时按 `onConflict` 跳过或整行覆盖。

        - `skip`：`ON CONFLICT (usc, ts) DO NOTHING`；请求内重复键不报错。
        - `update`：`ON CONFLICT (usc, ts) DO UPDATE`，覆盖除 `usc`/`ts` 外的
          全部业务列并刷新 `gmt_update`；请求内重复键提前返回 409（211102）。
        - `update` 为整行覆盖：未提供的业务列会被写成 `NULL`；只改部分字段请用
          `PATCH /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch`。
        - `prev_close` 为兼容字段：仅当 `open` 未传或为 `null` 时回填 `open`，自身不落库。
        - `change_price` / `change_ratio` 均未提供时，服务端按 `close` 与 `open`
          自动计算（参考价回退 `open`）。
      parameters:
        - $ref: '#/components/parameters/OnConflict'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/UpsertRequest'
            examples:
              updateMode:
                summary: 默认 update 模式（整行覆盖）
                value:
                  items:
                    - usc: GCMain
                      ts: 1700000000
                      date: 20231114
                      time: 93000
                      open: 3.51
                      high: 3.62
                      low: 3.50
                      close: 3.60
                      volume: 1000
                      turnover: 3560.00
                      provider: FT
              skipMode:
                summary: skip 模式（已存在键跳过）
                value:
                  items:
                    - usc: GCMain
                      ts: 1700000060
                      close: 3.65
      responses:
        '200':
          description: 写入成功
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UpsertResponse'
              examples:
                update:
                  value: { affected: 100, mode: update }
                skip:
                  value: { affected: 98, mode: skip }
        '400':
          $ref: '#/components/responses/BadRequest'
        '401':
          $ref: '#/components/responses/Unauthorized'
        '404':
          $ref: '#/components/responses/NotFound'
        '409':
          $ref: '#/components/responses/Conflict'
        '413':
          $ref: '#/components/responses/PayloadTooLarge'
        '422':
          $ref: '#/components/responses/UnprocessableEntity'
        '500':
          $ref: '#/components/responses/InternalError'
        '503':
          $ref: '#/components/responses/ServiceUnavailable'
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
      description: |
        登录接口 `POST /API/V1/Meta/Finv/Auth/Login` 获取令牌后，
        以 `Authorization: Bearer <token>` 携带。
  parameters:
    OnConflict:
      name: onConflict
      in: query
      required: false
      description: |
        冲突处理模式。兼容旧下划线名 `on_conflict`（同时接受，推荐小驼峰）。
      schema:
        type: string
        enum: [skip, update]
        default: update
  schemas:
    UpsertRequest:
      type: object
      required: [items]
      additionalProperties: false
      properties:
        items:
          type: array
          description: 待写入的 K 线条目；1 ≤ 条数 ≤ `FINV_KLINE_MAX_BATCH_ROWS`（默认 5000）。
          minItems: 1
          maxItems: 5000
          items:
            $ref: '#/components/schemas/KLineItem'
    KLineItem:
      type: object
      required: [usc, ts]
      additionalProperties: false
      description: |
        单条分钟行情。未列出的字段名会触发 422 / 211005（严格解码，未知字段报错）。
      properties:
        usc:
          type: string
          minLength: 1
          maxLength: 255
          description: 统一证券代码（全局唯一，如 `GCMain` / `518880`）。
        ts:
          type: integer
          format: int64
          minimum: 0
          maximum: 100000000000
          description: |
            UTC 时间戳（秒级）。上限 100000000000：超过且服务端配置
            `FINV_KLINE_TS_UNIT=ms` 时按毫秒除以 1000 归一，否则返回 422 / 211004。
        date:
          type: [integer, 'null']
          minimum: 19500101
          maximum: 21001231
          description: 交易日期 `yyyyMMdd`。
        time:
          type: [integer, 'null']
          minimum: 0
          maximum: 235959
          description: 交易时间 `HHmmss`。
        open:
          type: [number, 'null']
          description: 起始价。未传或 `null` 时由 `prev_close` 回填（兼容旧客户端）。
        high:
          type: [number, 'null']
          description: 区间最高价。
        low:
          type: [number, 'null']
          description: 区间最低价。
        close:
          type: [number, 'null']
          description: 最新价/收盘价。
        volume:
          type: [number, 'null']
          minimum: 0
          description: 成交量（不得为负）。
        turnover:
          type: [number, 'null']
          minimum: 0
          description: 成交金额（不得为负）。
        change_price:
          type: [number, 'null']
          description: |
            涨跌额。与 `change_ratio` 均未提供时由服务端按 `close - open`（参考价回退
            `open`）自动计算。
        change_ratio:
          type: [number, 'null']
          description: 涨跌幅。与 `change_price` 均未提供时由服务端自动计算。
        paocd:
          type: [number, 'null']
          description: 当日累计均价。
        ext_field:
          type: [string, 'null']
          description: 预留扩展字段。
        remark:
          type: [string, 'null']
          description: 备注。
        provider:
          type: [string, 'null']
          maxLength: 255
          description: 数据来源（如 `FT`）。
        prev_close:
          type: [number, 'null']
          description: |
            兼容字段：仅在 `open` 未传或 `null` 时回填 `open`；自身不落库。
    UpsertResponse:
      type: object
      required: [affected, mode]
      additionalProperties: false
      properties:
        affected:
          type: integer
          format: int64
          minimum: 0
          description: |
            `skip` 模式为实际插入行数；`update` 模式为插入 + 覆盖影响的行数。
        mode:
          type: string
          enum: [skip, update]
          description: 本次请求实际使用的冲突处理模式。
    ErrorResponse:
      type: object
      required: [code, message, detail, request_id, error]
      additionalProperties: false
      properties:
        code:
          type: integer
          description: 业务错误码（见第 5 节错误目录）。
        message:
          type: string
          description: 面向调用方的安全文案。
        detail:
          type: string
          description: 兼容字段，与 `message` 相同（旧客户端依赖）。
        request_id:
          type: string
          description: 请求标识，便于与服务端日志关联。
        error:
          $ref: '#/components/schemas/ErrorObject'
        debug:
          type: object
          additionalProperties: { type: string }
          description: |
            仅当 `FINV_ERROR_DETAIL_MODE=admin` 且请求来自内网 CIDR 或携带有效
            `X-Debug-Token` 时返回；键为白名单（见第 5 节）。
    ErrorObject:
      type: object
      required: [code, catalog_version, retryable]
      additionalProperties: false
      properties:
        code:
          type: integer
        catalog_version:
          type: string
          description: 错误目录版本（当前 `finvquant-1`）。
        retryable:
          type: boolean
          description: 是否建议重试（仅 211301 / 211302 为 `true`）。
  responses:
    BadRequest:
      description: 缺少必填参数 / 请求体格式错误 / 枚举非法（211001、211002、211006）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    Unauthorized:
      description: 未登录或令牌无效（由认证中间件返回，非 KLine 错误目录）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    NotFound:
      description: 记录不存在（211103）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    Conflict:
      description: 唯一键冲突 / 请求内重复主键 / 状态冲突（211101、211102、211104）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    PayloadTooLarge:
      description: 批量条数或列表长度超过上限（211201、211202）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    UnprocessableEntity:
      description: 字段类型/范围/未知字段/数据库约束问题（211003、211004、211005、211303）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    InternalError:
      description: 服务内部错误（211301，可重试）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
    ServiceUnavailable:
      description: 数据服务不可用（211302，可重试）
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/ErrorResponse'
```

## 3. 请求参数说明

### 3.1 Query 参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 取值 | 说明 |
|---|---|---|---|---|---|---|
| `onConflict` | query | string | 否 | `update` | `skip` / `update` | 冲突处理模式；兼容旧名 `on_conflict`，推荐小驼峰 |
| `Authorization` | header | string | 是 | — | `Bearer <token>` | 认证令牌（登录接口获取） |
| `Content-Type` | header | string | 是 | — | `application/json` | 请求体为 JSON |

### 3.2 请求体 `UpsertRequest`

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `items` | array\<KLineItem\> | 是 | 1 ≤ 条数 ≤ 5000 | 待写入条目；上限由 `FINV_KLINE_MAX_BATCH_ROWS` 控制（默认 5000） |

> 请求体采用**严格解码**：出现未定义字段直接返回 422 / `211005`，不会静默忽略。

### 3.3 条目 `KLineItem`

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
| `volume` | number \| null | 否 | ≥ 0 | 是 | 成交量 |
| `turnover` | number \| null | 否 | ≥ 0 | 是 | 成交金额 |
| `change_price` | number \| null | 否 | — | 是 | 涨跌额；与 `change_ratio` 均未传时自动计算 |
| `change_ratio` | number \| null | 否 | — | 是 | 涨跌幅；与 `change_price` 均未传时自动计算 |
| `paocd` | number \| null | 否 | — | 是 | 当日累计均价 |
| `ext_field` | string \| null | 否 | — | 是 | 预留扩展字段 |
| `remark` | string \| null | 否 | — | 是 | 备注 |
| `provider` | string \| null | 否 | ≤ 255 字符 | 是 | 数据来源（如 `FT`） |
| `prev_close` | number \| null | 否 | — | **否** | 兼容字段：仅用于回填 `open`，自身不落库 |

数值字段只接受 **JSON 数字或 `null`**（字符串 `"3.60"` 会触发 422 / `211003`）。

## 4. 行为与语义

### 4.1 冲突处理

| `onConflict` | 生成 SQL | 请求内重复 `(usc, ts)` | `affected` 含义 |
|---|---|---|---|
| `skip` | `ON CONFLICT (usc, ts) DO NOTHING` | 不报错，天然去重 | 实际插入行数 |
| `update`（默认） | `ON CONFLICT (usc, ts) DO UPDATE SET …` | 提前返回 409 / `211102` | 插入 + 覆盖影响行数 |

`update` 覆盖的列为除 `usc`、`ts` 外的全部业务列（`date`、`time`、`open`、`high`、`low`、
`close`、`volume`、`turnover`、`change_price`、`change_ratio`、`paocd`、`ext_field`、
`remark`、`provider`），并刷新 `gmt_update = now()`。

### 4.2 整行覆盖

`update` 模式是**整行覆盖**：请求中未提供（或显式 `null`）的业务列会被写成 `NULL`。
如需只改部分字段，请使用 `PATCH /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch`。

### 4.3 字段补全与兼容

- `prev_close` → `open`：`open` 未传或为 `null` 时用 `prev_close` 回填；`prev_close` 本身不落库。
- `change_price` / `change_ratio`：两者都未提供时，服务端用 `close` 与参考价（`open`）计算。
- `gmt_create` / `gmt_update` 由数据库默认值与触发器维护，请求体不接受。

### 4.4 事务与幂等

- 单事务：条目校验 → 请求内重复检查（仅 `update`）→ 分批 `INSERT` → 提交；任一步失败整体回滚；
- `skip` 模式天然幂等；`update` 模式重复提交相同内容结果一致。

## 5. 错误目录

| 错误码 | HTTP | 名称 | retryable | 触发场景 | 白名单 `debug` 键 |
|---|---|---|---|---|---|
| `211001` | 400 | `KLineMissingParam` | 否 | 缺少 `usc`/`ts`/`items` | `field` |
| `211002` | 400 | `KLineMalformedJSON` | 否 | 请求体不是合法 JSON | `err_class` |
| `211003` | 422 | `KLineTypeMismatch` | 否 | 字段类型不匹配（如数值字段传字符串） | `field`、`want`、`got` |
| `211004` | 422 | `KLineOutOfRange` | 否 | `usc` 超长、`ts`/`date`/`time` 越界、负 `volume`/`turnover` | `field`、`min`、`max` |
| `211005` | 422 | `KLineUnknownField` | 否 | 请求含未定义字段 | `field` |
| `211006` | 400 | `KLineInvalidEnum` | 否 | `onConflict` 不是 `skip`/`update` | `field`、`allowed` |
| `211101` | 409 | `KLineDuplicateKey` | 否 | 数据库主键/唯一键冲突 | `pg_code`、`sqlstate`、`constraint` |
| `211102` | 409 | `KLineDuplicateInRequest` | 否 | `update` 模式下请求内重复 `(usc, ts)` | `first_dup`、`count` |
| `211103` | 404 | `KLineNotFound` | 否 | 记录不存在（本接口不产生，供其它 K 线接口复用） | `usc`、`ts` |
| `211104` | 409 | `KLineStateConflict` | 否 | 当前状态不允许该操作 | `state`、`reason_class` |
| `211201` | 413 | `KLineBatchTooLarge` | 否 | `items` 条数超过上限 | `limit`、`got` |
| `211202` | 413 | `KLineListTooLarge` | 否 | 列表长度超过上限（其它接口复用） | `limit`、`got` |
| `211301` | 500 | `KLineDBError` | **是** | 数据库执行错误 | `pg_code`、`sqlstate`、`err_class` |
| `211302` | 503 | `KLineDBUnavailable` | **是** | 连接池不可用/数据库不可达 | `pool_state` |
| `211303` | 422 | `KLineDBConstraint` | 否 | 违反业务约束（CHECK/外键等） | `pg_code`、`constraint` |

错误响应示例（`update` 模式请求内重复）：

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
  },
  "debug": {
    "first_dup": "GCMain@1700000000",
    "count": "2"
  }
}
```

> `debug` 仅在 `FINV_ERROR_DETAIL_MODE=admin` 且来自内网 CIDR 或携带有效
> `X-Debug-Token` 时出现；默认安全模式不返回。

## 6. 示例

### 6.1 curl

```bash
curl -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert?onConflict=update' \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{
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
  }'
```

### 6.2 成功响应（HTTP 200）

```json
{"affected": 1, "mode": "update"}
```

### 6.3 常见错误

| 场景 | HTTP | 响应 `code` |
|---|---|---|
| 缺少 `items` 或 `items: []` | 400 | `211001` |
| `ts` 传字符串 `"1700000000"` | 422 | `211003` |
| 多传字段 `foo` | 422 | `211005` |
| `onConflict=merge` | 400 | `211006` |
| `update` 模式两条相同 `(usc, ts)` | 409 | `211102` |
| `items` 超过 5000 条 | 413 | `211201` |

## 7. 已使用位置登记

- 分钟行情采集端增量回补（`onConflict=update`）
- 采集端重复投递兜底（`onConflict=skip`）

## 8. 编写约定（样板说明）

新接口文档按本文件结构组织，**不允许**省略契约：

1. **机器可读契约**：给出完整 OpenAPI 3.1 `openapi` 代码块（`paths`、`parameters`、
   `requestBody`、`responses`、`components` 全量），不得用「同 XXX 文档」代替；
2. **字段表**：每个请求/响应字段列出类型、必填、约束、落库与否、说明；
3. **行为语义**：冲突处理、事务边界、幂等、兼容字段、自动计算规则写清楚；
4. **错误目录**：列全本接口可能返回的 `code`、HTTP、`retryable`、触发场景与 debug 键；
5. **示例**：至少给出 curl、成功响应、典型错误；
6. **索引同步**：新增/修改接口须同步 `Docs/API/APIs.md` 与 `Docs/API/README.md`；
7. **变更日志**：使用者可感知的契约变更须同步 `Docs/CHANGELOG.md`。
