# 归档主数据与规则（Config）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 归档主数据与规则」
> 路由：`/Meta/Finv/Quote/DeDup/Export/Config`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/ConfigView.vue`
> 菜单 key：`dedup-export-config`
> 数据表：`cdeu.finv_quote_cdeu_usc`（证券主数据与 `day_expect_rows`）、`cdeu.finv_quote_cdeu_priority`（优先级 / SLA）、`cdeu.finv_quote_cdeu_trade_calendar`（交易日历）
> 接口：`/Meta/Finv/Quote/DeDup/Usc/Page`、`/Meta/Finv/Quote/DeDup/Usc`、`/Meta/Finv/Quote/DeDup/Priority`、`/Meta/Finv/Quote/DeDup/Calendar/Page`、`/Meta/Finv/Quote/DeDup/Calendar`
> 关联接口文档：[`Docs/API/QuoteDeDup/Usc.md`](../../../../API/QuoteDeDup/Usc.md)、[`Docs/API/QuoteDeDup/Priority.md`](../../../../API/QuoteDeDup/Priority.md)、[`Docs/API/QuoteDeDup/Calendar.md`](../../../../API/QuoteDeDup/Calendar.md)

## 1. 用途

维护归档链路的证券主数据（关键参数 `day_expect_rows`）、优先级 / SLA 与交易日历，均支持管理员编辑。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 证券主数据 | 按证券编码查询，可改关键参数。 |
| 优先级 / SLA | 按证券设置优先级与 SLA。 |
| 交易日历 | 维护各市场交易日。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/Usc/Page` | 证券主数据分页 |
| `PUT` | `/Meta/Finv/Quote/DeDup/Usc` | 更新单个证券的关键参数 |
| `PUT` | `/Meta/Finv/Quote/DeDup/Priority` | 更新优先级 / SLA |
| `GET` | `/Meta/Finv/Quote/DeDup/Calendar/Page` | 交易日历分页 |
| `PUT` | `/Meta/Finv/Quote/DeDup/Calendar` | 更新交易日历 |

## 4. 说明

三块都是写操作，保存前弹确认；口径变更会影响调度与预算计算。
