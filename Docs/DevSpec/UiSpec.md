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

## 3. 主题（深色 / 浅色）切换

- **主题机制**：由 Vuetify `useTheme()` 驱动（`light` / `dark` 两套内置主题），切换后全站生效，无需刷新。
- **切换入口**：右上角用户菜单 →「深色主题」开关（遵循 §1：开启 `green`、关闭 `grey`，`density="compact"` + `hide-details`）。
- **初始主题优先级**：`localStorage['finvquant.ui.theme']`（用户本地选择） > 浏览器 `prefers-color-scheme` > 深色默认（项目默认）。
- **持久化范围**：仅保存在**用户浏览器本地**，**不做服务端持久化**（待办，见 `Docs/RequirementList.md` 10811）。
- **新增页面/组件约束**（强制）：
  - 禁止硬编码深色背景与浅色文本，优先使用主题变量（`rgb(var(--v-theme-surface))`、`rgb(var(--v-theme-on-surface))`、`rgb(var(--v-theme-primary))` 等）；
  - Canvas 图表（ECharts）的颜色必须按 `theme.global.name` 取值，并在主题切换后重绘（参考 `HistoryQuoteQueryView.vue` 的 `chartPalette()` + `watch(theme.global.name)`、`DeDupKLineStatsView.vue` 的 `splitColor()` + 重绘）；
  - 登录页（`LoginView.vue`）保持独立深色视觉，不跟随主题。
- **回归自检**：切换主题后逐个页面检查表格斑马纹、图表坐标轴/图例/tooltip、输入框边框、页脚文字是否可读；发现硬编码颜色应在同一变更中改为主题变量。

## 4. 操作反馈提示（Toast / Snackbar）规范

**一次性操作结果必须用悬浮 Toast 展示，禁止用内嵌 `v-alert` 占位。** 内嵌提示会永久占据页面固定位置、把内容顶下去，且需要用户手动忽略（需求 10913）。

### 4.1 统一入口

| 项 | 约定 |
|------|------|
| 调用方式 | `import { showToast } from '<相对路径>/ui/toast'` 后 `showToast(文案, 类型?)` |
| 组件 | 全局单例 `Web/src/components/AppToast.vue`，在 `Web/src/App.vue` 挂载一次 |
| 类型 | `'success'`（默认）/ `'error'` / `'info'` / `'warning'` |
| 时长 | 默认 `TOAST_DEFAULT_TIMEOUT = 5000`（毫秒，5 秒自动消失）；传 `0` 表示不自动消失 |
| 位置 | 视口顶部居中悬浮（`location="top"`），**不占布局空间**、不顶动页面内容 |
| 交互 | 可点右侧关闭按钮立即关闭；多条消息 FIFO 逐条展示，不堆叠 |
| 空文案 | `showToast('')` 直接忽略（不产生空提示） |

标准写法：

```ts
import { showToast } from '../../../../../ui/toast' // 按文件层级推算

async function save(): Promise<void> {
  saving.value = true
  try {
    await apiPost('/xxx/Save', payload)
    showToast('保存成功：下一轮备份将按新配置执行')
  } catch (e) {
    showToast((e as Error).message, 'error')
  } finally {
    saving.value = false
  }
}
```

### 4.2 禁止事项

- ❌ 用 `const message = ref('')` + `<v-alert v-if="message">{{ message }}</v-alert>` 承载「保存成功」这类一次性结果（历史写法，已全部改造）；
- ❌ 页面内自建 `v-snackbar`（重复实现时长/队列/样式，统一用 `AppToast`）；
- ❌ 成功后写「操作成功」却不消失的固定提示条；
- ❌ 在 Toast 文案里放 HTML（按纯文本渲染，需要富文本请在页面内用状态区展示）。

### 4.3 内联 `v-alert` 的适用场景（仍需保留）

Toast 只负责「一次性反馈」，以下场景**必须**用内联提示或页面状态区：

| 场景 | 形式 | 原因 |
|------|------|------|
| 页面/表格**加载失败**（如 `load()` 的 catch） | `<v-alert v-if="error" type="error">` | 页面无数据时用户需要持续看到失败原因 |
| 持续状态（备份运行中、磁盘水位告警、备份目录不可用） | 状态区 / `v-alert` + 状态字段 | 是「状态」不是「消息」，随刷新更新 |
| 表单字段校验 | 字段下 `:rules` 提示 | 需与输入框就近对应 |
| 需要用户确认后自行关闭的长文本（错误详情、SQL 回显） | 可关闭的 `v-alert`（`dismissible`） | 内容长、需要复制或对照 |

判断口径：**「用户操作一次就该消失的」→ Toast；「用户需要一直看到的」→ 内联。**

### 4.4 自检

```bash
# 1) 不应再有承载一次性提示的 message ref
grep -rn "const message = ref" Web/src/views        # 期望 0 行

# 2) 不应再有页面内自建 snackbar / toast 实现
grep -rn "v-snackbar" Web/src/views                 # 期望 0 行（只有 App.vue 的 AppToast）

# 3) 类型检查
cd Web && npx vue-tsc -b

# 4) 人工回归：保存/启停/触发备份后提示应悬浮在顶部并在约 5 秒后自动消失，
#    消失后页面布局不应发生跳动
```

## 5. 表单取值归一（v-select / v-combobox）

**提交前必须把表单字段归一成字符串，不得直接 `.trim()`。** Vuetify 的 `v-combobox`（以及部分 `v-select` 配置）在用户自由输入或从 `{title,value}` 下拉选中时，`v-model` 可能写回**字符串或对象**两种形态，直接 `value.trim()` 会抛 `xxx.trim is not a function`（需求 10912 上线后实际踩过）。

### 5.1 标准写法

```ts
// 归一：字符串原样去空白；数组取首项；对象取 value/title；空值 → ''
function toText(value: unknown): string {
  if (typeof value === 'string') return value.trim()
  if (Array.isArray(value)) return value.length > 0 ? toText(value[0]) : ''
  if (value && typeof value === 'object') {
    const item = value as Record<string, unknown>
    return toText(item.value ?? item.title ?? '')
  }
  return value == null ? '' : String(value).trim()
}
```

### 5.2 约定

- 表单里**需要自由输入**的字段（厂商名、备注、路径等）优先用 `v-text-field`，避免 combobox 的双形态返回值；
- 必须用 `v-combobox`（可选可填）时，提交前一律经过 `toText()`；
- `v-text-field` / `v-switch` / `v-textarea` 的绑定值是确定的类型，可直接使用；
- `v-select` 配 `item-title`/`item-value` 时返回 `item-value`，但仍建议提交前归一，防止后续换成 combobox 时漏改。

参考实现：`Web/src/views/Meta/BRS/Backup/Finv/BrsOssConfigView.vue`（`toText()` + `provider` 用 `v-text-field`）。
