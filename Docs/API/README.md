# FinvQuant 服务端 API 文档

> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）
> 服务端端口：**16001**；默认地址：`http://localhost:16001`

## 通用约定

### 登录认证（需求 10761）

除 `/Health/Live`、`/Health/Ready`、`/Version`、`/Meta/Finv/Auth/Login` 外的全部接口都要求登录：

```
Authorization: Bearer <token>
```

令牌由 `POST /Meta/Finv/Auth/Login` 签发，存 Redis，有效期取 `sys_config` 20012（默认 480 分钟）。缺失或失效统一返回 `401`「未登录或登录已过期」。详见 [`Auth/Auth.md`](Auth/Auth.md)。

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
| `1000-2999` | 平台、安全和依赖错误（如 `2006`） |
| `≥3000` | 项目自定义业务错误码；**分钟行情 API 使用 `211xxx` 具体码**（见 [ErrorCodeSpec](../DevSpec/ErrorCodeSpec.md)） |

> 错误码口径（DC-05）：需要区分具体错误时用 `211xxx`（如 `211101` 唯一键冲突）；其余/兜底错误复用原 HTTP 错误码（`400/404/409/413/422/503`，`500` 用例沿用 `2006`）。存量接口旧号 `4001/4004/4009` 已停用，分别收敛为 `400/404/211104`。
> 错误详情分级（DC-06）：默认只返回安全文案；`FINV_ERROR_DETAIL_MODE=admin` 且携带有效 `X-Debug-Token`（或来自内网 CIDR）时才返回白名单 `debug` 字段。

（详见 [Docs/DevSpec/ErrorCodeSpec.md](../DevSpec/ErrorCodeSpec.md)）

## 接口索引

