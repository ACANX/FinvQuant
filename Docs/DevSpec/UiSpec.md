# 前端 UI 组件规范（UiSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）Vuetify 组件的统一样式与使用约定。
> 目的：统一交互组件视觉与行为，保证页面风格一致、状态可辨识。

## 1. 开关组件（v-switch）规范

**开关组件是状态切换的核心交互**，开启/关闭状态必须有明确视觉区分，禁止使用默认主题色（开启/关闭区分度不足）。

### 1.1 状态色规范

- **开启（true）**：`color="green"`（绿色）
- **关闭（false）**：`color="grey"`（灰色）

### 1.2 标准写法

动态绑定 `:color`，按当前值切换绿/灰：

```vue
<!-- 表格列中的开关（只读展示 + 切换回调） -->
<v-switch
  :model-value="item.allow_backtest === '1'"
  density="compact"
  hide-details
  :color="item.allow_backtest === '1' ? 'green' : 'grey'"
  @update:model-value="toggle(item)"
/>

<!-- 表单中的开关（v-model 双向绑定） -->
<v-switch
  v-model="useInitialCapital"
  density="compact"
  hide-details
  :color="useInitialCapital ? 'green' : 'grey'"
/>
```

### 1.3 参考实现

- 账户管理表格「回测开关」列：`Web/src/views/Meta/Finv/Quant/Account/AccountManageView.vue`
- 环境管理表格「回测开关」列：`Web/src/views/Meta/Finv/Quant/Backtest/EnvironmentView.vue`
- 策略管理表格「回测开关」列：`Web/src/views/Meta/Finv/Quant/Strategy/StrategyManageView.vue`
- 黄金期货回测验证「限制条件」开关：`Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue`

### 1.4 注意

- 表格列中的开关用 `:model-value`（单向展示）+ `@update:model-value` 触发后端切换回调；
- 表单编辑中的开关用 `v-model` 双向绑定；
- 统一使用 `density="compact"` + `hide-details` 保持紧凑；
- 语义化状态（成功/失败/运行中等）优先用 ICON + 颜色区分（见 MenuSpec 状态列规范）。

## 2. 状态展示规范（ICON + 颜色）

任务/流程状态在表格中**禁止直接展示状态英文文本**，统一用 ICON + 颜色区分，hover 提示文本（见 MenuSpec.md）。

| 状态 | ICON | 颜色 |
|------|------|------|
| PENDING（待执行） | `mdi-clock-outline` | grey |
| RUNNING（执行中） | `mdi-loading`（旋转） | primary |
| SUCCEEDED（成功） | `mdi-check-circle` | green |
| FAILED（失败） | `mdi-close-circle` | red |
| CANCELLED（已取消） | `mdi-cancel` | orange |

参考实现：`Web/src/views/Meta/Finv/Quant/Backtest/BacktestGoldFuturesView.vue`（`statusMeta` / `statusIcon*` 辅助函数 + `.spin` 旋转动画）。
