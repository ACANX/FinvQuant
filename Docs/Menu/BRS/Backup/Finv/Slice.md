# 大表增量备份（Slice）

> 所属：导航「备份与恢复 → 备份管理 → 大表增量备份」
> 路由：`/Meta/BRS/Backup/Finv/Slice`
> 视图：`Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue`
> 菜单 key：`brs-slice`
> 数据表：`brs` 增量切片与产物表
> 接口：`/Meta/BRS/Backup/Finv/Slice`
> 关联接口文档：[`Docs/API/BRS/Slice.md`](../../../../API/BRS/Slice.md)

## 1. 用途

查看并触发「大表增量备份」：按切片展示进度与产物。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 切片列表 | 按大表与切片区间展示进度、产物与状态。 |
| 触发 | 运行中按钮置灰；可指定切片范围重跑。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/BRS/Backup/Finv/Slice` | 查询切片与运行状态 |
| `POST` | `/Meta/BRS/Backup/Finv/Slice` | 触发一次增量备份 |

## 4. 说明

增量切片依赖上次水位，重置水位会影响增量范围。
