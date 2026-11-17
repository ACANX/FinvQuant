# 环境管理（Environment）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/EnvironmentManage.md`
> 对应视图：`Web/src/views/Meta/Finv/Quant/Backtest/EnvironmentView.vue`
> 接口契约：见 [Docs/API/Backtest/EnvironmentList.md](../../API/Backtest/EnvironmentList.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 环境管理
- **菜单名称**：环境管理
- **菜单 key**：`environment`
- **菜单图标**：`mdi-application-cog-outline`
- **URL 路由**：`/Meta/Finv/Quant/Environment`
- **对应视图组件**：`Web/src/views/Meta/Finv/Quant/Backtest/EnvironmentView.vue`

## 2. 业务功能概述

环境（回测/模拟盘/仿真/实盘 × 地区市场）的独立维护：

| 功能 | 说明 |
|------|------|
| 环境查询 | 按类型过滤分页展示（编码/名称/类型/地区/交易时段/tick_size/默认标志/回测开关） |
| 环境新增/编辑 | JSON 配置编辑：trading_sessions（交易时段）、trading_rules（T+N/涨跌停/合约乘数/tick_size）、cost、fill_mode、currency、preferences |
| 环境开关/删除 | 回测开关切换；已关联任务的禁止删除 |

## 3. 使用方法

```
选择类型过滤 → 「新建环境」/ 行内「编辑」→ 填写编码/名称/类型/地区 + 环境配置 JSON → 「保存」
```

## 4. 处理逻辑

- **成本覆盖链**：环境 cost > 任务 options > 策略 cost > 账户（引擎按此优先级解析）；
- **交易时段**：`trading_sessions` 为 hhmmss 数组，引擎逐 bar 校验，非时段信号登记拒绝事件「不在环境交易时段内」；
- **tick_size**：`trading_rules.tick_size > 0` 时成交价自动对齐到最小变动单位；
- **动态切换**：黄金期货回测验证页环境下拉（BACKTEST 类型）选择，任务保存环境快照（API 响应 `environment_snapshot`，DB 列 `env_snapshot`）保证可复现；
- 默认环境（is_default='1'）：任务未指定环境时自动回退到账户 env_id → 系统默认回测环境；
- 内置环境 user_id='system' 全局可见，列表返回 system + 当前用户环境。

## 5. 注意事项

- 环境配置 JSON 语法错误时前端本地拦截；trading_sessions 起止需 6 位 hhmmss；
- 环境被回测任务引用后禁止删除（可改禁用）；
- 修改环境配置只影响**之后**创建的任务（历史任务保留环境快照）。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Environment/List` | [EnvironmentList.md](../../API/Backtest/EnvironmentList.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Environment/Get` | [EnvironmentGet.md](../../API/Backtest/EnvironmentGet.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Environment/Save` | [EnvironmentSave.md](../../API/Backtest/EnvironmentSave.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Environment/Toggle` | [EnvironmentToggle.md](../../API/Backtest/EnvironmentToggle.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Environment/Delete` | [EnvironmentDelete.md](../../API/Backtest/EnvironmentDelete.md) |
