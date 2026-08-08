# Web 前端业务菜单总览（Menus）

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Menus.md`
> 适用范围：`Web/` 前端侧边导航（`Web/src/App.vue` 中 `menuItems` 定义）中的全部业务菜单索引。
> 菜单文档规范见 [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md)；接口文档见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)。

## 菜单层级结构

```
仪表盘（/dashboard）
历史行情（分组）
  └─ 历史行情查询（/Meta/Finv/Quote/History/HistoryQuoteQuery）
元数据管理（分组）
  └─ 业务元数据维护（二级分组）
      ├─ 交易所信息维护（/meta/exchange）
      ├─ 交易所下设市场信息维护（/meta/market）
      ├─ 规范证券信息维护（/meta/security）
      └─ 历史行情数据导入（/meta/import）
配置管理（/Meta/Finv/Quant/Config）
环境管理（/Meta/Finv/Quant/Environment）
模板管理（/Meta/Finv/Quant/Template）
账户管理（/Meta/Finv/Quant/Account）
资金管理（/Meta/Finv/Quant/Fund）
持仓管理（/Meta/Finv/Quant/Position）
量化策略验证（分组）
  └─ 黄金期货合约回测验证（/Meta/Finv/Quant/Backtest/GoldFutures）
策略管理（/Meta/Finv/Quant/Strategy）
回测分析（/Meta/Finv/Quant/Backtest/Analysis）
仿真数据验证（/Meta/Finv/Quant/Simulation/Data）
模拟盘验证（/Meta/Finv/Quant/Simulation/Paper）
实盘仿真验证（/Meta/Finv/Quant/Simulation/LiveSim）
实盘交易（/Meta/Finv/Quant/LiveTrading）
```

> 每个叶子菜单对应独立 URL 路由路径（见 `Web/src/router.ts`）；侧边导航支持两级分组展开。
> **路径规范**：量化交易模块菜单/路由统一加 `Meta/Finv/Quant/` 前缀，**路径各段使用大驼峰（PascalCase）**（如 `/Meta/Finv/Quant/Backtest/GoldFutures`），
> 与后端 `/API/V1/Meta/Finv/Quant/Backtest/**` 接口路径保持同一前缀体系。

## 菜单清单

| # | 菜单名称 | 层级 | key | 图标 | 路由 | 对应视图组件 | 使用的后端接口 | 菜单文档 |
|---|----------|------|-----|------|------|--------------|----------------|----------|
| 1 | 仪表盘 | 一级叶子 | `dashboard` | `mdi-view-dashboard` | `/dashboard` | `Web/src/views/DashboardView.vue` | `GET /API/V1/Health/Live`（存活探针） | — |
| 2 | 历史行情 | 一级分组 | `history-quote` | `mdi-chart-box` | — | — | — | — |
| 3 | 历史行情查询 | 二级叶子 | `quote-query` | `mdi-chart-line` | `/Meta/Finv/Quote/History/HistoryQuoteQuery` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` | `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [HistoryQuote/HistoryQuoteQuery.md](HistoryQuote/HistoryQuoteQuery.md) |
| 4 | 元数据管理 | 一级分组 | `metadata` | `mdi-database-cog` | — | — | — | — |
| 5 | 业务元数据维护 | 二级分组 | `meta-maintenance` | `mdi-database-search` | — | — | — | — |
| 6 | 交易所信息维护 | 三级叶子 | `meta-exchange` | `mdi-office-building` | `/meta/exchange` | `Web/src/views/Meta/Finv/MetaData/MetaExchangeView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Toggle` | [Meta/MetaExchange.md](Meta/MetaExchange.md) |
| 7 | 交易所下设市场信息维护 | 三级叶子 | `meta-market` | `mdi-chart-areaspline` | `/meta/market` | `Web/src/views/Meta/Finv/MetaData/MetaMarketView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Toggle` | [Meta/MetaMarket.md](Meta/MetaMarket.md) |
| 8 | 规范证券信息维护 | 三级叶子 | `meta-security` | `mdi-tag-multiple` | `/meta/security` | `Web/src/views/Meta/Finv/MetaData/MetaSecurityView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Security/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Toggle`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) |
| 9 | 历史行情数据导入 | 三级叶子 | `meta-import` | `mdi-database-import` | `/meta/import` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue` | `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [HistoryQuote/HistoryQuoteImport.md](HistoryQuote/HistoryQuoteImport.md) |
| 10 | 配置管理 | 一级叶子 | `config-manage` | `mdi-cog-outline` | `/Meta/Finv/Quant/Config` | `Web/src/views/Meta/Finv/Quant/Backtest/ConfigManageView.vue` | —（规划中） | — |
| 11 | 环境管理 | 一级叶子 | `environment` | `mdi-application-cog-outline` | `/Meta/Finv/Quant/Environment` | `Web/src/views/Meta/Finv/Quant/Backtest/EnvironmentView.vue` | `GET/POST .../Backtest/Environment/*` | [Backtest/EnvironmentManage.md](Backtest/EnvironmentManage.md) |
| 12 | 模板管理 | 一级叶子 | `template` | `mdi-content-copy` | `/Meta/Finv/Quant/Template` | `Web/src/views/Meta/Finv/Quant/Backtest/TemplateView.vue` | `GET/POST .../Backtest/Template/*` | [Backtest/TemplateManage.md](Backtest/TemplateManage.md) |
| 13 | 账户管理 | 一级叶子 | `account` | `mdi-account-cog-outline` | `/Meta/Finv/Quant/Account` | `Web/src/views/Meta/Finv/Quant/Account/AccountManageView.vue` | `GET/POST .../Backtest/Account/*` | [Backtest/AccountManage.md](Backtest/AccountManage.md) |
| 14 | 资金管理 | 一级叶子 | `fund` | `mdi-cash-multiple` | `/Meta/Finv/Quant/Fund` | `Web/src/views/Meta/Finv/Quant/Fund/FundManageView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity` | [Backtest/FundManage.md](Backtest/FundManage.md) |
| 15 | 持仓管理 | 一级叶子 | `position` | `mdi-briefcase-variant-outline` | `/Meta/Finv/Quant/Position` | `Web/src/views/Meta/Finv/Quant/Position/PositionManageView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades` | [Backtest/PositionManage.md](Backtest/PositionManage.md) |
| 16 | 量化策略验证 | 一级分组 | `quant-backtest` | `mdi-flask-outline` | — | — | — | — |
| 17 | 黄金期货合约回测验证 | 二级叶子 | `backtest-gold-futures` | `mdi-chart-bell-curve` | `/Meta/Finv/Quant/Backtest/GoldFutures` | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue` | `GET .../Backtest/Strategy/List`、`GET .../Backtest/Account/List`、`GET .../Backtest/Environment/List`、`POST .../Backtest/Run/Create`、`GET .../Backtest/Run/List` | [Backtest/BacktestGoldFutures.md](Backtest/BacktestGoldFutures.md) |
| 18 | 策略管理 | 一级叶子 | `strategy` | `mdi-sitemap-outline` | `/Meta/Finv/Quant/Strategy` | `Web/src/views/Meta/Finv/Quant/Strategy/StrategyManageView.vue` | `GET/POST .../Backtest/Strategy/*` | [Backtest/StrategyManage.md](Backtest/StrategyManage.md) |
| 19 | 回测分析 | 一级叶子 | `backtest-analysis` | `mdi-chart-timeline-variant` | `/Meta/Finv/Quant/Backtest/Analysis` | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Report`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades`、`GET .../Backtest/Run/Cashflows`、`GET .../Backtest/Run/PositionLogs`、`GET .../Backtest/Run/EventTraces` | [Backtest/BacktestAnalysis.md](Backtest/BacktestAnalysis.md) |
| 20 | 仿真数据验证 | 一级叶子 | `simulation-data` | `mdi-database-sync-outline` | `/Meta/Finv/Quant/Simulation/Data` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationData.md](Backtest/SimulationData.md) |
| 21 | 模拟盘验证 | 一级叶子 | `simulation-paper` | `mdi-account-cash-outline` | `/Meta/Finv/Quant/Simulation/Paper` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationPaper.md](Backtest/SimulationPaper.md) |
| 22 | 实盘仿真验证 | 一级叶子 | `simulation-live-sim` | `mdi-robot-outline` | `/Meta/Finv/Quant/Simulation/LiveSim` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationLiveSim.md](Backtest/SimulationLiveSim.md) |
| 23 | 实盘交易 | 一级叶子 | `live-trading` | `mdi-cash-register` | `/Meta/Finv/Quant/LiveTrading` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/LiveTrading.md](Backtest/LiveTrading.md) |

## 说明

- 菜单定义与视图组件一一对应，全部在 `Web/src/App.vue` 的 `menuItems` 中登记；新增菜单时须同步更新本索引。
- 图标统一使用 Material Design Icons（`@mdi/font`）；**注意：`mdi-chart-candlestick` 在 @mdi/font 7.4.47 中不存在**，K 线类菜单使用 `mdi-chart-line` / `mdi-chart-box`。
- 三个字典维护菜单（交易所/市场/证券）的 List 分页：**启用的（flag_enable='1'）优先展示，禁用的排后面**，同状态按 code 升序（见 `internal/meta/service.go`）。
- 「菜单文档」列指向对应业务菜单介绍文档（`Docs/Menu/xxx/XXXX.md`，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）；暂无文档的菜单以 `—` 标注，待补充。
- 「使用的后端接口」列与对应 API 接口文档的「已使用位置登记」互为索引（见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 7 节）。
- 视图组件名称与菜单 key 的对应关系（视图文件统一存放于 `Web/src/views/` 并按菜单层级分子目录，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）：

| 菜单 key | 视图组件（相对 `Web/src/views/`） |
|----------|----------------------------------|
| `dashboard` | `DashboardView.vue` |
| `quote-query` | `Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` |
| `meta-exchange` | `Meta/Finv/MetaData/MetaExchangeView.vue` |
| `meta-market` | `Meta/Finv/MetaData/MetaMarketView.vue` |
| `meta-security` | `Meta/Finv/MetaData/MetaSecurityView.vue` |
| `meta-import` | `Meta/Finv/Quote/History/HistoryQuoteImportView.vue` |
| `config-manage` | `Meta/Finv/Quant/Backtest/ConfigManageView.vue` |
| `environment` | `Meta/Finv/Quant/Backtest/EnvironmentView.vue` |
| `template` | `Meta/Finv/Quant/Backtest/TemplateView.vue` |
| `backtest-gold-futures` | `Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue` |
| `account` | `Meta/Finv/Quant/Account/AccountManageView.vue` |
| `fund` | `Meta/Finv/Quant/Fund/FundManageView.vue` |
| `position` | `Meta/Finv/Quant/Position/PositionManageView.vue` |
| `strategy` | `Meta/Finv/Quant/Strategy/StrategyManageView.vue` |
| `backtest-analysis` | `Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue` |
| `backtest-report`（报告页） | `Meta/Finv/Quant/Backtest/BacktestReportView.vue` |
| `run-delete-tasks`（删除管理） | `Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue` |
| `simulation-data` / `simulation-paper` / `simulation-live-sim` / `live-trading` | `Common/PlaceholderView.vue`（占位，四个菜单共用） |

## 相关文档

- [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md) — 前端菜单开发规范
- [Docs/API/README.md](../API/README.md) — 服务端 API 文档入口
- [Docs/API/APIs.md](../API/APIs.md) — 服务端 API 总览（接口清单索引）
