# 行情快照手动同步 — 异步启动 + 逐页日志轮询

> 网页「行情 → 实时行情 → 行情快照手动同步」使用的后端端点。
> 机器可读契约：[`StockSnapshotSyncRun.yml`](StockSnapshotSyncRun.yml)（OpenAPI 3.1），两者须成对更新。
>
> - 采集逻辑：`internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go`（单文件，可整体迁往 MetaExt）
> - 运行状态与日志：`internal/meta/finv/quote/cdeu/stocklist_run.go`（内存保留最近 10 次运行、每次 500 行日志）
> - 关联需求：10976（配置表）、10977（同步接口）、10982（异步任务 + 前端菜单）

## 1. 两个端点

| 方法 | 路径 | 作用 |
|---|---|---|
| `POST` | `/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run` | 异步启动一次采集，立即返回 `run_id` |
| `GET` | `/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run?runId=` | 轮询运行状态、逐页日志与采集配置 |

两者都需要登录。同步版本（无 `/Run` 后缀）仍保留，供脚本一次性调用并等待结果。

## 2. 启动任务

### 2.1 请求体（全部可选）

| 字段 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `instrument_symbol` | string | 全部 | 只采集该品种 |
| `market_type` | string | 全部 | 只采集该市场（`1`=HK、`2`=US、`4`=CN） |
| `count_plan` | integer | 配置值 | 覆盖计划采集数量 |
| `page_size` | integer | `50` | 每页条数（1–500） |
| `page_start` | integer | `1` | 页码起始值（已实测 page=0 报错，不得传 0） |
| `interval_ms` | integer | `6000` | 页间间隔（限速） |
| `max_retries` | integer | `2` | 单页失败重试次数 |
| `retry_backoff_ms` | integer | `2000` | 重试退避基数 |
| `request_timeout_ms` | integer | `20000` | 单次请求超时 |
| `max_pages` | integer | `0` | 页数上限，0=不限 |
| `continue_on_error` | boolean | `true` | 单市场失败后是否继续 |
| `extra` | object | — | 预留扩展参数 |

```bash
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run' \
  -H 'Authorization: Bearer <token>' -H 'Content-Type: application/json' \
  -d '{"market_type":"1","count_plan":60,"page_size":20,"interval_ms":300,"max_pages":3}'
```

### 2.2 响应

```json
{"code":0,"message":"任务已启动","data":{"run_id":"run-20261104T180350-ec31","status":"RUNNING", "...": "..."}}
```

**并发约束**：同一时间只允许一个 `RUNNING` 任务；已有任务在跑时返回 `400` 并提示当前 `run_id`。

## 3. 轮询状态

`GET .../FTStockQuoteSnapshotSync/Run`（`runId` 省略时取最近一次）：

```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "run": {
      "run_id": "run-20261104T180350-ec31",
      "status": "SUCCESS",
      "elapsed_ms": 4775,
      "results": [
        {"instrument_symbol":"stock","market_type":"1","pages":3,"rows_receive":60,
         "insert":1,"update":59,"count_exists":3337,"total_available":60,"elapsed_ms":4769}
      ],
      "logs": [
        {"time":"18:03:50","level":"INFO","market":"","page":0,"message":"任务已启动：每页 20 条、页间间隔 300ms、重试 2 次"},
        {"time":"18:03:53","level":"INFO","market":"stock/1","page":0,"message":"开始采集：市场总数 3337，本轮计划 60，按每页 20 共 3 页"},
        {"time":"18:03:53","level":"INFO","market":"stock/1","page":1,"message":"第 1/3 页：20 条（累计 新增 0 / 覆盖 20）\n[U] 02328   [U] 00267   [U] 02259   [U] 06160   [U] 01276   [U] 06030   [U] 00016   [U] 09618   [U] 80016   [U] 01919\n[U] 03993   ..."},
        {"time":"18:03:54","level":"INFO","market":"stock/1","page":2,"message":"第 2/3 页：20 条（累计 新增 1 / 覆盖 39）"},
        {"time":"18:03:55","level":"INFO","market":"stock/1","page":3,"message":"第 3/3 页：20 条（累计 新增 1 / 覆盖 59）"},
        {"time":"18:03:55","level":"INFO","market":"stock/1","page":3,"message":"采集结束：3 页、收到 60 条、新增 1、覆盖 59"}
      ]
    },
    "configs": [
      {"instrument_symbol":"stock","market_type":"1","count_plan":2500,"count_exists":3337,"total_available":60}
    ]
  }
}
```

| 字段 | 说明 |
|---|---|
| `data.run.status` | `RUNNING` / `SUCCESS` / `PARTIAL` / `FAILED` |
| `data.run.results[]` | 每个市场的结果（失败时带 `error`） |
| `data.run.logs[]` | 逐页日志：`begin` / `page` / `retry` / `done`，`level` 为 `INFO`/`WARN`/`ERROR` |
| `data.configs[]` | 配置表当前内容（含回写的 `count_exists` / `total_available`） |
| 无运行记录 | `run.status = IDLE`、`run_id` 空串 |

## 4. 日志级别规则

| 事件 | level | 说明 |
|---|---|---|
| 任务启动 | INFO | 含每页条数、间隔、重试次数 |
| 市场开始 | INFO | 含市场总数、本轮计划、总页数 |
| 单页完成 | INFO | 首行为页码/条数/累计新增覆盖；其后为**逐页明细**：每行 10 个证券、每页最多 5 行，只展示代码，`[I]`=新增、`[U]`=更新，每格 11 显示列对齐 |
| 单页重试 | WARN | 含重试次数与失败原因 |
| 市场结束 | INFO / ERROR | 成功为 INFO 汇总，失败为 ERROR |

## 5. 错误目录

| `code` | HTTP | 触发场景 |
|---|---|---|
| `400` | 400 | 请求体非法；已有任务在运行 |
| `401` | 401 | 未登录或 token 失效 |

## 6. 前端

- 菜单：行情 → 实时行情 → 行情快照手动同步（`/Meta/Finv/Quote/Realtime/FTStockQuoteSnapshotSync`）
- 视图：`Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue`
- 交互：展示配置表 → 选择市场/覆盖计划数量/节奏参数 → 开始同步 → 每 1.5s 轮询并追加日志（自动滚到底部）
