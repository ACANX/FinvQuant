# 回测任务删除管理（RunDeleteTasks）

> 所属：导航「量化交易 → 量化策略验证 → 回测任务删除管理」
> 路由：`/Meta/Finv/Quant/Backtest/RunDeleteTasks`
> 视图：`Web/src/views/Meta/Finv/Quant/Backtest/RunDeleteTasksView.vue`
> 菜单 key：—（`menuConfig.ts` 未登记，仅注册路由）
> 数据表：回测任务运行表（见 `Docs/API/Backtest/`）
> 接口：`/Meta/Finv/Quant/Backtest/Run/DeleteTask/Retry`
> 关联接口文档：[`Docs/API/Backtest/RunList.md`](../../../API/Backtest/RunList.md)

## 1. 用途

管理已标记删除的回测任务：查看删除状态并重试删除。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 任务列表 | 按状态查看待删除 / 删除失败任务。 |
| 重试 | 对失败项发起重试。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/Meta/Finv/Quant/Backtest/Run/DeleteTask/Retry` | 重试删除任务 |

## 4. 说明

当前路由已注册但菜单未在 `menuConfig.ts` 登记，只能通过 URL 访问。
