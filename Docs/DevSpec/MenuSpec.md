# 前端菜单开发规范（MenuSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）视图菜单的新增、注册与业务文档。
> 文档规范见 [`DocSpec.md`](DocSpec.md)；API 接口文档见 [`ApiSpec.md`](ApiSpec.md)。

## 1. 菜单新增

- **前端视图组件统一存放于 `Web/src/views/` 目录下**，并**按照业务模块层级放在对应的子目录中**（每级目录使用大驼峰 PascalCase 命名），例如：
  - 策略管理菜单 → `Web/src/views/Meta/Finv/Quant/Strategy/StrategyManageView.vue`；
  - 回测分析菜单 → `Web/src/views/Meta/Finv/Quant/Backtest/BacktestAnalysisView.vue`；
  - 黄金期货合约回测验证 → `Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue`；
  - 账户管理 → `Web/src/views/Meta/Finv/Quant/Account/AccountManageView.vue`；
  - 历史行情查询 → `Web/src/views/Meta/Finv/Quote/History/HistoryQuoteQueryView.vue`；
  - 交易所信息维护 → `Web/src/views/Meta/Finv/MetaData/MetaExchangeView.vue`。
- **本项目业务视图统一归入 `Web/src/views/Meta/Finv/` 下**，按**平级业务域**分层（各域互不从属，协同支撑量化平台）：
  - **`Meta/Finv/Quant/`（Quant 量化交易）**：账户/资金/持仓/策略/回测等按菜单分层；
  - **`Meta/Finv/Quote/`（Quote 行情）**：历史行情查询、历史行情数据导入等；
  - **`Meta/Finv/MetaData/`（MetaData 元数据）**：交易所/市场/证券等业务元数据维护（与行情、量化都有关联，但不从属任一方）。
- 通用性视图（如 `DashboardView.vue`）可直接放在 `Web/src/views/` 根目录。
- 视图组件文件名使用大驼峰（PascalCase）并以 `View.vue` 结尾；必要时以模块前缀开头（如 `HistoryQuoteQueryView.vue`）避免同名歧义。
- 每个菜单对应一个视图组件（如 `HistoryQuoteQueryView.vue`）。
- 新增菜单时须在 `Web/src/App.vue` 的 `menuItems` 中登记：菜单 `key`（英文驼峰）、`title`（简体中文标题）、`icon`（mdi 图标）。
- 视图组件与 `menuItems` 中的菜单项一一对应，禁止出现未登记入口的孤立视图。

### 1.1 菜单路由路径命名规范（强制）

- **所有前端菜单路由路径统一使用大驼峰（PascalCase）命名**：路径每段以大写字母开头、其余小写，段间用 `/` 分隔，不得使用小驼峰、下划线或全小写。
- **量化交易模块（Quant）**：路由必须以 **`/Meta/Finv/Quant/`** 为前缀，其后每段采用大驼峰，例如：
  - 黄金期货合约回测验证 → `/Meta/Finv/Quant/Backtest/GoldFutures`
  - 回测分析 → `/Meta/Finv/Quant/Backtest/Analysis`
  - 投资策略回测收益分析报告 → `/Meta/Finv/Quant/Backtest/Analysis/Report`
  - 策略管理 → `/Meta/Finv/Quant/Strategy`
  - 配置管理 → `/Meta/Finv/Quant/Config`
  - 环境管理 → `/Meta/Finv/Quant/Environment`
  - 模板管理 → `/Meta/Finv/Quant/Template`
  - 实盘仿真验证 → `/Meta/Finv/Quant/Simulation/LiveSim`
