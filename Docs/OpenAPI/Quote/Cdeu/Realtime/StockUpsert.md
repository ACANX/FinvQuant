# POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert — 外部批量 Upsert

> 外部 OpenAPI 端点。业务逻辑复用内部 `POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`
> 的 handler；与内部接口的差异只在**认证方式**与**响应信封**。
> 正文（第 1~8 节）是人读说明，第 9 节附录链接机器可读契约
> [`StockUpsert.yml`](StockUpsert.yml)。契约变更时两者须同时更新。
>
> - 实现：`internal/api/router.go`（`/OpenAPI/V1` 路由组）、`internal/api/middleware.go`（`WrapOpenAPIResponse`）、
>   `internal/api/handler/cdeu_realtime_stock.go`
> - 说明：本文件是**对外独立契约**，字段 / 错误码 / 示例全部自包含，不引用内部 API 文档。

## 1. 概述

供外部采集端 / 数据同步脚本调用，批量写入**证券实时行情快照**
（表 `cdeu.finv_cdeu_secu_realtime_stock`，一行一证券，主键 `stock_id`）：

- 主键不存在 → 插入；已存在 → **只覆盖本次有值的列**（`COALESCE(EXCLUDED.x, t.x)`），
  源端缺字段不会清空已有值；
- 整批在**一条 SQL 语句**内完成，失败整批回滚，不存在部分写入成功；
- 请求体严格解码：出现未定义字段直接 400，不静默忽略；
- 成功响应统一为 `{code, data, message}` 信封，业务字段位于 `data` 节点。

## 2. 认证

> **当前版本暂不鉴权**：`/OpenAPI/V1` 路由组公开访问，无需 token 或 API Key。
> 上线前请通过防火墙 / 内网限制来源；API Key 体系落地后再恢复鉴权。

| 项 | 当前状态 | 后续（API Key 落地后） |
|---|---|---|
| 认证 | 无（公开） | `Authorization: Bearer <key>` 或 `X-API-Key: <key>` |
| 401 | 不会返回（除未来启用鉴权） | 未携带 / 错误密钥 → `401` |

## 3. 请求

### 3.1 Header

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `Content-Type` | string | 是 | `application/json` |

无 URL 查询参数。

### 3.2 请求体

请求体结构为 `{"items": [RealtimeStockItem, ...]}`；`items` 必填，1 ≤ 条数 ≤ 500。

| 字段 | 类型 | 可空 | 必填 | 约束 | 落库 | 说明 |
|---|---|---|---|---|---|---|
| `stock_id` | integer | 否 | **是** | 1 ≤ stock_id ≤ 9223372036854775807（int64）；批内唯一 | 是 | 源系统证券 ID（富途 64 位编号），主键 |
| `usc` | string | 是 | 否 | — | 是 | Finv 统一证券编码（如 `SH600000`） |
| `name` | string | 是 | 否 | — | 是 | 证券名称 |
| `stock_code` | string | 是 | 否 | — | 是 | 行情代码（不含市场前缀） |
| `market_type` | string | 是 | 否 | — | 是 | 市场类型（源系统市场分类编码） |
| `market_label` | string | 是 | 否 | — | 是 | 市场标签（市场显示名称） |
| `instrument_type` | string | 是 | 否 | — | 是 | 品种类型（股票/基金/债券等） |
| `currency` | string | 是 | 否 | ISO 4217 三位字母代码 | 是 | 货币类型（`CNY`/`USD`/`HKD` 等） |
| `market_value` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 总市值 |
| `total_shares` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 总股本（股） |
| `circulation_total_shares` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 流通股本（股） |
| `trade_trunover` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 成交额（当日累计）；列名沿用上游拼写 `trunover` |
| `trade_volumn` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 成交量（当日累计）；列名沿用上游拼写 `volumn` |
| `circulation_market_value` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 流通市值 |
| `dt_update` | integer(int64) | 是 | 否 | ≥ 0（Unix **毫秒**） | 是 | 快照更新时间；省略或 `null` 时由服务端按本次写入时刻填充 |
| `price_nominal` | number | 是 | 否 | `NUMERIC(20,6)` | 是 | 名义价格/单价（源值纯十进制文本） |
| `change_ratio` | number | 是 | 否 | 百分点数值，`NUMERIC(20,6)` | 是 | 涨跌幅（1.03 表示 +1.03%） |
| `trust_aum` | number | 是 | 否 | 单位：元，`NUMERIC(24,6)` | 是 | 信托/基金资产规模（源值万/亿/万亿已换算为元） |
| `trade_change_ratio` | number | 是 | 否 | 百分点数值，`NUMERIC(20,6)` | 是 | 成交额变化率（0.12 表示 +0.12%） |
| `price_amplitude` | number | 是 | 否 | 百分点数值，`NUMERIC(20,6)` | 是 | 价格振幅（1.30 表示 1.30%） |
| `volumn_ratio` | number | 是 | 否 | 无量纲，`NUMERIC(20,6)` | 是 | 量比（1.15 即 1.15 倍） |
| `buysell_ratio` | number | 是 | 否 | 百分点数值，可为负，`NUMERIC(20,6)` | 是 | 买卖比（0.00 表示 0.00%） |
| `change_price` | number | 是 | 否 | `NUMERIC(20,6)` | 是 | 涨跌额（源值 `+0.750` 去正号后写入） |
| `price_direct` | string | 是 | 否 | `CHAR(1)`，取值 `+` / `-` / `0` | 是 | 涨跌方向：`+`=涨（源 up）、`-`=跌（源 down）、`0`=平（源 flat） |
| `leverage` | number | 是 | 否 | `NUMERIC(20,6)`，可为负 | 是 | 杠杆（源值为数字文本，空值不写入） |
| `page` | integer | 是 | 否 | 1 ≤ page ≤ 2147483647（`INTEGER`）；表侧 `page_chk` 约束拒绝 ≤ 0 | 是 | 来源页码：该证券最新快照由**市值排行接口分页拉取**所得时的页码（从 1 开始）；省略/`null` 不影响已有值 |

