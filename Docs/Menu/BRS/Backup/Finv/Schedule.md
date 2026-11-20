# 备份调度配置（Schedule）

> 所属：导航「备份与恢复 → 备份管理 → 备份调度配置」
> 路由：`/Meta/BRS/Backup/Finv/Schedule`
> 视图：`Web/src/views/Meta/BRS/Backup/Finv/BrsScheduleView.vue`
> 菜单 key：`brs-schedule`
> 数据表：备份调度配置（时间窗、并发、保留）
> 接口：`/Meta/BRS/Backup/Finv/Schedule`
> 关联接口文档：[`Docs/API/BRS/Schedule.md`](../../../../API/BRS/Schedule.md)

## 1. 用途

维护全量 / 增量备份的调度时间与并发等参数，保存后由调度器按新参数执行。

## 2. 页面构成

| 区域 | 说明 |
|---|---|
| 调度表 | 全量 / 增量各自的时间窗与频率。 |
| 参数 | 并发数、超时、产物保留天数。 |

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/Meta/BRS/Backup/Finv/Schedule` | 读取调度配置 |
| `POST` | `/Meta/BRS/Backup/Finv/Schedule` | 保存调度配置 |

## 4. 说明

时间窗与导出链路的低峰窗口应错开，避免互相抢占资源。
