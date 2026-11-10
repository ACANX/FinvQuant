# 字典筛选控件规范（DictFilterSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）所有**有字典 / 可枚举**的查询筛选控件（`v-select` / `v-autocomplete` / `v-btn-toggle` / 日期区间）。
> 关联：[`DictColumnSpec.md`](DictColumnSpec.md)（同一个字典在**表格列**里的口径：只显示中文）、[`UiSpec.md`](UiSpec.md)（组件与反馈统一）、[`MenuSpec.md`](MenuSpec.md)（菜单与状态列）、[`DocSpec.md`](DocSpec.md)。
> 参考实现：`Web/src/views/Meta/Finv/Quote/Realtime/UscMappingView.vue`（规则 / 市场 / 状态 / 类别筛选）、`Web/src/views/Meta/Finv/Quote/DeDup/Export/DeDupExportFileView.vue`（上传状态 / 文件类型）、`Web/src/views/Meta/BRS/Backup/Finv/BrsSliceView.vue`（分片维度）。

## 1. 核心口径

**筛选控件的每个选项都写成 `中文[值]`**：中文在前，原始枚举值放半角方括号里。

```text
全部状态
生效[ACTIVE]
待生效[PENDING]
已停用[RETIRED]
```

### 1.1 为什么要带键

筛选控件是**做选择**的地方，用户需要确认「我选的到底是哪个枚举」：

- 中文可能重名或近似（`生效` / `待生效`；`板块` / `大盘`）；
- 键能对上接口字段、日志、数据库与运维沟通口径（`ACTIVE` 而不是「那个生效的」）；
- 排查问题时不用再回查字典表。

> 注意：这个口径**只用于筛选控件**。同一个字典在**表格列**里只显示中文，见 [`DictColumnSpec.md`](DictColumnSpec.md)。

## 2. 适用范围

符合以下三条的控件按本规范处理：

- 取值集合有限且由程序定义（数据库 `CHECK` 约束、Go 常量/枚举、后端字典接口）；
- 有稳定中文口径（字典表或后端下发，例如 `public.finv_market.market_name`、`sys_dict`）；
- 值本身是英文/数字标识（`ACTIVE`、`US`、`stock`、`insert`）。

不适用本规范：

| 类型 | 处理方式 |
|------|----------|
| 自由文本（关键字、代码搜索） | 文本输入框，见 §5.2 |
| 日期 / 时间区间 | 日期选择器，label 写明区间含义 |
| 布尔开关 | `v-switch`（绿/灰），见 [`UiSpec.md`](UiSpec.md) |
| 实体选择（策略、账户、证券） | 选项为「名称 + 标识」，不是字典翻译，可写 `名称（标识）` |

## 3. 选项格式

- 中文在前、键在后，形态固定为 `中文[值]`，使用**半角方括号**；
- 键保持原始大小写：`ACTIVE` 不写 `active`，`US` 不写 `us`，`FTMM` 不写 `ftmm`；
- 中文里可以带括号补充，但不能省掉方括号里的键：
  - `沪市（上交所）[SH]`
  - `ETF / 交易所交易基金[etf]`
  - `沪市指数四位码[SH4]`
- 选项顺序按**业务优先级**排，不按字母序；高风险/停用态放最后（`已停用[RETIRED]`）。

## 4. 首项与空值语义

- 单选筛选的第一个选项固定为「**全部 X**」，值为**空串**（`''`），表示不加该条件；
- 空值语义必须写在选项文本里（`全部市场` / `全部状态` / `全部类别` / `全部规则`），不允许留空白的哑项；
- 后端参数为空即不过滤；前端不得把「全部」翻译成某个具体值提交（例如不要提交 `status=ALL`）。

## 5. 触发时机

### 5.1 下拉 / 单选 / 多选

选中即查：`@update:model-value="page = 1; load()"`，并同时把页码重置为 1。

### 5.2 文本输入

三个入口保持一致，不要让用户猜：

- 回车查询（`@keyup.enter`）；
- 「查询」按钮（`prepend-icon="mdi-magnify"`）；
- `clearable` + 清空即查（`@click:clear`）。

### 5.3 日期区间

选择完成即查；半开/闭区间、时区口径必须在 label 或页面「说明」卡片里写明（见 [`ExplainCardSpec.md`](ExplainCardSpec.md)）。

## 6. 选项来源与核对

- **优先用后端下发的字典**（字典表 / 枚举接口），避免前后端口径漂移；
- 固定集合允许写在前端常量里，但必须满足：
  - 变量名 `XXX_OPTIONS`；
  - 紧邻注释写明**来源**（哪张表 / 哪个 `CHECK` 约束）+ **需求号**；
  - 与数据库实际取值**核对过**（`SELECT DISTINCT 列 FROM 表`），不得凭记忆列枚举。

```ts
// 市场：来源 futu.finv_futu_secu_key.market 实际出现过的 16 种，
// 中文对齐 public.finv_market.market_name（需求 11101）
const MARKET_OPTIONS = [
  { title: '美股[US]', value: 'US' },
  { title: '沪市（上交所）[SH]', value: 'SH' },
  // …
]
```

## 7. 布局与一致性

- 筛选项顺序固定为「业务主键 → 状态 → 关键字」（例：`规则 → 市场 → 状态 → 关键字`），同类页面保持一致；
- 宽度：短枚举 `150`～`200px`；长枚举（如市场）`200`～`240px`；关键字输入 `220`～`320px`；
- 同类控件统一 `density="compact" variant="outlined" hide-details`，不混用 `filled` / `solo`；
- 「新增 / 批量操作」按钮放最右，用 `v-spacer` 隔开，不与筛选项混排。

## 8. 标准写法

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

<!-- 枚举来自常量：仍然 中文[值] -->
<v-select
  v-model="filterMarket"
  :items="[{ title: '全部市场', value: '' }, ...MARKET_OPTIONS]"
  label="市场"
  density="compact"
  variant="outlined"
  hide-details
  style="max-width: 200px"
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

## 9. 自检清单（评审用）

- [ ] 每个字典筛选控件的选项都是 `中文[值]`，键的大小写与代码/数据库一致。
- [ ] 首项是「全部 X」且值为空串；空值语义写清。
- [ ] 下拉选中即查并重置页码；文本筛选回车 / 按钮 / 清空三入口都生效。
- [ ] 前端常量形式的选项有来源注释（表/约束 + 需求号），并与库中实际取值核对过。
- [ ] 筛选项顺序、宽度、`density` / `variant` 与同类页面一致。
- [ ] 提交给后端的是原始值，不是中文，也不是 `ALL` 之类的自造值。

## 10. 反例

| 反例 | 后果 |
|------|------|
| 选项只写中文（`生效`） | 中文重名或近似时用户选错；排查时对不上 API/日志 |
| 选项只写键（`ACTIVE`） | 不认识英文的用户看不懂，学习成本高 |
| 首项写「全部」但值为 `ALL` | 后端需要额外适配，容易静默过滤成空结果 |
| 前端常量凭记忆列枚举 | 少项/多项，用户找不到要选的值 |
| 把中文当值提交（`status=生效`） | 后端校验失败或数据写脏 |
