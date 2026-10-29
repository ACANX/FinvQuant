# POST /API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert — 批量 Upsert

> **本文是接口文档的 OpenAPI 3.1 样板**：正文（第 1~6 节）是人类可读说明；
> 第 8 节附录链接到同名 `KLineUpsert.yml`（机器可读 `openapi` 契约）。后续新增/修改接口文档请按本结构编写
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

## 2. 请求参数说明

### 2.1 Query 参数

| 参数 | 位置 | 类型 | 必填 | 默认 | 取值 | 说明 |
|---|---|---|---|---|---|---|
| `onConflict` | query | string | 否 | `update` | `skip` / `update` | 冲突处理模式；兼容旧名 `on_conflict`，推荐小驼峰 |
| `Authorization` | header | string | 是 | — | `Bearer <token>` | 认证令牌（登录接口获取） |
| `Content-Type` | header | string | 是 | — | `application/json` | 请求体为 JSON |

### 2.2 请求体 `UpsertRequest`

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `items` | array\<KLineItem\> | 是 | 1 ≤ 条数 ≤ 5000 | 待写入条目；上限由 `FINV_KLINE_MAX_BATCH_ROWS` 控制（默认 5000） |

> 请求体采用**严格解码**：出现未定义字段直接返回 422 / `211005`，不会静默忽略。

### 2.3 条目 `KLineItem`

| 字段 | 类型 | 可空 | 必填 | 约束 | 落库 | 说明 |
| --- | --- | --- | --- | --- | --- | --- |
| `usc` | string | 否 | **是** | 1–255 字符 | 是 | 统一证券代码（`GCMain` / `518880`） |
| `ts` | integer(int64) | 否 | **是** | 0 ≤ ts ≤ 100000000000 | 是 | UTC 秒级时间戳；`FINV_KLINE_TS_UNIT=ms` 时可传 13 位毫秒并自动 /1000 |
| `date` | integer | 是 | 否 | 19500101–21001231 | 是 | 交易日期 `yyyyMMdd` |
| `time` | integer | 是 | 否 | 0–235959 | 是 | 交易时间 `HHmmss` |
| `open` | number | 是 | 否 | — | 是 | 起始价；缺失时用 `last_close` 回填 |
| `high` | number | 是 | 否 | — | 是 | 区间最高价 |
| `low` | number | 是 | 否 | — | 是 | 区间最低价 |
| `close` | number | 是 | 否 | — | 是 | 最新价/收盘价 |
| `last_close` | number | 是 | 否 | — | 是 | 上一收盘价；参与涨跌计算，`open` 缺失时回填 `open` |
| `volume` | number | 是 | 否 | ≥ 0 | 是 | 成交量（支持小数，`numeric(20,6)`） |
| `turnover` | number | 是 | 否 | ≥ 0 | 是 | 成交金额 |
| `change_price` | number | 是 | 否 | — | 是 | 涨跌额；与 `change_ratio` 均未传时自动计算 |
| `change_ratio` | number | 是 | 否 | — | 是 | 涨跌幅；与 `change_price` 均未传时自动计算 |
| `pacd` | number | 是 | 否 | — | 是 | 当日累计均价（库列名 `paocd`） |
| `ext_field` | string | 是 | 否 | — | 是 | 预留扩展字段 |
| `remark` | string | 是 | 否 | — | 是 | 备注 |
| `provider` | string | 是 | 否 | ≤ 255 字符 | 是 | 数据来源（如 `FT`） |


