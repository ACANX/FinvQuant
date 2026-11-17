# 策略管理（StrategyManage）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Backtest/StrategyManage.md`
> 对应视图：`Web/src/views/StrategyManageView.vue`
> 接口契约：见 [Docs/API/Backtest/StrategyList.md](../../API/Backtest/StrategyList.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 策略管理
- **菜单名称**：策略管理（结构化策略定义）
- **菜单 key**：`strategy`
- **菜单图标**：`mdi-sitemap-outline`
- **URL 路由**：`/meta/finvquant/strategy`
- **对应视图组件**：`Web/src/views/StrategyManageView.vue`

## 2. 业务功能概述

结构化回测策略定义管理：

| 功能 | 说明 |
|------|------|
| 查询展示 | 分页展示策略（编码/名称/类型/标的/周期/版本/回测开关） |
| 新建策略 | 选择模板（服务端 `Template/List` 内置/自定义策略模板，含双均线/RSI/布林带/MACD）或手写 JSON 定义，保存时关联 `template_id` |
| 编辑策略 | 修改基本信息与 JSON 定义（保存时服务端编译校验信号表达式 + 标识符交叉校验） |
| 回测开关 | 行内切换 allow_backtest（关闭后回测页不可选） |
| 删除 | 已关联回测任务的策略禁止删除 |

## 3. 使用方法

```
输入关键字 → 「查询」→ 列表分页浏览
→ 「新建策略」→ 填写编码/名称/周期/标的 → 选择模板或粘贴 JSON → 「保存」
→ 行内「编辑」调整 → 行内开关切换回测可用性 → 「删除」（无任务关联时）
```

## 4. 处理逻辑

- **策略定义模型**：`universe`（标的池）/ `data`（周期·撮合模式）/ `indicators`（指标管线）/
  `signals`（买卖信号表达式）/ `rules`（数量模式与限制）/ `risk`（风控）/ `cost`（成本覆盖），
  模型规范见 [BacktestStrategySpec.md](../../DevSpec/BacktestStrategySpec.md)；
- **表达式引擎**：支持 `cross_up/cross_down/ref/highest/lowest/abs` 与比较、AND/OR/NOT，保存时编译校验，语法错误立即返回；
- **保存**：`POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Save`（definition 为 JSON 对象，可携带 template_id 记录来源）；
- **模板**：优先从 `GET .../Template/List?templateType=STRATEGY` 加载（内置 TPL-STRAT-DUALMA/RSI/BOLL/MACD + 用户自定义），模板接口不可用时回退本地内置 4 模板；点击「载入模板」填充 JSON 编辑区。

## 5. 注意事项

- `strategy_code` 全局唯一；`strategy_type` 当前仅支持 `RULE_BASED`；
- 信号表达式中的指标 id 必须与 `indicators` 中声明的 id 一致，否则编译报错；
- JSON 编辑区语法错误时前端本地拦截，不发起保存请求。

## 6. 使用的后端接口索引

| 接口 | 接口文档 |
|------|----------|
| `GET /API/V1/Meta/Finv/Quant/Backtest/Strategy/List` | [StrategyList.md](../../API/Backtest/StrategyList.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Save` | [StrategySave.md](../../API/Backtest/StrategySave.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Toggle` | [StrategyToggle.md](../../API/Backtest/StrategyToggle.md) |
| `POST /API/V1/Meta/Finv/Quant/Backtest/Strategy/Delete` | [StrategyDelete.md](../../API/Backtest/StrategyDelete.md) |
| `GET /API/V1/Meta/Finv/Quant/Backtest/Template/List` | [TemplateList.md](../../API/Backtest/TemplateList.md) |
