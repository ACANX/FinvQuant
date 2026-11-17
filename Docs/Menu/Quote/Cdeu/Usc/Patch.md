# 过时USC映射到新USC（Patch）

> 所属：导航「行情 → 收剔导传ETL → CDEU自动化 → 过时USC映射到新USC」
> 路由：`/Meta/Finv/Quote/Cdeu/Usc/Patch`
> 视图：`Web/src/views/Meta/Finv/Quote/Cdeu/Usc/PatchView.vue`
> 菜单 key：`cdeu-usc-patch`
> 数据表：`cdeu.finv_quote_secu_kline_min`、`cdeu.finv_quote_cdeu_quote_maturity_mon`、`archive.finv_quote_cdeu_quote_maturity_mon_purge_*`
> 接口：`/Meta/Finv/Quote/Cdeu/Usc/Patch`
> 关联接口文档：[`Docs/API/Cdeu/UscPatch.md`](../../../../API/Cdeu/UscPatch.md)

## 1. 用途

当证券编码口径调整（旧 usc → 新 usc）时，把主表与成熟度表里的旧编码批量迁移到新编码，避免同一证券出现两种写法。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 映射输入 | 按行填写「旧 usc → 新 usc」，可一次提交多条。 |
| 预演 / 应用 | 先预演受影响行数，确认后应用。 |
| 结果 | 返回主表迁移行数、成熟度合并/删除行数、备份表行数。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/Meta/Finv/Quote/Cdeu/Usc/Patch` | 按映射清单把主表里的旧 usc 改到新 usc，并合并成熟度表、删除前整行备份 |

## 4. 说明

删除成熟度旧记录前整行备份到 `archive` 表；操作幂等，重复提交相同映射不再产生变化。