完整请求示例（包含全部 17 个字段）：

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
      "last_close": 3.50,
      "volume": 1000,
      "turnover": 3560.00,
      "change_price": 0.10,
      "change_ratio": 2.857143,
      "pacd": 3.56,
      "ext_field": null,
      "remark": null,
      "provider": "FT"
    }
  ]
}
```

数值字段只接受 **JSON 数字或 `null`**（字符串 `"3.60"` 会触发 422 / `211003`）。

## 3. 行为与语义

### 3.1 冲突处理

| `onConflict` | 生成 SQL | 请求内重复 `(usc, ts)` | `affected` 含义 |
|---|---|---|---|
| `skip` | `ON CONFLICT (usc, ts) DO NOTHING` | 不报错，天然去重 | 实际插入行数 |
| `update`（默认） | `ON CONFLICT (usc, ts) DO UPDATE SET …` | 提前返回 409 / `211102` | 插入 + 覆盖影响行数 |

`update` 覆盖的列为除 `usc`、`ts` 外的全部业务列（`date`、`time`、`open`、`high`、`low`、
`close`、`last_close`、`volume`、`turnover`、`change_price`、`change_ratio`、`pacd`、`ext_field`、
`remark`、`provider`），并刷新 `gmt_update = now()`。

### 3.2 整行覆盖

`update` 模式是**整行覆盖**：请求中未提供（或显式 `null`）的业务列会被写成 `NULL`。
如需只改部分字段，请使用 `PATCH /API/V1/Meta/Finv/Quote/Cdeu/Min/Batch`。

### 3.3 字段补全与兼容

- `last_close`：落库（上一收盘价）；`open` 未传或为 `null` 时用 `last_close` 回填 `open`。
- `change_price` / `change_ratio`：两者都未提供时，服务端用 `close` 与参考价（`open`）计算。
- `gmt_create` / `gmt_update` 由数据库默认值与触发器维护，请求体不接受。

### 3.4 事务与幂等

- 单事务：条目校验 → 请求内重复检查（仅 `update`）→ 分批 `INSERT` → 提交；任一步失败整体回滚；
- `skip` 模式天然幂等；`update` 模式重复提交相同内容结果一致。

## 4. 错误目录

| 错误码 | HTTP | 名称 | retryable | 触发场景 | 白名单 `debug` 键 |
|---|---|---|---|---|---|
| `211001` | 400 | `KLineMissingParam` | 否 | 缺少 `usc`、`ts`、`items` | `field` |
| `211002` | 400 | `KLineMalformedJSON` | 否 | 请求体不是合法 JSON | `err_class` |
| `211003` | 422 | `KLineTypeMismatch` | 否 | 字段类型不匹配（如数值字段传字符串） | `field`、`want`、`got` |
| `211004` | 422 | `KLineOutOfRange` | 否 | `usc` 超长，`ts`、`date`、`time` 越界，`volume`、`turnover` 为负 | `field`、`min`、`max` |
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

## 5. 示例

### 5.1 curl

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
        "last_close": 3.50,
        "volume": 1000,
        "turnover": 3560.00,
        "change_price": 0.10,
        "change_ratio": 2.857143,
        "pacd": 3.56,
        "ext_field": null,
        "remark": null,
        "provider": "FT"
      }
    ]
  }'
```

### 5.2 成功响应（HTTP 200）

```json
{
  "affected": 1,
  "mode": "update"
}
```

### 5.3 常见错误

| 场景 | HTTP | 响应 `code` |
|---|---|---|
| 缺少 `items` 或 `items: []` | 400 | `211001` |
| `ts` 传字符串 `"1700000000"` | 422 | `211003` |
| 多传字段 `foo` | 422 | `211005` |
| `onConflict=merge` | 400 | `211006` |
| `update` 模式两条相同 `(usc, ts)` | 409 | `211102` |
| `items` 超过 5000 条 | 413 | `211201` |

## 6. 已使用位置登记

- 分钟行情采集端增量回补（`onConflict=update`）
- 采集端重复投递兜底（`onConflict=skip`）

## 7. 编写约定（样板说明）

新接口文档按本文件结构组织，**不允许**省略契约：

1. **机器可读契约**：给出完整 OpenAPI 3.1 `openapi` 代码块（`paths`、`parameters`、
   `requestBody`、`responses`、`components` 全量），不得用「同 XXX 文档」代替；
2. **字段表**：每个请求/响应字段列出类型、必填、约束、落库与否、说明；
3. **行为语义**：冲突处理、事务边界、幂等、兼容字段、自动计算规则写清楚；
4. **错误目录**：列全本接口可能返回的 `code`、HTTP、`retryable`、触发场景与 debug 键；
5. **示例**：至少给出 curl、成功响应、典型错误；
6. **索引同步**：新增/修改接口须同步 `Docs/API/APIs.md` 与 `Docs/API/README.md`；
7. **变更日志**：使用者可感知的契约变更须同步 `Docs/CHANGELOG.md`。
## 8. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`KLineUpsert.yml`](KLineUpsert.yml)（`openapi: 3.1.0`，含 `paths` /
`components` / `securitySchemes`）。本文正文与该文件逐项对应，**契约变更时两者须同时更新**。

```bash
# 规范校验（任选其一）
npx @redocly/cli lint Docs/API/Quote/Cdeu/Min/KLineUpsert.yml
npx swagger-cli validate Docs/API/Quote/Cdeu/Min/KLineUpsert.yml

# 生成交互式文档
npx @redocly/cli preview-docs Docs/API/Quote/Cdeu/Min/KLineUpsert.yml

# 生成客户端/服务端骨架
npx @openapitools/openapi-generator-cli generate \
  -i Docs/API/Quote/Cdeu/Min/KLineUpsert.yml -g typescript-fetch -o ./generated
```
