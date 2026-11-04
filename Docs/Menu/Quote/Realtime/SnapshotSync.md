# 行情快照手动同步（SnapshotSync）

> 菜单 key：`quote-realtime-snapshot-sync`
> 路由：`/Meta/Finv/Quote/Realtime/FTStockQuoteSnapshotSync`
> 所属：导航「行情 → 实时行情 → 行情快照手动同步」
> 视图：`Web/src/views/Meta/Finv/Quote/Realtime/StockSnapshotSyncView.vue`
> 关联需求：10982（本页）、10976（配置表）、10977（同步服务）
> 关联接口：[`API/Cdeu/StockSnapshotSyncRun.md`](../../../API/Cdeu/StockSnapshotSyncRun.md)（异步启动 + 轮询）

## 1. 用途

手动触发「证券列表采集」并按市场分页拉取 moomoo `get-stock-list`，把结果写入
`cdeu.finv_cdeu_secu_realtime_stock`，同时查看每一步的状态与逐页日志。

## 2. 页面结构

### 2.0 证券品种 Tab（需求 10986）

页面顶部按**证券品种**分 Tab，Tab 列表由配置表的 `instrument_symbol` 去重生成：

| 品种符号 | Tab 名 |
|---|---|
| `stock` | 股票（当前唯一） |
| `etf` | ETF |
| `future` | 期货 |
| `forex` | 外汇 |
| `option` | 期权 |
| `crypto` | 加密货币 |

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
| 开始同步 / 刷新状态 / 重置参数 | — | 「开始同步」只采**当前 Tab 品种**（请求体带 `instrument_symbol`）；任务运行中按钮置灰 |

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
[U] 02328 PICC P&C     [U] 00267 CITIC        [U] 02259 ZIJIN GOLD I [U] 06160 BEONE MEDICI [U] 01276 HENGRUI PHAR [U] 06030 CITIC SEC    [U] 00016 SHK PPT      [U] 09618 JD-SW        [U] 80016 SHK PPT-R    [U] 01919 COSCO SHIP H
[U] 03993 CMOC         [U] 00992 LENOVO GROUP [U] 02601 CPIC         [U] 06809 MONTAGE TECH [U] 00001 CKH HOLDINGS [U] 00669 TECHTRONIC I [U] 02378 PRU          [U] 89618 JD-SWR       [U] 03986 GIGADEVICE   [U] 02513 Z.AI
[U] ...（每页最多 5 行 × 每行 10 个）
[18:17:47] INFO  (stock/1) 采集结束：1 页、收到 50 条、新增 0、覆盖 50
```

逐页明细规则：

| 项 | 说明 |
|---|---|
| 排版 | 每个证券一格，**每行 10 个**，每页 50 条时正好 5 行 |
| 标记 | `[I]` = 新增（该 `stock_id` 此前不在表中）、`[U]` = 更新 |
| 列宽 | 每格固定 22 显示列（CJK 字符按 2 列计），超出截断、不足补空格，多行严格对齐 |
| 内容 | `[标记] 证券代码 名称`（代码为空时改用 `stock_id`） |
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
