# POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert — 证券实时行情快照批量 Upsert

> 所属：行情 → CDEU → 证券实时行情快照
> 数据表：`cdeu.finv_cdeu_secu_realtime_stock`（建表迁移 `V300115`，需求 10956）
> 关联需求：10957（建表 10956）
> 实现：`internal/api/handler/cdeu_realtime_stock.go`（`RealtimeStockUpsert`）、
> `internal/meta/finv/quote/cdeu/realtime_stock.go`（模型与校验）、
> `internal/meta/finv/quote/cdeu/store_realtime_stock.go`（`UpsertRealtimeStocks`）
> 机器可读契约：[`RealtimeStockUpsert.yml`](RealtimeStockUpsert.yml)（OpenAPI 3.1）

## 1. 概述

一次请求批量写入**证券实时行情快照**，主键为 `stock_id`（一行一证券）：

- 主键不存在 → 插入；已存在 → **只覆盖本次有值的列**（`COALESCE(EXCLUDED.x, t.x)`），
  源端缺字段不会清空已有值（与日K `UpsertDayKlines` 一致）；
- 审计时间列（`gmt_create` / `gmt_update`，需求 11029，`V300128` 新增）**不接受本接口写入**：
  新增行由 `DEFAULT now()` 取当前时间，此后每次 UPDATE 由触发器 `public.vq_set_gmt_update()` 自动刷新 `gmt_update`；

  `rmb_trade_trunover` / `rmb_price_nominal`，需求 11023，`V300126` 新增）**暂不支持本接口写入**：
  请求体里的同名字段不存在，落库为 NULL（表示「尚未补全」，不是 0）；补全任务后续实现；
- 本表是快照表，只保留最新状态、不保留历史；历史序列看
  `cdeu.finv_quote_secu_kline_min`（分钟线）与 `cdeu.finv_quote_secu_kline_day`（日线）；
- 整批在**一条 SQL 语句**内完成（`jsonb_to_recordset` 展开 + 单次 `INSERT`），
  语句失败即整批回滚，不存在「部分写入成功」；
- 请求体严格解码：出现未定义字段直接 400，不静默忽略；
- 同时注册对外 OpenAPI 路径 `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`
  （同一 handler，当前暂不鉴权；对外契约见 [`Docs/OpenAPI/Quote/Cdeu/Realtime/StockUpsert.md`](../../OpenAPI/Quote/Cdeu/Realtime/StockUpsert.md)）。

## 2. 请求参数

### 2.1 Header 与 Query

| 参数 | 位置 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|---|
| `Authorization` | header | string | 是 | — | 认证令牌 `Bearer <token>`（登录接口获取） |
| `Content-Type` | header | string | 是 | — | `application/json` |

无 URL 查询参数。

### 2.2 请求体 `RealtimeStockUpsertRequest`

| 字段 | 类型 | 必填 | 约束 | 说明 |
|---|---|---|---|---|
| `items` | array\<RealtimeStockItem\> | 是 | 1 ≤ 条数 ≤ 500 | 待写入的快照行；上限常量 `MaxRealtimeStockBatchRows` |

### 2.3 条目 `RealtimeStockItem`

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
| `remark` | string | 是 | 否 | 无（`TEXT`） | 是 | 备注（需求 11028）：人工标注或采集端说明，不参与换算/排序；空串视同未提供，**未提供不会清空已有备注** |
| `trade_volumn` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 成交量（当日累计）；列名沿用上游拼写 `volumn` |
| `circulation_market_value` | number | 是 | 否 | `NUMERIC(24,6)` 可表示范围 | 是 | 流通市值 |
| `dt_update` | integer(int64) | 是 | 否 | ≥ 0（Unix **毫秒**） | 是 | 快照更新时间；省略或 `null` 时服务端填当前 Unix 毫秒 |
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

> 数值字段只接受 **JSON 数字或 `null`**（`"123.45"` 这种字符串会被拒绝）。
> 数值以原始十进制文本入库并按 `NUMERIC(24,6)` 定点转换，不经过 `float64`，
> 因此不会出现二进制浮点往返误差；超出定点范围或被数据库拒绝时返回 400。

完整请求示例：

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
      "total_shares": 2935208.0,
      "circulation_total_shares": 2935208.0,
      "trade_trunover": 1234567.89,
      "trade_volumn": 100000.0,
      "circulation_market_value": 120000000.5,
      "dt_update": 1767225600000
    }
  ]
}
```

## 3. 行为与语义

### 3.1 冲突处理

```sql
INSERT INTO cdeu.finv_cdeu_secu_realtime_stock (...)
SELECT ... FROM src
ON CONFLICT (stock_id) DO UPDATE SET
    usc                      = COALESCE(EXCLUDED.usc, t.usc),
    name                     = COALESCE(EXCLUDED.name, t.name),
    ...（除主键外全部 14 列均为 COALESCE(EXCLUDED.x, t.x)）
