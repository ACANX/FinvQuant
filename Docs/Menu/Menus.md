# Web 前端业务菜单总览（Menus）

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Menus.md`
> 适用范围：`Web/` 前端导航（`Web/src/config/menuConfig.ts` 中 `menuSystems` 定义）中的全部业务菜单索引。
> 菜单文档规范见 [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md)；接口文档见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)。
> 导航改造方案见 [Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md](../Refactor/2610-012-WebMenuFourLevelRefactor.md)。

## 菜单层级结构

导航为**四级菜单树**：**一级 = 系统**（顶栏切换器），其二/三/四级为左侧抽屉菜单。

```
【一级系统】金融投资（内部 key=finvquant，path=/Meta/Finv → /Dashboard）
├─ 仪表盘（二级叶子，/Dashboard）
├─ 数据采集（二级分组）
│  ├─ 行情数据去重（三级分组）
│  │  ├─ 去重分钟行情查询（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query）
│  │  └─ 去重分钟行情统计（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats）
│  └─ 行情数据归档（三级分组）
│     ├─ 归档监控总览（/Meta/Finv/Quote/DeDup/Export/Overview）
│     ├─ 端到端链路（/Meta/Finv/Quote/DeDup/Export/Chain）
│     ├─ 工作流控制与监控（/Meta/Finv/Quote/DeDup/CDEU）
│     ├─ 导出任务（/Meta/Finv/Quote/DeDup/Export/Job）
│     ├─ 文件与上传（/Meta/Finv/Quote/DeDup/Export/File）
│     ├─ 归档统计与预算（/Meta/Finv/Quote/DeDup/Export/Stats）
│     ├─ 归档主数据与规则（/Meta/Finv/Quote/DeDup/Export/Config）
│     └─ 归档日志与补丁（/Meta/Finv/Quote/DeDup/Export/Log）
├─ 历史行情（二级分组）
│  └─ 历史行情查询（三级叶子，/Meta/Finv/Quote/History/HistoryQuoteQuery）
├─ 元数据管理（二级分组）
│  └─ 业务元数据维护（三级分组）
│     ├─ 交易所信息维护（/meta/exchange）
│     ├─ 交易所下设市场信息维护（/meta/market）
│     ├─ 规范证券信息维护（/meta/security）
│     └─ 历史行情数据导入（/meta/import）
├─ 配置管理（二级叶子，/Meta/Finv/Quant/Config）
├─ 环境管理（二级叶子，/Meta/Finv/Quant/Environment）
├─ 模板管理（二级叶子，/Meta/Finv/Quant/Template）
├─ 账户管理（二级叶子，/Meta/Finv/Quant/Account）
├─ 资金管理（二级叶子，/Meta/Finv/Quant/Fund）
├─ 持仓管理（二级叶子，/Meta/Finv/Quant/Position）
├─ 量化策略验证（二级分组）
│  └─ 黄金期货合约回测验证（三级叶子，/Meta/Finv/Quant/Backtest/GoldFutures）
├─ 策略管理（二级叶子，/Meta/Finv/Quant/Strategy）
├─ 回测分析（二级叶子，/Meta/Finv/Quant/Backtest/Analysis）
├─ 仿真数据验证（二级叶子，/Meta/Finv/Quant/Simulation/Data）
├─ 模拟盘验证（二级叶子，/Meta/Finv/Quant/Simulation/Paper）
├─ 实盘仿真验证（二级叶子，/Meta/Finv/Quant/Simulation/LiveSim）
└─ 实盘交易（二级叶子，/Meta/Finv/Quant/LiveTrading）

