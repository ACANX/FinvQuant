# 文件上传（File）

> 所属：导航「行情 → 收剔导传ETL → 归档 → 文件上传」
> 路由：`/Meta/Finv/Quote/DeDup/Export/File`
> 视图：`Web/src/views/Meta/Finv/Quote/DeDup/Export/FileView.vue`
> 菜单 key：`dedup-export-file`
> 数据表：`cdeu.finv_quote_cdeu_export_file`
> 接口：`/Meta/Finv/Quote/DeDup/File/List`、`/Meta/Finv/Quote/DeDup/File/Retry`
> 关联接口文档：[`Docs/API/QuoteDeDup/FileList.md`](../../../../API/QuoteDeDup/FileList.md)、[`Docs/API/QuoteDeDup/FileRetry.md`](../../../../API/QuoteDeDup/FileRetry.md)

## 1. 用途

分页查看导出产物文件与上传状态，对上传失败的正式文件发起重试。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 筛选 | 上传状态、文件类型、证券、业务月。 |
| 文件表 | 文件名、类型、大小、状态、重试次数、时间。 |
| 重试 | 仅上传失败可重试。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/Finv/Quote/DeDup/File/List` | 分页查询导出文件（上传状态 / 文件类型 / 证券 / 业务月过滤） |
| `POST` | `/Meta/Finv/Quote/DeDup/File/Retry` | 人工重试上传失败文件 |

## 4. 说明

上传目标仓库与凭据在「系统配置」与归档上传配置中维护，本页不涉及。
