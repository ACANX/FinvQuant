# 备份登记维护（Table）

> 所属：导航「备份与恢复 → 备份管理 → 备份登记维护」
> 路由：`/Meta/BRS/Backup/Finv/Table`
> 视图：`Web/src/views/Meta/BRS/Backup/Finv/BrsBackupTableView.vue`
> 菜单 key：`brs-table`
> 数据表：`brs.brs_bak_rdb_table_full` / `brs.brs_bak_rdb_table_incr`（备份登记表）
> 接口：`/Meta/BRS/Backup/Finv/Table`
> 关联接口文档：[`Docs/API/BRS/BackupTable.md`](../../../../API/BRS/BackupTable.md)

## 1. 用途

维护纳入备份的表清单：库表、备份类型（全量 / 增量）、优先级与启用状态。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 筛选 | 库名、表名、启用状态。 |
| 登记表 | 一行一表：库表、类型、优先级、最近备份时间、启用状态。 |
| 编辑 | 弹窗编辑后保存，保存即生效（下一次调度按新清单执行）。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/BRS/Backup/Finv/Table` | 分页查询备份登记行 |
| `POST` | `/Meta/BRS/Backup/Finv/Table` | 新增 / 更新 / 启停登记行 |

## 4. 说明

登记变更只影响后续备份任务，不改动已有产物。
