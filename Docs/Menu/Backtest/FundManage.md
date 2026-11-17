# 资金管理（FundManage）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/FundManage.md`
> 对应视图：`Web/src/views/FundManageView.vue`
> 接口契约：见 [Docs/API/Backtest/RunList.md](../../API/Backtest/RunList.md) 与 [RunEquity.md](../../API/Backtest/RunEquity.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 资金管理
- **菜单名称**：资金管理
- **菜单 key**：`fund`
- **菜单图标**：`mdi-cash-multiple`
- **URL 路由**：`/meta/finvquant/fund`
- **对应视图组件**：`Web/src/views/FundManageView.vue`

## 2. 业务功能概述

选择已成功回测任务 → 查看现金与总资产曲线（ECharts，按报告精度）：

| 功能 | 说明 |
|------|------|
| 任务选择 | 下拉仅列出 SUCCEEDED 回测任务（任务号/策略/标的/区间） |
| 资金曲线 | 现金（cash）与总资产（equity=现金+持仓市值）双线展示 |

> 资金流水的结构化明细可通过 `Run/Cashflows` 接口获取（见「回测分析」菜单的 ⑨ 链路追踪页签）。

## 3. 使用方法

```
选择回测任务（下拉，仅列出 SUCCEEDED）→ 自动加载并渲染资金曲线
```

## 4. 处理逻辑

| 数据 | 接口 |
|------|------|
| 任务列表 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List?status=SUCCEEDED&page=1&pageSize=100` |
| 资金曲线 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Equity?runId=&page=1&pageSize=5000` |

- 资金曲线：`equity`（总资产=现金+持仓市值）与 `cash`（现金）双线；
- x 轴标签按报告精度生成（Day=日期，Min/Hour=日期+时间）。

## 5. 注意事项

- 仅展示 SUCCEEDED 任务；曲线精度与任务创建时的 report_precision 一致；
- 更完整的资金流水/持仓变化/事件追踪明细见「回测分析」菜单的 ⑨ 链路追踪页签。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List` | [RunList.md](../../API/Backtest/RunList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Equity` | [RunEquity.md](../../API/Backtest/RunEquity.md) |
