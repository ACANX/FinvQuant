# 端到端链路（Chain）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 端到端链路」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Chain`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/ChainView.vue`
> 菜单 key：`dedup-export-chain`
> 数据表：`cdeu.finv_quote_cdeu_job_run` / `_job_step_run` 等运行表 + `sys_config`（低峰窗口）
> 接口：`/Meta/Finv/Quote/DeDup/Status`、`/Meta/Finv/Quote/DeDup/Job/Runtime`、`/Meta/Finv/Quote/DeDup/Settings`、`/Meta/Finv/Quote/DeDup/Settings/Window`
> 关联接口文档：[`Docs/API/QuoteDeDup/Status.md`](../../../../API/QuoteDeDup/Status.md)、[`Docs/API/QuoteDeDup/JobRuntime.md`](../../../../API/QuoteDeDup/JobRuntime.md)、[`Docs/API/QuoteDeDup/Settings.md`](../../../../API/QuoteDeDup/Settings.md)

## 1. 用途

用数据流向图展示「主表 → 成熟度/预算 → 任务领取 → LOAD → EXPORT → 登记 → CLEANUP → 上传 → 网盘 → 清理」全链路，并给出总体视角与任务视角两种读法。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 总体视角 | 步骤、卡点与数据流；每个环节带当前状态。 |
| 任务视角 | 10 个 Job 的说明、状态与所在环节。 |
| 低峰窗口 | 查看并可调整调度窗口。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Status` | 业务指标 |
| `GET` | `/Meta/Finv/Quote/DeDup/Job/Runtime` | Job 阶段、进度与心跳 |
| `GET` | `/Meta/Finv/Quote/DeDup/Settings` | 低峰窗口等运行配置 |
| `POST` | `/Meta/Finv/Quote/DeDup/Settings/Window` | 调整低峰窗口 |

## 4. 说明

只看链路与状态，不直接改任务数据。
