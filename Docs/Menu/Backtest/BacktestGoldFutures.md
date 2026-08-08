# 黄金期货合约回测验证（BacktestGoldFutures）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/BacktestGoldFutures.md`
> 对应视图：`Web/src/views/BacktestGoldFuturesView.vue`
> 接口契约：见 [Docs/API/Backtest/](../../API/Backtest/)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 量化策略验证 → 黄金期货合约回测验证
- **菜单名称**：黄金期货合约回测验证
- **菜单 key**：`backtest-gold-futures`
- **菜单图标**：`mdi-chart-bell-curve`
- **URL 路由**：`/meta/finvquant/backtest/gold-futures`
- **对应视图组件**：`Web/src/views/BacktestGoldFuturesView.vue`

## 2. 业务功能概述

基于已导入的 GCMain 黄金期货主连 2018~2026 分钟行情，配置回测条件并启动回测、跟踪任务进度：

| 功能 | 说明 |
|------|------|
| 回测条件配置 | 选择策略 / 账户 / 环境 / 标的（默认 GCMain）/ 回测区间 / 周期 / 报告精度 |
| 限制条件覆盖 | 初始资金覆盖、每日最大成交笔数、限定交易时间点（hhmmss，逗号分隔） |
| 回测开关 | 开启/关闭（关闭时拒绝启动）；策略/账户/环境三层开关任一关闭同样拒绝 |
| 启动回测 | 点击「启动回测」创建任务（异步执行，返回任务号与状态） |
| 任务跟踪 | 按标的查看最近回测任务列表（状态 / 进度条 / 失败原因），RUNNING/PENDING 任务自动轮询进度且可一键取消，成功后跳转回测分析查看报告 |

## 3. 使用方法（操作流程）

```
进入菜单 → 选择交易策略（下拉，来自策略管理）→ 选择回测账户 → 选择回测环境（默认环境自动选中）
→ 确认标的（默认随策略带出 GCMain）→ 选择开始/结束日期（留空=行情全区间）
→ 选择数据周期与报告精度 → （可选）展开限制条件覆盖
→ 打开回测开关 → 点击「启动回测」→ 下方任务列表查看进度
→ 任务 SUCCEEDED 后点击「查看报告」跳转 /meta/finvquant/backtest/analysis
```

## 4. 处理逻辑

### 4.1 前端交互

- 页面加载时并行拉取策略（allowBacktest=1）、账户（allowBacktest=1）、环境（BACKTEST 且开关开启）下拉；
  默认选中 GCMain 相关策略、首个账户、默认环境（is_default='1'）。
- 选择策略后自动带出标的（secu_code）与默认周期（data_period）。
- 日期输入为 yyyy-mm-dd，提交时转换为 yyyymmdd 整数；留空则不传（服务端取行情最早/最晚日期）。

### 4.2 后端调用链路

| 步骤 | 接口 |
|------|------|
| 加载下拉选项 | `GET /API/V1/Meta/Finv/Quant/Backtest/Strategy/List`、`.../Account/List`、`.../Environment/List` |
| 启动回测 | `POST /API/V1/Meta/Finv/Quant/Backtest/Run/Create`（含 strategy_id/account_id/env_id/secu_code/区间/周期/精度/options） |
| 任务列表 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List?secuCode=GCMain`（RUNNING/PENDING 时每 5s 轮询） |
| 取消任务 | `POST /API/V1/Meta/Finv/Quant/Backtest/Run/Cancel`（RUNNING/PENDING 可取消） |

### 4.3 后端处理（Run/Create）

策略/账户/环境三层回测开关校验 → 标的与区间解析（缺省取策略 universe / 行情范围）→
写入任务（PENDING，含策略/账户/环境快照）→ 异步执行：
加载行情 → 聚合 → 回测引擎回放（环境交易时段过滤、tick_size 对齐、成本覆盖链 环境>任务>策略>账户）→
落库曲线/成交/链路追踪 → SUCCEEDED。

## 5. 注意事项

- **回测开关**：页面开关 + 策略/账户/环境的 allow_backtest 三层任一关闭都会拒绝启动；
- **数据量**：8 年分钟级回测约 48 万根 K 线，任务执行需要数秒~数十秒，请勿重复点击启动；
- **报告查看**：仅 SUCCEEDED 任务可查看报告；FAILED 任务在列表展示 error_message；
- **环境自适应**：切换环境会影响交易时段过滤（非时段信号将被拒绝）与 tick_size 价格对齐。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Strategy/List` | [StrategyList.md](../../API/Backtest/StrategyList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Account/List` | [AccountList.md](../../API/Backtest/AccountList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Environment/List` | [EnvironmentList.md](../../API/Backtest/EnvironmentList.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Run/Create` | [RunCreate.md](../../API/Backtest/RunCreate.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List` | [RunList.md](../../API/Backtest/RunList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Get` | [RunGet.md](../../API/Backtest/RunGet.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Run/Cancel` | [RunCancel.md](../../API/Backtest/RunCancel.md) |