数值字段只接受 **JSON 数字或 `null`**（字符串 `"123.45"` 会被拒绝）。数值以原始十进制文本入库，
按 `NUMERIC(24,6)` 定点转换，不经过二进制浮点，不会产生精度漂移。

```json
{
  "items": [
    {
      "usc": "SH600000",
      "stock_id": 1001,
      "name": "浦发银行",
      "stock_code": "600000",
      "market_type": "1",
      "market_label": "SH",
      "instrument_type": "1",
      "currency": "CNY",
      "market_value": 123456789.123456,
      "total_shares": 2935208,
      "circulation_total_shares": 2935208,
      "trade_trunover": 1234567.89,
      "trade_volumn": 100000,
      "circulation_market_value": 120000000.5,
      "dt_update": 1767225600000
    }
  ]
}
```

## 4. 行为与语义

### 4.1 冲突处理（字段级覆盖）

```sql
INSERT INTO cdeu.finv_cdeu_secu_realtime_stock (...)
SELECT ... FROM src
ON CONFLICT (stock_id) DO UPDATE SET
    usc                      = COALESCE(EXCLUDED.usc, t.usc),
    name                     = COALESCE(EXCLUDED.name, t.name),
    ...（除主键外全部 14 列均为 COALESCE(EXCLUDED.x, t.x)）
```

- 已存在的 `stock_id`：只覆盖请求中**有值**的列，未提供或显式 `null` 的列保留旧值；
- 空字符串视同「未提供」（数据库侧 `btrim` + `nullif` 后为 `NULL`），不会把旧值写成空串；
- 新行：未提供的列落库为 `NULL`；
- 本接口不支持「主动把某列置空」，需要清列请走维护 SQL。

### 4.2 `dt_update` 刷新（例外）

`dt_update` 不参与 COALESCE：

- 新行：省略或 `null` → 填当前 Unix 毫秒；
- 已存在的行：省略或 `null` → 刷新为本次写入时刻的 Unix 毫秒（不保留旧值）；
- 显式传入 → 按传入值落库（便于采集侧回填采集时刻）。

### 4.3 批内重复与事务

- 批内同一 `stock_id` 出现两次 → 400（同一条语句无法对同一行生效两次，提前拦截并给出位置）；
- 整批一条语句，成功则全部生效、失败则全部回滚；
- 幂等：同一批内容重复提交，`insert` 变为 0、`update` 为条数，业务字段结果一致
  （仅 `dt_update` 随写入时刻推进）。

## 5. 响应

### 5.1 成功（HTTP 200）

```json
{
  "code": 0,
  "data": {
    "total": 1,
    "insert": 1,
    "update": 0
  },
  "message": "OK"
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `code` | integer | 成功码，**默认 `0`**；`1` / `200` / `202` 同样视为正常响应（兼容） |
| `data.total` | integer(int64) | 本次处理行数（= insert + update） |
| `data.insert` | integer(int64) | 新增行数 |
| `data.update` | integer(int64) | 覆盖更新行数 |
| `message` | string | 成功固定为 `OK` |

### 5.2 失败（非 2xx）

沿用内部错误信封，不再套一层 `data`：

```json
{
  "code": 400,
  "message": "实时行情快照入参非法: stock_id=1001 在第 1 条与第 2 条重复"
}
```

## 6. 错误码

| `code` | HTTP | retryable | 触发场景 |
|---|---|---|---|
| `400` | 400 | 否 | 请求体不是合法 JSON / 含未定义字段 / 有多个 JSON 值 |
| `400` | 400 | 否 | `items` 为空或超过 500 条 |
| `400` | 400 | 否 | 缺少 `stock_id`、`stock_id` 非正数或超出 `BIGINT`（int64）范围 |
| `400` | 400 | 否 | 批内 `stock_id` 重复 |
| `400` | 400 | 否 | `dt_update` 为负数 |
| `400` | 400 | 否 | `page` 非正数或超出 `INTEGER` 范围（含 PG `23514`：违反 `page_chk`） |
| `400` | 400 | 否 | 数值超出 `NUMERIC(24,6)` 范围或数值文本非法（PG `22003`/`22P02`） |
| `2006` | 500 | **是** | 写库失败（连不上库、表不存在等） |

## 7. 示例

```bash
# 当前无需认证
curl -X POST 'http://127.0.0.1:16001/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert' \
  -H 'Content-Type: application/json' \
  -d '{"items":[{"stock_id":1001,"usc":"SH600000","currency":"CNY","market_value":123456789.12}]}'
```

## 8. 已使用位置登记

- 外部数据采集 / 行情快照同步脚本（示例接入，端到端联调）

## 9. 附录：OpenAPI 3.1 契约（机器可读）

完整契约见同目录 [`StockUpsert.yml`](StockUpsert.yml)（`openapi: 3.1.0`，**自包含**；
`schema` 已内联，不依赖仓库内部契约）。

```bash
npx @redocly/cli lint Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.yml
npx @redocly/cli preview-docs Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.yml
```
