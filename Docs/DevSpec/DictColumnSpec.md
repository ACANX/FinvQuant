# 字典表格列规范（DictColumnSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）所有**有字典 / 可枚举**的表格列（`v-table` / `v-data-table` 的单元格）。
> 关联：[`DictFilterSpec.md`](DictFilterSpec.md)（同一个字典在**筛选控件**里的口径：`中文[值]`）、[`TableTextSpec.md`](TableTextSpec.md)（长文本走 ⓘ 悬浮）、[`MenuSpec.md`](MenuSpec.md)（菜单与状态列）、[`UiSpec.md`](UiSpec.md)（状态色 / 开关配色）、[`DocSpec.md`](DocSpec.md)。
> 参考实现：`Web/src/views/Meta/Finv/Quote/Realtime/UscMappingView.vue`（市场 / 品种 / 动作 / 状态 / 差异类别列）、`Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue`（上传状态 / 文件类型列）、`Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue`（分片维度列）。

## 1. 核心口径

**表格列只显示字典的中文值，不带键。**

```text
状态列：  生效           （不是 生效[ACTIVE]）
市场列：  美股           （不是 美股[US]…… 也不写 US）
动作列：  取富途全码      （不是 取富途全码[FTMM]）
类别列：  新增           （不是 新增[insert]）
```

`中文[值]` 只用于**筛选控件**（见 [`DictFilterSpec.md`](DictFilterSpec.md)）与悬浮提示，不进入表格单元格。

## 2. 为什么

| 场景 | 用户在做的事 | 结论 |
|------|--------------|------|
| 表格 | 扫读、横向比较（找异常、看分布、对齐行） | 键没有信息量，只会占宽、干扰比较；中文已达意 |
| 筛选控件 | 做选择（确认自己选的是哪个枚举） | 需要键来对齐 API / 日志 / 数据库 |

表格的价值是「一眼扫完、横向比较」。多一截 `[ACTIVE]` 会让列变宽、行变挤，收益为零 —— 所以**表格里不带键**。

## 3. 适用范围

符合以下三条的表格列按本规范处理：

- 列值来自有限取值集合（数据库 `CHECK` 约束、Go 常量/枚举、后端字典接口）；
- 有稳定中文口径（字典表或后端下发）；
- 值本身是英文/数字标识（`ACTIVE`、`US`、`stock`、`insert`、`'1'/'0'`）。

不适用本规范：

| 类型 | 处理方式 | 参照 |
|------|----------|------|
| 长文本（说明、备注、出处、依据、错误信息） | ⓘ 图标 + 悬浮提示 | [`TableTextSpec.md`](TableTextSpec.md) |
| 实体名（证券中文名、策略名、账户名） | 直接展示原文 | — |
| 标识符（`rule_no`、`ftmm_symbol`、`usc`、`stock_id`） | 直接展示，不做翻译 | — |
| 数值 / 时间 / 金额 | 统一格式化函数（千分位、`YYYY-MM-DD HH:mm:ss`） | [`MenuSpec.md`](MenuSpec.md) |
| 布尔开关（`'1'`/`'0'`） | 中文 `启用`/`停用` 文案，或 `v-switch`（绿/灰） | [`UiSpec.md`](UiSpec.md) |

## 4. 渲染规则

### 4.1 只显示中文，未登记值原样显示

```ts
const STATUS_LABELS: Record<string, string> = {
  ACTIVE: '生效',
  PENDING: '待生效',
  RETIRED: '已停用',
}

// 空值给占位符；未登记值原样显示（便于发现后端新增了枚举）
function statusText(value?: string | null): string {
  if (!value) return '—'
  return STATUS_LABELS[value] ?? value
}
```

```vue
<td>{{ statusText(item.status) }}</td>
<td>{{ marketText(item.market) }}</td>
<td>{{ actionText(item.action) }}</td>
```

### 4.2 空值

统一显示 `—`（全角破折号）。不要显示空串、`null`、`undefined`、`-`（半角）、`N/A`。

### 4.3 未登记值

**原样显示**该值（`LABELS[v] ?? v`）。这既避免信息丢失，也是一条自检信号：
看到表格里冒出英文枚举，说明前端 `LABELS` 该补了 —— 不要用 `未知` 盖住。

