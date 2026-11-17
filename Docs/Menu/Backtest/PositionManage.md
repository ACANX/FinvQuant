# 持仓管理（PositionManage）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/PositionManage.md`
> 对应视图：`Web/src/views/PositionManageView.vue`
> 接口契约：见 [Docs/API/Backtest/RunList.md](../../API/Backtest/RunList.md)、[RunEquity.md](../../API/Backtest/RunEquity.md) 与 [RunTrades.md](../../API/Backtest/RunTrades.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 持仓管理
- **菜单名称**：持仓管理
- **菜单 key**：`position`
- **菜单图标**：`mdi-briefcase-variant-outline`
- **URL 路由**：`/meta/finvquant/position`
- **对应视图组件**：`Web/src/views/PositionManageView.vue`

## 2. 业务功能概述

选择已成功回测任务 → 查看持仓数量与持仓市值双轴曲线 + 开平仓记录（最多 500 条）：

| 功能 | 说明 |
|------|------|
| 任务选择 | 下拉仅列出 SUCCEEDED 回测任务 |
| 持仓曲线 | 持仓市值（position_value）与持仓数量（position_qty）双轴曲线 |
| 开平仓记录 | 表格展示（时间/方向/价格/数量/金额/信号） |

## 3. 使用方法

```
选择回测任务（下拉，仅列出 SUCCEEDED）→ 自动加载并渲染持仓曲线与开平仓记录
```

## 4. 处理逻辑

| 数据 | 接口 |
|------|------|
| 任务列表 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List?status=SUCCEEDED&page=1&pageSize=100` |
| 持仓曲线 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Equity?runId=&page=1&pageSize=5000` |
| 开平仓记录 | `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Trades?runId=&page=1&pageSize=500` |

- 持仓曲线：`position_value`（持仓市值）与 `position_qty`（持仓数量）双轴；
- x 轴标签按报告精度生成（Day=日期，Min/Hour=日期+时间）。

## 5. 注意事项

- 仅展示 SUCCEEDED 任务；曲线精度与任务创建时的 report_precision 一致；
- 更完整的持仓变化（开/加/减/平 + 变动前后加权成本）明细见「回测分析」菜单的 ⑨ 链路追踪页签（`Run/PositionLogs`）。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/List` | [RunList.md](../../API/Backtest/RunList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Equity` | [RunEquity.md](../../API/Backtest/RunEquity.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Run/Trades` | [RunTrades.md](../../API/Backtest/RunTrades.md) |
