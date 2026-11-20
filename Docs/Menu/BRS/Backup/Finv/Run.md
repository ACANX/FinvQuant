# 小表全量备份（Run）

> 所属：导航「备份与恢复 → 备份管理 → 小表全量备份」
> 路由：`/Meta/BRS/Backup/Finv/Run`
> 视图：`Web/src/views/Meta/BRS/Backup/Finv/BrsRunView.vue`
> 菜单 key：`brs-run`
> 数据表：`brs` 备份运行与产物表
> 接口：`/Meta/BRS/Backup/Finv/Run`
> 关联接口文档：[`Docs/API/BRS/Run.md`](../../../../API/BRS/Run.md)

## 1. 用途

查看并触发「小表全量备份」：展示每次运行的进度、产物与结果。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 运行列表 | 每次运行的时间、表数、产物大小、状态。 |
| 触发 | 同一时间只允许一个运行实例，运行中按钮置灰。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/BRS/Backup/Finv/Run` | 查询运行记录与状态 |
| `POST` | `/Meta/BRS/Backup/Finv/Run` | 触发一次全量备份 |

## 4. 说明

运行期间页面自动轮询状态；失败项可在列表中查看原因。
