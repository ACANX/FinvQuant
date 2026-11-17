# 行情快照手动同步 — 异步启动 + 逐页日志轮询

> 网页「行情 → 实时行情 → 行情快照手动同步」使用的后端端点。
> 机器可读契约：[`StockSnapshotSyncRun.yml`](StockSnapshotSyncRun.yml)（OpenAPI 3.1），两者须成对更新。
>
> - 采集逻辑：`internal/meta/finv/quote/cdeu/quote_list_stock_snapshot_sync.go`（股票，单文件可整体迁往 MetaExt）
>   与 `quote_list_futures_snapshot_sync.go`（期货，需求 11274，同款单文件）
> - 运行状态与日志：`internal/meta/finv/quote/cdeu/stocklist_run.go`（内存保留最近 10 次运行、每次 500 行日志）
> - 关联需求：10976（配置表）、10977（同步接口）、10982（异步任务 + 前端菜单）、11273/11274（期货）

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
| `instrument_symbol` | string | 全部 | 只采集该品种（`future`=期货分支，见 2.4） |
| `market_type` | string | 全部 | 只采集该市场（`1`=HK、`2`=US、`4`=CN）；期货分支不生效（市场取配置行） |
| `futures_type` | string | 全部 | 只采集该期货分类（`1`~`8`，仅 `instrument_symbol=future` 生效；`all`=全部） |
| `count_plan` | integer | 配置值 | 覆盖计划采集数量 |
| `page_size` | integer | `50` | 每页条数（1–500） |
| `page_start` | integer | `0` | 页码起始值（本接口 **0-based**，`0` 即第一页；直接透传上游 `GetStockList`） |
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

### 2.3 按品种分支：BDFS 采集（需求 11262 / 11263）

`instrument_symbol` 为 `bsestock` 或 `cnetf` 时走**独立分支**，不读 `cdeu.finv_cdeu_secu_realtime_sync_config`、
不分市场，直接采集百度股市通成分股：

| 品种 | 来源 | instrument_type |
|---|---|---|
| `bsestock` | 板块 `GN2309`（北交所上市企业） | `stock` |
| `cnetf` | 指数 `399380`（国证ETF，沪深京 ETF） | `etf` |

其余参数语义对两者一致：

| 项 | 行为 |
|---|---|
| 请求体 | 只需 `instrument_symbol: "bsestock"`；`market_type` / `count_plan` 不生效（前端固定传 `market_type: "BJ"`，服务端忽略） |
| 生效参数 | `page_size`（前端默认 500，接口 `rn`；服务端缺省也按 500）、`request_timeout_ms`（默认 20000） |
| 不生效参数 | `interval_ms` / `max_retries` / `retry_backoff_ms` / `max_pages`（单次全量拉取，无页间节流） |
| 落库表 | `cdeu.finv_quote_secu_realtime`，`provider` 固定 `BDFS`，按 `usc` 整行覆盖 |
| usc 解析 | `(exchange, code)` → `public.finv_security.usc`；解析不到的行跳过并记 WARN 日志 |
| 运行状态 / 日志 | 与 moomoo 分支共用同一运行持有者与轮询端点，`results[].instrument_symbol` 为 `bsestock`、`market_type` 为 `BJ` |
| 主要失败原因 | 接口风控（`hit risk`，需降频或换出口网络）、板块无数据、全部行未解析到 usc |

### 2.4 按品种分支：期货列表采集（需求 11274）

`instrument_symbol` 为 `future` 时走**独立分支**：读 `cdeu.finv_quote_cdeu_future_sync_config`，
按分类分页拉取 moomoo `get-futures-list`，逐页写 `cdeu.finv_quote_realtime_future`。