### 4.4 状态列用 chip + 语义色

枚举型状态在表格里用 `v-chip`（`size="x-small" variant="tonal"`），颜色按语义固定，同一枚举跨页面同色：

| 语义 | 颜色 |
|------|------|
| 成功 / 生效 / 完成 | `success` |
| 待处理 / 进行中（需关注） | `warning` |
| 失败 / 冲突 | `error` |
| 停用 / 未变 / 中性 | `grey` |
| 无专用语义（如键表回写） | `info` 或 `primary` |

```vue
<v-chip :color="statusColor(item.status)" size="x-small" variant="tonal">
  {{ statusText(item.status) }}
</v-chip>
```

颜色细则与状态 ICON 约定见 [`MenuSpec.md`](MenuSpec.md) / [`UiSpec.md`](UiSpec.md)。

## 5. 确实需要看到「键」时

表格列本身不显示键。若排查需要：

| 做法 | 说明 |
|------|------|
| 单独一列「编码」 | 列头写清（`rule_no`、`status_code`），与中文列并存，不合并 |
| ⓘ 悬浮提示 | 浮层里可写 `中文[值]`，见 [`TableTextSpec.md`](TableTextSpec.md) |
| 详情弹窗 | 大段 JSON / 审计 before-after，用「查看」按钮开 `v-dialog`（支持复制） |

## 6. 代码组织约定

字典在表格侧固定两件套：

| 名称 | 用途 | 形态 |
|------|------|------|
| `XXX_LABELS` | 值 → 中文 | `{ ACTIVE: '生效', … }` |
| `xxxText()` | 单元格渲染 | `LABELS[v] ?? v`，空值返回 `—` |

筛选控件侧的 `XXX_OPTIONS`（`中文[值]`）见 [`DictFilterSpec.md`](DictFilterSpec.md)。两者可以并存，`XXX_LABELS` 常由 `XXX_OPTIONS` 派生：

```ts
const STATUS_OPTIONS = [
  { title: '生效[ACTIVE]', value: 'ACTIVE' },
  { title: '待生效[PENDING]', value: 'PENDING' },
  { title: '已停用[RETIRED]', value: 'RETIRED' },
]
const STATUS_LABELS: Record<string, string> = Object.fromEntries(
  STATUS_OPTIONS.map((o) => [o.value, o.title.slice(0, o.title.indexOf('['))]),
)
```

**警戒**：若下拉与表格共用同一份 items（例如 `modeItems`），item 必须同时带 `title` 与 `label`，
表格只读 `label`，绝不复用 `title`：

```ts
const modeItems = [
  { title: '按自然日[date]', label: '按自然日', value: 'date' },
]
// 下拉：:items="modeItems"（显示 title）
// 表格：modeItems.find((x) => x.value === v)?.label ?? v
```

## 7. 自检清单（评审用）

- [ ] 每个字典列的单元格都经过 `xxxText()`，没有裸输出英文枚举。
- [ ] 单元格里**没有**方括号键（`生效[ACTIVE]` / `新增[insert]` 只允许出现在筛选下拉与悬浮提示）。
- [ ] 空值显示 `—`；未登记值原样显示（不是 `未知`、不是空串）。
- [ ] 状态列用 `v-chip` + 固定语义色，同一枚举跨页面中文与颜色一致。
- [ ] 列宽不再随最长文本变化（字典列天然短；长文本已按 `TableTextSpec` 收进 ⓘ）。
- [ ] 前端常量有来源注释（表/约束 + 需求号），并与库中实际取值核对过。

## 8. 反例（真实事故）

| 反例 | 后果 |
|------|------|
| 单元格写 `{{ cn }}[{{ v }}]`（需求 11099 的过度套用，需求 11108 已修正） | 列变宽、行变挤、扫读变慢；键在表格里没有任何信息量 |
| 单元格复用下拉 `title`（`BrsSliceView` 分片维度列，需求 11108 已修正） | 表格显示 `按自然日[date]`；下拉与表格共用 items 时必须给它 `label` |
| 单元格裸输出 `{{ row.status }}`（`ACTIVE`） | 不认识英文的用户读不懂；中英混排破坏一致性 |
| 未登记值渲染成 `未知` / 空串 | 后端新增枚举后前端静默丢信息，排查困难 |
