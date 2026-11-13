# 说明卡片规范（ExplainCardSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）全部业务页面的「静态文字说明」摆放方式。
> 关联：[`UiSpec.md`](UiSpec.md)（组件与反馈统一）、[`MenuSpec.md`](MenuSpec.md)（菜单与视图组织）、[`DocSpec.md`](DocSpec.md)（文档规范）。
> 参考实现：`Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue`（需求 11005 / 11006）。

## 1. 目的与结论

**页面的静态文字说明一律收进页面最底部一个独立的「说明」卡片**，不写在标题下方、不夹在表格前后、不散落在主体内容之间；表格与操作用户真正要看的东西保持干净、少干扰。

这是本项目的**默认风格**：新增页面按此写；改造页面时逐步迁移（见 §6），不做一次性全量改写。

一句话规则：

> 数据与操作在上，说明在下；说明只出现在「说明」卡片里，且一个页面只有一个。

## 2. 什么算「静态文字说明」

**进说明卡片的**（解释性、长期有效）：

- 数据表名/口径说明（「`public.finv_fxcer`：一行 = 一个货币对…」）；
- 数据来源、换算口径、单位与精度约定；
- 页面/表格的读法、图例（`*` 表示派生、`—` 表示无数据）；
- 业务规则、约束、注意事项（「在岸/离岸存在基差，不要混用」）；
- 与其它页面/接口的配合关系（「两种模式各自触发不同来源」）。

**不进说明卡片的**（运行时状态，仍留在主体内容区）：

| 类型 | 位置 | 组件 |
|------|------|------|
| 请求失败 / 业务错误 | 主体内容区顶部 | `v-alert type="error"` |
| 空数据提示（「还没有数据，请点立即更新」） | 主体内容区（表格上方） | `v-alert type="warning"` |
| 数据过期 / 降级 / 只读等运行状态 | 主体内容区顶部 | `v-alert type="warning" / "info"` 按需 |
| 一次性操作反馈（保存成功/失败） | 悬浮 toast | `showToast`（见 UiSpec） |
| 指标汇总（条数、时间等） | 主体内容区 | `v-chip` / 指标卡 |

判断标准：**会随运行状态变化的**留在上面；**写死在代码里的解释性文字**进说明卡片。

## 3. 位置与结构

- 位置：**页面最底部**，位于最后一张主内容卡片之后（页面有多张卡片时排在最后）。
- 数量：**一个页面只有一个**说明卡片；三段五段都塞进同一张卡片，用分节区分。
- 标题固定：`mdi-information-outline` + 文案「说明」（不再另起「备注」「提示」「文档」等标题）。

```vue
<template>
  <!-- 主体：标题 + 操作 + 状态告警 + 表格/图表 -->
  <v-card>…</v-card>

  <!-- 说明卡片：页面最底部、唯一、标题固定为「说明」 -->
  <v-card class="mt-4">
    <v-card-title class="d-flex align-center">
      <v-icon icon="mdi-information-outline" class="mr-2" />
      说明
    </v-card-title>
    <v-card-text>
      <div class="finv-note">
        <div class="finv-note-title">表口径</div>
        <div class="text-body-2 finv-note-body">…</div>
      </div>

      <v-divider class="my-3" />

      <div class="finv-note">
        <div class="finv-note-title">数据源与互补</div>
        <div class="text-body-2 finv-note-body">…</div>
      </div>
    </v-card-text>
  </v-card>
</template>
```

## 4. 分节样式（统一 token）

| 元素 | 约定 |
|------|------|
| 分节标题 | `font-size: 0.875rem`、`font-weight: 500`、`color: rgb(var(--v-theme-primary))`、`margin-bottom: 2px` |
| 分节容器 | `border-left: 3px solid rgba(var(--v-theme-primary), 0.6)`、`padding-left: 10px` |
| 正文 | `class="text-body-2"`，`opacity: 0.85`、`line-height: 1.6`（解释性小字允许 `text-caption`） |
| 段间分隔 | `v-divider` + `class="my-3"` |
| 代码/字段名 | 用 `<code>`，不裸写英文列名 |
| 强调 | `<strong>`，仅用于「不要混用」这类必须看见的约束，一段至多一处 |

样式类建议集中在页面 `<style scoped>` 中定义（参考实现里是 `.finv-fxcer-note` / `.finv-fxcer-note-title` / `.finv-fxcer-note-body`）。
若后续抽成全局组件或全局样式类，本节随之更新。

### 4.1 分节切分建议

- 一段一个主题：`表口径` / `数据源` / `读法` / `注意事项` / `与其它页面的关系` 等，标题用名词短语；
- 一段超过 3 行就拆成两条正文或再分一节；
- 只在特定模式/角色下有效的说明，用 `v-if` 保留（例：矩阵模式的「矩阵读法」在表格模式隐藏）。

## 5. 禁止写法（反例）

| 反例 | 问题 |
|------|------|
| 标题下方 `v-card-subtitle` 写数据表口径 | 与标题抢视线，用户每次进页面都被干扰 |
| 表格上方 `v-alert type="info"` 写固定说明 | 被误认为运行状态提示，与真正的失败/空表告警混淆 |
| 表格下方 `text-caption` 写读法/图例 | 与分页、统计贴在一起，容易被忽略且样式不统一 |
| 页面根容器前后各放一段说明 | 说明被拆散，用户需要上下找 |
| 多处重复同一句说明 | 改一处忘另一处，口径漂移 |

## 6. 存量页面迁移

- 新建页面：**必须**按本规范写。
- 存量页面：改造该页面时顺手迁移，不单独开批量改造；迁移时保持文本内容不变（仅调整位置与样式），避免与业务变更混在一起。
- 迁移前先确认该文字是否属于「静态说明」：属于运行时状态的一律留在主体内容区（见 §2）。

## 7. 核对方式（评审/自检）

对本仓库任意页面，用下列检查确认符合规范：

```bash
# 1) 说明卡片存在且唯一（每个页面至多 1 处）
grep -c '<v-icon icon="mdi-information-outline"' Web/src/views/<相关页面>.vue   # 期望 0 或 1
# 2) 标题下方不再承载说明文字
grep -n 'v-card-subtitle' Web/src/views/<相关页面>.vue                          # 期望仅用于副标题性信息或不存在
# 3) 固定的解释性文字不应出现在 info 提示里
grep -n 'type="info"' Web/src/views/<相关页面>.vue                              # 期望仅用于运行状态
```

人工判定点：

1. 进入页面，说明文字只在最底部一张卡片里，且标题为「说明」；
2. 该卡片之前的主内容区没有写死的解释性段落；
3. 失败/空表/过期等提示仍在主内容区，且会随状态出现或消失；
4. 说明卡片内分节标题样式一致（字号、字重、primary 色、左侧色条）、段间有分隔线。

## 8. 参考实现

`Web/src/views/Meta/Finv/Quote/Realtime/FxCerView.vue`：

- 主卡片：标题 + 模式切换 + 立即更新/刷新 + 状态告警 + 汇总 chip + 表格；
- 说明卡片：标题「说明」+ 三个分节（表口径 / 数据源与互补 / 矩阵读法，最后一段仅矩阵模式显示）。

对应需求：11005（三段说明统一移到表格下方独立卡片）、11006（加标题与分节样式）。
