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
│  │  ├─ 去重分钟行情查询（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineQuery）
│  │  └─ 去重分钟行情统计（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineStats）
│  └─ 行情数据归档（三级分组）
│     ├─ 归档监控总览（/Meta/Finv/Quote/DeDup/Export/Overview）
│     ├─ 端到端链路（/Meta/Finv/Quote/DeDup/Export/Chain）
│     ├─ 工作流控制与监控（/Meta/Finv/Quote/Cdeu/CdeuOverview）
│     ├─ 日K线数据同步（/Meta/Finv/Quote/Cdeu/DayKlineSync，需求 10952）
│     ├─ 导出任务（/Meta/Finv/Quote/DeDup/Export/Job）
│     ├─ 文件与上传（/Meta/Finv/Quote/DeDup/Export/File）
│     ├─ 归档统计与预算（/Meta/Finv/Quote/DeDup/Export/Stats）
│     ├─ 归档主数据与规则（/Meta/Finv/Quote/DeDup/Export/Config）
│     └─ 归档日志与补丁（/Meta/Finv/Quote/DeDup/Export/Log）
├─ 历史行情（二级分组）
│  └─ 历史行情查询（三级叶子，/Meta/Finv/Quote/History/HistoryQuoteQuery）
├─ 实时行情（二级分组）
│  ├─ 证券行情排行（/Meta/Finv/Quote/Realtime/FTStockRank，需求 10962/10963/10964）
│  ├─ 期货行情排行（/Meta/Finv/Quote/Realtime/FTFutureRank，需求 11322）
│  ├─ 加密货币行情排行（/Meta/Finv/Quote/Realtime/FTCryptoRank，需求 11323）
│  ├─ 外汇汇率（/Meta/Finv/Quote/Realtime/FXCEROverview，需求 10999/11000，11159 改名并前移）
│  └─ 证券指标（/Meta/Finv/Quote/Realtime/SecuIndicator，需求 11251，11252 改名）
├─ 行情采集（二级分组，需求 11281/11282）
│  ├─ 状态总览（三级叶子，/Meta/Finv/Quote/RealtimeCollect/Overview，需求 11283，本分组第一位）
│  ├─ 行情快照同步（三级叶子，/Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSync，需求 10982，11158 改名，11281 迁入本分组）
│  ├─ 待采证券录入（三级叶子，/Meta/Finv/Quote/RealtimeCollect/PendingSecu，需求 10995，11160 改名，11282 迁入）
│  ├─ 证券参数补全（三级叶子，/Meta/Finv/Quote/RealtimeCollect/SecuParamComplete，需求 11161，11282 迁入）
│  ├─ 证券参数同步（三级叶子，/Meta/Finv/Quote/RealtimeCollect/SecuParamSync，需求 11042/11043/11044，11282 迁入）
│  ├─ USC 映射规则（三级叶子，/Meta/Finv/Quote/RealtimeCollect/UscMapping，需求 11072，11282 迁入）
│  └─ 行情采集系统配置（三级叶子，/Meta/Finv/Quote/RealtimeCollect/CollectSysConfig，需求 11343）
├─ 元数据管理（二级分组）
│  └─ 业务元数据维护（三级分组）
│     ├─ 交易所信息维护（/Meta/Finv/MetaData/Exchange）
│     ├─ 交易所下设市场信息维护（/Meta/Finv/MetaData/Market）
│     ├─ 规范证券信息维护（/Meta/Finv/MetaData/Security）
│     └─ 历史行情数据导入（/Meta/Finv/Quote/History/HistoryQuoteImport）
├─ 开放平台（二级分组，需求 11171 建立，11172 改名；本身不可点）
│  ├─ 接入管理（三级分组，需求 11173）
│  │  ├─ OpenAPI 调用方（/Meta/Finv/OpenAPI/Client，需求 11169）
│  │  └─ OpenAPI 凭证（/Meta/Finv/OpenAPI/Credential，需求 11169）
│  ├─ 运行监控（三级分组）
│  │  └─ OpenAPI 调用审计（/Meta/Finv/OpenAPI/InvokeLog，需求 11169）
│  └─ 安全设置（三级分组）
│     ├─ OpenAPI 鉴权配置（/Meta/Finv/OpenAPI/Settings，需求 11169）
│     └─ OpenAPI 密钥配置（/Meta/Finv/OpenAPI/Secret，需求 11175）
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
| 5 | 去重分钟行情查询 | 金融投资 | 四级叶子 | `dedup-kline-query` | `mdi-table-search` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineQuery` | `Web/src/views/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineQueryView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Page`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [Quote/DeDup/Secu/KLine/Min/KLineQuery.md](Quote/DeDup/Secu/KLine/Min/KLineQuery.md) |
| 6 | 去重分钟行情统计 | 金融投资 | 四级叶子 | `dedup-kline-stats` | `mdi-chart-box-outline` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineStats` | `Web/src/views/Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineStatsView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Stats`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [Quote/DeDup/Secu/KLine/Min/KLineStats.md](Quote/DeDup/Secu/KLine/Min/KLineStats.md) |
| 7 | 行情数据归档 | 金融投资 | 三级分组 | `quote-archive` | `mdi-database-export-outline` | — | — | — | — |
| 8 | 归档监控总览 | 金融投资 | 四级叶子 | `dedup-export-overview` | `mdi-monitor-dashboard` | `/Meta/Finv/Quote/DeDup/Export/Overview` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/OverviewView.vue` | `GET .../Quote/Cdeu/Overview` | [Quote/DeDup/Export/Overview.md](Quote/DeDup/Export/Overview.md) |
| 9 | 端到端链路 | 金融投资 | 四级叶子 | `dedup-export-chain` | `mdi-transit-connection-variant` | `/Meta/Finv/Quote/DeDup/Export/Chain` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/ChainView.vue` | `GET .../Quote/Cdeu/Chain` | [Quote/DeDup/Export/Chain.md](Quote/DeDup/Export/Chain.md) |
| 10 | CDEU 工作流控制面板 | 金融投资 | 四级叶子 | `dedup-export-cdeu` | `mdi-sitemap-outline` | `/Meta/Finv/Quote/Cdeu/CdeuOverview` | `Web/src/views/Meta/Finv/Quote/Cdeu/CdeuOverviewView.vue` | `GET .../Quote/Cdeu/Status`、`POST .../Quote/Cdeu/Step/Run` | [Quote/Cdeu/CdeuOverview.md](Quote/Cdeu/CdeuOverview.md) |
| 11 | 文件导出 | 金融投资 | 四级叶子 | `dedup-export-job` | `mdi-clipboard-list-outline` | `/Meta/Finv/Quote/DeDup/Export/Job` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/JobView.vue` | `GET .../Quote/Cdeu/Export/Job/List` | [Quote/DeDup/Export/Job.md](Quote/DeDup/Export/Job.md) |
| 12 | 文件上传 | 金融投资 | 四级叶子 | `dedup-export-file` | `mdi-file-cloud-outline` | `/Meta/Finv/Quote/DeDup/Export/File` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/FileView.vue` | `GET .../Quote/Cdeu/Export/File/List` | [Quote/DeDup/Export/File.md](Quote/DeDup/Export/File.md) |
| 13 | 归档统计与预算 | 金融投资 | 四级叶子 | `dedup-export-stats` | `mdi-chart-box-multiple-outline` | `/Meta/Finv/Quote/DeDup/Export/Stats` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/StatsView.vue` | `GET .../Quote/Cdeu/Export/Stats` | [Quote/DeDup/Export/Stats.md](Quote/DeDup/Export/Stats.md) |
| 14 | 归档主数据与规则 | 金融投资 | 四级叶子 | `dedup-export-config` | `mdi-tune-variant` | `/Meta/Finv/Quote/DeDup/Export/Config` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/ConfigView.vue` | `GET/POST .../Quote/Cdeu/Export/Config` | [Quote/DeDup/Export/Config.md](Quote/DeDup/Export/Config.md) |
| 15 | 归档日志与补丁 | 金融投资 | 四级叶子 | `dedup-export-log` | `mdi-text-box-search-outline` | `/Meta/Finv/Quote/DeDup/Export/Log` | `Web/src/views/Meta/Finv/Quote/DeDup/Export/LogView.vue` | `GET .../Quote/Cdeu/Export/Log` | [Quote/DeDup/Export/Log.md](Quote/DeDup/Export/Log.md) |
| 16 | 历史行情 | 金融投资 | 二级分组 | `history-quote` | `mdi-chart-box` | — | — | — | — |
| 17 | 历史行情查询 | 金融投资 | 三级叶子 | `quote-query` | `mdi-chart-line` | `/Meta/Finv/Quote/History/HistoryQuoteQuery` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` | `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options` | [Quote/History/HistoryQuoteQuery.md](Quote/History/HistoryQuoteQuery.md) |
| 18 | 元数据管理 | 金融投资 | 二级分组 | `metadata` | `mdi-database-cog` | — | — | — | — |
| 19 | 业务元数据维护 | 金融投资 | 三级分组 | `meta-maintenance` | `mdi-database-search` | — | — | — | — |
| 20 | 交易所信息维护 | 金融投资 | 四级叶子 | `meta-exchange` | `mdi-office-building` | `/Meta/Finv/MetaData/Exchange` | `Web/src/views/Meta/Finv/MetaData/ExchangeView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Exchange/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/Toggle` | [MetaData/Exchange.md](MetaData/Exchange.md) |
| 21 | 交易所下设市场信息维护 | 金融投资 | 四级叶子 | `meta-market` | `mdi-chart-areaspline` | `/Meta/Finv/MetaData/Market` | `Web/src/views/Meta/Finv/MetaData/MarketView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Market/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Market/Toggle` | [MetaData/Market.md](MetaData/Market.md) |
| 22 | 规范证券信息维护 | 金融投资 | 四级叶子 | `meta-security` | `mdi-tag-multiple` | `/Meta/Finv/MetaData/Security` | `Web/src/views/Meta/Finv/MetaData/SecurityView.vue` | `GET /API/V1/Meta/Finv/Quant/Metadata/Security/List`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Save`、`POST /API/V1/Meta/Finv/Quant/Metadata/Security/Toggle`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [MetaData/Security.md](MetaData/Security.md) |
| 23 | 历史行情数据导入 | 金融投资 | 四级叶子 | `meta-import` | `mdi-database-import` | `/Meta/Finv/Quote/History/HistoryQuoteImport` | `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteImportView.vue` | `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Options`、`GET /API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | [Quote/History/HistoryQuoteImport.md](Quote/History/HistoryQuoteImport.md) |
| 24 | 系统配置 | 金融投资 | 二级叶子 | `config-manage` | `mdi-cog-outline` | `/Meta/Finv/Quant/Config` | `Web/src/views/Meta/Finv/Quant/ConfigView.vue` | —（规划中） | [Quant/Config.md](Quant/Config.md) |
| 25 | 环境管理 | 金融投资 | 二级叶子 | `environment` | `mdi-application-cog-outline` | `/Meta/Finv/Quant/Environment` | `Web/src/views/Meta/Finv/Quant/EnvironmentView.vue` | `GET/POST .../Backtest/Environment/*` | [Quant/Environment.md](Quant/Environment.md) |
| 26 | 模板管理 | 金融投资 | 二级叶子 | `template` | `mdi-content-copy` | `/Meta/Finv/Quant/Template` | `Web/src/views/Meta/Finv/Quant/TemplateView.vue` | `GET/POST .../Backtest/Template/*` | [Quant/Template.md](Quant/Template.md) |
| 27 | 账户管理 | 金融投资 | 二级叶子 | `account` | `mdi-account-cog-outline` | `/Meta/Finv/Quant/Account` | `Web/src/views/Meta/Finv/Quant/AccountView.vue` | `GET/POST .../Backtest/Account/*` | [Quant/Account.md](Quant/Account.md) |
| 28 | 资金管理 | 金融投资 | 二级叶子 | `fund` | `mdi-cash-multiple` | `/Meta/Finv/Quant/Fund` | `Web/src/views/Meta/Finv/Quant/FundView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity` | [Quant/Fund.md](Quant/Fund.md) |
| 29 | 持仓管理 | 金融投资 | 二级叶子 | `position` | `mdi-briefcase-variant-outline` | `/Meta/Finv/Quant/Position` | `Web/src/views/Meta/Finv/Quant/PositionView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades` | [Quant/Position.md](Quant/Position.md) |
| 30 | 量化策略验证 | 金融投资 | 二级分组 | `quant-backtest` | `mdi-flask-outline` | — | — | — | — |
| 31 | 黄金期货合约回测验证 | 金融投资 | 三级叶子 | `backtest-gold-futures` | `mdi-chart-bell-curve` | `/Meta/Finv/Quant/Backtest/GoldFutures` | `Web/src/views/Meta/Finv/Quant/Backtest/GoldFuturesView.vue` | `GET .../Backtest/Strategy/List`、`GET .../Backtest/Account/List`、`GET .../Backtest/Environment/List`、`POST .../Backtest/Run/Create`、`GET .../Backtest/Run/List` | [Quant/Backtest/GoldFutures.md](Quant/Backtest/GoldFutures.md) |
| 32 | 策略管理 | 金融投资 | 二级叶子 | `strategy` | `mdi-sitemap-outline` | `/Meta/Finv/Quant/Strategy` | `Web/src/views/Meta/Finv/Quant/StrategyView.vue` | `GET/POST .../Backtest/Strategy/*` | [Quant/Strategy.md](Quant/Strategy.md) |
| 33 | 回测分析 | 金融投资 | 二级叶子 | `backtest-analysis` | `mdi-chart-timeline-variant` | `/Meta/Finv/Quant/Backtest/Analysis` | `Web/src/views/Meta/Finv/Quant/Backtest/AnalysisView.vue` | `GET .../Backtest/Run/List`、`GET .../Backtest/Run/Report`、`GET .../Backtest/Run/Equity`、`GET .../Backtest/Run/Trades`、`GET .../Backtest/Run/Cashflows`、`GET .../Backtest/Run/PositionLogs`、`GET .../Backtest/Run/EventTraces` | [Quant/Backtest/Analysis.md](Quant/Backtest/Analysis.md) |
| 34 | 仿真数据验证 | 金融投资 | 二级叶子 | `simulation-data` | `mdi-database-sync-outline` | `/Meta/Finv/Quant/Simulation/Data` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Quant/Simulation/Data.md](Quant/Simulation/Data.md) |
| 35 | 模拟盘验证 | 金融投资 | 二级叶子 | `simulation-paper` | `mdi-account-cash-outline` | `/Meta/Finv/Quant/Simulation/Paper` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Quant/Simulation/Paper.md](Quant/Simulation/Paper.md) |
| 36 | 实盘仿真验证 | 金融投资 | 二级叶子 | `simulation-live-sim` | `mdi-robot-outline` | `/Meta/Finv/Quant/Simulation/LiveSim` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Quant/Simulation/LiveSim.md](Quant/Simulation/LiveSim.md) |
| 37 | 实盘交易 | 金融投资 | 二级叶子 | `live-trading` | `mdi-cash-register` | `/Meta/Finv/Quant/LiveTrading` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Quant/LiveTrading.md](Quant/LiveTrading.md) |
| 38 | 内容管理 | 内容管理 | 一级系统 | `cms` | `mdi-laptop` | `/Meta/CMS` | — | — | — |
| 39 | 内容管理控制台 | 内容管理 | 二级叶子 | `cms-home` | `mdi-view-dashboard-outline` | `/Meta/CMS` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中，系统占位） | [CMS/CMS.md](CMS/CMS.md) |
| 40 | 日K线数据同步 | 金融投资 | 四级叶子 | `cdeu-daykline-sync` | `mdi-candle` | `/Meta/Finv/Quote/Cdeu/DayKlineSync` | `Web/src/views/Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` | `POST /API/V1/Meta/Finv/Quote/Cdeu/DayKline/Sync`、`GET .../DayKline/Stat`、`GET .../DayKline/UscList` | [Quote/Cdeu/DayKlineSync.md](Quote/Cdeu/DayKlineSync.md) |
| 41 | 证券行情排行 | 金融投资 | 四级叶子 | `quote-realtime-ftstockrank` | `mdi-chart-timeline-variant` | `/Meta/Finv/Quote/Realtime/FTStockRank` | `Web/src/views/Meta/Finv/Quote/Realtime/FTStockRankView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Stock/Page` | [Quote/Realtime/FTStockRank.md](Quote/Realtime/FTStockRank.md) |
| 42 | 期货行情排行 | 金融投资 | 四级叶子 | `quote-realtime-ftfuture-rank` | `mdi-finance` | /Meta/Finv/Quote/Realtime/FTFutureRank | `Web/src/views/Meta/Finv/Quote/Realtime/FutureRankView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Future/Page`、`GET .../Realtime/Future/Types` | [Quote/Realtime/FutureRank.md](Quote/Realtime/FutureRank.md) |
| 43 | 加密货币行情排行 | 金融投资 | 四级叶子 | `quote-realtime-ftcrypto-rank` | `mdi-currency-btc` | /Meta/Finv/Quote/Realtime/FTCryptoRank | `Web/src/views/Meta/Finv/Quote/Realtime/CryptoRankView.vue` | `GET /API/V1/Meta/Finv/Quote/Cdeu/Realtime/Crypto/Page`、`GET .../Realtime/Crypto/QuoteCurrencies` | [Quote/Realtime/CryptoRank.md](Quote/Realtime/CryptoRank.md) |
| 44 | 外汇汇率 | 金融投资 | 四级叶子 | `quote-realtime-fxcer-overview` | `mdi-cash-sync` | `/Meta/Finv/Quote/Realtime/FXCEROverview` | `Web/src/views/Meta/Finv/Quote/Realtime/FXCEROverviewView.vue` | `POST .../Fx/CerSync`、`GET .../Fx/Cer` | [Quote/Realtime/FXCEROverview.md](Quote/Realtime/FXCEROverview.md) |
| 45 | 行情快照同步 | 金融投资 | 四级叶子（分组「行情采集」，需求 11281） | `quote-realtime-snapshot-sync` | `mdi-cloud-sync-outline` | `/Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSync` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSyncView.vue` | `POST/GET .../Realtime/FTStockQuoteSnapshotSync/Run` | [Quote/RealtimeCollect/SecuQuoteSnapshotSync.md](Quote/RealtimeCollect/SecuQuoteSnapshotSync.md) |
| 46 | 待采证券录入 | 金融投资 | 四级叶子 | `quote-realtime-pending-secu` | `mdi-database-plus-outline` | `/Meta/Finv/Quote/RealtimeCollect/PendingSecu` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/PendingSecuView.vue` | `GET .../Realtime/Stock/Pending`、`POST .../Stock/USCBackfill`、`POST .../Stock/Pending/Enqueue*` | [Quote/RealtimeCollect/PendingSecu.md](Quote/RealtimeCollect/PendingSecu.md) |
| 47 | 证券参数同步 | 金融投资 | 四级叶子 | `quote-realtime-secu-param-sync` | `mdi-database-sync-outline` | `/Meta/Finv/Quote/RealtimeCollect/SecuParamSync` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/SecuParamSyncView.vue` | `GET .../SecuKey/Status`、`POST .../SecuKey/Config`、`POST .../SecuKey/Sync` | [Quote/RealtimeCollect/SecuParamSync.md](Quote/RealtimeCollect/SecuParamSync.md) |
| 48 | 证券参数补全 | 金融投资 | 四级叶子 | `quote-realtime-secu-param-complete` | `mdi-table-cog` | `/Meta/Finv/Quote/RealtimeCollect/SecuParamComplete` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/SecuParamCompleteView.vue` | `GET .../Secu/Param/Complete/Pending`、`GET .../Secu/Param/Complete/Prefill`、`POST .../Secu/Param/Complete` | [Quote/RealtimeCollect/SecuParamComplete.md](Quote/RealtimeCollect/SecuParamComplete.md) |
| 49 | OpenAPI 调用方 | 金融投资 | 四级叶子 | `openapi-client` | `mdi-account-key-outline` | `/Meta/Finv/OpenAPI/Client` | `Web/src/views/Meta/Finv/OpenAPI/ClientView.vue` | `GET .../OpenAPI/Client/Page`、`POST .../OpenAPI/Client/Save`、`POST .../OpenAPI/Client/Enable` | [OpenAPI/Client.md](OpenAPI/Client.md) |
| 50 | OpenAPI 凭证 | 金融投资 | 四级叶子 | `openapi-credential` | `mdi-key-chain` | `/Meta/Finv/OpenAPI/Credential` | `Web/src/views/Meta/Finv/OpenAPI/CredentialView.vue` | `GET .../OpenAPI/Credential/Page`、`POST .../OpenAPI/Credential/Create`、`POST .../OpenAPI/Credential/Enable`、`POST .../OpenAPI/Credential/Revoke`、`POST .../OpenAPI/Credential/Rotate` | [OpenAPI/Credential.md](OpenAPI/Credential.md) |
| 51 | OpenAPI 调用审计 | 金融投资 | 四级叶子 | `openapi-invoke-log` | `mdi-text-box-search-outline` | `/Meta/Finv/OpenAPI/InvokeLog` | `Web/src/views/Meta/Finv/OpenAPI/InvokeLogView.vue` | `GET .../OpenAPI/InvokeLog/Page` | [OpenAPI/InvokeLog.md](OpenAPI/InvokeLog.md) |
| 52 | OpenAPI 鉴权配置 | 金融投资 | 四级叶子 | `openapi-settings` | `mdi-tune-variant` | `/Meta/Finv/OpenAPI/Settings` | `Web/src/views/Meta/Finv/OpenAPI/SettingsView.vue` | `GET .../OpenAPI/Settings`、`PUT .../OpenAPI/Settings` | [OpenAPI/Settings.md](OpenAPI/Settings.md) |
| 53 | OpenAPI 密钥配置 | 金融投资 | 四级叶子 | `openapi-secret` | `mdi-shield-key-outline` | `/Meta/Finv/OpenAPI/Secret` | `Web/src/views/Meta/Finv/OpenAPI/SecretView.vue` | `GET .../OpenAPI/Secret/Status`、`POST .../OpenAPI/Secret/Save` | [OpenAPI/Secret.md](OpenAPI/Secret.md) |
| 54 | 证券指标 | 金融投资 | 四级叶子 | `quote-realtime-secu-indicator` | `mdi-table-large` | `/Meta/Finv/Quote/Realtime/SecuIndicator` | `Web/src/views/Meta/Finv/Quote/Realtime/SecuIndicatorView.vue` | `GET /API/V1/Meta/Finv/Quote/Secu/Indicator/Columns`、`GET .../Secu/Indicator/Page` | [Quote/Realtime/SecuIndicator.md](Quote/Realtime/SecuIndicator.md) |
| 55 | 采集参数 | 金融投资 | 四级叶子 | `cdeu-collect-param` | `mdi-tune-variant` | `/Meta/Finv/Quote/Cdeu/CollectParam` | `Web/src/views/Meta/Finv/Quote/Cdeu/CollectParamView.vue` | `GET /API/V1/Meta/Finv/Quote/Secu/Collect/Page`、`POST .../Secu/Collect/Update` | [Quote/Cdeu/CollectParam.md](Quote/Cdeu/CollectParam.md) |
| 56 | CDEU 全流程拓扑 | 金融投资 | 四级叶子 | `cdeu-workflow`（未登记） | `mdi-sitemap` | `/Meta/Finv/Quote/Cdeu/CdeuWorkflow` | `Web/src/views/Meta/Finv/Quote/Cdeu/CdeuWorkflowView.vue` | `GET .../DeDup/CDEU/Overview` | [Quote/Cdeu/CdeuWorkflow.md](Quote/Cdeu/CdeuWorkflow.md) |
| 57 | 历史月份存量数据清点 | 金融投资 | 四级叶子 | `cdeu-his-month-exist-secu-stat` | `mdi-database-eye-outline` | `/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat` | `Web/src/views/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStatView.vue` | `GET .../DeDup/Export/Stock/Page` | [Quote/Cdeu/HisMonthExistSecuStat.md](Quote/Cdeu/HisMonthExistSecuStat.md) |
| 58 | 过时USC映射到新USC | 金融投资 | 四级叶子 | `cdeu-usc-patch` | `mdi-swap-horizontal-bold` | `/Meta/Finv/Quote/Cdeu/Usc/Patch` | `Web/src/views/Meta/Finv/Quote/Cdeu/Usc/PatchView.vue` | `POST .../Quote/Cdeu/Usc/Patch` | [Quote/Cdeu/Usc/Patch.md](Quote/Cdeu/Usc/Patch.md) |
| 59 | CDEU自动化 | 金融投资 | 四级叶子 | `cdeu-automation` | `mdi-robot-happy-outline` | `/Meta/Finv/Quote/Cdeu/Automation` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [Quote/Cdeu/Automation.md](Quote/Cdeu/Automation.md) |
| 60 | USC 映射规则 | 金融投资 | 四级叶子 | `quote-realtime-usc-mapping` | `mdi-transit-connection-variant` | `/Meta/Finv/Quote/RealtimeCollect/UscMapping` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/UscMappingView.vue` | `GET/POST .../Quote/UscMap` | [Quote/RealtimeCollect/UscMapping.md](Quote/RealtimeCollect/UscMapping.md) |
| 60.1 | 行情采集系统配置 | 金融投资 | 四级叶子 | `quote-collect-sysconfig` | `mdi-cog-sync-outline` | `/Meta/Finv/Quote/RealtimeCollect/CollectSysConfig` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/CollectSysConfigView.vue` | `GET/POST .../Quote/Collect/Config` | [Quote/RealtimeCollect/CollectSysConfig.md](Quote/RealtimeCollect/CollectSysConfig.md) |
| 61 | OpenAPI 接口清单 | 金融投资 | 四级叶子 | `openapi-endpoint` | `mdi-api` | `/Meta/Finv/OpenAPI/Endpoint` | `Web/src/views/Meta/Finv/OpenAPI/EndpointView.vue` | `GET/POST .../OpenAPI/Admin/Endpoint` | [OpenAPI/Endpoint.md](OpenAPI/Endpoint.md) |
| 62 | 回测任务删除管理 | 金融投资 | 四级叶子 | `run-delete-tasks`（未登记） | `mdi-delete-clock-outline` | `/Meta/Finv/Quant/Backtest/RunDeleteTasks` | `Web/src/views/Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue` | `POST .../Backtest/Run/DeleteTask/Retry` | [Quant/Backtest/RunDeleteTasks.md](Quant/Backtest/RunDeleteTasks.md) |
| 63 | 投资策略回测收益分析报告 | 金融投资 | 四级叶子 | `backtest-report`（未登记） | `mdi-file-chart-outline` | `/Meta/Finv/Quant/Backtest/Analysis/Report` | `Web/src/views/Meta/Finv/Quant/Backtest/Analysis/ReportView.vue` | `GET .../Backtest/Analysis` | [Quant/Backtest/Analysis/Report.md](Quant/Backtest/Analysis/Report.md) |
| 64 | 备份登记维护 | 备份与恢复 | 四级叶子 | `brs-table` | `mdi-table-cog` | `/Meta/BRS/Backup/Finv/Table` | `Web/src/views/Meta/BRS/Backup/Finv/BrsBackupTableView.vue` | `GET/POST .../BRS/Backup/Finv/Table` | [BRS/Backup/Finv/Table.md](BRS/Backup/Finv/Table.md) |
| 65 | 备份记录与产物 | 备份与恢复 | 四级叶子 | `brs-record` | `mdi-file-clock-outline` | `/Meta/BRS/Backup/Finv/Record` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [BRS/Backup/Finv/Record.md](BRS/Backup/Finv/Record.md) |
| 66 | 对象存储配置 | 备份与恢复 | 四级叶子 | `brs-storage` | `mdi-cloud-cog-outline` | `/Meta/BRS/Backup/Finv/OSSConfig` | `Web/src/views/Meta/BRS/Backup/Finv/BrsOssConfigView.vue` | `GET/POST .../BRS/Backup/Finv/OSSConfig` | [BRS/Backup/Finv/OSSConfig.md](BRS/Backup/Finv/OSSConfig.md) |
| 67 | 备份调度配置 | 备份与恢复 | 四级叶子 | `brs-schedule` | `mdi-calendar-clock` | `/Meta/BRS/Backup/Finv/Schedule` | `Web/src/views/Meta/BRS/Backup/Finv/BrsScheduleView.vue` | `GET/POST .../BRS/Backup/Finv/Schedule` | [BRS/Backup/Finv/Schedule.md](BRS/Backup/Finv/Schedule.md) |
| 68 | 小表全量备份 | 备份与恢复 | 四级叶子 | `brs-run` | `mdi-database-export` | `/Meta/BRS/Backup/Finv/Run` | `Web/src/views/Meta/BRS/Backup/Finv/BrsRunView.vue` | `GET/POST .../BRS/Backup/Finv/Run` | [BRS/Backup/Finv/Run.md](BRS/Backup/Finv/Run.md) |
| 69 | 大表增量备份 | 备份与恢复 | 四级叶子 | `brs-slice` | `mdi-table-large` | `/Meta/BRS/Backup/Finv/Slice` | `Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue` | `GET/POST .../BRS/Backup/Finv/Slice` | [BRS/Backup/Finv/Slice.md](BRS/Backup/Finv/Slice.md) |
| 70 | 数据恢复 | 备份与恢复 | 四级叶子 | `brs-restore-run` | `mdi-database-arrow-left` | `/Meta/BRS/Restore/Run` | `Web/src/views/Common/PlaceholderView.vue` | —（规划中） | [BRS/Restore/Run.md](BRS/Restore/Run.md) |
| 71 | 状态总览 | 金融投资 | 四级叶子 | `quote-collect-overview` | `mdi-view-dashboard-outline` | `/Meta/Finv/Quote/RealtimeCollect/Overview` | `Web/src/views/Meta/Finv/Quote/RealtimeCollect/OverviewView.vue` | `GET .../Realtime/FTStockQuoteSnapshotSync/Run`、`GET .../Secu/Collect/Stats` | [Quote/RealtimeCollect/Overview.md](Quote/RealtimeCollect/Overview.md) |

## 说明

- 菜单定义与视图组件一一对应，全部在 `Web/src/config/menuConfig.ts` 的 `menuSystems` 中登记；新增菜单时须同步更新本索引。
- **一级系统**由顶栏切换器选择（`Web/src/components/menu/SystemSwitcher.vue`），左侧抽屉只渲染当前系统的二~四级菜单；系统归属按 `MenuSystem.match` 前缀判定（最长前缀优先）。
- 各系统上次访问的路由记录在 `localStorage['finvquant.nav.lastPathBySystem.v1']`，切换系统时回到该系统上次页面，首次进入落到系统落地路由。
- 菜单展开状态按系统分桶持久化在 `localStorage['finvquant.nav.openKeys.v1']`；深链接直开时自动展开祖先链。
- 导航视觉固定为 WebCMS 风格：当前菜单项左侧 3px 主色激活条、祖先分组标题主色（无风格开关；如需调整见 2610-012 §3.7）。
- 登录后每个页面底部显示页脚 `© 2026 ACANX`。
- 图标统一使用 Material Design Icons（`@mdi/font`），**二/三/四级菜单均保留图标**；**注意：`mdi-chart-candlestick` 在 @mdi/font 7.4.47 中不存在**，K 线类菜单使用 `mdi-chart-line` / `mdi-chart-box`。
- 三个字典维护菜单（交易所/市场/证券）的 List 分页：**启用的（flag_enable='1'）优先展示，禁用的排后面**，同状态按 code 升序（见 `internal/meta/service.go`）。
- 「菜单文档」列指向对应业务菜单介绍文档（`Docs/Menu/xxx/XXXX.md`，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）；暂无文档的菜单以 `—` 标注，待补充。
- 序号 **54–68** 为 2026-11 补登记（CDEU 工作流 / 历史月份存量清点 / USC 补丁 / USC 映射 / OpenAPI 接口清单 / 回测删除与报告 / BRS 备份与恢复），按补登顺序追加，不重排既有序号。
- 「使用的后端接口」列与对应 API 接口文档的「已使用位置登记」互为索引（见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 7 节）。
- 视图组件名称与菜单 key 的对应关系（视图文件统一存放于 `Web/src/views/` 并按业务域分子目录，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）：

| 菜单 key | 视图组件（相对 `Web/src/views/`） |
|----------|----------------------------------|
| `dashboard` | `DashboardView.vue` |
| `quote-query` | `Meta/Finv/Quote/History/HistoryQuoteQueryView.vue` |
| `meta-import` | `Meta/Finv/Quote/History/HistoryQuoteImportView.vue` |
| `quote-realtime-ftstockrank` | `Meta/Finv/Quote/Realtime/FTStockRankView.vue` |
| `quote-realtime-ftfuture-rank` | `Meta/Finv/Quote/Realtime/FutureRankView.vue` |
| `quote-realtime-ftcrypto-rank` | `Meta/Finv/Quote/Realtime/CryptoRankView.vue` |
| `quote-realtime-fxcer-overview` | `Meta/Finv/Quote/Realtime/FXCEROverviewView.vue` |
| `quote-collect-overview` | `Meta/Finv/Quote/RealtimeCollect/OverviewView.vue` |
| `quote-realtime-snapshot-sync` | `Meta/Finv/Quote/RealtimeCollect/SecuQuoteSnapshotSyncView.vue` |
| `quote-realtime-pending-secu` | `Meta/Finv/Quote/RealtimeCollect/PendingSecuView.vue` |
| `quote-realtime-secu-param-complete` | `Meta/Finv/Quote/RealtimeCollect/SecuParamCompleteView.vue` |
| `quote-realtime-secu-param-sync` | `Meta/Finv/Quote/RealtimeCollect/SecuParamSyncView.vue` |
| `quote-realtime-usc-mapping` | `Meta/Finv/Quote/RealtimeCollect/UscMappingView.vue` |
| `quote-realtime-secu-indicator` | `Meta/Finv/Quote/Realtime/SecuIndicatorView.vue` |
| `cdeu-collect-param` | `Meta/Finv/Quote/Cdeu/CollectParamView.vue` |
| `cdeu-daykline-sync` | `Meta/Finv/Quote/Cdeu/DayKlineSyncView.vue` |
| `dedup-export-cdeu` | `Meta/Finv/Quote/Cdeu/CdeuOverviewView.vue` |
| `cdeu-workflow`（未登记） | `Meta/Finv/Quote/Cdeu/CdeuWorkflowView.vue` |
| `cdeu-his-month-exist-secu-stat` | `Meta/Finv/Quote/Cdeu/HisMonthExistSecuStatView.vue` |
| `cdeu-usc-patch` | `Meta/Finv/Quote/Cdeu/Usc/PatchView.vue` |
| `dedup-kline-query` | `Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineQueryView.vue` |
| `dedup-kline-stats` | `Meta/Finv/Quote/DeDup/Secu/KLine/Min/KLineStatsView.vue` |
| `dedup-export-overview` | `Meta/Finv/Quote/DeDup/Export/OverviewView.vue` |
| `dedup-export-chain` | `Meta/Finv/Quote/DeDup/Export/ChainView.vue` |
| `dedup-export-job` | `Meta/Finv/Quote/DeDup/Export/JobView.vue` |
| `dedup-export-file` | `Meta/Finv/Quote/DeDup/Export/FileView.vue` |
| `dedup-export-stats` | `Meta/Finv/Quote/DeDup/Export/StatsView.vue` |
| `dedup-export-config` | `Meta/Finv/Quote/DeDup/Export/ConfigView.vue` |
| `dedup-export-log` | `Meta/Finv/Quote/DeDup/Export/LogView.vue` |
| `meta-exchange` | `Meta/Finv/MetaData/ExchangeView.vue` |
| `meta-market` | `Meta/Finv/MetaData/MarketView.vue` |
| `meta-security` | `Meta/Finv/MetaData/SecurityView.vue` |
| `config-manage` | `Meta/Finv/Quant/ConfigView.vue` |
| `environment` | `Meta/Finv/Quant/EnvironmentView.vue` |
| `template` | `Meta/Finv/Quant/TemplateView.vue` |
| `backtest-gold-futures` | `Meta/Finv/Quant/Backtest/GoldFuturesView.vue` |
| `account` | `Meta/Finv/Quant/AccountView.vue` |
| `fund` | `Meta/Finv/Quant/FundView.vue` |
| `position` | `Meta/Finv/Quant/PositionView.vue` |
| `strategy` | `Meta/Finv/Quant/StrategyView.vue` |
| `backtest-analysis` | `Meta/Finv/Quant/Backtest/AnalysisView.vue` |
| `backtest-report`（未登记） | `Meta/Finv/Quant/Backtest/Analysis/ReportView.vue` |
| `run-delete-tasks`（未登记） | `Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue` |
| `openapi-client` / `openapi-credential` / `openapi-invoke-log` / `openapi-settings` / `openapi-secret` / `openapi-endpoint` | `Meta/Finv/OpenAPI/{Client,Credential,InvokeLog,Settings,Secret,Endpoint}View.vue` |
| `brs-table` / `brs-storage` / `brs-schedule` / `brs-run` / `brs-slice` | `Meta/BRS/Backup/Finv/Brs{BackupTable,OssConfig,Schedule,Run,Slice}View.vue` |
| `simulation-data` / `simulation-paper` / `simulation-live-sim` / `live-trading` / `cms-home` / `cdeu-automation` / `brs-record` / `brs-restore-run` | `Common/PlaceholderView.vue`（占位，多个菜单共用） |

## 相关文档

- [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md) — 前端菜单开发规范
- [Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md](../Refactor/2610-012-WebMenuFourLevelRefactor.md) — 四级菜单改造方案
- [Docs/API/README.md](../API/README.md) — 服务端 API 文档入口
- [Docs/API/APIs.md](../API/APIs.md) — 服务端 API 总览（接口清单索引）
