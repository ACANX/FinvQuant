# FinvQuant 服务端 API 文档

> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）
> 服务端端口：**16001**；默认地址：`http://localhost:16001`

## 通用约定

### 响应信封

所有 REST JSON 响应顶层固定输出：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `code` | int | ✅ | 业务码；成功码集合 `{0, 1, 200, 202}` |
| `message` | string | ✅ | 文本消息 |
| `data` | object | 可选 | 业务数据 |
| `error` | object | 可选 | 错误时必填；含 `code`、`catalog_version`、`retryable` |
| `details` | object | 可选 | 补充详情 |
| `request_id` | string | 可选 | 请求追踪 ID |
| `trace_id` | string | 可选 | 链路追踪 ID |

所有 wire 字段使用 **snake_case**。

### 错误码

| 号段 | 含义 |
|------|------|
| `{0, 1, 200, 202}` | 成功 |
| `1000-2999` | 平台、安全和依赖错误 |
| `≥3000` | 项目自定义业务错误码 |

（详见 [Docs/DevSpec/ErrorCodeSpec.md](../DevSpec/ErrorCodeSpec.md)）

## 接口索引

- **接口总览（方法 / 路径 / 说明 / 详细文档 / 已使用位置）**：见 [APIs.md](APIs.md)。
- **详细接口文档**：
  - [`HistoryQuote/ImportsUpload.md`](HistoryQuote/ImportsUpload.md) — `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`（MVSV 历史行情导入）
  - [`HistoryQuote/HistoryQuote.md`](HistoryQuote/HistoryQuote.md) — `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`（历史行情查询）
  - [`Meta/MetaExchange.md`](Meta/MetaExchange.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/*`（交易所信息维护）
  - [`Meta/MetaMarket.md`](Meta/MetaMarket.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Market/*`（交易所下设市场信息维护）
  - [`Meta/MetaSecurity.md`](Meta/MetaSecurity.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Security/*`（规范证券信息维护，含 `Options` 下拉与 `Lookup` 详情查询）
  - 回测策略管理（`/API/V1/Meta/Finv/Quant/Backtest/Strategy/*`，**一个接口一个文档**）：
    - [`Backtest/StrategyList.md`](Backtest/StrategyList.md) — `GET .../Strategy/List`（分页查询策略）
    - [`Backtest/StrategyGet.md`](Backtest/StrategyGet.md) — `GET .../Strategy/Get`（查询策略详情）
    - [`Backtest/StrategySave.md`](Backtest/StrategySave.md) — `POST .../Strategy/Save`（新增/修改策略）
    - [`Backtest/StrategyToggle.md`](Backtest/StrategyToggle.md) — `POST .../Strategy/Toggle`（切换回测开关）
    - [`Backtest/StrategyDelete.md`](Backtest/StrategyDelete.md) — `POST .../Strategy/Delete`（删除策略）
  - 回测账户管理（`/API/V1/Meta/Finv/Quant/Backtest/Account/*`）：
    - [`Backtest/AccountList.md`](Backtest/AccountList.md) — `GET .../Account/List`（分页查询账户）
    - [`Backtest/AccountGet.md`](Backtest/AccountGet.md) — `GET .../Account/Get`（查询账户详情）
    - [`Backtest/AccountSave.md`](Backtest/AccountSave.md) — `POST .../Account/Save`（新增/修改账户）
    - [`Backtest/AccountToggle.md`](Backtest/AccountToggle.md) — `POST .../Account/Toggle`（切换回测开关）
    - [`Backtest/AccountDelete.md`](Backtest/AccountDelete.md) — `POST .../Account/Delete`（删除账户）
  - 回测任务（`/API/V1/Meta/Finv/Quant/Backtest/Run/*`）：
    - [`Backtest/RunCreate.md`](Backtest/RunCreate.md) — `POST .../Run/Create`（创建并启动回测任务）
    - [`Backtest/RunList.md`](Backtest/RunList.md) — `GET .../Run/List`（分页查询任务）
    - [`Backtest/RunGet.md`](Backtest/RunGet.md) — `GET .../Run/Get`（查询任务详情）
    - [`Backtest/RunCancel.md`](Backtest/RunCancel.md) — `POST .../Run/Cancel`（取消任务）
    - [`Backtest/RunReport.md`](Backtest/RunReport.md) — `GET .../Run/Report`（查询回测报告）
    - [`Backtest/RunEquity.md`](Backtest/RunEquity.md) — `GET .../Run/Equity`（净值曲线数据）
    - [`Backtest/RunTrades.md`](Backtest/RunTrades.md) — `GET .../Run/Trades`（成交记录）
    - [`Backtest/RunCashflows.md`](Backtest/RunCashflows.md) — `GET .../Run/Cashflows`（资金流水明细）
    - [`Backtest/RunPositionLogs.md`](Backtest/RunPositionLogs.md) — `GET .../Run/PositionLogs`（持仓变化明细）
    - [`Backtest/RunEventTraces.md`](Backtest/RunEventTraces.md) — `GET .../Run/EventTraces`（事件追踪）
  - 回测环境管理（`/API/V1/Meta/Finv/Quant/Backtest/Environment/*`）：
    - [`Backtest/EnvironmentList.md`](Backtest/EnvironmentList.md) — `GET .../Environment/List`（分页查询环境）
    - [`Backtest/EnvironmentGet.md`](Backtest/EnvironmentGet.md) — `GET .../Environment/Get`（查询环境详情）
    - [`Backtest/EnvironmentSave.md`](Backtest/EnvironmentSave.md) — `POST .../Environment/Save`（新增/修改环境）
    - [`Backtest/EnvironmentToggle.md`](Backtest/EnvironmentToggle.md) — `POST .../Environment/Toggle`（切换回测开关）
    - [`Backtest/EnvironmentDelete.md`](Backtest/EnvironmentDelete.md) — `POST .../Environment/Delete`（删除环境）
  - 回测模板管理（`/API/V1/Meta/Finv/Quant/Backtest/Template/*`）：
    - [`Backtest/TemplateList.md`](Backtest/TemplateList.md) — `GET .../Template/List`（分页查询模板）
    - [`Backtest/TemplateGet.md`](Backtest/TemplateGet.md) — `GET .../Template/Get`（查询模板详情）
    - [`Backtest/TemplateSave.md`](Backtest/TemplateSave.md) — `POST .../Template/Save`（新增/修改模板）
    - [`Backtest/TemplateDelete.md`](Backtest/TemplateDelete.md) — `POST .../Template/Delete`（删除模板）