| 项 | 行为 |
|---|---|
| 请求体 | `instrument_symbol: "future"` 必填；`futures_type` 可选（`1`~`8`，空/`all`=全部启用分类） |
| 生效参数 | `page_size`（默认 **50**，与股票/ETF 采集一致；上游 `DefaultFuturesPageSize` 为 40，本采集器显式覆盖）、`page_start`（默认 0，本接口 0-based）、`interval_ms`（默认 2000）、`max_retries`（2）、`retry_backoff_ms`（2000）、`request_timeout_ms`（20000）、`max_pages`（0=不限）、`count_plan`（覆盖每分类计划数量） |
| 不生效参数 | `market_type`（每个分类的 `marketType` 取配置行，默认 `0`=聚合口径）、`plate_type`、`rank_type` |
| 落库表 | `cdeu.finv_quote_realtime_future`，**主键单列 `stock_id`**，整行覆盖；`futures_type` 由最后采集的分类覆盖（采集按分类升序） |
| 字段口径 | 接口 `fields` 是位置化字典：本仓库只把 `1`=合约代码、`2`=名称、`3`=价格、`4`=涨跌幅（去 `%` 按百分点）、`19`=市场标签提成列；其余原样进 `fields_json`，`field_list_json` 留档 |
| `usc` 解析 | ① `stock_id` → `futu.finv_futu_quote_collect.usc`；② `upper(市场标签.合约代码)` → `futu.finv_futu_secu_key.usc`；两级都不命中写 NULL（不用合约代码冒充） |
| 下架标记 | 整轮「全部启用分类都成功」时，把 `dt_last_seen < 本轮开始时刻` 的在售行置 `flag_online='0'`；单分类运行或任一分类失败**跳过**（结果表主键单列 `stock_id`，按分类分批置下架会误标其它分类） |
| 运行状态 / 日志 | 与 moomoo 股票分支共用同一运行持有者与轮询端点；日志 `market` 段为 `future/<分类>`，整轮下架汇总为 `future/all` |
| 结果行 | `results[]` 每项带 `futures_type` / `futures_type_name`，`market_type` 为配置行的市场取值 |

```bash
curl -sS -X POST 'http://127.0.0.1:16001/API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run' \
  -H 'Authorization: Bearer <token>' -H 'Content-Type: application/json' \
  -d '{"instrument_symbol":"future","futures_type":"5","interval_ms":200}'
```

> 分类中文名已于 2026-11-18 回填（迁移 `V300217`）：`1=指数期货、2=能源化工、3=农产品、4=金属期货、
> 5=利率期货、6=汇率期货、7=加密货币期货、8=股票期货`。仍未填时响应与页面降级显示「分类 N」，
> 单独改名只需 `UPDATE cdeu.finv_quote_cdeu_future_sync_config SET futures_type_name='…' WHERE futures_type='5'`。

### 2.5 按品种分支：ETF 列表采集（需求 11279）

`instrument_symbol` 为 `etf` 时接口换成 moomoo `get-etf-list`，其余流程（分页/限速/重试/截断/落库/进度回写/明细日志）与股票分支一致。

| 项 | 行为 |
|---|---|
| 请求 | `get-etf-list?routeType=1&plateId=<板块>&rankType=6&leverage=&page=0&pageSize=50`（`leverage` 空串也必须参与签名，上游已处理） |
| 市场判定 | 该接口没有 `marketType`，市场由 `plateId` 决定：**`900075`=港股 ETF、`10002799`=美股 ETF**（代码常量 `etfPlateIDByMarket`；未登记市场直接报错、不发请求） |
| 配置行 | 迁移 `V300218` 已 seed `instrument_symbol='etf'` + `market_type='1'/'2'` 两行（`count_plan` 为空=全量） |
| 覆盖 | 请求体 `plate_id` 可临时指定板块 |
| 落库 | `cdeu.finv_quote_secu_realtime`（`provider=FTMM`），条目与股票同构，`instrument_type` 由接口给出（ETF 为 `4`） |
| 结果行 | `results[].instrument_symbol='etf'`、`market_type` 为 `1`/`2`；日志 `market` 段 `etf/1`、`etf/2` |

