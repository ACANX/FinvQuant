# POST /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync — 行情列表股票快照同步

> 手动触发「证券列表采集」：读配置表 → 按市场分页拉取 moomoo `get-stock-list`
> → 逐页 upsert 到 `cdeu.finv_cdeu_secu_realtime_stock` → 回写 `count_exists` / `total_available`。
> 机器可读契约：[`StockListCollect.yml`](StockListCollect.yml)（OpenAPI 3.1），两者须成对更新。
>
> - 实现：`internal/api/handler/cdeu_realtime_stock.go`（`RealtimeStockListCollect`）、
>   `internal/api/router.go`、`internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go`（采集逻辑单文件）
> - 落库与配置读写：`internal/meta/finv/quote/cdeu/store_stocklist.go`
> - 关联需求：10976（配置表）、10977（本接口）

## 1. 概述

| 项 | 说明 |
|---|---|
| 认证 | 需登录（`Authorization: Bearer <token>`），未登录返回 401 |
| 请求体 | **全部字段可选**，缺省用配置表 + 默认控制参数 |
| 采集范围 | 来自 `cdeu.finv_cdeu_secu_realtime_sync_config`（`flag_enable='1'` 的行），可用 `instrument_symbol` / `market_type` 过滤，用 `count_plan` 覆盖计划数量 |
| 采集结果 | 写入 `cdeu.finv_cdeu_secu_realtime_stock`（`page` 记录来源页码，`dt_update` 为写入时刻） |
| 配置回写 | 每页回写进度；结束时把接口 `pagination.total` 写入 `count_exists`，把本轮实际可采集数量写入 `total_available` |

## 2. 请求

### 2.1 请求体（全部可选）

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `instrument_symbol` | string | 全部 | 只采集该品种（`stock`/`etf`/`future`） |
| `market_type` | string | 全部 | 只采集该市场（`1`=HK、`2`=US、`4`=CN） |
| `count_plan` | integer | 用配置值 | 覆盖配置表中的计划采集数量 |
| `plate_type` | string | `1` | 接口 `plateType` |
| `rank_type` | string | `3` | 接口 `rankType`（3=市值排行） |
| `page_size` | integer | `50` | 每页条数（1–500） |
| `page_start` | integer | `1` | 页码起始值，默认 `1`（已实测：page=0 时服务端报「page 不能小于 1」） |
| `interval_ms` | integer | `6000` | 页间间隔，避免被服务端限流 |
| `max_retries` | integer | `2` | 单页失败重试次数 |
| `retry_backoff_ms` | integer | `2000` | 重试退避基数（第 n 次等待 n×该值） |
| `request_timeout_ms` | integer | `20000` | 单次请求超时 |
| `max_pages` | integer | `0` | 页数上限，0=不限（仍受计划数量约束） |
| `continue_on_error` | boolean | `true` | 单市场失败后是否继续后续市场 |
| `extra` | object\<string,string\> | — | 预留扩展参数，当前未消费 |

> 扩展约定：新增控制项时优先走 `extra` 透传，或按同名规则在
> `cdeu.StockListCollectOptions` 上加字段，不改动采集主流程。

### 2.2 示例

```bash
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync' \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"market_type":"1","count_plan":100,"page_size":50,"interval_ms":300,"max_pages":2}'
```

## 3. 响应