```

无 `onConflict` 参数：本接口只有一种模式——**字段级覆盖**（只覆盖本次有值的列）。

### 3.2 字段级覆盖（源端缺字段不清空）

已存在的 `stock_id` 只覆盖请求中**有值**的列；未提供或显式 `null` 的列保留数据库中的旧值。
这与日K `UpsertDayKlines` 的口径一致：采集端本次拿不到的字段（如仅部分行情源提供的
`circulation_total_shares`）不会把其它来源已写入的值抹掉。

- 空字符串视同「未提供」：数据库侧 `btrim` + `nullif` 后按 `NULL` 处理，不会把旧值写成空串；
- 需要把某列主动置空时，本接口不支持（快照表不允许通过 upsert 清列），请走单独的维护 SQL；
- **例外**：`dt_update` 不受 COALESCE 保护——见 3.3。

### 3.3 `dt_update` 刷新

- 新行：`dt_update` 省略或为 `null` → 填当前 Unix 毫秒；
- 已存在的行：`dt_update` 省略或为 `null` → 刷新为本次写入时刻的 Unix 毫秒（不保留旧值），
  以便通过该列判断「这行最近一次被写入的时间」；显式传入则按传入值落库（便于采集侧回填采集时刻）。

### 3.4 批内重复与事务

- 批内同一 `stock_id` 出现两次 → 400（同一条语句无法对同一行生效两次，提前拦截并给出重复位置）；
- 整批一条语句，成功则全部生效、失败则全部回滚；
- 幂等：同一批内容重复提交，`insert` 变为 0、`update` 为条数，业务字段结果一致（仅 `dt_update` 随写入时刻推进）。

## 4. 响应

成功（HTTP 200）：

```json
{
  "code": 0,
  "message": "写入完成",
  "data": {
    "total": 1,
    "insert": 1,
    "update": 0
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.total` | integer | 本次处理行数 |
| `data.insert` | integer | 新增行数（`xmax = 0`） |
| `data.update` | integer | 覆盖更新行数 |

## 5. 错误目录

| `code` | HTTP | retryable | 触发场景 | 说明 |
|---|---|---|---|---|
| `400` | 400 | 否 | 请求体不是合法 JSON / 含未定义字段 / 多余 JSON 值 | 返回解析错误详情 |
| `400` | 400 | 否 | `items` 为空或超过 500 条 | 提示条数上限 |
| `400` | 400 | 否 | 缺少 `stock_id`、`stock_id` 非正数或超出 `BIGINT`（int64）范围 | 提示第几条 |
| `400` | 400 | 否 | 批内 `stock_id` 重复 | 提示重复的 `stock_id` 与两个位置 |
| `400` | 400 | 否 | `dt_update` 为负数 | 提示第几条 |
| `400` | 400 | 否 | `page` 非正数或超出 `INTEGER` 范围（含 PG `23514`：违反 `page_chk`） | 提示第几条 |
| `400` | 400 | 否 | 数值超出 `NUMERIC(24,6)` 范围或数值文本非法（PG `22003`/`22P02`） | 数据问题，修正后重试 |
| `2006` | 500 | 是 | 写库失败（连不上库、表不存在等） | 服务端错误 |

## 6. 调用示例

```bash
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert' \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "items": [
          {"stock_id": 1001, "usc": "SH600000", "name": "浦发银行", "stock_code": "600000",
           "market_type": "1", "market_label": "SH", "instrument_type": "1", "currency": "CNY",
           "market_value": 123456789.123456, "total_shares": 2935208,
           "circulation_total_shares": 2935208, "trade_trunover": 1234567.89,
           "trade_volumn": 100000, "circulation_market_value": 120000000.5,
           "dt_update": 1767225600000}
        ]
      }'
```

典型错误（批内重复）：

```json
{"code": 400, "message": "实时行情快照入参非法: stock_id=1001 在第 1 条与第 2 条重复"}
```

## 7. 已使用位置

| 使用方 | 说明 |
|---|---|
| 采集端（仓库外工具 / 上游同步脚本） | 按快照批次调用本接口写入 `cdeu.finv_cdeu_secu_realtime_stock`；当前仓库内无前端菜单调用 |
| 外部 OpenAPI 调用方 | `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockUpsert`（暂不鉴权，见对外文档） |

## 8. 校验与生成

```bash
# 契约语法校验（需安装 spectral 或 openapi-generator）
npx --yes @stoplight/spectral-cli lint Docs/API/Cdeu/RealtimeStockUpsert.yml

# 契约与路由一致性（人工核对）
grep -n 'Realtime/FTStockUpsert' internal/api/router.go
```

## 9. 附录：OpenAPI 3.1 契约

机器可读契约见同目录 [`RealtimeStockUpsert.yml`](RealtimeStockUpsert.yml)（`paths`、`components.<schemas>` 与
`securitySchemes` 均自包含）。契约与本文档同步维护，任一侧变更须同时更新另一侧。
