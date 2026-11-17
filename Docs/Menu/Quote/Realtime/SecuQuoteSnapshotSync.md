# 行情快照同步（SecuQuoteSnapshotSync）

> 菜单 key：`quote-realtime-snapshot-sync`
> 路由：`/Meta/Finv/Quote/Realtime/SecuQuoteSnapshotSync`（需求 11255 由 `FTStockQuoteSnapshotSync` 改名，后端接口路径不变）
> 所属：导航「行情 → 实时行情 → 行情快照同步」（需求 11158 由「行情快照手动同步」改名）
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/SecuQuoteSnapshotSyncView.vue`
> 关联需求：10982（本页）、10976（配置表）、10977（同步服务）、11255（路由改名）、11256（占位 Tab）
> 关联接口：[`API/Cdeu/StockSnapshotSyncRun.md`](../../../API/Cdeu/StockSnapshotSyncRun.md)（异步启动 + 轮询）

## 1. 用途

手动触发「证券列表采集」并按市场分页拉取 moomoo `get-stock-list`，把结果写入
`cdeu.finv_cdeu_secu_realtime_stock`，同时查看每一步的状态与逐页日志。

## 2. 页面结构

### 2.0 证券品种 Tab（需求 10986）

页面顶部按**证券品种**分 Tab，Tab 列表 = 配置表的 `instrument_symbol` 去重（当前 `stock`）+ 占位品种（需求 11256，固定顺序补齐）：

| 品种符号 | Tab 名 | 来源 |
|---|---|---|
| `stock` | 股票 | 配置表（当前唯一有配置行的品种） |
| `etf` | ETF | 占位 |
| `index` | 指数 | 占位 |
| `future` | 期货 | 占位 |
| `crypto` | 加密货币 | 占位 |
| `bsestock` | 北交所股票 | BDFS 板块采集（需求 11262，固定来源，无需配置行） |
| `sector` | 板块 | 占位 |
| `portfolio` | 投资组合 | 占位 |
| `gold` | 黄金 | 占位 |
| `forex` | 外汇 | 配置表（插行后出现） |
| `option` | 期权 | 配置表（插行后出现） |

**占位 Tab（需求 11256）**：`etf` / `index` / `future` / `crypto` / `sector` / `portfolio` / `gold`
七个品种按上表顺序**跟在配置表已有品种之后常显**，由前端 `PLACEHOLDER_SYMBOLS` 硬编码（不依赖配置行）：
配置表暂无该品种的行时，Tab 照常可点，配置表显示「该品种为占位 Tab：配置表暂无该品种的采集行，暂不参与采集」，
「开始同步」按钮禁用；后续接入时只要往 `cdeu.finv_cdeu_secu_realtime_sync_config` 插入同符号的配置行，
即自动转为可采集（无需改前端）。

**固定来源 Tab（需求 11262）**：`bsestock`（北交所股票）已接入 BDFS 采集，由前端 `FIXED_SYMBOLS` 常显，
不读配置表、也不需要配置行。Tab 顺序统一由前端 `SYMBOL_ORDER` 决定
（`stock` / `etf` / `index` / `future` / `crypto` / **`bsestock`** / `sector` / `portfolio` / `gold`），
接入采集**不改变位置**（仍在「指数」之后）——「开始同步」直接可用，配置表区显示
「该品种为 BDFS 板块采集（板块 GN2309 北交所上市企业）…结果写入 `cdeu.finv_quote_secu_realtime`（provider=BDFS）」。

#### 2.0.1 BDFS 板块采集（`bsestock`，需求 11262）

| 项 | 内容 |
|---|---|
| 数据源 | 百度股市通 FinScope `GET /sapi/v1/constituents`（经 MetaExt `component/quote/baidu/finscope` 的 `GetAllConstituents`） |
| 板块 | `block` / `GN2309`（北交所上市企业），`market=ab` |
| 翻页 | 按**偏移量** `pn += rn` 递增（`rn` = 页面「每页条数」，默认 500），返回条数 < `rn` 即末页；另有 200 页上限防死循环 |
| 落库表 | `cdeu.finv_quote_secu_realtime`（`V300209`，主键 `usc`，`provider` 固定 `BDFS`） |
| usc 解析 | 按 `(exchange, code)` 关联 `public.finv_exchange.exchange_flag` + `public.finv_security.security_code`；解析不到的行跳过并在日志记 WARN 计数 |
| 写入语义 | 按 `usc` 整行覆盖（`ON CONFLICT (usc) DO UPDATE`），`stock_id` 恒为 NULL（BDFS 不提供源系统 ID） |
| 节流 | 无页间间隔（单次全量可在数百毫秒内完成）；「页间隔 / 重试 / 退避」参数对该 Tab 不生效 |

**字段映射（BDFS 成分股 → `cdeu.finv_quote_secu_realtime`）**：

| 目标列 | 来源 | 口径 |
|---|---|---|
| `usc` | `(exchange, code)` → `public.finv_security.usc` | 主键 |
| `stock_code` | `code` | 原样 |
| `name` | `name` | 原样 |
| `market_type` | `exchange` | `BJ` / `SH` / `SZ` |
| `market_label` | `exchange` | 北交所 / 上交所 / 深交所 |
| `instrument_type` | 固定 `stock` | 该板块仅返回股票 |
| `currency` | 固定 `CNY` | A 股 |
| `price_nominal` | `rawData.lastPx` | 元 |
| `change_price` | `rawData.pxChange` | 元，可负 |
| `change_ratio` | `rawData.pxChangeRate / 100` | 表内口径为**小数比例**（`+0.83%` → `0.0083`） |
| `trade_volumn` | `rawData.volume × 100` | 接口单位为**手**，表内按**股** |
| `trade_trunover` | `rawData.amount` | 元 |
| `market_value` | `rawData.marketValue` | 元 |
| `price_direct` | `pxChange` 符号 | `+` / `-` / `0` |
| `page` | 本次采集页码 | 从 1 起（`行序号 / 每页条数 + 1`） |
| `dt_update` | 采集时刻 | Unix 毫秒 |
| `provider` | 固定 `BDFS` | — |
| `remark` | 固定文本 | `来源 BDFS 板块 GN2309（BDFS）` |
| 其余列 | 无来源 | 写 NULL（`hasQuote=0` 时行情列同样写 NULL，不写 0） |

扩展方式：往 `cdeu.finv_cdeu_secu_realtime_sync_config` 插一行对应 `instrument_symbol` 的配置，
**无需改前端**——Tab 自动出现，该 Tab 的市场下拉也由该品种的配置行生成。

切换 Tab 时：清空「市场 / 计划数量覆盖」筛选值，其余节奏参数保留；结果表、汇总与日志按当前品种过滤
（日志按 `market` 字段前缀 `品种/市场` 匹配，任务级日志始终显示）。

### 2.1 采集配置（只读表格）

展示**当前 Tab 品种**的配置，数据来自 `cdeu.finv_cdeu_secu_realtime_sync_config`（`flag_enable='1'`）：

| 列 | 说明 |
|---|---|
| 品种 | `instrument_symbol`（当前 `stock`） |
| 市场 | `market_type` 映射为 `HK 港股` / `US 美股` / `CN A 股` |
| 计划数量 | `count_plan`，空显示「全量」 |
| 市场证券总数 | `count_exists`（采集成功后回写，空显示 `—`） |
| 本轮可采集 | `total_available`（采集成功后回写，空显示 `—`） |

### 2.2 本次采集参数（可编辑）

| 控件 | 字段 | 说明 |
|---|---|---|
| 市场 | `market_type` | 「全部市场」+ 当前品种配置中出现过的市场（由配置行生成） |
| 计划数量覆盖 | `count_plan` | 留空用配置值 |
| 每页条数 | `page_size` | 默认 50 |
| 页间隔(ms) | `interval_ms` | 默认 6000，避免被服务端限流 |
| 重试次数 | `max_retries` | 默认 2 |
| 退避(ms) | `retry_backoff_ms` | 默认 2000 |
| 页数上限 | `max_pages` | 0=不限 |
| 开始同步 / 刷新状态 / 重置参数 | — | 「开始同步」只采**当前 Tab 品种**（请求体带 `instrument_symbol`）；任务运行中或该品种无配置行（含占位 Tab）时按钮置灰 |

### 2.3 汇总与结果

- 状态徽章：运行中 / 成功 / 部分成功 / 失败 + `run_id`（全局最近一次运行）
- 汇总行：耗时、收到、新增、覆盖、错误摘要（仅当前品种）
- 结果表格：当前品种各市场的页数 / 收到 / 新增 / 覆盖 / 市场总数 / 本轮可采 / 结果

### 2.4 采集执行日志

等宽字体滚动区（最大高度 320px，自动滚到底部），每行格式：

```
[HH:mm:ss] LEVEL (品种/市场) 消息
```

日志级别：`INFO` 常规进度、`WARN` 重试、`ERROR` 失败。逐页输出示例：

```
[18:03:53] INFO  (stock/1) 开始采集：市场总数 3337，本轮计划 60，按每页 20 共 3 页
[18:17:47] INFO  (stock/1) 第 1/1 页：50 条（累计 新增 0 / 覆盖 50）
[U] 02328   [U] 00267   [U] 02259   [U] 06160   [U] 01276   [U] 06030   [U] 00016   [U] 09618   [U] 80016   [U] 01919
[U] 03993   [U] 00992   [U] 02601   [U] 06809   [U] 00001   [U] 00669   [U] 02378   [U] 89618   [U] 03986   [U] 02513
[U] ...（每页最多 5 行 × 每行 10 个）
[18:17:47] INFO  (stock/1) 采集结束：1 页、收到 50 条、新增 0、覆盖 50
```

逐页明细规则：

| 项 | 说明 |
|---|---|
| 排版 | 每个证券一格，**每行 10 个**，每页 50 条时正好 5 行 |
| 标记 | `[I]` = 新增（该 `stock_id` 此前不在表中）、`[U]` = 更新 |
| 列宽 | 每格固定 11 显示列，不足补空格，多行严格对齐 |
| 内容 | `[标记] 证券代码`（只展示代码，不含名称；代码为空时改用 `stock_id`） |
| 输出位置 | 前端日志面板（`white-space: pre-wrap`）+ 容器控制台 `docker logs` |

## 3. 交互流程

1. 进入页面：拉取最近一次运行状态与配置（`GET .../FTStockQuoteSnapshotSync/Run`）。
2. 调整参数后点「开始同步」：`POST .../FTStockQuoteSnapshotSync/Run`，拿到 `run_id`。
3. 前端每 1.5 秒轮询同一 GET 接口，任务为 `RUNNING` 时持续刷新日志；结束即停止轮询。
4. 「刷新状态」可手动拉一次；「重置参数」恢复默认值。

## 4. 约束

- 后端同一时间只允许**一个**运行中的采集任务，重复启动返回 400 并提示当前 `run_id`。
- 运行状态与日志保存在内存：**进程重启后清空**，历史运行不落库。
- 日志上限：每次运行 500 行，内存保留最近 10 次运行。

## 日志口径（需求 10989）

- 每轮首条日志 `开始采集` 之后会打印一行 `首页采样`，形如
  `首页采样 cfg=stock/1(计划 2500) 样本=00700|TENCENT|4.07T 00005|HSBC|2.51T 01299|AIA|795.2B`。
- 该行直接反映服务端返回的**语言与单位口径**：中文为 `腾讯控股|4.07万亿`，英文为 `TENCENT|4.07T`。
- 采集请求固定携带 `accept-language: zh-CN,zh;q=0.9,en;q=0.8`；数值解析同时支持 `万亿/亿/万` 与 `T/B/M/K`。
- 若本轮 `total` 与配置表记录的上次值相差 2 倍以上，日志会打印 `警告 ... 可能被按 IP/语言返回了不同口径`，并在页面日志里以 WARN 展示——同一接口在不同出口 IP/语言下返回的**集合**本身就可能不同（不只是单位）。
