# 归档统计与预算（Stats）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 归档统计与预算」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Stats`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/StatsView.vue`
> 菜单 key：`dedup-export-stats`
> 数据表：`cdeu.finv_quote_cdeu_ingest_stats`、`_daily_budget`、`_scan_watermark`、`_quote_maturity_mon`
> 接口：`/Meta/Finv/Quote/DeDup/Stats/Page`、`/Meta/Finv/Quote/DeDup/Budget/Page`、`/Meta/Finv/Quote/DeDup/Budget/Override`、`/Meta/Finv/Quote/DeDup/Watermark/List`、`/Meta/Finv/Quote/DeDup/Watermark`、`/Meta/Finv/Quote/DeDup/Maturity/Refresh`
> 关联接口文档：[`Docs/API/QuoteDeDup/Stats.md`](../../../../API/QuoteDeDup/Stats.md)、[`Docs/API/QuoteDeDup/Budget.md`](../../../../API/QuoteDeDup/Budget.md)、[`Docs/API/QuoteDeDup/Watermark.md`](../../../../API/QuoteDeDup/Watermark.md)、[`Docs/API/QuoteDeDup/Maturity.md`](../../../../API/QuoteDeDup/Maturity.md)

## 1. 用途

四个 Tab：入口日统计、每日预算（可覆盖）、扫描水位（可重置）、成熟度（可重算）。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 入口日统计 | 按日查看入库行数与来源分布。 |
| 每日预算 | 查看预算并可覆盖指定日期。 |
| 扫描水位 | 查看各来源水位并可重置。 |
| 成熟度 | 按月查看并可触发重算。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Stats/Page` | 入口日统计 |
| `GET` | `/Meta/Finv/Quote/DeDup/Budget/Page` | 每日预算 |
| `PUT` | `/Meta/Finv/Quote/DeDup/Budget/Override` | 覆盖指定日期的预算 |
| `GET` | `/Meta/Finv/Quote/DeDup/Watermark/List` | 扫描水位列表 |
| `POST` | `/Meta/Finv/Quote/DeDup/Watermark` | 重置扫描水位 |
| `POST` | `/Meta/Finv/Quote/DeDup/Maturity/Refresh` | 重算成熟度 |

## 4. 说明

覆盖预算与重置水位是写操作，均需二次确认。