- **行情模块（Quote）**：路由统一加 **`/Meta/Finv/Quote/`** 前缀，其后每段采用大驼峰（如 `/Meta/Finv/Quote/History/HistoryQuoteQuery`）。
- **元数据模块（MetaData）**：路由统一加 **`/Meta/Finv/MetaData/`** 前缀，其后每段采用大驼峰（如 `/Meta/Finv/MetaData/Exchange`）。元数据独立于行情/量化（平级关联，不从属），路由与目录均置于 `Meta/Finv/MetaData/`。
- **其他非 Finv 模块**：路由路径同样遵循大驼峰命名（如 `/Meta/Exchange`、`/Quote/Query`），不在此路径层级中引入下划线或小写片段；存量小写路由（如 `/dashboard`、`/meta/exchange`）在维护时逐步迁移对齐。
- **禁止**使用 `-`（连字符）、`_`（下划线）或全小写路径片段（如 `/meta/finvquant/backtest/analysis` 为违规写法）。
- 路由在 `Web/src/router.ts` 的 `path` 与 `Web/src/App.vue` 的 `menuItems[].path` 中**两处必须一致**，且与 `Docs/Menu/Menus.md` 索引中的「路由」列保持一致。
- 后端 API 接口路径前缀保持 `/API/V1/Meta/Finv/Quant/...`（大驼峰），前端路由与其保持同一前缀体系（见 `Docs/DevSpec/ApiSpec.md`）。

## 2. 菜单文档

### 2.1 文档必备性

- **新添加的前端视图菜单，必须补充对应的业务菜单介绍文档**，介绍该菜单的业务功能、处理逻辑、使用方法、注意事项等。

### 2.2 一个菜单一个文档（强制）

- **每个业务菜单必须独立成一篇文档文件**，**不允许多个业务菜单放到同一文件中描述**；一篇文档只对应一个菜单（唯一菜单 key / 唯一路由）；
- 文档文件名与菜单/视图名对应，使用 **PascalCase**：如 `FundManage.md` 对应资金管理菜单（视图 `FundManageView.vue`）、`PositionManage.md` 对应持仓管理菜单（视图 `PositionManageView.vue`）；
- **多个菜单共用一个视图组件时，仍须为每个菜单分别编写独立文档，不得合并**（例如占位页共用的多个规划菜单，各自独立成篇）；
- **禁止**以 `xxxAll.md`、`xxxManage.md` 等合并文件描述多个菜单，也禁止把不同菜单的内容写在同一个文件中用多级标题区分。

### 2.3 文档位置与命名

- 文档位置通常在 `Docs/Menu/xxx/XXXX.md`（按业务模块建目录，文档名与菜单/视图名对应）；
- 参考示例：[`Docs/Menu/HistoryQuote/HistoryQuoteQuery.md`](../Menu/HistoryQuote/HistoryQuoteQuery.md)（对应视图 `Web/src/views/QuoteQueryView.vue`）。
- 菜单文档随视图代码在同一变更中提交，保持「代码 + 文档」同步交付。

### 2.4 索引登记

- 新增菜单后须在 [`Menus.md`](../Menu/Menus.md) 菜单总索引中登记（菜单名称 / key / 图标 / 视图组件 / 使用的后端接口 / 菜单文档）。

## 3. 文档内容要求

业务菜单介绍文档应至少覆盖以下内容：

| 章节 | 说明 |
|------|------|
| 菜单入口 | 菜单位置、名称、key、图标、对应视图组件 |
| 业务功能概述 | 该菜单提供哪些能力（功能清单） |
| 使用方法 | 用户操作流程与步骤说明 |
| 处理逻辑 | 前端交互逻辑与后端处理链路（含请求参数、响应处理） |
| 注意事项 | 常见问题、限制与排查提示 |

- **业务菜单中使用到的后端 API 接口，须在业务菜单对应的文档中添加对应的 API 接口或接口文档引用**：逐一列出菜单调用的接口路径（如 `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`）并链接到对应接口文档（`Docs/API/xxx/XXXX.md`），便于定位后端契约与排查问题。
- 菜单文档中的接口引用与 API 文档中的「已使用位置登记」（见 [ApiSpec.md](ApiSpec.md) 第 7.5 节）形成双向索引，接口文档登记使用方、菜单文档登记所用接口，须保持一致。
