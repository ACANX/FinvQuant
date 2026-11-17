# 状态总览（Overview）

> 菜单 key：`quote-collect-overview`
> 路由：`/Meta/Finv/Quote/RealtimeCollect/Overview`
> 所属：导航「行情 → 行情采集 → 状态总览」（本分组第一个菜单）
> 视图：`Web/src/views/Meta/Finv/Quote/RealtimeCollect/OverviewView.vue`
> 关联需求：11283（本页）、11273/11274（期货两类配置与运行结果）、11279（ETF 配置行）
> 关联接口：[`API/Cdeu/StockSnapshotSyncRun.md`](../../../API/Cdeu/StockSnapshotSyncRun.md)（`GET .../Run`，与「行情快照同步」页同一端点）、
> [`API/Cdeu/CollectStats.md`](../../../API/Cdeu/CollectStats.md)（`GET .../Secu/Collect/Stats`，表规模指标）

## 1. 菜单入口

| 项 | 值 |
|---|---|
| 菜单位置 | 行情 → **行情采集 → 状态总览**（该分组第一位，在「行情快照同步」之前） |
| 菜单 key | `quote-collect-overview` |
| 图标 | `mdi-view-dashboard-outline` |
| 路由 | `/Meta/Finv/Quote/RealtimeCollect/Overview` |
| 视图组件 | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/OverviewView.vue` |

## 2. 业务功能概述

一个**只读**汇总页，把「行情采集」分组下各链路的配置与最近一次运行结果放在同一屏，省得逐 Tab 点进去核对：

1. **最近一次运行摘要**：状态 chip（运行中 / 成功 / 部分成功 / 失败 / 暂无运行记录）、运行编号、开始/结束时间、
   耗时、收到条数与新增/覆盖合计、失败市场或分类数。
2. **表规模指标**（需求 11285）：`cdeu.finv_quote_secu_indicator` 与 `futu.finv_futu_quote_collect`
   的**当前行数**（千分位展示），来自 `GET .../Secu/Collect/Stats`；两张表都是一行一证券，
   行数应基本相当，差值可用来判断采集覆盖是否有缺口。
3. **证券列表采集配置**：`cdeu.finv_cdeu_secu_realtime_sync_config` 的启用行（品种 = 股票 / ETF，按市场分行），
   列：品种 / 市场 / 计划数量 / 市场证券总数 / 本轮可采集。
4. **期货分类采集配置**：`cdeu.finv_quote_cdeu_future_sync_config` 的 8 行（`futuresType` 1~8），
   列：分类 / 分类名称 / 市场 / 计划数量 / 分类合约总数 / 本轮可采集。
5. **最近一次运行结果**：逐市场（股票/ETF）与逐分类（期货）的页数、收到、新增、覆盖、接口总数、本轮可采、成功/错误；
   底部附运行级 `error`。
6. 页面最底部**说明卡片**（`mdi-information-outline` + 「说明」），按 `ExplainCardSpec` 分三节：
   `数据来源` / `读法` / `注意事项`。

本页**不触发任何采集**，也没有写操作。

## 3. 使用方法

1. 从左侧菜单进入「行情 → 行情采集 → 状态总览」，页面自动拉一次数据；
2. 看顶部摘要判断最近一轮是否成功、耗时多久；
3. 对照上面两张配置表的「本轮可采集」与下表运行的「本轮可采」，确认配置是否被正确执行
   （例如某品种 `count_plan` 设了 60，运行结果里也应停在 60 条）；
4. 需要刷新时点右上角「刷新」（页面不会自动轮询——它不是监控页，只是快照）；
5. 需要真正采集时，到「[行情快照同步](SecuQuoteSnapshotSync.md)」页选品种后点「开始同步」。

## 4. 处理逻辑

### 4.1 接口与参数

```text
GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/FTStockQuoteSnapshotSync/Run   # 配置 + 最近一次运行
GET /API/V1/Meta/Finv/Quote/Secu/Collect/Stats                          # 表规模（需求 11285）
```

- 不带 `runId`（取最近一次运行），无需任何查询参数；
- 与「行情快照同步」页共用端点，因此**不需要新的后端接口**；
- 响应同时带回 `configs`（证券列表配置）、`futures_configs`（期货分类配置）与 `run`（最近一次运行）。

### 4.2 响应两种形态（前端已归一）

| 场景 | 响应形态 |
|---|---|
| 有运行记录 | `data = { run: {...}, configs: [...], futures_configs: [...], error, futures_error }` |
| 无运行记录（服务刚重启） | `data` 为**扁平结构**：`{ run_id: "", status: "IDLE", results: [], configs: [...], futures_configs: [...], error, futures_error }` |

前端把两种形态统一成内部 `RunInfo`：`run_id` 为空即「暂无运行记录」，配置表照常渲染。

### 4.3 字段口径

| 页面列 | 来源 | 说明 |
|---|---|---|
| 计划数量 | 配置表 `count_plan` | 空显示「全量」 |
| 市场证券总数 / 分类合约总数 | 配置表 `count_exists` | 接口 `pagination.total` 的**回写值**，采集成功后刷新 |
| 本轮可采集 | 配置表 `total_available` | `min(count_plan, 总数)`，同样采集成功后刷新 |
| 接口总数 / 本轮可采（下表） | 运行结果 `results[]` | **最近一次运行的现场值**，用于和上表配置对照 |
| 表规模指标 | `GET .../Secu/Collect/Stats` | 两张表的实时 `count(*)`；表不存在显示 `—`，统计失败只在卡片区提示 |

失败市场/分类的 `error` 直接显示在「结果」列，不折叠、不截断（该字段本身很短）。

## 5. 注意事项

- **运行记录只在内存**：服务重启后本页只剩配置，运行区显示「暂无运行记录」，这是正常现象，不是数据丢失。
- **配置有值 ≠ 采集过**：`count_exists` / `total_available` 只在采集成功后回写，新插入的配置行两列为空（显示 `—`）。
- **不要用它当监控页**：页面不会自动轮询；需要看实时逐页日志请到「行情快照同步」页（那里每 1.5s 轮询并滚动输出日志）。
- **同一时间只允许一个采集任务**：在「行情快照同步」页重复点「开始同步」会被拒绝并提示当前 `run_id`；
  本页只能看到那个 `run_id`，无法取消任务。
- 期货分类的中文名来自配置表 `futures_type_name`（迁移 `V300217` 已回填 8 类）；若某环境仍为空，页面降级显示「分类 N」。
- **表规模指标不做缓存**：每次进页面或点「刷新」都实时 `count(*)`；7 万行是毫秒级，但不要把它挂到高频轮询上。
