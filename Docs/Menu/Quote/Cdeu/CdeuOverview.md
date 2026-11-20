# CDEU 工作流控制面板（CdeuOverview）

> 所属：导航「行情 → 收剔导传ETL → CDEU自动化 → CDEU 工作流控制面板」
> 路由：`/Meta/Finv/Quote/Cdeu/CdeuOverview`
> 视图：`Web/src/views/Meta/Finv/Quote/Cdeu/CdeuOverviewView.vue`
> 菜单 key：`dedup-export-cdeu`
> 数据表：`cdeu.finv_quote_cdeu_workflow` / `_phase` / `_process` / `_job` / `_job_step` 定义表，`_workflow_run` 系列运行实例表
> 接口：`/Meta/Finv/Quote/DeDup/CDEU/Overview`、`/Meta/Finv/Quote/DeDup/Job/Run`
> 关联接口文档：[`Docs/API/QuoteDeDup/CDEUOverview.md`](../../../API/QuoteDeDup/CDEUOverview.md)、[`Docs/API/QuoteDeDup/JobControl.md`](../../../API/QuoteDeDup/JobControl.md)

## 1. 用途

以 Workflow → Phase → Process → JOB → Step 五级结构展示 CDEU 自动化流程的定义与最近运行状态，是跑批链路的总入口视图；可按 JOB 手动执行并回看到每次运行的状态。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 顶部筛选 | 按 Workflow / 运行状态过滤；可设置历史运行条数（`historyLimit`）。 |
| 流程树 | 按层级展开定义表结构，节点上带最近运行状态色标（运行中 / 成功 / 失败 / 未运行）。 |
| 手动执行 | 对可选 JOB 触发 `POST .../DeDup/Job/Run`，页面轮询状态直到结束。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/CDEU/Overview` | Workflow → Phase → Process → JOB → Step 拓扑与最近运行状态 |
| `POST` | `/Meta/Finv/Quote/DeDup/Job/Run` | 手动触发一次 JOB（异步） |

## 4. 说明

本页只读定义、只触发运行，不修改流程定义；定义表由迁移与运维脚本维护。
