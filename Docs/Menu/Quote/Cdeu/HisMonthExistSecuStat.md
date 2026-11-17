# 历史月份存量数据清点（HisMonthExistSecuStat）

> 所属：导航「行情 → 收剔导传ETL → CDEU自动化 → 历史月份存量数据清点」
> 路由：`/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat`
> 视图：`Web/src/views/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStatView.vue`
> 菜单 key：`cdeu-his-month-exist-secu-stat`
> 数据表：`cdeu.finv_quote_secu_kline_min`（主表存量）、`cdeu.finv_quote_cdeu_quote_maturity_mon`（成熟度）、`cdeu.finv_quote_cdeu_export_job` / `_export_file`（导出状态与文件名）
> 接口：`/Meta/Finv/Quote/DeDup/Export/Stock/Page`
> 关联接口文档：[`Docs/API/QuoteDeDup/Stats.md`](../../../API/QuoteDeDup/Stats.md)

## 1. 用途

按「业务月份 + 证券」统计主表该月的存量数据，让运维在跑导出流程前先确认「这次会导出哪些证券、各多少行、文件大致多大」。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 月份选择 | 选业务月份（如 `202610`），可叠加证券关键字与状态筛选。 |
| 清点表 | 一行一证券：存量行数、成熟度、导出状态、已生成文件名、体积估算。 |
| 分页 | 服务端分页，支持按存量行数排序。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Export/Stock/Page` | 按业务月份 + 证券分页返回该月存量行数、成熟度、导出状态与已生成文件名 |

## 4. 说明

本页只读清点，不触发导出；导出由「文件导出」菜单负责。
