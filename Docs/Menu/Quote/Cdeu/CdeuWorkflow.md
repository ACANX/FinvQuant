# 全流程拓扑（CdeuWorkflow）

> 所属：导航「行情 → 收剔导传ETL → CDEU自动化 → 全流程拓扑」
> 路由：`/Meta/Finv/Quote/Cdeu/CdeuWorkflow`
> 视图：`Web/src/views/Meta/Finv/Quote/Cdeu/CdeuWorkflowView.vue`
> 菜单 key：—（`menuConfig.ts` 未登记，仅注册路由）
> 数据表：同「CDEU 工作流控制面板」
> 接口：`/Meta/Finv/Quote/DeDup/CDEU/Overview`
> 关联接口文档：[`Docs/API/QuoteDeDup/CDEUOverview.md`](../../../API/QuoteDeDup/CDEUOverview.md)

## 1. 用途

把 CDEU 全流程画成拓扑图：阶段以虚线框展开为 Process 节点，并绘制跨阶段依赖边，用于确认阶段之间的先后与卡点位置。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 拓扑图 | 阶段框 → Process 节点 → 依赖边；节点带状态色标。 |
| 自动刷新 | 页面定时重新拉取拓扑与状态。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/CDEU/Overview` | 阶段拓扑与 Process 节点；页面取 `historyLimit=1` |

## 4. 说明

当前路由已注册但菜单未在 `menuConfig.ts` 登记，只能通过 URL 直接访问；如需上菜单须补登记（并同步 `Menus.md`）。
