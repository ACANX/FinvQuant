# 字典值展示规范（DictDisplaySpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）所有**有字典/可枚举**的查询筛选控件与表格列。
> 关联：[`UiSpec.md`](UiSpec.md)（组件与反馈统一）、[`MenuSpec.md`](MenuSpec.md)（菜单与状态列）、[`TableTextSpec.md`](TableTextSpec.md)（长文本走 ⓘ 悬浮）、[`ExplainCardSpec.md`](ExplainCardSpec.md)（整页说明卡片）、[`DocSpec.md`](DocSpec.md)。
> 参考实现：`Web/src/views/Meta/Finv/Quote/Realtime/UscMappingView.vue`（市场 / 品种 / 动作 / 状态 / 差异类别）、`Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue`（上传状态 / 文件类型）、`Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue`（分片维度）。

## 1. 核心口径（两条，一句话记住）

| 场景 | 展示格式 | 例子 |
|------|----------|------|
| **查询筛选控件**（下拉 / 单选 / 多选） | `中文[值]` | `生效[ACTIVE]`、`美股[US]`、`新增[insert]` |
| **表格列**（同一个字典） | **只显示中文** | `生效`、`美股`、`新增` |

补充三条铁律：

- **未登记的值原样显示**（`LABELS[v] ?? v`），不得渲染成空串、`未知`、`undefined`；
- **提交/请求一律用原始值**（`ACTIVE` / `US` / `insert`），不得把中文翻译当成值提交；
- **表格单元格不复用下拉的 `title`**（`title` 带键，复用就会把键带进表格）。

## 2. 为什么两种场景不一样

| 场景 | 用户在做的事 | 需要「键」吗 |
|------|--------------|--------------|
| 表格列 | **扫读、横向比较**（找异常、看分布） | 不需要。中文已达意，键只是占宽、干扰比较 |
| 下拉控件 | **做选择**（这一项到底是哪个枚举） | 需要。选项列表里带键，能对上 API 字段值、日志、数据库，也能区分中文重名的枚举 |

这条口径是**踩过两次坑**后定下的（见 §8），不要凭直觉改回去。

## 3. 什么算「字典 / 可枚举」

同时满足以下三条，按本规范处理：

- 取值集合**有限且由程序定义**：数据库 `CHECK` 约束、Go 常量/枚举、后端字典接口；
- 有**稳定的中文口径**（字典表或后端下发，例如 `public.finv_market.market_name`、`sys_dict`）；
- 值本身是英文/数字标识（`ACTIVE`、`US`、`stock`、`insert`）而不是给人读的中文。

**不适用本规范**（走对应规范或就地格式化）：

| 类型 | 处理方式 | 参照 |
|------|----------|------|
| 自由文本（说明、备注、出处、错误信息） | 长文本走 ⓘ 悬浮 | [`TableTextSpec.md`](TableTextSpec.md) |
| 实体名（证券中文名、策略名、账户名） | 直接展示 | — |
| 数值 / 时间 / 金额 | 统一格式化函数（千分位、`YYYY-MM-DD HH:mm:ss`） | [`MenuSpec.md`](MenuSpec.md) |
| 布尔开关（`'1'`/`'0'`） | 中文 `启用`/`停用` 或 `v-switch`（绿/灰） | [`UiSpec.md`](UiSpec.md) |

## 4. 查询筛选控件规范

### 4.1 选项格式

- 选项文本统一 `中文[值]`，中文在前、键放在方括号里（半角方括号）；
- 键保持**原样大小写**（`ACTIVE` 不写 `active`；`US` 不写 `us`）；
- 中文口径里允许出现括号补充说明，但**不能省掉方括号里的键**，例如
  `沪市（上交所）[SH]`、`ETF / 交易所交易基金[etf]`。

### 4.2 首项与空值语义

- 单选筛选的第一个选项固定为「**全部 X**」，值为空串（`''`），表示不加该条件；
- 空值语义必须在选项文本里写清（`全部市场` / `全部状态` / `全部类别`），不要只留一个空白项；
- 后端参数为空即「不过滤」，前端不得把 `全部` 翻译成某个具体值提交。

### 4.3 触发时机

| 控件类型 | 触发 | 说明 |
|----------|------|------|
| 下拉 / 单选 / 多选 | 选中即查（`@update:model-value`） | 并同时把页码重置为 1 |
| 文本输入（关键字） | 回车查 + 「查询」按钮 + `clearable` 清空即查 | 三入口一致，不要求用户猜 |
| 日期区间 | 选择完成即查 | 半开区间要在 label 或说明里写明 |

### 4.4 选项来源

- **优先用后端下发的字典**（字典表 / 枚举接口），避免前后端口径漂移；
- 固定集合允许写在前端常量里，但必须：
  - 变量名 `XXX_OPTIONS`，紧邻一段注释写明「来源：哪张表/哪个约束 + 需求号」；
  - 与数据库实际取值核对过（`SELECT DISTINCT col FROM tbl`），不做想当然的枚举。

### 4.5 布局与一致性

- 筛选项顺序固定为「业务主键 → 状态 → 关键字」（如 `规则 → 市场 → 状态 → 关键字`），同类页面保持一致；
- 宽度：短枚举 `150`～`200px`，长枚举（如市场）`200`～`240px`，关键字 `220`～`320px`；
- 同类控件样式统一 `density="compact" variant="outlined" hide-details`，不混用 `filled`/`solo`。

### 4.6 标准写法

