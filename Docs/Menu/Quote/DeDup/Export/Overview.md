# 归档监控总览（Overview）

> 所属：导航「行情 → 收剔导传ETL → 文件导出 / 归档 → 归档监控总览」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Overview`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/OverviewView.vue`
> 菜单 key：`dedup-export-overview`
> 数据表：`cdeu.finv_quote_cdeu_export_job`、`cdeu.finv_quote_cdeu_export_file`、`cdeu.finv_quote_cdeu_daily_budget` 等聚合
> 接口：`/Meta/Finv/Quote/DeDup/Status`、`/Meta/Finv/Quote/DeDup/Job/Run`
> 关联接口文档：[`Docs/API/QuoteDeDup/Status.md`](../../../../API/QuoteDeDup/Status.md)、[`Docs/API/QuoteDeDup/JobControl.md`](../../../../API/QuoteDeDup/JobControl.md)

## 1. 用途

归档链路的监控总览：指标卡片、告警、运行参数与 Job 调度概览，用来快速判断当前链路是否健康。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 指标卡片 | 待导出 / 运行中 / 失败任务数、当日预算与已用、水位与成熟度摘要。 |
| 告警 | 异常任务与超窗提示。 |
| 手动执行 | 可触发一次 JOB 并轮询状态。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Status` | 归档链路指标、告警与运行参数聚合 |
| `POST` | `/Meta/Finv/Quote/DeDup/Job/Run` | 手动触发导出 JOB |

## 4. 说明

数据全部来自后端聚合接口，页面不直接读表。