```json
{
  "code": 0,
  "message": "采集完成",
  "data": {
    "results": [
      {
        "instrument_symbol": "stock",
        "market_type": "1",
        "pages": 2,
        "rows_receive": 100,
        "insert": 4,
        "update": 96,
        "count_exists": 3338,
        "total_available": 100,
        "elapsed_ms": 2118
      }
    ],
    "rows_receive": 100,
    "insert": 4,
    "update": 96,
    "elapsed_ms": 2124
  }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `data.results[]` | array | 每个市场一项（含失败市场，失败时带 `error`） |
| `data.results[].pages` | integer | 本次完成页数 |
| `data.results[].rows_receive` | integer | 收到条目数 |
| `data.results[].insert` / `.update` | integer | 新增 / 覆盖行数 |
| `data.results[].count_exists` | integer | 接口返回的该市场证券总数 |
| `data.results[].total_available` | integer | 本轮实际可采集数量 |
| `data.rows_receive` / `insert` / `update` | integer | 各市场汇总 |
| `data.error` | string | 首个失败市场的错误摘要（全部成功时缺省） |

> 单个市场失败不影响响应码：`code` 仍为 `0`，失败信息在 `data.results[].error`。

## 4. 行为说明

1. **采集范围**：`total_available = min(count_plan, pagination.total)`；`count_plan` 为空取总数。
2. **页数**：`ceil(total_available / page_size)`，不超过接口 `pageCount`，并受 `max_pages` 约束。
3. **限速**：页与页之间等待 `interval_ms`；首页不再额外等待。
4. **重试**：单页失败按 `max_retries` 重试，等待时间 `attempt × retry_backoff_ms`；仍失败则该市场中止并记录 `error`。
5. **截断**：最后一页按剩余计划数量截断，不会多采。
6. **数值口径**（与既有快照一致）：`万/亿/万亿/B/M` 换算成元或股，百分号去掉后按百分点数值入库，`up/down/flat` → `+/-/0`；解析失败的字段留空，不猜值。
7. **幂等**：同一批重复采集只会产生 `update`，业务字段以最后一次为准。
8. **逐页明细日志**：每页落库后按「每行 10 个、每格 11 显示列」打印该页证券代码清单（不含名称），`[I]`=新增、`[U]`=更新（`stock_id` 此前是否存在于 `finv_cdeu_secu_realtime_stock`），并同时写入容器日志（`docker logs`）与异步任务的日志流。

## 5. 错误目录

| `code` | HTTP | 触发场景 | 说明 |
|---|---|---|---|
| `400` | 400 | 请求体不是合法 JSON / 含未定义字段 | 返回解析错误详情 |
| `401` | 401 | 未登录或 token 失效 | 统一鉴权拦截 |
| `2006` | 500 | 无匹配的采集配置 / 读配置失败 | 检查配置表 `flag_enable` 与过滤条件 |

## 6. 相关表

| 表 | 作用 |
|---|---|
| `cdeu.finv_cdeu_secu_realtime_sync_config` | 采集配置（市场 + 品种、计划数量、`count_exists`、`total_available`） |
| `cdeu.finv_cdeu_secu_realtime_stock` | 采集结果（一行一证券，`page` 记录来源页码） |

### 单位与语言口径（需求 10989）

moomoo 同一接口会按**出口 IP + 请求语言**返回中文或英文口径，两者字段格式不同：

| 口径 | 名称示例 | 市值示例 | 单位后缀 |
|------|----------|----------|----------|
| 中文 | `腾讯控股` | `4.07万亿` | `万亿` / `亿` / `万` |
| 英文 | `TENCENT` | `4.07T` | `T`(1e12) / `B`(1e9) / `M`(1e6) / `K`(1e3) |

采集器两层保障：

1. **锁语言**：列表采集的 HTTP 客户端固定携带 `accept-language: zh-CN,zh;q=0.9,en;q=0.8`（与本地 Python 采集程序一致），避免依赖出口 IP；
2. **双口径解析**：`stockListAmount` 同时支持 `万亿/亿/万` 与 `T/B/M/K`（大小写均可），换算到「元/股」基准单位。

> 历史问题：早期只支持中文口径与 `B/M`，在海外 IP 且未锁语言时，`4.07T` 这类万亿级市值解析失败，导致市值 ≥ 1 万亿的证券（腾讯、汇丰、阿里、茅台等）整批异常。首页日志 `[cdeu][stocklist] 首页采样` 会打印服务端原始口径，便于现场核对。