- **接口总览（方法 / 路径 / 说明 / 详细文档 / 已使用位置）**：见 [APIs.md](APIs.md)。
- **详细接口文档**：
  - 登录认证（需求 10761）：
    - [`Auth/Auth.md`](Auth/Auth.md) — `POST /Meta/Finv/Auth/Login`（登录发令牌）、`POST /Meta/Finv/Auth/Logout`（吊销会话）、`GET /Meta/Finv/Auth/Profile`（当前用户）；含认证模型、401 口径与口令轮换方式
  - 分钟行情 K 线（`/API/V1/Meta/Finv/Quote/Cdeu/Min/*`，**一个接口一个文档**）：
    - [`Quote/Cdeu/Min/KLineAdd.md`](Quote/Cdeu/Min/KLineAdd.md) — `POST .../Min/Add`（单条插入）
    - [`Quote/Cdeu/Min/KLineBatchInsert.md`](Quote/Cdeu/Min/KLineBatchInsert.md) — `POST .../Min/Batch`（批量插入）
    - [`Quote/Cdeu/Min/KLineUpsert.md`](Quote/Cdeu/Min/KLineUpsert.md) — `POST .../Min/Upsert`（批量 upsert；**OpenAPI 3.1 文档样板**：人读 [.md](Quote/Cdeu/Min/KLineUpsert.md) + 机读 [.yml](Quote/Cdeu/Min/KLineUpsert.yml)，见 [ApiSpec §7.7](../DevSpec/ApiSpec.md)）
    - [`Quote/Cdeu/Min/KLinePage.md`](Quote/Cdeu/Min/KLinePage.md) — `GET .../Min/Page`（分页，对象数组）
    - [`Quote/Cdeu/Min/KLinePageUni.md`](Quote/Cdeu/Min/KLinePageUni.md) — `GET .../Min/PageUni`（分页，column/row；契约 [.yml](Quote/Cdeu/Min/KLinePageUni.yml)）
    - [`Quote/Cdeu/Min/KLineQuery.md`](Quote/Cdeu/Min/KLineQuery.md) — `POST .../Min/Query`（批量查询）
    - [`Quote/Cdeu/Min/KLineBatchPatch.md`](Quote/Cdeu/Min/KLineBatchPatch.md) — `PATCH .../Min/Batch`（批量增量更新）
    - [`Quote/Cdeu/Min/KLineBatchDelete.md`](Quote/Cdeu/Min/KLineBatchDelete.md) — `DELETE .../Min/Batch`（批量删除）
    - [`Quote/Cdeu/Min/KLineCount.md`](Quote/Cdeu/Min/KLineCount.md) — `GET .../Min/Count`（统计总数）
    - [`Quote/Cdeu/Min/KLineStats.md`](Quote/Cdeu/Min/KLineStats.md) — `GET .../Min/Stats`（汇总统计与可视化数据）
    - [`Quote/Cdeu/Min/KLineBounds.md`](Quote/Cdeu/Min/KLineBounds.md) — `GET .../Min/Bounds`（交易日期边界）
    - [`Quote/Cdeu/Min/KLineHealth.md`](Quote/Cdeu/Min/KLineHealth.md) — `GET /Health`（探活，含 `/healthz` 过渡别名）
  - **外部 OpenAPI**（`/OpenAPI/V1/*`，当前公开访问、暂不鉴权；API Key 后续支持）：
    - [`OpenAPI/Quote/Cdeu/Min/Upsert.md`](../OpenAPI/Quote/Cdeu/Min/Upsert.md) — `POST /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/Upsert`（批量 upsert；成功码默认 0；契约 [.yml](../OpenAPI/Quote/Cdeu/Min/Upsert.yml)）
    - [`OpenAPI/Quote/Cdeu/Min/PageUni.md`](../OpenAPI/Quote/Cdeu/Min/PageUni.md) — `GET /OpenAPI/V1/Meta/Finv/Quote/Cdeu/Min/PageUni`（分页，column/row；成功码默认 0；契约 [.yml](../OpenAPI/Quote/Cdeu/Min/PageUni.yml)）
  - [`HistoryQuote/ImportsUpload.md`](HistoryQuote/ImportsUpload.md) — `POST /API/V1/Meta/Finv/Quant/Quote/Import/Upload`（MVSV 历史行情导入）
  - [`HistoryQuote/HistoryQuote.md`](HistoryQuote/HistoryQuote.md) — `GET /API/V1/Meta/Finv/Quant/Quote/History/QuoteQuery`（历史行情查询）
  - [`Meta/MetaExchange.md`](Meta/MetaExchange.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Exchange/*`（交易所信息维护）
  - [`Meta/MetaMarket.md`](Meta/MetaMarket.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Market/*`（交易所下设市场信息维护）
  - [`Meta/MetaSecurity.md`](Meta/MetaSecurity.md) — `GET/POST /API/V1/Meta/Finv/Quant/Metadata/Security/*`（规范证券信息维护，含 `Options` 下拉与 `Lookup` 详情查询）
  - 证券行情月归档导出与上传（`/API/V1/Meta/Finv/Quote/DeDup/*`，**一个接口一个文档**）：
    - [`QuoteDeDup/Status.md`](QuoteDeDup/Status.md) — `GET .../DeDup/Status`（归档链路指标、运行参数与调度表）
    - [`QuoteDeDup/JobRuntime.md`](QuoteDeDup/JobRuntime.md) — `GET .../DeDup/Job/Runtime`（10 个 Job 运行快照：阶段 / 进度 / 心跳 / 最近结果）
    - [`QuoteDeDup/Settings.md`](QuoteDeDup/Settings.md) — `GET .../DeDup/Settings` + `PUT .../Settings/Window`（低峰窗口在线修改）
    - [`QuoteDeDup/CDEUOverview.md`](QuoteDeDup/CDEUOverview.md) — `GET .../DeDup/CDEU/Overview`（CDEU 工作流定义树 + 运行状态 + 历史运行）
    - [`QuoteDeDup/CDEUWorkflowRun.md`](QuoteDeDup/CDEUWorkflowRun.md) — `POST .../DeDup/CDEU/Workflow/Run`（手动启动完整 CDEU 工作流；兼容旧 `/Process/Run`）
    - [`QuoteDeDup/JobList.md`](QuoteDeDup/JobList.md) — `GET .../DeDup/Job/List`（分页查询导出任务）
    - [`QuoteDeDup/FileList.md`](QuoteDeDup/FileList.md) — `GET .../DeDup/File/List`（分页查询导出文件与上传状态）
    - [`QuoteDeDup/JobRetry.md`](QuoteDeDup/JobRetry.md) — `POST .../DeDup/Job/Retry`（重置任务为 `WAITING`）
    - [`QuoteDeDup/FileRetry.md`](QuoteDeDup/FileRetry.md) — `POST .../DeDup/File/Retry`（重置上传为 `PENDING_UPLOAD`）
    - [`QuoteDeDup/JobRun.md`](QuoteDeDup/JobRun.md) — `POST .../DeDup/Job/Run`（手动触发归档 Job，异步）
    - [`QuoteDeDup/Usc.md`](QuoteDeDup/Usc.md) — `GET .../Usc/Page` + `PUT .../Usc`（证券主数据查询与编辑）
    - [`QuoteDeDup/Priority.md`](QuoteDeDup/Priority.md) — `GET/PUT/DELETE .../Priority`（优先级 / SLA）
    - [`QuoteDeDup/Calendar.md`](QuoteDeDup/Calendar.md) — `GET .../Calendar/Page` + `PUT/DELETE .../Calendar`（交易日历）
    - [`QuoteDeDup/Maturity.md`](QuoteDeDup/Maturity.md) — `GET .../Maturity/Page` + `POST .../Maturity/Refresh`
    - [`QuoteDeDup/Stats.md`](QuoteDeDup/Stats.md) — `GET .../Stats/Page`（入口日统计）
    - [`QuoteDeDup/Budget.md`](QuoteDeDup/Budget.md) — `GET .../Budget/Page` + `PUT .../Budget/Override`
    - [`QuoteDeDup/Watermark.md`](QuoteDeDup/Watermark.md) — `GET .../Watermark/List` + `PUT .../Watermark`（重置）
    - [`QuoteDeDup/RunLog.md`](QuoteDeDup/RunLog.md) — `GET .../RunLog/Page`（Job 运行日志）+ `GET .../RunLog/Detail`（单次详细日志）
    - [`QuoteDeDup/UploadLog.md`](QuoteDeDup/UploadLog.md) — `GET .../UploadLog/Page`（上传审计）
    - [`QuoteDeDup/Patch.md`](QuoteDeDup/Patch.md) — `GET .../Patch/Page` + `POST .../Patch/Retry`
    - [`QuoteDeDup/JobControl.md`](QuoteDeDup/JobControl.md) — `POST .../Job/Pause|Cancel`（暂停 / 取消）
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
