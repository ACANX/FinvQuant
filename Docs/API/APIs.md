# 服务端 API 总览（APIs）

> 所属：FinvQuant 服务端 · 存放：`Docs/API/APIs.md`
> 适用范围：服务端（Go + Gin）`/API/V1` 路由组下的全部接口清单索引。
> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）；服务端端口 **16001**。
> 接口文档规范见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)；菜单使用方索引见 [Docs/Menu/Menus.md](../Menu/Menus.md)。

## API 清单

### 认证（全部业务接口的入口，需求 10761）

| # | 方法 | 路径 | 说明 | 详细文档 | 已使用位置（业务菜单） |
|---|------|------|------|----------|------------------------|
| A1 | POST | `/API/V1/Meta/Finv/Auth/Login` | 校验用户名口令并签发会话令牌（argon2id 校验，失败统一「用户名或密码错误」） | [Auth/Auth.md](Auth/Auth.md) | 登录页（`LoginView.vue`） |
| A2 | POST | `/API/V1/Meta/Finv/Auth/Logout` | 吊销当前会话（删除 Redis 令牌，立即失效） | [Auth/Auth.md](Auth/Auth.md) | 顶栏用户菜单 |
| A3 | GET | `/API/V1/Meta/Finv/Auth/Profile` | 查询当前登录用户资料（不含口令字段） | [Auth/Auth.md](Auth/Auth.md) | 顶栏用户菜单 |
| A4 | POST | `/API/V1/Meta/Finv/Auth/ChangePassword` | 修改当前用户密码（校验当前口令 + 强度规则，改后重新加盐哈希并吊销其它设备会话） | [Auth/Auth.md](Auth/Auth.md) | 顶栏用户菜单 → 修改密码 |

> 🔒 除 `/Health/Live`、`/Health/Ready`、`/Version`、`/Meta/Finv/Auth/Login` 外，下表所有接口都要求 `Authorization: Bearer <token>`，否则返回 `401`。