> `cnetf`（沪深京 ETF）走 BDFS 分支，与本节无关。

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
    ],
    "futures_configs": [
      {"market_type":"0","futures_type":"5","futures_type_name":null,"count_plan":null,"count_exists":14,"total_available":14}
    ],
    "error": null,
    "futures_error": null
  }
}
```

| 字段 | 说明 |
|---|---|
| `data.run.status` | `RUNNING` / `SUCCESS` / `PARTIAL` / `FAILED` |
| `data.run.results[]` | 每个市场的结果（失败时带 `error`） |
| `data.run.logs[]` | 逐页日志：`begin` / `page` / `retry` / `done`，`level` 为 `INFO`/`WARN`/`ERROR` |
| `data.configs[]` | 股票等 moomoo 列表采集配置（含回写的 `count_exists` / `total_available`）；**任何分支都会返回**，含"暂无运行记录" |
| `data.futures_configs[]` | 期货分类配置（`cdeu.finv_quote_cdeu_future_sync_config`，`futures_type` 1~8）；同样任何分支都返回（需求 11274） |
| `data.error` | 读取股票配置失败的原因（正常为 `null`）；前端据此提示，避免只显示"暂无启用的采集配置" |
| `data.futures_error` | 读取期货配置失败的原因（正常为 `null`） |
| 无运行记录 | 响应为**扁平结构**（不是 `data.run`）：`run_id` 空串、`status = IDLE`、`logs`/`results` 空数组，且同样带 `configs` 与 `error` |

> 注意：服务刚重启时内存中的运行记录为空，此时若 `configs` 不返回，前端会误判为"该品种暂无启用的采集配置"（需求 10990）。
> 因此本接口在「有运行记录」与「无运行记录」两个分支都必须返回 `configs`。

```json
{"code":0,"message":"暂无运行记录","data":{"run_id":"","status":"IDLE","logs":[],"results":[],
 "configs":[{"instrument_symbol":"stock","market_type":"1","count_plan":2500,"count_exists":3337,"total_available":200}],
 "error":null}}
```

## 4. 日志级别规则

| 事件 | level | 说明 |
|---|---|---|
| 任务启动 | INFO | 含每页条数、间隔、重试次数 |
| 市场开始 | INFO | 含市场总数、本轮计划、总页数 |
| 单页完成 | INFO | 首行为页码/条数/累计新增覆盖；其后为**逐页明细**：每行 10 个证券、每页最多 5 行，只展示代码，`[I]`=新增、`[U]`=更新，每格 11 显示列对齐（期货分支每行 5 个合约、每格 20 显示列，容得下最长 11 字符的合约代码） |
| 字段字典采样 | INFO | 期货分支首页额外一行：`fieldId=fieldName/typeN/precN` + `fields` 独有键（用于确认位置化字段口径） |
| 整轮下架汇总 | INFO | 期货分支整轮成功后一行：`下架标记：N 条合约本轮未再出现…`；跳过时容器日志另有「跳过下架标记（本轮 x/y 个分类、失败 z 个）」 |
| 单页重试 | WARN | 含重试次数与失败原因 |
| 市场结束 | INFO / ERROR | 成功为 INFO 汇总，失败为 ERROR |

## 5. 错误目录

| `code` | HTTP | 触发场景 |
|---|---|---|
| `400` | 400 | 请求体非法；已有任务在运行 |
| `401` | 401 | 未登录或 token 失效 |

## 6. 前端

- 菜单：行情 → 行情采集 → 行情快照同步（`/Meta/Finv/Quote/Collect/SecuQuoteSnapshotSync`；需求 11255 路由改名，需求 11281 随菜单迁入「行情采集」分组，旧路径保留重定向；后端接口路径不变）
- 视图：`Web/src/views/Meta/Finv/Quote/Collect/SecuQuoteSnapshotSyncView.vue`
- 交互：展示配置表 → 选择市场/覆盖计划数量/节奏参数 → 开始同步 → 每 1.5s 轮询并追加日志（自动滚到底部）
