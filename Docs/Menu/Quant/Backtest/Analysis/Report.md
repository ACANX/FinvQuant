# 投资策略回测收益分析报告（Report）

> 所属：导航「量化交易 → 量化策略验证 → 回测分析 → 投资策略回测收益分析报告」
> 路由：`/Meta/Finv/Quant/Backtest/Analysis/Report`
> 视图：`Web/src/views/Meta/Finv/Quant/Backtest/Analysis/ReportView.vue`
> 菜单 key：—（`menuConfig.ts` 未登记，仅注册路由）
> 数据表：回测运行、净值、成交、持仓日志等（见 `Docs/API/Backtest/`）
> 接口：`/Meta/Finv/Quant/Backtest/Analysis`
> 关联接口文档：[`Docs/API/Backtest/RunReport.md`](../../../../API/Backtest/RunReport.md)

## 1. 用途

以运行号为入口查看单次回测的收益分析报告：净值曲线、回撤、关键指标与分组统计。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 运行号 | 从「回测分析」带入或手填。 |
| 报告 | 净值 / 回撤曲线与指标卡片。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quant/Backtest/Analysis` | 按运行号取收益分析结果（净值 / 回撤 / 指标） |

## 4. 说明

路径为 `/Backtest/Analysis/Report`，属「回测分析」的下级页面；当前菜单未登记，只能通过 URL 或页面内跳转访问。