### 业务接口

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
| 46 | POST | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Add` | 分钟行情 K 线单条插入（原 FinvQuoteDataDeDup API-01） | [Quote/Cdeu/Min/KLineAdd.md](Quote/Cdeu/Min/KLineAdd.md) | 分钟行情采集端 |
| 47 | POST | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Batch` | 批量插入（冲突整体回滚、请求内重复拦截） | [Quote/Cdeu/Min/KLineBatchInsert.md](Quote/Cdeu/Min/KLineBatchInsert.md) | 分钟行情采集端 |
| 48 | POST | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` | 批量 upsert（`onConflict=skip\|update`，整行覆盖） | [Quote/Cdeu/Min/KLineUpsert.md](Quote/Cdeu/Min/KLineUpsert.md) | 分钟行情采集端 |
| 49 | GET | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Page` | 条件分页查询（紧凑字段对象数组） | [Quote/Cdeu/Min/KLinePage.md](Quote/Cdeu/Min/KLinePage.md) | 采集端/核对工具、去重分钟行情查询（`DeDupKLineQueryView.vue`） |
| 50 | GET | `/API/V1/Meta/Finv/Quote/Cdeu/Min/PageUni` | 条件分页查询（column/row 行数组） | [Quote/Cdeu/Min/KLinePageUni.md](Quote/Cdeu/Min/KLinePageUni.md) | 表格类前端/核对工具 |
| 51 | POST | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Query` | 按 `(usc,ts)`/`uscs`/`provider` 批量查询 | [Quote/Cdeu/Min/KLineQuery.md](Quote/Cdeu/Min/KLineQuery.md) | 采集端回读校验 |
| 52 | PATCH | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Batch` | 批量增量更新（仅更新显式字段） | [Quote/Cdeu/Min/KLineBatchPatch.md](Quote/Cdeu/Min/KLineBatchPatch.md) | 数据修正工具 |
| 53 | DELETE | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Batch` | 批量删除（keys/uscs OR 语义） | [Quote/Cdeu/Min/KLineBatchDelete.md](Quote/Cdeu/Min/KLineBatchDelete.md) | 数据清理工具 |
| 54 | GET | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Count` | 分钟行情记录总数统计 | [Quote/Cdeu/Min/KLineCount.md](Quote/Cdeu/Min/KLineCount.md) | 采集端入库量统计 |
| 55 | GET | `/Health` | 探活（探数据库；`/healthz` 为过渡别名） | [Quote/Cdeu/Min/KLineHealth.md](Quote/Cdeu/Min/KLineHealth.md) | 监控/容器 healthcheck |
| 56 | GET | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Stats` | 去重行情汇总统计（汇总/每日/证券分布/空值完整性） | [Quote/Cdeu/Min/KLineStats.md](Quote/Cdeu/Min/KLineStats.md) | 去重分钟行情统计（`DeDupKLineStatsView.vue`） |
| 57 | GET | `/API/V1/Meta/Finv/Quote/Cdeu/Min/Bounds` | 交易日期边界（min/max date，轻量） | [Quote/Cdeu/Min/KLineBounds.md](Quote/Cdeu/Min/KLineBounds.md) | 去重分钟行情查询/统计页默认日期范围（`DeDupKLineQueryView.vue`、`DeDupKLineStatsView.vue`） |
| 58 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Status` | 归档链路指标（任务/上传状态、积压、预算、补丁、告警）与调度表 | [QuoteDeDup/Status.md](QuoteDeDup/Status.md) | 归档监控总览 |
| 59 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Job/List` | 分页查询证券月导出任务 | [QuoteDeDup/JobList.md](QuoteDeDup/JobList.md) | 导出任务 |
| 60 | GET | `/API/V1/Meta/Finv/Quote/DeDup/File/List` | 分页查询导出文件与上传状态 | [QuoteDeDup/FileList.md](QuoteDeDup/FileList.md) | 文件与上传 |
| 61 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Job/Retry` | 重试导出任务（重置为 `WAITING`） | [QuoteDeDup/JobRetry.md](QuoteDeDup/JobRetry.md) | 导出任务 |
| 62 | POST | `/API/V1/Meta/Finv/Quote/DeDup/File/Retry` | 重试上传（重置为 `PENDING_UPLOAD`） | [QuoteDeDup/FileRetry.md](QuoteDeDup/FileRetry.md) | 文件与上传 |
| 63 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Job/Run` | 手动触发归档 Job（异步，返回 `run_log_id`，可选续跑后续 JOB） | [QuoteDeDup/JobRun.md](QuoteDeDup/JobRun.md) | 归档监控总览 |
| 64 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Usc/Page` | 分页查询证券主数据（含 `day_expect_rows` 校准值） | [QuoteDeDup/Usc.md](QuoteDeDup/Usc.md) | 归档主数据与规则 |
| 65 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Usc` | 新增或更新证券主数据 | [QuoteDeDup/Usc.md](QuoteDeDup/Usc.md) | 归档主数据与规则 |
| 66 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Priority/List` | 查询证券优先级 / SLA | [QuoteDeDup/Priority.md](QuoteDeDup/Priority.md) | 归档主数据与规则 |
| 67 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Priority` | 新增或更新优先级 | [QuoteDeDup/Priority.md](QuoteDeDup/Priority.md) | 归档主数据与规则 |
| 68 | DELETE | `/API/V1/Meta/Finv/Quote/DeDup/Priority` | 删除优先级 | [QuoteDeDup/Priority.md](QuoteDeDup/Priority.md) | 归档主数据与规则 |
| 69 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Calendar/Page` | 分页查询交易日历 | [QuoteDeDup/Calendar.md](QuoteDeDup/Calendar.md) | 归档主数据与规则 |
| 70 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Calendar` | 新增或更新交易日历标记 | [QuoteDeDup/Calendar.md](QuoteDeDup/Calendar.md) | 归档主数据与规则 |
| 71 | DELETE | `/API/V1/Meta/Finv/Quote/DeDup/Calendar` | 删除交易日历记录 | [QuoteDeDup/Calendar.md](QuoteDeDup/Calendar.md) | 归档主数据与规则 |
| 72 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Maturity/Page` | 分页查询成熟度 | [QuoteDeDup/Maturity.md](QuoteDeDup/Maturity.md) | 归档统计与预算 |
| 73 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Maturity/Refresh` | 异步触发成熟度重算 | [QuoteDeDup/Maturity.md](QuoteDeDup/Maturity.md) | 归档统计与预算 |
| 74 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Stats/Page` | 分页查询入口日统计 | [QuoteDeDup/Stats.md](QuoteDeDup/Stats.md) | 归档统计与预算 |
| 75 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Budget/Page` | 分页查询每日预算 | [QuoteDeDup/Budget.md](QuoteDeDup/Budget.md) | 归档统计与预算 |
| 76 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Budget/Override` | 管理员覆盖当日预算 | [QuoteDeDup/Budget.md](QuoteDeDup/Budget.md) | 归档统计与预算 |
| 77 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Watermark/List` | 查询扫描水位 | [QuoteDeDup/Watermark.md](QuoteDeDup/Watermark.md) | 归档统计与预算 |
| 78 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Watermark` | 重置扫描水位（高危） | [QuoteDeDup/Watermark.md](QuoteDeDup/Watermark.md) | 归档统计与预算 |
| 79 | GET | `/API/V1/Meta/Finv/Quote/DeDup/RunLog/Page` | 分页查询 Job 运行日志（含阶段 / 进度 / 心跳） | [QuoteDeDup/RunLog.md](QuoteDeDup/RunLog.md) | 归档日志与补丁 |
| 80 | GET | `/API/V1/Meta/Finv/Quote/DeDup/UploadLog/Page` | 分页查询上传审计 | [QuoteDeDup/UploadLog.md](QuoteDeDup/UploadLog.md) | 归档日志与补丁 |
| 81 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Patch/Page` | 分页查询补丁任务 | [QuoteDeDup/Patch.md](QuoteDeDup/Patch.md) | 归档日志与补丁 |
| 82 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Patch/Retry` | 重试补丁任务（重置为 `PENDING`） | [QuoteDeDup/Patch.md](QuoteDeDup/Patch.md) | 归档日志与补丁 |
| 83 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Job/Pause` | 暂停导出任务 | [QuoteDeDup/JobControl.md](QuoteDeDup/JobControl.md) | 导出任务 |
| 84 | POST | `/API/V1/Meta/Finv/Quote/DeDup/Job/Cancel` | 取消导出任务 | [QuoteDeDup/JobControl.md](QuoteDeDup/JobControl.md) | 导出任务 |
| 85 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Job/Runtime` | 10 个 Job 的运行快照（当前阶段 / 进度 / 心跳 / 最近结果 / 24 小时统计） | [QuoteDeDup/JobRuntime.md](QuoteDeDup/JobRuntime.md) | 端到端链路 |
| 86 | GET | `/API/V1/Meta/Finv/Quote/DeDup/Settings` | 查询运行时设置（低峰窗口、来源、更新时间、是否在窗口内） | [QuoteDeDup/Settings.md](QuoteDeDup/Settings.md) | 端到端链路 |
| 87 | PUT | `/API/V1/Meta/Finv/Quote/DeDup/Settings/Window` | 在线修改低峰窗口（保存后立即生效） | [QuoteDeDup/Settings.md](QuoteDeDup/Settings.md) | 端到端链路 |
| 88 | GET | `/API/V1/Meta/Finv/Quote/DeDup/CDEU/Overview` | CDEU 工作流定义树、指定运行状态与历史运行 | [QuoteDeDup/CDEUOverview.md](QuoteDeDup/CDEUOverview.md) | 工作流控制与监控 |
| 89 | POST | `/API/V1/Meta/Finv/Quote/DeDup/CDEU/Workflow/Run` | 手动启动完整 CDEU 工作流（后台按 Phase → Process → JOB → Step 执行） | [QuoteDeDup/CDEUWorkflowRun.md](QuoteDeDup/CDEUWorkflowRun.md) | 工作流控制与监控 |
| 90 | GET | `/API/V1/Meta/Finv/Quote/DeDup/RunLog/Detail` | 查询单次 Job 运行汇总与详细日志行 | [QuoteDeDup/RunLog.md](QuoteDeDup/RunLog.md) | 归档日志与补丁 / 工作流控制与监控 |
| 91 | POST | `/OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert` | 外部 OpenAPI 示例：批量 upsert（静态密钥认证；成功响应 `{code,data,message}`，code 默认 0） | [../OpenAPI/Quote/Cdeu/Min/Upsert.md](../OpenAPI/Quote/Cdeu/Min/Upsert.md) | 外部 HOOK / 采集端 |

## 说明

- 接口清单以 `internal/api/router.go` 实际注册为准；新增接口须同步更新本索引及 [Docs/API/README.md](README.md) 端点索引。
- 分钟行情 K 线接口**统一使用** `/API/V1/Meta/Finv/Quote/Cdeu/Min/*`；旧前缀 `/API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min` 与 Python 服务前缀 `/API/V1/Meta/Finv/Quote/Secu/KLine/Min` **均已不再注册**（需求 10777 / 10781 / 10782）。
- 「详细文档」列指向对应接口文档（`Docs/API/xxx/XXXX.md`，见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 7 节）；暂无独立文档的接口以 `—` 标注。
- 「已使用位置（业务菜单）」列与菜单文档中的接口引用互为索引（见 [MenuSpec.md](../DevSpec/MenuSpec.md) 第 3 节）；接口新增使用方或下线时须同步更新。
- 通用探针/信息接口（health、version）不强制要求独立文档，但契约变更须同步本索引。

## 相关文档

- [Docs/API/README.md](README.md) — API 文档入口（响应信封、错误码、端点索引）
- [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md) — API 开发规范
- [Docs/Menu/Menus.md](../Menu/Menus.md) — Web 前端业务菜单总览
