# 文件导出（Job）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 文件导出」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Job`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/JobView.vue`
> 菜单 key：`dedup-export-job`
> 数据表：`cdeu.finv_quote_cdeu_export_job`
> 接口：`/Meta/Finv/Quote/DeDup/Job/List`、`/Meta/Finv/Quote/DeDup/Job/Retry`、`/Meta/Finv/Quote/DeDup/Job/Run`
> 关联接口文档：[`Docs/API/QuoteDeDup/JobList.md`](../../../../API/QuoteDeDup/JobList.md)、[`Docs/API/QuoteDeDup/JobRetry.md`](../../../../API/QuoteDeDup/JobRetry.md)、[`Docs/API/QuoteDeDup/JobControl.md`](../../../../API/QuoteDeDup/JobControl.md)

## 1. 用途

分页查看导出任务，按状态 / 证券 / 业务月过滤，对失败任务发起人工重试。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 筛选 | 状态、证券、业务月；切换即查询。 |
| 任务表 | 任务号、证券、业务月、状态、行数、文件、时间。 |
| 重试 | 仅失败任务可重试。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Job/List` | 分页查询导出任务（状态 / 证券 / 业务月过滤） |
| `POST` | `/Meta/Finv/Quote/DeDup/Job/Retry` | 人工重试失败任务 |
| `POST` | `/Meta/Finv/Quote/DeDup/Job/Run` | 手动触发导出 |

## 4. 说明

任务由调度与「归档监控总览」触发；本页主要用于核对与重试。