【一级系统】内容管理（内部 key=cms，path=/Meta/CMS）
└─ 内容管理控制台（二级叶子，/Meta/CMS，占位页 PlaceholderView）
```

> 「金融投资」即 WebCMS 一级菜单中的金融投资分系统（路径前缀 `/Meta/Finv`）；「内容管理」对应 WebCMS 的 CMS 分系统（`/Meta/CMS`），本轮仅占位。
> **路径规范**：量化交易模块菜单/路由统一加 `Meta/Finv/Quant/` 前缀，**路径各段使用大驼峰（PascalCase）**（如 `/Meta/Finv/Quant/Backtest/GoldFutures`），
> 与后端 `/API/V1/Meta/Finv/Quant/Backtest/**` 接口路径保持同一前缀体系。

## 菜单清单

| # | 菜单名称 | 所属系统 | 层级 | key | 图标 | 路由 | 对应视图组件 | 使用的后端接口 | 菜单文档 |
|---|----------|----------|------|-----|------|------|--------------|----------------|----------|
| 1 | 金融投资 | 金融投资 | 一级系统 | `finvquant` | `mdi-finance` | `/Meta/Finv`（重定向 `/Dashboard`） | — | — | — |
| 2 | 仪表盘 | 金融投资 | 二级叶子 | `dashboard` | `mdi-view-dashboard` | `/Dashboard` | `Web/src/views/DashboardView.vue` | `GET /API/V1/Health/Live`（存活探针） | — |
| 3 | 数据采集 | 金融投资 | 二级分组 | `data-collection` | `mdi-database-arrow-down` | — | — | — | — |
| 4 | 行情数据去重 | 金融投资 | 三级分组 | `quote-dedup` | `mdi-content-duplicate` | — | — | — | — |
| 5 | 去重分钟行情查询 | 金融投资 | 四级叶子 | `dedup-kline-query` | `mdi-table-search` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query` | `Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Page`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [HistoryQuote/DeDupKLineQuery.md](HistoryQuote/DeDupKLineQuery.md) |
| 6 | 去重分钟行情统计 | 金融投资 | 四级叶子 | `dedup-kline-stats` | `mdi-chart-box-outline` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats` | `Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Stats`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [HistoryQuote/DeDupKLineStats.md](HistoryQuote/DeDupKLineStats.md) |
| 7 | 行情数据归档 | 金融投资 | 三级分组 | `quote-archive` | `mdi-database-export-outline` | — | — | — | — |
| 8 | 归档监控总览 | 金融投资 | 四级叶子 | `dedup-export-overview` | `mdi-monitor-dashboard` | `/Meta/Finv/Quote/DeDup/Export/Overview` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` | `GET .../Quote/Cdeu/Overview` | — |
| 9 | 端到端链路 | 金融投资 | 四级叶子 | `dedup-export-chain` | `mdi-transit-connection-variant` | `/Meta/Finv/Quote/DeDup/Export/Chain` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` | `GET .../Quote/Cdeu/Chain` | — |
| 10 | 工作流控制与监控 | 金融投资 | 四级叶子 | `dedup-export-cdeu` | `mdi-sitemap-outline` | `/Meta/Finv/Quote/DeDup/CDEU` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` | `GET .../Quote/Cdeu/Status`、`POST .../Quote/Cdeu/Step/Run` | — |
| 11 | 导出任务 | 金融投资 | 四级叶子 | `dedup-export-job` | `mdi-clipboard-list-outline` | `/Meta/Finv/Quote/DeDup/Export/Job` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportJobView.vue` | `GET .../Quote/Cdeu/Export/Job/List` | — |
| 12 | 文件与上传 | 金融投资 | 四级叶子 | `dedup-export-file` | `mdi-file-cloud-outline` | `/Meta/Finv/Quote/DeDup/Export/File` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue` | `GET .../Quote/Cdeu/Export/File/List` | — |
| 13 | 归档统计与预算 | 金融投资 | 四级叶子 | `dedup-export-stats` | `mdi-chart-box-multiple-outline` | `/Meta/Finv/Quote/DeDup/Export/Stats` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveStatsView.vue` | `GET .../Quote/Cdeu/Export/Stats` | — |
| 14 | 归档主数据与规则 | 金融投资 | 四级叶子 | `dedup-export-config` | `mdi-tune-variant` | `/Meta/Finv/Quote/DeDup/Export/Config` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveConfigView.vue` | `GET/POST .../Quote/Cdeu/Export/Config` | — |
| 15 | 归档日志与补丁 | 金融投资 | 四级叶子 | `dedup-export-log` | `mdi-text-box-search-outline` | `/Meta/Finv/Quote/DeDup/Export/Log` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupArchiveLogView.vue` | `GET .../Quote/Cdeu/Export/Log` | — |
| 16 | 历史行情 | 金融投资 | 二级分组 | `history-quote` | `mdi-chart-box` | — | — | — | — |
| 17 | 历史行情查询 | 金融投资 | 三级叶子 | `quote-query` | `mdi-chart-line` | `/Meta/Finv/Quote/History/HistoryQuoteQuery` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` | `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [HistoryQuote/HistoryQuoteQuery.md](HistoryQuote/HistoryQuoteQuery.md) |
| 18 | 元数据管理 | 金融投资 | 二级分组 | `metadata` | `mdi-database-cog` | — | — | — | — |
| 19 | 业务元数据维护 | 金融投资 | 三级分组 | `meta-maintenance` | `mdi-database-search` | — | — | — | — |
| 20 | 交易所信息维护 | 金融投资 | 四级叶子 | `meta-exchange` | `mdi-office-building` | `/meta/exchange` | `Web/src/views/Meta/Finv/MetaData/MetaExchangeView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Toggle` | [Meta/MetaExchange.md](Meta/MetaExchange.md) |
| 21 | 交易所下设市场信息维护 | 金融投资 | 四级叶子 | `meta-market` | `mdi-chart-areaspline` | `/meta/market` | `Web/src/views/Meta/Finv/MetaData/MetaMarketView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Toggle` | [Meta/MetaMarket.md](Meta/MetaMarket.md) |
| 22 | 规范证券信息维护 | 金融投资 | 四级叶子 | `meta-security` | `mdi-tag-multiple` | `/meta/security` | `Web/src/views/Meta/Finv/MetaData/MetaSecurityView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Security/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Toggle`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) |
| 23 | 历史行情数据导入 | 金融投资 | 四级叶子 | `meta-import` | `mdi-database-import` | `/meta/import` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue` | `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [HistoryQuote/HistoryQuoteImport.md](HistoryQuote/HistoryQuoteImport.md) |
| 24 | 配置管理 | 金融投资 | 二级叶子 | `config-manage` | `mdi-cog-outline` | `/Meta/Finv/Quant/Config` | `Web/src/views/Meta/Finv/Quant/Backtest/ConfigManageView.vue` | —（规划中） | — |
| 25 | 环境管理 | 金融投资 | 二级叶子 | `environment` | `mdi-application-cog-outline` | `/Meta/Finv/Quant/Environment` | `Web/src/views/Meta/Finv/Quant/Backtest/EnvironmentView.vue` | `GET/POST .../Backtest/Environment/*` | [Backtest/EnvironmentManage.md](Backtest/EnvironmentManage.md) |
| 26 | 模板管理 | 金融投资 | 二级叶子 | `template` | `mdi-content-copy` | `/Meta/Finv/Quant/Template` | `Web/src/views/Meta/Finv/Quant/Backtest/TemplateView.vue` | `GET/POST .../Backtest/Template/*` | [Backtest/TemplateManage.md](Backtest/TemplateManage.md) |
| 27 | 账户管理 | 金融投资 | 二级叶子 | `account` | `mdi-account-cog-outline` | `/Meta/Finv/Quant/Account` | `Web/src/views/Meta/Finv/Quant/Account/AccountManageView.vue` | `GET/POST .../Backtest/Account/*` | [Backtest/AccountManage.md](Backtest/AccountManage.md) |
| 28 | 资金管理 | 金融投资 | 二级叶子 | `fund` | `mdi-cash-multiple` | `/Meta/Finv/Quant/Fund` | `Web/src/views/Meta/Finv/Quant/Fund/FundManageView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity` | [Backtest/FundManage.md](Backtest/FundManage.md) |
| 29 | 持仓管理 | 金融投资 | 二级叶子 | `position` | `mdi-briefcase-variant-outline` | `/Meta/Finv/Quant/Position` | `Web/src/views/Meta/Finv/Quant/Position/PositionManageView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades` | [Backtest/PositionManage.md](Backtest/PositionManage.md) |
| 30 | 量化策略验证 | 金融投资 | 二级分组 | `quant-backtest` | `mdi-flask-outline` | — | — | — | — |
| 31 | 黄金期货合约回测验证 | 金融投资 | 三级叶子 | `backtest-gold-futures` | `mdi-chart-bell-curve` | `/Meta/Finv/Quant/Backtest/GoldFutures` | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue` | `GET .../Backtest/Strategy/List`、`GET .../Backtest/Account/List`、`GET .../Backtest/Environment/List`、`POST .../Backtest/Run/Create`、`GET .../Backtest/Run/List` | [Backtest/BacktestGoldFutures.md](Backtest/BacktestGoldFutures.md) |
| 32 | 策略管理 | 金融投资 | 二级叶子 | `strategy` | `mdi-sitemap-outline` | `/Meta/Finv/Quant/Strategy` | `Web/src/views/Meta/Finv/Quant/Strategy/StrategyManageView.vue` | `GET/POST .../Backtest/Strategy/*` | [Backtest/StrategyManage.md](Backtest/StrategyManage.md) |
| 33 | 回测分析 | 金融投资 | 二级叶子 | `backtest-analysis` | `mdi-chart-timeline-variant` | `/Meta/Finv/Quant/Backtest/Analysis` | `Web/src/views/Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Report`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades`、`GET .../Backtest/Run/Cashflows`、`GET .../Backtest/Run/PositionLogs`、`GET .../Backtest/Run/EventTraces` | [Backtest/BacktestAnalysis.md](Backtest/BacktestAnalysis.md) |
| 34 | 仿真数据验证 | 金融投资 | 二级叶子 | `simulation-data` | `mdi-database-sync-outline` | `/Meta/Finv/Quant/Simulation/Data` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationData.md](Backtest/SimulationData.md) |
| 35 | 模拟盘验证 | 金融投资 | 二级叶子 | `simulation-paper` | `mdi-account-cash-outline` | `/Meta/Finv/Quant/Simulation/Paper` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationPaper.md](Backtest/SimulationPaper.md) |
| 36 | 实盘仿真验证 | 金融投资 | 二级叶子 | `simulation-live-sim` | `mdi-robot-outline` | `/Meta/Finv/Quant/Simulation/LiveSim` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/SimulationLiveSim.md](Backtest/SimulationLiveSim.md) |
| 37 | 实盘交易 | 金融投资 | 二级叶子 | `live-trading` | `mdi-cash-register` | `/Meta/Finv/Quant/LiveTrading` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Backtest/LiveTrading.md](Backtest/LiveTrading.md) |
| 38 | 内容管理 | 内容管理 | 一级系统 | `cms` | `mdi-laptop` | `/Meta/CMS` | — | — | — |
| 39 | 内容管理控制台 | 内容管理 | 二级叶子 | `cms-home` | `mdi-view-dashboard-outline` | `/Meta/CMS` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中，系统占位） | [CMS/CMSHome.md](CMS/CMSHome.md) |

## 说明

- 菜单定义与视图组件一一对应，全部在 `Web/src/config/menuConfig.ts` 的 `menuSystems` 中登记；新增菜单时须同步更新本索引。
- **一级系统**由顶栏切换器选择（`Web/src/components/menu/SystemSwitcher.vue`），左侧抽屉只渲染当前系统的二~四级菜单；系统归属按 `MenuSystem.match` 前缀判定（最长前缀优先）。
- 各系统上次访问的路由记录在 `localStorage['finvquant.nav.lastPathBySystem.v1']`，切换系统时回到该系统上次页面，首次进入落到系统落地路由。
- 菜单展开状态按系统分桶持久化在 `localStorage['finvquant.nav.openKeys.v1']`；深链接直开时自动展开祖先链。
- 导航视觉风格由 `Web/src/config/uiConfig.ts` 控制：默认 `webcms`（当前菜单项左侧 3px 主色激活条），可在右上角用户菜单 →「WebCMS 视觉风格」开关即时切回改造前视觉（选择被记住）。
- 登录后每个页面底部显示页脚 `© 2026 ACANX`。
- 图标统一使用 Material Design Icons（`@mdi/font`），**二/三/四级菜单均保留图标**；**注意：`mdi-chart-candlestick` 在 @mdi/font 7.4.47 中不存在**，K 线类菜单使用 `mdi-chart-line` / `mdi-chart-box`。
- 三个字典维护菜单（交易所/市场/证券）的 List 分页：**启用的（flag_enable='1'）优先展示，禁用的排后面**，同状态按 code 升序（见 `internal/meta/service.go`）。
- 「菜单文档」列指向对应业务菜单介绍文档（`Docs/Menu/xxx/XXXX.md`，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）；暂无文档的菜单以 `—` 标注，待补充。
- 「使用的后端接口」列与对应 API 接口文档的「已使用位置登记」互为索引（见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 7 节）。
- 视图组件名称与菜单 key 的对应关系（视图文件统一存放于 `Web/src/views/` 并按业务域分子目录，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）：

| 菜单 key | 视图组件（相对 `Web/src/views/`） |
|----------|----------------------------------|
| `dashboard` | `DashboardView.vue` |
| `quote-query` | `Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` |
| `dedup-kline-query` | `Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue` |
| `dedup-kline-stats` | `Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue` |
| `dedup-export-overview` | `Meta/Finv/Quote/DeDup/Export/DeDupExportOverviewView.vue` |
| `dedup-export-chain` | `Meta/Finv/Quote/DeDup/Export/DeDupArchiveChainView.vue` |
| `dedup-export-cdeu` | `Meta/Finv/Quote/DeDup/Export/DeDupCDEUView.vue` |
| `dedup-export-job` | `Meta/Finv/Quote/DeDup/Export/DeDupExportJobView.vue` |
| `dedup-export-file` | `Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue` |
| `dedup-export-stats` | `Meta/Finv/Quote/DeDup/Export/DeDupArchiveStatsView.vue` |
| `dedup-export-config` | `Meta/Finv/Quote/DeDup/Export/DeDupArchiveConfigView.vue` |
| `dedup-export-log` | `Meta/Finv/Quote/DeDup/Export/DeDupArchiveLogView.vue` |
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
| `simulation-data` / `simulation-paper` / `simulation-live-sim` / `live-trading` / `cms-home` | `Common/PlaceholderView.vue`（占位，五个菜单共用） |

## 相关文档

- [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md) — 前端菜单开发规范
- [Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md](../Refactor/2610-012-WebMenuFourLevelRefactor.md) — 四级菜单改造方案
- [Docs/API/README.md](../API/README.md) — 服务端 API 文档入口
- [Docs/API/APIs.md](../API/APIs.md) — 服务端 API 总览（接口清单索引）
