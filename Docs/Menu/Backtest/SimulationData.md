# 仿真数据验证（SimulationData）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/SimulationData.md`
> 对应视图：`Web/src/views/PlaceholderView.vue`（占位，与模拟盘验证/实盘仿真验证/实盘交易共用视图组件）

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 仿真数据验证
- **菜单名称**：仿真数据验证
- **菜单 key**：`simulation-data`
- **菜单图标**：`mdi-database-sync-outline`
- **URL 路由**：`/meta/finvquant/simulation/data`
- **对应视图组件**：`Web/src/views/PlaceholderView.vue`
- **状态**：🚧 规划中（占位）

## 2. 业务功能概述（规划）

| 功能 | 规划说明 |
|------|----------|
| 仿真数据源配置与接入 | 仿真数据源管理与行情接入 |
| 行情数据质量校验 | 缺失 / 异常 / 跳变检测 |
| 真实 vs 仿真数据比对 | 仿真行情与真实行情一致性比对 |

> 上述能力与量化回测共用一套**环境模型**（`finv_quant_environment`，env_type 已包含
> PAPER / SIMULATION / LIVE）与策略模型，后续版本在现有架构上扩展。

## 3. 当前页面逻辑

占位页根据路由名展示对应模块的规划说明与功能清单（`PlaceholderView.vue` 内置 plans 映射），
不发起后端请求；页面含「规划中」标识，提示当前已落地能力与后续迭代方向。

## 4. 注意事项

- 本菜单当前为占位页，规划能力将在后续版本实现；
- 环境模型中的 SIMULATION 类型已可在「环境管理」中先行维护配置。

## 5. 使用的后端接口

当前占位页不调用后端接口；后续实现时按 [ApiSpec.md](../../DevSpec/ApiSpec.md) 补充文档并登记。