```vue
<!-- 单选筛选：首项「全部 X」= 空值；选中即查 -->
<v-select
  v-model="filterStatus"
  :items="[
    { title: '全部状态', value: '' },
    { title: '生效[ACTIVE]', value: 'ACTIVE' },
    { title: '待生效[PENDING]', value: 'PENDING' },
    { title: '已停用[RETIRED]', value: 'RETIRED' },
  ]"
  label="状态"
  density="compact"
  variant="outlined"
  hide-details
  style="max-width: 170px"
  @update:model-value="page = 1; load()"
/>

<!-- 文本筛选：回车 / 按钮 / 清空 三个入口都要能查 -->
<v-text-field
  v-model="filterKeyword"
  label="代码 / 取值 / 说明"
  density="compact"
  variant="outlined"
  hide-details
  clearable
  prepend-inner-icon="mdi-magnify"
  style="max-width: 220px"
  @keyup.enter="page = 1; load()"
  @click:clear="page = 1; load()"
/>
```

## 5. 表格列规范

### 5.1 只显示中文

```ts
const STATUS_LABELS: Record<string, string> = { ACTIVE: '生效', PENDING: '待生效', RETIRED: '已停用' }

// 表格单元格：中文优先，未登记值原样显示；空值给占位符
function statusText(value?: string | null): string {
  if (!value) return '—'
  return STATUS_LABELS[value] ?? value
}
```

```vue
<td>{{ statusText(item.status) }}</td>
<td>{{ marketText(item.market) }}</td>
```

### 5.2 状态列用 chip + 语义色

枚举型状态在表格里用 `v-chip`（`size="x-small" variant="tonal"`），颜色按语义固定
（成功/运行中=绿色系、警告/待处理=橙色系、失败/停用=红色或灰色），对照 [`MenuSpec.md`](MenuSpec.md) 状态列规范。

```vue
<v-chip :color="statusColor(item.status)" size="x-small" variant="tonal">
  {{ statusText(item.status) }}
</v-chip>
```

### 5.3 空值与未登记值

- 空值：显示 `—`（全角破折号），不要显示空串或 `null`；
- 未登记值：原样显示，便于发现后端新增了枚举而前端还没跟上（此时应补 `LABELS`，而不是让它显示空白）。

### 5.4 需要看到「键」的场合

表格列本身不显示键。若排查时确实需要键：

- 单独一列「编码」显示键（列头写清，如 `rule_no`、`status_code`），或
- 放进该行的 ⓘ 悬浮提示里（格式 `中文[值]`，见 [`TableTextSpec.md`](TableTextSpec.md)），或
- 放进「详情」弹窗。

### 5.5 禁止事项

- 禁止 `{{ row.status }}` 直接输出英文枚举；
- 禁止 `{{ cn }}[{{ row.status }}]` 这类把键带进单元格的写法；
- 禁止同一个枚举在不同页面用不同中文口径（口径统一取自字典/常量，不要在页面里各写一份）。

## 6. 代码组织约定

一个字典在前端固定三段式，命名不要自创：

| 名称 | 用途 | 形态 |
|------|------|------|
| `XXX_OPTIONS` | 下拉 / 筛选控件的 `:items` | `[{ title: '生效[ACTIVE]', value: 'ACTIVE' }, …]` |
| `XXX_LABELS` | 表格单元格 / chip 文案 | `{ ACTIVE: '生效', … }` |
| `xxxText()` | 表格渲染函数 | `LABELS[v] ?? v`，空值返回 `—` |

```ts
// 市场：来源 futu.finv_futu_secu_key.market（16 种），中文对齐 public.finv_market.market_name；
// 键只在筛选下拉里出现，表格只显示中文（需求 11101 / 11108）
const MARKET_OPTIONS = [
  { title: '美股[US]', value: 'US' },
  { title: '沪市（上交所）[SH]', value: 'SH' },
  // …
]
const MARKET_LABELS: Record<string, string> = Object.fromEntries(
  MARKET_OPTIONS.map((o) => [o.value, o.title.slice(0, o.title.indexOf('['))]),
)
function marketText(value?: string | null): string {
  if (!value) return '—'
  return MARKET_LABELS[value] ?? value
}
```

**若下拉与表格共用一份 items**（例如 `modeItems`），item 必须同时带 `title` 与 `label`：

```ts
const modeItems = [
  { title: '按自然日[date]', label: '按自然日', value: 'date' },
]
// 下拉：:items="modeItems"（显示 title）
// 表格：modeItems.find((x) => x.value === v)?.label ?? v
```

## 7. 自检清单（评审用）

- [ ] 每个筛选下拉的选项都是 `中文[值]`，首项是「全部 X」且值为空串。
- [ ] 每个有字典的表格列都过翻译函数，没有裸输出英文枚举。
- [ ] 表格列里**没有**方括号键（`生效[ACTIVE]` 这类只允许出现在下拉与悬浮提示里）。
- [ ] 未登记值走 `?? v` 原样显示，空值显示 `—`。
- [ ] 状态列用 chip + 固定语义色，同一枚举颜色/中文口径跨页一致。
- [ ] 下拉与表格若共用 items，item 带 `label`，单元格不读 `title`。
- [ ] 提交给后端的永远是原始值，不是中文。
- [ ] 前端固定枚举常量有注释（来源表/约束 + 需求号）。

## 8. 反例（真实事故）

| 反例 | 后果 |
|------|------|
| 表格列写 `{{ cn }}[{{ v }}]`（需求 11099 的过度套用） | 表格变宽、扫读变慢；键在表格里没有信息量。已于需求 11108 改回 |
| 表格单元格复用下拉 `title`（`BrsSliceView` 分片维度列） | 单元格显示 `按自然日[date]`。已于需求 11108 改为读 `label` |
| 下拉选项只写中文（`生效`） | 用户无法确认选的是哪个枚举，遇到中文重名时选错 |
| 未登记值渲染成空 / `未知` | 后端新增枚举后前端静默丢信息，排查困难 |
| 把中文当值提交（`status=生效`） | 后端校验失败或数据被写脏 |
