# 服务端 API 总览（APIs）

> 所属：FinvQuant 服务端 · 存放：`Docs/API/APIs.md`
> 适用范围：服务端（Go + Gin）`/API/V1` 路由组下的全部接口清单索引。
> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）；服务端端口 **16001**。
> 接口文档规范见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)；菜单使用方索引见 [Docs/Menu/Menus.md](../Menu/Menus.md)。

## API 清单

| # | 方法 | 路径 | 说明 | 详细文档 | 已使用位置（业务菜单） |
|---|------|------|------|----------|------------------------|
| 1 | GET | `/API/V1/Health/Live` | 存活探针：进程存活即 200，返回 `status` / `server` | — | 仪表盘（`DashboardView.vue`） |
| 2 | GET | `/API/V1/Health/Ready` | 就绪探针：PG 与 Redis 均可达才 200，否则 503 | — | — |
| 3 | GET | `/API/V1/Version` | 服务端版本信息：`name` / `version` / `go_version` / `commit` | — | — |
| 4 | POST | `/API/V1/Meta/Finv/Quant/Quote/Import/Upload` | 上传 MVSV 历史行情文件并导入 PG（字段级/整行覆盖） | [HistoryQuote/ImportsUpload.md](HistoryQuote/ImportsUpload.md) | 历史行情数据导入（`QuoteImportView.vue`） |
| 5 | GET | `/API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery` | 按证券代码+日期查询分钟级 K 线（周期 Min，1日/5日回溯，分页；可选回传证券名称 secu_name） | [HistoryQuote/HistoryQuote.md](HistoryQuote/HistoryQuote.md) | 历史行情查询（[Menus.md](../Menu/Menus.md#3) 之历史行情查询菜单，文档见 [HistoryQuote/HistoryQuoteQuery.md](../Menu/HistoryQuote/HistoryQuoteQuery.md)） |
| 6 | GET | `/API/V1/Meta/Finv/Quant/Metadata/Exchange/List` | 分页查询交易所字典 finv_exchange | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（[Menus.md](../Menu/Menus.md#6)，文档见 [Meta/MetaExchange.md](../Menu/Meta/MetaExchange.md)） |
| 7 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Exchange/Save` | 新增/修改交易所字典 | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（同上） |
| 8 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Exchange/Toggle` | 禁用/启用交易所 | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（同上） |
| 9 | GET | `/API/V1/Meta/Finv/Quant/Metadata/Market/List` | 分页查询交易市场字典 finv_market | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（[Menus.md](../Menu/Menus.md#7)，文档见 [Meta/MetaMarket.md](../Menu/Meta/MetaMarket.md)） |
| 10 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Market/Save` | 新增/修改交易市场字典 | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（同上） |
| 11 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Market/Toggle` | 禁用/启用交易市场 | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（同上） |
| 12 | GET | `/API/V1/Meta/Finv/Quant/Metadata/Security/List` | 分页查询证券代码字典 finv_security | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（[Menus.md](../Menu/Menus.md#8)，文档见 [Meta/MetaSecurity.md](../Menu/Meta/MetaSecurity.md)） |
| 13 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Security/Save` | 新增/修改证券代码字典 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上） |
| 14 | POST | `/API/V1/Meta/Finv/Quant/Metadata/Security/Toggle` | 禁用/启用证券代码 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上） |
| 15 | GET | `/API/V1/Meta/Finv/Quant/Metadata/Security/Options` | 证券下拉选项（usc+security_name_cn，仅启用状态） | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上）、历史行情查询（[HistoryQuote/HistoryQuoteQuery.md](../Menu/HistoryQuote/HistoryQuoteQuery.md)） |
| 16 | GET | `/API/V1/Meta/Finv/Quant/Metadata/Security/Lookup` | 按代码（usc 或源证券代码）查询证券详情，供导入双策略带出/匹配 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 历史行情数据导入（[Menus.md](../Menu/Menus.md#9)，视图 `QuoteImportView.vue`） |
| 17 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Strategy/List` | 分页查询回测策略（结构化定义） | [Backtest/StrategyList.md](Backtest/StrategyList.md) | 策略管理（[Backtest/StrategyManage.md](../Menu/Backtest/StrategyManage.md)）、黄金期货合约回测验证（[Backtest/BacktestGoldFutures.md](../Menu/Backtest/BacktestGoldFutures.md)） |
| 18 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Strategy/Get` | 查询策略详情 | [Backtest/StrategyGet.md](Backtest/StrategyGet.md) | 策略管理（同上） |
| 19 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Strategy/Save` | 新增/修改策略（定义编译校验） | [Backtest/StrategySave.md](Backtest/StrategySave.md) | 策略管理（同上） |
| 20 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Strategy/Toggle` | 切换策略回测开关 | [Backtest/StrategyToggle.md](Backtest/StrategyToggle.md) | 策略管理（同上） |
| 21 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Strategy/Delete` | 删除策略（已关联任务禁止） | [Backtest/StrategyDelete.md](Backtest/StrategyDelete.md) | 策略管理（同上） |
| 22 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Account/List` | 分页查询回测账户（多用户/多子账户） | [Backtest/AccountList.md](Backtest/AccountList.md) | 账户管理（[Backtest/AccountManage.md](../Menu/Backtest/AccountManage.md)）、黄金期货合约回测验证（同上） |
| 23 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Account/Get` | 查询账户详情 | [Backtest/AccountGet.md](Backtest/AccountGet.md) | 账户管理（同上） |
| 24 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Account/Save` | 新增/修改账户 | [Backtest/AccountSave.md](Backtest/AccountSave.md) | 账户管理（同上） |
| 25 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Account/Toggle` | 切换账户回测开关 | [Backtest/AccountToggle.md](Backtest/AccountToggle.md) | 账户管理（同上） |
| 26 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Account/Delete` | 删除账户（已关联任务禁止） | [Backtest/AccountDelete.md](Backtest/AccountDelete.md) | 账户管理（同上） |
| 27 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Run/Create` | 创建并启动回测任务（异步执行） | [Backtest/RunCreate.md](Backtest/RunCreate.md) | 黄金期货合约回测验证（[Backtest/BacktestGoldFutures.md](../Menu/Backtest/BacktestGoldFutures.md)） |
| 28 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/List` | 分页查询回测任务 | [Backtest/RunList.md](Backtest/RunList.md) | 回测分析（[Backtest/BacktestAnalysis.md](../Menu/Backtest/BacktestAnalysis.md)）、黄金期货合约回测验证、资金/持仓管理（[Backtest/FundManage.md](../Menu/Backtest/FundManage.md)、[Backtest/PositionManage.md](../Menu/Backtest/PositionManage.md)） |
| 29 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/Get` | 查询任务详情（含快照/报告） | [Backtest/RunGet.md](Backtest/RunGet.md) | 回测分析（同上） |
| 30 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Run/Cancel` | 取消运行中任务 | [Backtest/RunCancel.md](Backtest/RunCancel.md) | 回测分析（同上） |
| 31 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/Report` | 查询回测报告（汇总指标） | [Backtest/RunReport.md](Backtest/RunReport.md) | 回测分析（同上） |
| 32 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/Equity` | 净值曲线数据（余额/收益率/收益额/持仓金额/回撤） | [Backtest/RunEquity.md](Backtest/RunEquity.md) | 回测分析、资金/持仓管理（同上） |
| 33 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/Trades` | 成交记录 | [Backtest/RunTrades.md](Backtest/RunTrades.md) | 回测分析、持仓管理（同上） |
| 34 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/Cashflows` | 资金流水明细（⑨链路追踪） | [Backtest/RunCashflows.md](Backtest/RunCashflows.md) | 回测分析（同上） |
| 35 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/PositionLogs` | 持仓变化明细（⑨链路追踪） | [Backtest/RunPositionLogs.md](Backtest/RunPositionLogs.md) | 回测分析（同上） |
| 36 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Run/EventTraces` | 事件追踪（触发原因/成交结果/委托耗时/未成交原因） | [Backtest/RunEventTraces.md](Backtest/RunEventTraces.md) | 回测分析（同上） |
| 37 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Environment/List` | 分页查询回测环境（多类型×地区市场） | [Backtest/EnvironmentList.md](Backtest/EnvironmentList.md) | 环境管理（[Backtest/EnvironmentManage.md](../Menu/Backtest/EnvironmentManage.md)）、黄金期货合约回测验证（同上） |
| 38 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Environment/Get` | 查询环境详情 | [Backtest/EnvironmentGet.md](Backtest/EnvironmentGet.md) | 环境管理（同上） |
| 39 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Environment/Save` | 新增/修改环境 | [Backtest/EnvironmentSave.md](Backtest/EnvironmentSave.md) | 环境管理（同上） |
| 40 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Environment/Toggle` | 切换环境回测开关 | [Backtest/EnvironmentToggle.md](Backtest/EnvironmentToggle.md) | 环境管理（同上） |
| 41 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Environment/Delete` | 删除环境（已关联任务禁止） | [Backtest/EnvironmentDelete.md](Backtest/EnvironmentDelete.md) | 环境管理（同上） |
| 42 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Template/List` | 分页查询模板（策略/账户/环境） | [Backtest/TemplateList.md](Backtest/TemplateList.md) | 模板管理（[Backtest/TemplateManage.md](../Menu/Backtest/TemplateManage.md)） |
| 43 | GET | `/API/V1/Meta/Finv/Quant/Backtest/Template/Get` | 查询模板详情 | [Backtest/TemplateGet.md](Backtest/TemplateGet.md) | 模板管理（同上） |
| 44 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Template/Save` | 新增/修改模板 | [Backtest/TemplateSave.md](Backtest/TemplateSave.md) | 模板管理（同上） |
| 45 | POST | `/API/V1/Meta/Finv/Quant/Backtest/Template/Delete` | 删除模板（内置禁止） | [Backtest/TemplateDelete.md](Backtest/TemplateDelete.md) | 模板管理（同上） |

## 说明

- 接口清单以 `internal/api/router.go` 中 `/API/V1` 路由组注册为准；新增接口须同步更新本索引及 [Docs/API/README.md](README.md) 端点索引。
- 「详细文档」列指向对应接口文档（`Docs/API/xxx/XXXX.md`，见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 7 节）；暂无独立文档的接口以 `—` 标注。
- 「已使用位置（业务菜单）」列与菜单文档中的接口引用互为索引（见 [MenuSpec.md](../DevSpec/MenuSpec.md) 第 3 节）；接口新增使用方或下线时须同步更新。
- 通用探针/信息接口（health、version）不强制要求独立文档，但契约变更须同步本索引。

## 相关文档

- [Docs/API/README.md](README.md) — API 文档入口（响应信封、错误码、端点索引）
- [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md) — API 开发规范
- [Docs/Menu/Menus.md](../Menu/Menus.md) — Web 前端业务菜单总览
