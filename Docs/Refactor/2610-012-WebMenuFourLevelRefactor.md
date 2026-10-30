# Web 左侧菜单树四级化改造方案（v1.0，已实施）

> 所属：FinvQuant · 存放：`Docs/Refactor/`
> 状态：**已实施（2026-10-29）**
> 体系定位：**本项目 `FinvQuant` 就是 WebCMS 一级菜单中的「金融投资」分系统**（见 §2.5）
> 日期：2026-10-29
> 需求编号：`10803`–`10807`（已登记于 `Docs/RequirementList.md`）
> 参考实现：`D:\Code\JavaCode\Deamon\Web\WebCMS`（`src/config/menuConfig.js`、`src/components/TreeMenuItem.vue`、`src/App.vue`）
> 关联规范：[`MenuSpec.md`](../DevSpec/MenuSpec.md)、[`DocSpec.md`](../DevSpec/DocSpec.md)、[`FileNamingSpec.md`](../DevSpec/FileNamingSpec.md)

---

## 实施记录（2026-10-29）

| 项 | 实际实现 |
|---|---|
| 菜单配置 | 新增 `Web/src/config/menuConfig.ts`：2 个系统（`finvquant` / `cms`）、30 个叶子（29 业务 + 1 占位）、7 个分组；含 `resolveSystem` / `findMenuTrail` / `findFirstLeaf` / `trailToOpenKeys` |
| UI 开关 | `Web/src/config/uiConfig.ts`：仅保留 localStorage 读写工具与键常量；视觉固定为 WebCMS 风格（无开关、无 URL 参数，2026-10-29 定稿） |
| 菜单组件 | 新增 `Web/src/components/menu/`：`SystemSwitcher.vue`、`NavMenuTree.vue`、`NavMenuTreeNode.vue`（递归） |
| `App.vue` | 移除内联 `menuItems` 与两层 `v-list-group` 模板，改为 `<NavMenuTree>`；顶栏加系统切换器（登录用户菜单保持在最右）；抽屉宽度 300px；`v-main` 顶部加面包屑（webcms） |
| 路由 | 新增 `/Meta/Finv`（redirect `/Dashboard`）、`/Meta/CMS`（`name: 'cms-home'`，`PlaceholderView`）；其余路由零改动 |
| 占位页 | `PlaceholderView.vue` 的 `plans` 新增 `cms-home` 文案（唯一视图改动） |
| 文档 | `Menus.md` 重写（四级树 + 清单加「所属系统」列 + 仪表盘路由笔误修正）、`MenuSpec.md` §1/§1.2、`Docs/Menu/CMS/CMSHome.md` 新增、`CHANGELOG`、`RequirementList` 10803–10807 |
| 验证 | `cd Web && npm run build`（vue-tsc + vite）通过；菜单 31 条路径全部命中 `router.ts`；叶子 30 / 系统 2 / 最大层级 4；深链接链与 CMS 链正确 |

---

## 0. 摘要

- **现状**：`Web/src/App.vue` 内联硬编码 `menuItems`（29 个叶子），模板只支持「一级分组 → 二级分组 → 叶子」两级嵌套，菜单最大深度 3 级，无系统概念。
- **目标**：改为 **四级菜单树**，**第一级为系统级**（`金融投资`、`内容管理`），其下依次为二级、三级、四级菜单；现有三级叶子（如「去重分钟行情查询」）落到第四级。
- **形态（已裁决 Q1=B）**：一级系统放在**顶栏下拉切换器**中，切换系统后**左侧抽屉联动渲染该系统的二~四级菜单**；左侧菜单分组均可展开/折叠。「内容管理」本轮仅作为**缺省占位系统**（一个占位页），不实现业务功能。
- **体系定位（2026-10-29 补充）**：`FinvQuant` 即 WebCMS 一级菜单中的 **「金融投资」分系统**，因此顶栏显示名采用 CMS 口径——`金融投资`（内部 key 保持 `finvquant`）、`内容管理`（内部 key `cms`）；CMS 其余一级菜单（控制面板、元数据、系统管理、认证授权、研发运维、AGIS、任务调度）本轮不接入。
- **实现方式**：菜单数据外置为系统注册表（`Web/src/config/menuConfig.ts`）+ 递归菜单组件（参考 WebCMS 的 `menuConfig.js` + 递归 `TreeMenuItem.vue`），**路由路径与既有视图组件全部保持不变**。
- **视觉（2026-10-29 定稿）**：固定使用 WebCMS 风格（当前菜单项 3px 主色激活条 + 页脚 `© 2026 ACANX`）；风格开关、`?nav=` 参数、抽屉顶部系统标题块、面包屑均已按评审意见移除。
- **范围**：前端 `Web/` + 菜单相关文档；**不涉及后端、数据库、接口**。
- **审批状态（2026-10-29）**：Q1–Q11 与 R1–R7 **全部裁决完毕且均按推荐值采纳**（见 §10），方案 **v1.0 定稿并已实施**（实施记录见上）。

**实施情况：代码与文档已于 2026-10-29 落地，验证命令与结果见文首「实施记录」。**

---

## 1. 背景与目标

### 1.1 现状

| 项 | 现状 |
|---|---|
| 菜单数据位置 | `Web/src/App.vue` 第 10–111 行，内联 `MenuLeaf` / `MenuGroup` 类型与 `menuItems` 数组 |
| 菜单深度 | 支持两级分组嵌套：一级分组 → 二级分组 → 三级叶子（模板第 216–271 行硬编码两层 `v-list-group`） |
| 叶子数量 | 29 个（每个叶子对应 `Web/src/router.ts` 中一条独立路由） |
| 激活判断 | `isLeaf()` / `isActive()` / `isGroupOpen()`（第 147–160 行）递归判断当前 `route.path` |
| 抽屉宽度 | 260px（`Web/src/App.vue` 第 214 行） |
| 顶栏 | 标题固定为「FinvQuant 量化策略交易平台」+ 用户菜单，无系统切换 |
| 缩进 | 全局样式覆盖 `--list-indent-size: 4px`、`--list-prepend-gap: 10.67px`（第 288–298 行） |
| 视觉风格 | 分组展开用 `v-list-group`，激活仅背景高亮，无激活条、无抽屉标题、无面包屑 |
| 未登记菜单的路由 | 3 条二级页路由（`/Meta/Finv/Quote/DeDup/CDEUWorkflow`、`/Meta/Finv/Quant/Backtest/RunDeleteTasks`、`/Meta/Finv/Quant/Backtest/Analysis/Report`），由页面内跳转进入，不占菜单项 |

### 1.2 WebCMS 参考实现要点

| 项 | WebCMS 做法 |
|---|---|
| 菜单数据 | `src/config/menuConfig.js`：**四级**数据结构（`title` / `icon` / `path` / `subtitle` / `children`），一级为系统级（控制面板、元数据、系统管理、认证授权、内容管理、CMS、研发运维、AGIS、金融投资、任务调度…） |
| 一级切换 | `src/App.vue`：顶栏 `v-menu` 下拉切换一级系统，切换时跳到该系统「最近叶子」路径 |
| 左侧树 | `SideMenu.vue` → `TreeMenuItem.vue` **递归组件**：有 `children` 渲染可折叠节点（`mdi-chevron-down/right` + `v-show` 展开），无 `children` 渲染 `v-list-item`（`:to` 路由跳转） |
| 层级缩进 | 按 `depth` 递增：`depth-0` 12px、`depth-1` 32px、`depth-2` 52px、`depth-3` 72px（每级 +20px） |
| 视觉 | 叶子激活 = 背景高亮 + 左侧 3px 主色条；抽屉顶部有系统名标题区（`drawer-header`） |
| 展开状态 | 组件内 `ref(false)`，无路由联动、无持久化 |

### 1.3 目标与非目标

**目标**

1. 顶栏提供**一级系统切换器**（`金融投资`、`内容管理`），下拉选择后左侧菜单联动；
2. 左侧为**二~四级菜单树**（相对全局共四级：系统 → 二级 → 三级 → 四级），支持任意深度递归渲染；
3. 现有 29 个业务菜单一一映射到新层级（§2.1），**不新增业务菜单、不删除、不改路由**；
4. 「内容管理」作为**缺省占位系统**落地（一个占位菜单，路由可用，无业务功能）；
5. 保留并增强交互：路由激活高亮、分组自动展开、深链接自动展开祖先链、展开状态按系统持久化、移动端临时抽屉；
6. 新增 WebCMS 风格视觉（激活条 / 抽屉标题 / 面包屑），**可通过开关切回现有视觉**，默认开启新风格。

**非目标（本次不做）**

- 不改任何业务视图组件（`Web/src/views/**`）与业务逻辑；唯一例外是 `PlaceholderView.vue` 增加内容管理占位文案（见 §4.2）；
- 不改任何既有路由 `path`（深链接、书签、既有文档链接全部保持有效）；
- 不迁移历史小写路由（`/meta/exchange`、`/meta/market`、`/meta/security`、`/meta/import`），仅列入遗留项；
- 不实现内容管理的任何业务页面（仅占位）；
- 不做后端菜单动态化与权限驱动菜单过滤（后续另立方案）；
- 不引入新的第三方依赖。

---

## 2. 目标菜单模型

### 2.1 层级映射规则（已裁决 Q9=A）

**规则 R1**：改造后层级 = 现有层级 **+1**，一级固定为系统节点。

| 旧层级 | 新层级 | 举例 |
|---|---|---|
| —（新增） | 一级：系统 | `金融投资`、`内容管理` |
| 一级分组 | 二级分组 | 数据采集、历史行情、元数据管理（「量化策略验证」已于需求 10847 撤除，其叶子移入「量化 → 回测」） |
| 一级叶子 | 二级叶子 | 仪表盘、配置管理（回测分析 10847、仿真/模拟/实盘仿真验证 10848、实盘交易 10849、策略管理 10850 已移入「量化」下） |
| 四级叶子（需求 10846） | —（三级挂载） | 环境管理、模板管理、账户管理、资金管理、持仓管理（原二级叶子，现挂「量化 → MVP配置」下） |
| 二级分组 | 三级分组 | 行情数据去重、行情数据归档、业务元数据维护 |
| 二级叶子 | 三级叶子 | 历史行情查询（黄金期货合约回测验证已于 10847 移入「量化 → 回测」） |
| 三级叶子 | **四级叶子** | 去重分钟行情查询/统计、归档 8 项、元数据 4 项 |

**规则 R2**：树深度不强行拉平。允许某些分支只到二/三级（如「历史行情 → 历史行情查询」为二级分组 → 三级叶子），**不得**为了凑满四级而人为加分组层。

**规则 R3（系统归属判定）**：每条路由归属哪个系统，由系统的 `match` 前缀数组决定，而非单一路径前缀。原因：既有路由中 `/Dashboard` 与遗留 4 条 `/meta/*` 均不属于 `/Meta/Finv` 前缀。

```ts
// 系统归属判定（示意）
export interface MenuSystem extends MenuNode {
  match: string[]   // 命中任一前缀即属于该系统
}

// 金融投资（key=finvquant）：match = ['/Meta/Finv', '/Dashboard', '/meta/']
// 内容管理（key=cms）      ：match = ['/Meta/CMS']
```

### 2.2 目标菜单树（完整）

```
【一级系统：金融投资（内部 key=finvquant）】（顶栏切换，path=/Meta/Finv → 重定向 /Dashboard）
├─ 仪表盘（二级叶子，/Dashboard）
│  【顺序约定，需求 10840/10841/10842】仪表盘 → 元数据 → 配置管理 → 行情 → 量化 → 数据采集 → 其余
│  【量化（需求 10841/10843/10846）】二级分组，下列三级节点：
│      MVP配置（原占位，10846 起挂「环境管理/模板管理/账户管理/资金管理/持仓管理」5 个四级叶子）、
│      策略（10850 起挂「策略管理」1 个四级叶子）、
│      回测（10847 起挂「黄金期货合约回测验证」「回测分析」2 个四级叶子）、
│      验证（10848 起挂「仿真数据验证」「模拟盘验证」「实盘仿真验证」3 个四级叶子）、
│      交易（10849 建立时称「实盘」，10851 改名为「交易」；挂「实盘交易」1 个四级叶子）
├─ 数据采集（二级分组）
│  ├─ 行情数据去重（三级分组）
│  │  ├─ 去重分钟行情查询（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query）
│  │  └─ 去重分钟行情统计（四级叶子，/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats）
│  └─ 行情数据归档（三级分组）
│     ├─ 归档监控总览（四级叶子，/Meta/Finv/Quote/DeDup/Export/Overview）
│     ├─ 端到端链路（四级叶子，/Meta/Finv/Quote/DeDup/Export/Chain）
│     ├─ 导出任务（四级叶子，/Meta/Finv/Quote/DeDup/Export/Job）
│     ├─ 文件与上传（四级叶子，/Meta/Finv/Quote/DeDup/Export/File）
│     ├─ 归档统计与预算（四级叶子，/Meta/Finv/Quote/DeDup/Export/Stats）
│     ├─ 归档主数据与规则（四级叶子，/Meta/Finv/Quote/DeDup/Export/Config）
│     └─ 归档日志与补丁（四级叶子，/Meta/Finv/Quote/DeDup/Export/Log）
├─ 行情（二级分组，前缀 /Meta/Finv/Quote/，需求 10839）
│  ├─ 收剔导传ETL（三级分组，前缀 /Meta/Finv/Quote/Cdeu/）
│  │  ├─ 历史月份存量数据清点（四级叶子，/Meta/Finv/Quote/Cdeu/HisMonthExistSecuStat）
│  │  └─ CDEU 工作流控制面板（四级叶子，/Meta/Finv/Quote/Cdeu/CdeuOverview，需求 10845 改名并移入）
│  └─ 历史行情（三级分组，前缀 /Meta/Finv/Quote/His/）
│     ├─ 历史行情查询（四级叶子，/Meta/Finv/Quote/History/HistoryQuoteQuery）
│     └─ 历史行情数据导入（四级叶子，/meta/import，需求 10844 从元数据域移入）
├─ 元数据（二级分组，原名「元数据管理」，需求 10840 改名并移到「仪表盘」之后）
│  └─ Finv业务元数据（三级分组，原名「业务元数据维护」，需求 10852 改名）
│     ├─ 交易所信息管理（四级叶子，/meta/exchange，原名「交易所信息维护」，需求 10854 改名）
│     ├─ 市场信息管理（四级叶子，/meta/market，原名「交易所下设市场信息维护」，需求 10853 改名）
│     └─ 规范证券信息管理（四级叶子，/meta/security，原名「规范证券信息维护」，需求 10855 改名）
│        （历史行情数据导入已于需求 10844 移入「行情 → 历史行情」）
├─ 配置管理（二级叶子，/Meta/Finv/Quant/Config）

【一级系统：内容管理（内部 key=cms）】（顶栏切换，path=/Meta/CMS）
└─ 内容管理控制台（二级叶子，/Meta/CMS，占位页 PlaceholderView，标记「规划中」）
```

> 金融投资二级顺序沿用现有 `menuItems` 顺序，**不重排**；内容管理占位菜单位置与文案见 §2.4 与 R4。

### 2.3 节点数据模型（示意，批准后落地）

```ts
// Web/src/config/menuConfig.ts（文件名见 §10.1 Q8 裁决）
export interface MenuNode {
  key: string            // 同级唯一；展开状态用「父链 key」拼接（如 finvquant.data-collection.quote-dedup）
  title: string          // 简体中文标题
  subtitle?: string      // 可选副标题（已裁决保留字段，本轮不填内容，见 §3.9）
  icon?: string          // mdi 图标名；各层级均保留图标（已裁决 Q3=B）
  path?: string          // 叶子路由；分组节点不填
  children?: MenuNode[]  // 有 children 即分组节点
}

export interface MenuSystem extends MenuNode {
  icon: string
  path: string           // 系统落地路由
  match: string[]        // 系统归属前缀（见规则 R3）
  children: MenuNode[]
}

// 系统注册表：新增系统只在此数组追加一项，组件与 App.vue 无需改动
export const menuSystems: MenuSystem[] = [
  { key: 'finvquant', title: '金融投资', icon: 'mdi-finance', path: '/Meta/Finv',
    match: ['/Meta/Finv', '/Dashboard', '/meta/'], children: [/* §2.2 */] },
  { key: 'cms', title: '内容管理', icon: 'mdi-laptop', path: '/Meta/CMS',
    match: ['/Meta/CMS'], children: [/* 内容管理控制台占位 */] },
]

/** 叶子判定：有 path 且无子节点 */
export function isMenuLeaf(node: MenuNode): boolean
/** 按路由反查所属系统；未命中返回 menuSystems[0]（兜底） */
export function resolveSystem(path: string): MenuSystem
/** 返回从系统根到目标叶子的节点链（面包屑与祖先展开使用） */
export function findMenuTrail(path: string): MenuNode[]
/** 返回某分组下的第一个叶子路径（面包屑分组点击与系统落地使用） */
export function findFirstLeaf(node: MenuNode): string | undefined
```

**约定**

- 叶子节点的 `key` 与现有 `menuItems` 的 `key` **完全保持一致**（如 `dedup-kline-query`、`meta-exchange`），避免文档与索引二次改动；
- 新增系统/菜单的登记入口从 `App.vue` 迁移到 `Web/src/config/menuConfig.ts`，`MenuSpec.md` 第 1 节相应改口；
- 所有节点均须有 `key`（WebCMS 用 `path || title` 兜底，本项目改为强制 `key`，避免中文标题重复导致 key 冲突）；
- 分组节点即使有 `path` 也不作为叶子（本项目约定：**分组不填 path**，避免"点击分组既展开又跳转"的歧义）。

### 2.4 内容管理系统（内部 key `cms`，已裁决：本轮落地为占位）

| 项 | 方案 |
|---|---|
| 一级入口 | 顶栏系统切换器中出现「内容管理」（内部 key `cms`，图标 `mdi-laptop`） |
| 左侧菜单 | 仅一项「内容管理控制台」（二级叶子，`key=cms-home`） |
| 路由 | 新增 `{ path: '/Meta/CMS', name: 'cms-home', component: PlaceholderView, meta: { title: '内容管理控制台' } }` |
| 页面 | 复用 `Web/src/views/Common/PlaceholderView.vue`，在 `plans` 中新增 `cms-home` 一条说明（文案见 R4） |
| 文档 | 新增 `Docs/Menu/CMS/CMSHome.md`（MenuSpec §2.2「一个菜单一个文档」）+ `Menus.md` 登记 |
| 后续整合 | 内容管理实际业务菜单落地时，只在 `menuSystems[1].children` 追加节点，不动组件与路由骨架 |

> 选占位而非"空系统"的原因：空 `children` 的左侧抽屉会没有任何内容，用户会误判为加载失败；一个占位叶子能明确传达「系统已接入、业务待整合」。

### 2.5 与 WebCMS 一级菜单（系统级）的对应关系

WebCMS 的 `menuConfig.js` 一级菜单共 9 项；本项目按「一个一级菜单 = 一个系统」对齐，本轮只接入 2 项：

| WebCMS 一级菜单 | WebCMS 路径 | 本项目系统 key | 顶栏显示名 | 本轮状态 |
|---|---|---|---|---|
| 控制面板 | `/Admin/Dashboard` | — | — | 未接入 |
| 元数据 | `/Meta/Meta` | — | — | 未接入（本项目「元数据管理」业务菜单仍在金融投资下，见 R7） |
| 系统管理 | `/Meta/System` | — | — | 未接入 |
| 认证授权 | `/Meta/Auth` | — | — | 未接入 |
| 内容管理 | `/Meta/CMS` | `cms` | 内容管理 | 占位（本轮） |
| 研发运维 | `/Meta/DevOps` | — | — | 未接入 |
| AGIS | `/Meta/AGIS` | — | — | 未接入 |
| 金融投资 | `/Meta/Finv` | `finvquant` | 金融投资 | 已实现（本项目全部业务） |
| 任务调度 | `/Meta/Schedule` | — | — | 未接入 |

**命名约定**

- **内部 key 稳定不变**（`finvquant`、`cms`）：既有文档、需求清单与代码注释已大量引用，改 key 收益低、风险高；
- **显示名与 CMS 一级菜单完全一致**（`金融投资`、`内容管理`）：顶栏切换器、抽屉标题、面包屑首项均显示该名称，不暴露内部 key；
- **路径前缀对齐**：金融投资 = `/Meta/Finv`（现有业务路由已在该前缀下）、内容管理 = `/Meta/CMS`；
- 后续接入其它一级菜单时，只需在 `menuSystems` 追加一项（含 `match` 前缀），组件与 `App.vue` 零改动。

---

## 3. 交互设计

### 3.1 一级系统切换器（已裁决 Q1=B、Q6）

**位置**：顶栏 `v-app-bar`，紧跟平台标题之后、用户菜单之前。

```
[≡] FinvQuant 量化策略交易平台   [ 金融投资 ▾ ]           [ 用户 ▾ ]
```

| 项 | 设计 |
|---|---|
| 触发控件 | `v-menu` + `v-btn`（`variant="text"`，`prepend-icon` 为当前系统图标，`append-icon="mdi-menu-down"`，文本为当前系统 `title`） |
| 下拉内容 | `menuSystems` 全量；每项显示 `icon` + `title`（有 `subtitle` 时显示）；当前系统项 `active` + `disabled`（沿用 WebCMS 做法，避免重复切换） |
| 点击其他系统 | ① 切换当前系统；② 按 R2 策略决定是否跳转路由；③ 左侧菜单立即联动为新系统的二~四级；④ 恢复该系统上次的展开状态 |
| 点击当前系统 | 无动作（`disabled`） |
| 移动端（`< 960px`） | 隐藏按钮文字，仅显示当前系统图标 + chevron（`d-none d-sm-inline` 控制文本）；下拉宽度 220px |

**为什么不用"顶部 Tabs"**：系统数量会增长（后续可能加入更多系统），下拉在窄屏与多系统场景下更稳。

### 3.2 左侧树渲染范围与联动（已裁决 Q1=B）

- 左侧抽屉**只渲染当前系统**的 `children`（即二~四级节点），一级系统不重复出现在树内；
- 切换系统：整棵树替换，滚动条回到顶部，展开状态按系统分别恢复；
- 当前路由不属于任何系统（兜底）：按 `resolveSystem()` 回落到「金融投资」（`menuSystems[0]`），并在控制台输出一条 `warn` 便于排查配置遗漏；
- 左侧菜单层级可展开/折叠（Q6 已确认，见 §3.3）。

### 3.3 展开/折叠（已裁决 Q5、Q6）

- 分组节点（二级/三级）点击 → 切换展开；右侧 chevron `mdi-chevron-right` ↔ `mdi-chevron-down`（参考 WebCMS）；
- 展开内容用 `v-show`（保留 DOM，切换无重排抖动）；子容器 `padding-left` 按 `depth` 计算；
- **Q5「一级系统互斥展开」在本形态下的落法**：同一时刻只渲染一个系统的树（天然互斥），且**展开状态按系统隔离存储**——系统 A 的展开集合不会影响系统 B，切回 A 时恢复 A 的展开状态；
- 叶子节点点击 → `router.push(path)`；移动端同时关闭抽屉（沿用现有 `display.mobile` 逻辑）。

### 3.4 激活与祖先高亮

- 叶子激活：`route.path === node.path`（与现状一致，精确匹配）；
- 链路上祖先节点：子孙含激活叶子时标为"含激活项"，文字用主色（背景不变，避免视觉过重）；
- 激活叶子样式按视觉开关分两套（见 §3.7）：
  - `webcms`：背景高亮 + 左侧 3px 主色条（`border-left`）；
  - `classic`：仅背景高亮（现状）。

### 3.5 深链接自动展开

- 组件挂载/系统切换时：解析 `activePath`，把其**祖先链 key** 全部写入当前系统的展开集合；
- `watch(() => route.fullPath)`：路由变化时**追加**祖先链展开，不自动收起用户手动展开的其它分支；
- 直接访问深链接（如 `/Meta/Finv/Quote/DeDup/Export/Stats`）→ 系统判定为「金融投资」、二/三级分组自动展开、四级叶子高亮可见。

### 3.6 展开状态持久化（已裁决 Q4=A）

- 键名：`finvquant.nav.openKeys.v1`，值为按系统分组的对象：

```json
{
  "finvquant": ["finvquant.data-collection", "finvquant.data-collection.quote-archive"],
  "cms": ["cms"]
}
```

- 只保存**分组节点**的父链 key（叶子不需要展开状态）；
- 加载优先级：默认展开（激活链） → 并入 `localStorage` 内容 → 再并入当前激活链（保证激活项一定可见）；
- 异常容错：`JSON.parse` 失败、值非对象、数组元素非字符串 → 整体忽略并以激活链为准，不抛错、不阻塞渲染；
- 写入时机：`toggle` 后立即写入（节流非必需，菜单点击频率低）。

### 3.7 导航视觉（2026-10-29 定稿：固定 WebCMS 风格，不再提供开关）

**决策变更**：Q10 原方案为「classic / webcms 双视觉 + 开关」，经三轮评审后于 2026-10-29 定稿为
**固定使用 WebCMS 视觉**——移除界面开关、移除 `?nav=` 查询参数、移除 classic 分支代码。

| 视觉项 | 改造前（classic） | 现在（固定 WebCMS 风格） |
|---|---|---|
| 叶子激活 | 仅背景高亮 | 背景高亮 + **左侧 3px 主色激活条**（`inset` 阴影，无布局位移）+ 标题加粗 |
| 祖先分组标题 | 默认色 | 子孙含激活叶子时标题用主色 |
| 抽屉顶部系统标题 | 无 | **无**（与顶栏切换器重复，已移除） |
| 面包屑 | 无 | **无**（已移除，避免与顶栏系统名重复） |
| 页脚 | 无 | **有**：居中 `© 2026 ACANX`（内容短时贴底，登录页不显示） |
| 展开箭头 / 缩进 / 字号 | — | `mdi-chevron-down` 与 `mdi-chevron-right`；二级 12px 起每级 +16px；二级 14px、三/四级 13px |
| 抽屉宽度 | 260px | 300px |

**实现**：

- 激活条样式在 `Web/src/components/menu/NavMenuTreeNode.vue`（`.finv-nav-leaf--active` 恒生效）；
- `Web/src/config/uiConfig.ts` 只保留 localStorage 读写工具与两个键常量（展开状态、各系统上次路由），不再有视觉风格状态；
- 旧版本写入的 `localStorage['finvquant.ui.navVisualStyle']` 不再被读取，残留值无影响（可手动清除）。

**调整方式（改代码，不再有运行时开关）**

1. 需要回到改造前视觉：在 `NavMenuTreeNode.vue` 删除 `finv-nav-leaf--active` 的 class 绑定与对应样式；
2. 需要更换视觉细节（宽度/字号/激活色）：集中改 `NavMenuTreeNode.vue` 的样式与 `App.vue` 的 `:width`；
3. 需要页脚变更：改 `App.vue` 的 `<footer class="finv-page-footer">` 文案与样式。

### 3.7.1 与 WebCMS 视觉对比（待裁决 V1–V5）

对比基准：`D:\Code\JavaCode\Deamon\Web\WebCMS\src\components\SideMenuItem.vue`、`TreeMenuItem.vue`、`App.vue`（`drawer-header` / `side-drawer` / `main-menu-btn`）。

| 项 | WebCMS 实测值 | 本项目（webcms 风格） | 处理 |
|---|---|---|---|
| 抽屉宽度 | 280px | 300px | 待裁决 V1 |
| 层级缩进 | 12 / 28 / 44 / 60（每级 +16） | 12 / 28 / 44（每级 +16，无五级） | 一致 |
| 叶子标题字号 | 14px / 500 | 14px（二级）/ 13px（三、四级） | 待裁决 V2 |
| 副标题 | 12px / opacity 0.7，几乎每条都填 | 字段支持、本轮未填 | 待裁决 V5 |
| 激活叶子 | 背景 `rgba(25,118,210,.12)` + 左侧 `3px solid #1976D2` | Vuetify 默认激活背景 + `inset 3px` 主色 | 近似（V3） |
| 悬停背景 | `rgba(25,118,210,.08)` | Vuetify 默认 | 近似（V3） |
| 展开箭头 | `mdi-chevron-down/right`（append） | 同 | 一致 |
| 分组点击行为 | 仅展开/折叠 | 同 | 一致 |
| 抽屉顶部系统标题 | 有（`drawer-header`，灰底 + 16px 标题） | **已移除**（2026-10-29 按反馈，与顶栏切换器重复） | 有意不同 |
| 面包屑 | 无 | **无**（2026-10-29 已移除） | 一致 |
| 页脚 | 有（`© 2026 ACANX`） | **有**（2026-10-29 已补齐） | 一致 |
| 顶栏平台名 | `max-width: 220px` 截断 | 完整显示（仅 <900px 限宽） | 有意不同 |
| 一级切换 | 「最近叶子」跳转 | 记住各系统上次路由，无记录时落落地页 | 本项目更连续 |

**V1–V5 裁决结果（2026-10-29 已确认）**

| 编号 | 选项 | 裁决 | 当前实现 |
|---|---|---|---|
| V1 | 抽屉宽度 | **保持 300px**（不采用 WebCMS 的 280px） | 300px |
| V2 | 三/四级字号 | **保持 13px** | 13px |
| V3 | 激活/悬停背景 | **保持跟随主题色**（不对齐 WebCMS 蓝色） | 主题色 |
| V4 | 页脚 `© 2026 ACANX` | **补齐** | 已添加 |
| V5 | 29 条副标题 | **不补齐**（`subtitle` 保持可选字段） | 未填 |
| V6 | 面包屑 | **移除** | 已移除 |
| V7 | 视觉风格开关 | **移除开关，固定 WebCMS 视觉（默认开启）** | 已移除（无 UI 开关、无 `?nav=` 参数） |

对比方式：视觉已固定为 WebCMS 风格，不再需要切换；如需继续对照，可把 WebCMS 与本项目并排开两个窗口比较（差异项见上表）。

**当前效果**：左侧当前菜单项左侧固定显示 3px 主色竖条、标题加粗，祖先分组标题为主色；页面底部固定显示 `© 2026 ACANX`。

### 3.8 缩进、图标与宽度（已裁决 Q3=B：图标全保留）

| 项 | 现有 | 改造后（建议） |
|---|---|---|
| 抽屉宽度 | 260px | **300px**（四级带图标需要空间；小屏取 `min(300, 85vw)`，见 R5） |
| 每级缩进 | 全局覆盖约 4px | 二级 12px 起，每级 +16px（三级 28px / 四级 44px / 五级预留 60px） |
| 图标 | 全节点 `prepend-icon` | **全节点保留**（一级系统图标用于顶栏切换器与抽屉标题，二~四级沿用现有图标集） |
| 图标尺寸 | 默认 | 二~四级 `size="small"`（20px），减少宽度占用 |
| 展开箭头 | `v-list-group` 自带 | 自定义 `append` chevron（`size="small"`） |
| 字号 | 默认 | 二级 `text-body-1`；三/四级 `text-body-2`（13px） |
| 超长标题 | 换行 | 单行 `text-no-wrap` + `title` 原生提示（如「交易所下设市场信息维护」），避免四行菜单 |

### 3.9 副标题（已裁决 Q7=可选字段）

- `MenuNode.subtitle?: string` 字段**保留**，本轮配置**不填内容**（不造词）；
- 若希望本轮就补齐副标题文案，请在审核回复中直接给出（29 条中文短句），否则默认留空、后续按需补；
- 渲染规则：有 `subtitle` 时以 `v-list-item` 的 `subtitle` 展示（两行高度），无则单行（不影响现有密度）；
- 后续补齐文案只需改 `menuConfig.ts`，组件零改动。

### 3.10 移动端与响应式

- `< 960px`（`display.mobile`）：抽屉 `temporary`，点击叶子后自动关闭（沿用现有 `watch(route.fullPath)`）；
- 顶栏切换器在小屏收起文字（保留图标），避免与用户菜单挤压；
- 抽屉内树整体滚动（与 WebCMS 的 `overflow-y: auto` 一致），抽屉标题区不吸顶（避免小屏占高）；
- 不改动 `v-main` 布局与页面内 `v-container`。

### 3.11 无障碍与键盘

- 分组节点作为按钮：补 `aria-expanded`、`aria-controls`（指向子容器 id），保留 Vuetify 自带 `tabindex` 与回车/空格触发；
- 叶子节点保留 `:to`（渲染为 `<a>`），`aria-current="page"` 由 router-link 自动处理；
- 顶栏切换器按钮补 `aria-haspopup="menu"` 与 `aria-expanded`；
- 可访问名称使用 `title`（不额外加 `aria-label`，避免读屏重复）。

---

## 4. 代码改造清单

### 4.1 新增文件

| 文件 | 作用 | 预估规模 |
|---|---|---|
| `Web/src/config/menuConfig.ts` | 系统注册表（2 系统）+ 四级菜单（29 业务叶子 + 1 内容管理占位叶子 + 7 个分组）、类型、`isMenuLeaf` / `resolveSystem` / `findMenuTrail` / `findFirstLeaf` | 约 190 行 |
| `Web/src/config/uiConfig.ts` | 视觉风格开关（`classic` / `webcms`）、解析优先级、localStorage 键常量 | 约 45 行 |
| `Web/src/components/menu/SystemSwitcher.vue` | 顶栏一级系统切换器（`v-menu`） | 约 70 行 |
| `Web/src/components/menu/NavMenuTree.vue` | 树容器：展开集合、持久化、系统切换联动、移动端事件上抛 | 约 110 行 |
| `Web/src/components/menu/NavMenuTreeNode.vue` | 递归节点：分组折叠、叶子跳转、深度缩进、激活/祖先高亮、激活条 | 约 120 行 |

### 4.2 修改文件

| 文件 | 改动内容 |
|---|---|
| `Web/src/App.vue` | ① 删除 `MenuLeaf` / `MenuGroup` 类型、`menuItems` 数组（第 10–111 行）、`isLeaf` / `isActive` / `isGroupOpen`（第 147–160 行）；② `<v-navigation-drawer>` 内整段 `<v-list>` 模板（第 216–271 行）替换为 `<NavMenuTree … />`；③ 顶栏加入 `<SystemSwitcher />`（标题之后）；④ `v-main` 内新增页脚 `© 2026 ACANX`；⑤ `:width="260"` → `:width="300"`；⑥ 第 288–298 行缩进样式移入 `NavMenuTreeNode.vue` |
| `Web/src/router.ts` | ① 新增 `{ path: '/Meta/Finv', redirect: '/Dashboard' }`；② 新增 `/Meta/CMS`（`name: 'cms-home'`，`PlaceholderView`，`meta.title: '内容管理控制台'`）；③ 其余路由**零改动** |
| `Web/src/views/Common/PlaceholderView.vue` | 在 `plans` 中新增 `cms-home` 文案（唯一的视图文件改动，不含业务逻辑） |
| `Docs/DevSpec/MenuSpec.md` | §1 改为登记 `Web/src/config/menuConfig.ts`；新增「四级菜单与系统注册」条款（层级语义、一级系统、`match` 归属、key 规则、`subtitle` 可选、深度上限） |
| `Docs/Menu/Menus.md` | ① 重写「菜单层级结构」为「系统 → 二/三/四级」；② 清单表「层级」列改为 `一级系统`、`二级分组`、`二级叶子`、`三级分组`、`三级叶子`、`四级叶子`，并新增「所属系统」列（取值 `金融投资` / `内容管理`，与 CMS 一级菜单同名）；③ 修正仪表盘路由 `/dashboard` → `/Dashboard`（与代码不一致，现存笔误）；④ 登记内容管理占位菜单 |
| `Docs/Menu/CMS/CMSHome.md` | 新增（一个菜单一个文档），说明占位菜单入口、规划、后续整合方式 |
| `Docs/CHANGELOG.md` | `[未发布]` → `变更`：左侧菜单四级化 + 顶栏系统切换（金融投资 / 内容管理）+ 内容管理占位 + 视觉开关（说明导航结构变化，路由不变） |
| `Docs/RequirementList.md` | 追加需求条目（`10803` 起，建议 5 条：四级树、顶栏系统切换、内容管理占位、展开状态持久化、视觉开关），核对方式为命令 + 人工判定点 |

### 4.3 明确不改动

- `Web/src/views/**` 其余全部视图组件；
- 除新增 2 条外的所有路由 `path` / `name` / `meta`；
- `Web/src/api.ts`、`auth.ts`、`main.ts`、`vite.config.ts`；
- 后端 `cmd/`、`internal/`、`Deploy/`；
- 登录页与公共路由渲染分支（`route.meta.public`）。

### 4.4 关键实现示意（伪代码，非最终代码）

递归节点（`NavMenuTreeNode.vue`）：

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { MenuNode } from '../../config/menuConfig'
import { isMenuLeaf } from '../../config/menuConfig'

const props = defineProps<{
  node: MenuNode
  depth: number          // 0 = 二级（当前系统第一层子菜单）
  parentKey: string      // 父链 key，用于拼唯一展开键
  openKeys: Set<string>
  activePath: string
  style: 'classic' | 'webcms'
}>()
const emit = defineEmits<{ toggle: [key: string]; navigate: [] }>()

const nodeKey = computed(() => (props.parentKey ? `${props.parentKey}.${props.node.key}` : props.node.key))
const leaf = computed(() => isMenuLeaf(props.node))
const expanded = computed(() => props.openKeys.has(nodeKey.value))
const indent = computed(() => 12 + props.depth * 16)   // px

// 子孙含激活项（祖先高亮）
function containsActive(node: MenuNode, path: string): boolean { /* 递归 */ }
</script>
```

状态流（`NavMenuTree.vue`）：

```
props：system（当前系统）、activePath
初始：openKeys = localStorage[system.key] ∪ activePath 祖先链
toggle(nodeKey)：更新集合 → 写 localStorage（按系统分桶）
props.system 变化：保存旧系统集合 → 载入新系统集合 ∪ 新 activePath 祖先链
route 变化：并入新祖先链（不收起其它分支）
```

视觉（`uiConfig.ts`，无开关）：

```
固定 WebCMS 视觉：NavMenuTreeNode 恒应用 .finv-nav-leaf--active（激活条）+ 祖先主色
页脚：App.vue 的 <footer class="finv-page-footer">© 2026 ACANX</footer>
```

---

## 5. 路由与兼容性

| 关注点 | 结论 |
|---|---|
| 深链接 | 既有 URL 全部不变（仅新增 `/Meta/Finv` redirect 与 `/Meta/CMS` 占位） |
| 浏览器书签 | 不受影响 |
| 已发布文档中的路由引用 | 不受影响（`Docs/Menu/**`、`Docs/API/**` 中的 `path` 均不改） |
| `/Dashboard` 归属 | 通过 `match` 前缀归属「金融投资」（不在 `/Meta/Finv` 之下） |
| 遗留小写路由 `/meta/*` | 归属「金融投资」（`match` 含 `/meta/`），本次不迁移 |
| 系统切换后的路由 | 按 R2 策略（推荐：记住每个系统上次访问的路由，首次进入落到系统落地路由） |
| 现有 `LEGACY_PATH_MAP` | 不用改（`/login`、`/dashboard` 归一化逻辑保留） |
| 面包屑与页面布局 | `v-breadcrumbs` 插在 `v-main` 内、`v-container` 之上；`classic` 风格不渲染，页面结构零影响 |
| 与 CMS 一级菜单一致性 | 系统显示名与 CMS 同名（`金融投资` / `内容管理`），路径前缀一致（`/Meta/Finv`、`/Meta/CMS`）；CMS「金融投资」下的行情/数据/分析/量化四分组与本项目现有二级的差异见 R6 |

---

## 6. 文档同步清单（随代码同一提交）

1. `Docs/DevSpec/MenuSpec.md`：登记位置、四级层级语义、系统注册表与 `match` 约束、`subtitle` 可选、视觉开关说明；
2. `Docs/Menu/Menus.md`：层级树 + 清单表（层级列、所属系统列）+ 仪表盘路由笔误修正 + 内容管理占位登记；
3. `Docs/Menu/CMS/CMSHome.md`：新建（一个菜单一个文档）；
4. `Docs/CHANGELOG.md`：`[未发布] / 变更` 一条（含视觉开关与回退方式）；
5. `Docs/RequirementList.md`：新增需求条目（`10803` 起，只增不改）；
6. 本方案文档 `2610-012`：实施完成后在文首追加「实施记录」小节（状态改为已实施）。

---

## 7. 实施步骤（建议 5 个提交，可增量验收）

| 步骤 | 内容 | 验收点 |
|---|---|---|
| S1 | 新增 `menuConfig.ts`，把现有 `menuItems` 原样搬入（暂不改层级、暂不加内容管理），`App.vue` 改为引用 | 界面与改造前**完全一致**（纯重构，`npm run build` 通过） |
| S2 | 新增 `NavMenuTree.vue` + `NavMenuTreeNode.vue` 递归渲染，`App.vue` 删除硬编码模板；层级仍为三层 | 29 个叶子可达、激活正确、分组展开/收起正常 |
| S3 | 插入一级系统（金融投资 + 内容管理占位）+ `SystemSwitcher`，层级下移到四级；新增 `/Meta/Finv`、`/Meta/CMS` 路由；抽屉宽度/缩进调整；深链接自动展开 + 按系统持久化 | 顶栏切换联动、四级树形、深链接直开、移动端关闭抽屉均正常 |
| S4 | 新增页脚与激活条样式（WebCMS 视觉）；`uiConfig.ts` 仅保留 localStorage 工具 | 激活条与页脚生效；无风格开关 |
| S5 | 文档同步（§6） | `go test ./internal/docs` 通过、`Menus.md` 与 `menuConfig.ts` 一致 |

预估工作量：**1～1.5 人日**（含自测与文档；比 v0.1 增加内容管理占位、系统切换器、双视觉与面包屑）。

---

## 8. 验收标准

### 8.1 可执行校验

```bash
# 1) 前端构建（含 vue-tsc 类型检查）
cd Web && npm run build

# 2) 菜单叶子路径 ⊆ 路由表路径（两者差集应为空）
grep -oE "path: '[^']+'" Web/src/config/menuConfig.ts | sort -u > /tmp/menu.txt
grep -oE "path: '[^']+'" Web/src/router.ts        | sort -u > /tmp/router.txt
comm -23 /tmp/menu.txt /tmp/router.txt   # 期望无输出

# 3) 过程文档编号唯一（本机 WSL 若未装 Go，改在 Windows 侧执行）
go test ./internal/docs
```

### 8.2 人工验收清单

| 编号 | 检查项 | 判定标准 |
|---|---|---|
| A1 | 顶栏系统切换器 | 顶栏显示当前系统名 + 下拉箭头；下拉中出现「金融投资」与「内容管理」（内部 key `finvquant` / `cms`）；当前项灰置不可点 |
| A2 | 切换联动 | 切到「内容管理」：左侧只剩「内容管理控制台」；切回「金融投资」：左侧恢复 29 项树形 |
| A3 | 四级深度 | 「数据采集 → 行情数据归档 → 归档统计与预算」上下共四级（相对全局），缩进递增清晰 |
| A4 | 图标 | 二/三/四级叶子均显示图标（已裁决不隐藏） |
| A5 | 叶子可达性 | 30 个叶子（29 业务 + 1 内容管理占位）逐一点击均正常渲染，无 404/白屏 |
| A6 | 激活高亮 | 叶子点击后自身高亮、祖先标题变色、无多重高亮；`webcms` 风格下激活叶子有 3px 主色条 |
| A7 | 深链接直开 | 直接访问 `/Meta/Finv/Quote/DeDup/Export/Stats`：系统判定为「金融投资」、祖先链自动展开、叶子高亮 |
| A8 | 展开持久化 | 展开「元数据管理」并切到「内容管理」再切回：金融投资的展开状态保留 |
| A9 | 页脚 | 登录后每个页面底部显示居中的 `© 2026 ACANX`；内容较短时贴底；登录页不显示 |
| A10 | 视觉风格 | 无风格开关（用户菜单中不再出现）；当前菜单项左侧固定有 3px 主色激活条，祖先标题主色；`grep -c 'WebCMS 视觉风格' dist/assets/index-*.js` 为 0 |
| A11 | 移动端 | 窄屏抽屉为浮层，点击叶子后自动关闭；切换器收起文字不挤压用户菜单 |
| A12 | 键盘 | Tab 可遍历全部节点，回车/空格可展开与跳转 |
| A13 | 无回归 | 登录/登出、修改密码、用户菜单、现有页面布局均不受影响 |
| A14 | 文档 | `Menus.md` 层级与 `menuConfig.ts` 一致；`MenuSpec.md` 已更新；`CMSHome.md` 存在 |

---

## 9. 风险与回滚

| 风险 | 影响 | 缓解 |
|---|---|---|
| 自定义展开组件替代 `v-list-group`，丢失 Vuetify 内置动画/无障碍 | 观感略变、读屏差异 | 自行补 `aria-expanded` + chevron 过渡；视觉与 WebCMS 对齐 |
| 四级 + 全图标导致长标题拥挤（「交易所下设市场信息维护」） | 文本被截断 | 抽屉 300px、三/四级图标 `size="small"`、单行 + `title` 悬浮提示（R5 可再调宽度） |
| 顶栏新增切换器挤压标题/用户菜单 | 小屏排版错乱 | 小屏隐藏切换器文字、标题 `max-width` 收缩（沿用 WebCMS 的 `app-bar-title` 处理） |
| 页脚改变页面底部留白 | 页面视觉位移 | 页脚置于 `v-main` 末尾且容器 `flex-grow-1`，仅登录后渲染；不需要时删 `<footer>` 一段即可 |
| 旧版视觉开关的 localStorage 残留 | 无实际影响（不再读取） | 文档写明旧键 `finvquant.ui.navVisualStyle` 已废弃，可手动清除 |
| 系统归属判定遗漏（`/Dashboard`、`/meta/*`） | 左侧菜单与当前页不一致 | 规则 R3 的 `match` 前缀数组 + 未命中时 `warn` 日志 |
| 展开状态持久化产生"脏数据" | 刷新后展开异常 | `try/catch` + 结构校验，失败回退到激活链 |

**回滚**：全部改动集中在前端 + 文档，单次提交可整体 `git revert`；视觉部分可只切开关（不改代码）回退；无数据库迁移、无接口变更、无部署脚本变更。

---

## 10. 决策记录与剩余待确认

### 10.1 已裁决（2026-10-29，本方案据此更新）

| 编号 | 问题 | 裁决结果 | 落点 |
|---|---|---|---|
| Q1 | 一级菜单形态 | **顶栏下拉切换**；切换后左侧二/三/四级联动（不是左侧树内一级） | §3.1、§3.2 |
| Q2 | 内容管理（CMS）是否本轮落地 | **落地为缺省占位**（系统显示名「内容管理」，内部 key `cms`），不实现业务功能，后续整合 | §2.4 |
| Q3 | 三/四级叶子图标 | **不隐藏，全层级保留图标** | §3.8 |
| Q4 | 展开状态持久化 | **localStorage 持久化**（按系统分桶） | §3.6 |
| Q5 | 一级系统互斥 | **互斥**：单系统渲染 + 展开状态按系统隔离 | §3.3 |
| Q6 | 一级系统点击行为 | **点击后左侧菜单自动完成切换，且左侧菜单可展开/折叠**（在顶栏形态下等价于：切换系统 → 左侧联动 + 分组可折叠；切换时是否跳路由由 R2 决定） | §3.1、§3.2 |
| Q7 | 副标题 `subtitle` | **保留可选字段**，本轮不填内容 | §3.9 |
| Q8 | 配置文件命名 | **参考 CMS 的 `src/config/menuConfig`** → 本项目 `Web/src/config/menuConfig.ts` | §4.1 |
| Q9 | 层级映射 | **旧层级 +1，不强行拉平深度** | §2.1 |
| Q10 | WebCMS 风格视觉 | **全部增加**（主色激活条 / 抽屉标题 / 面包屑），**带开关可切回现状视觉**，默认新风格 | §3.7 |
| Q11 | 系统命名口径（2026-10-29 补充） | **`FinvQuant` 即 CMS 一级菜单「金融投资」分系统**：顶级显示名用「金融投资」，内部 key 保持 `finvquant`；内容管理同理（key `cms`） | §2.5 |

### 10.2 R1–R7（2026-10-29 第二轮：全部采纳推荐值）

| 编号 | 问题 | 裁决（采纳推荐） | 未采纳备选与影响 |
|---|---|---|---|
| R1 | 顶栏平台标题文案：现有「FinvQuant 量化策略交易平台」在切到「内容管理」时仍显示是否合适？ | **A：保持现有文案不变**（不改品牌文案，系统名由切换器体现） | B：`classic` 保持原文案、`webcms` 显示中性标题（如「Meta 管控平台」）；C：标题恒为中性文案（影响 FinvQuant 品牌展示） |
| R2 | 切换系统时的路由策略 | **A：记住每个系统上次访问的路由**，首次进入落到系统落地路由（`金融投资` → `/Dashboard`，`内容管理` → `/Meta/CMS`） | B：每次切换都回系统落地路由（行为最简单、可预期）；C：只切换左侧菜单不改路由（主内容与菜单不一致） |
| R3 | 视觉开关默认值 | **A：默认 `webcms`（新风格）**，方便直接对比效果 | B：默认 `classic`，需手动打开开关才看新风格（更保守） |
| R4 | 内容管理占位菜单的名称与占位文案 | **A：菜单名「内容管理控制台」+ 占位页说明「内容管理系统已接入，业务菜单待整合」** | B：菜单名「内容管理」；C：其他名称/文案 |
| R5 | 抽屉宽度由 260px 增至 300px 是否可接受？ | **A：可接受**（四级带图标必要） | B：保持 260px（三/四级允许省略图标或标题两行）；C：更宽（320px，右侧内容区变窄） |
| R6 | 金融投资的二级结构是否要对齐 CMS「金融投资」下的行情/数据/分析/量化四分组？ | **A：本轮不重组**，保持现有二级（数据采集、历史行情、元数据管理、配置管理…），等与 CMS 业务整合时统一对齐 | B：本轮按行情/数据/分析/量化重组（现有一二级需改挂，可能突破四级深度）；C：部分对齐 |
| R7 | 本项目「元数据管理」菜单（交易所/市场/证券/历史行情导入）在 CMS 体系里属于「元数据」一级系统（`/Meta/Meta`），本轮是否迁移？ | **A：本轮不迁**，保留在金融投资下，记录为整合待办 | B：迁到独立「元数据」系统（需新增一级系统 + 6 条路由前缀重构）；C：仅拆出「历史行情数据导入」 |

> 另：**O1**（`Menus.md` 中仪表盘路由 `/dashboard` → `/Dashboard` 笔误修正）与 **O2**（菜单登记位置从 `App.vue` 迁到 `menuConfig.ts` 并同步改 `MenuSpec.md`）已按默认同意处理。

**审批记录**

| 轮次 | 日期 | 内容 | 结果 |
|---|---|---|---|
| 第一轮 | 2026-10-29 | Q1–Q10 交互与视觉裁决 | 全部裁决，见 §10.1 |
| 第二轮 | 2026-10-29 | Q11 体系定位补充（FinvQuant = CMS「金融投资」）+ R1–R7 | 全部按推荐值采纳 |
| 待实施 | — | 下达「开始实施」指令后按 S1–S5 执行 | 未开始 |

---

## 11. 附：现有菜单与目标层级对照

### 11.1 金融投资（内部 key `finvquant`：29 个叶子 + 7 个分组；分组行路由列为 `—`）

| # | 菜单标题 | 现有层级 | 目标层级 | key | 路由 |
|---|---|---|---|---|---|
| 1 | 仪表盘 | 一级叶子 | 二级叶子 | `dashboard` | `/Dashboard` |
| 2 | 数据采集 | 一级分组 | 二级分组 | `data-collection` | — |
| 3 | 行情数据去重 | 二级分组 | 三级分组 | `quote-dedup` | — |
| 4 | 去重分钟行情查询 | 三级叶子 | 四级叶子 | `dedup-kline-query` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query` |
| 5 | 去重分钟行情统计 | 三级叶子 | 四级叶子 | `dedup-kline-stats` | `/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats` |
| 6 | 行情数据归档 | 二级分组 | 三级分组 | `quote-archive` | — |
| 7 | 归档监控总览 | 三级叶子 | 四级叶子 | `dedup-export-overview` | `/Meta/Finv/Quote/DeDup/Export/Overview` |
| 8 | 端到端链路 | 三级叶子 | 四级叶子 | `dedup-export-chain` | `/Meta/Finv/Quote/DeDup/Export/Chain` |
| 9 | 工作流控制与监控 | 三级叶子 | 四级叶子 | `dedup-export-cdeu` | `/Meta/Finv/Quote/DeDup/CDEU` |
| 10 | 导出任务 | 三级叶子 | 四级叶子 | `dedup-export-job` | `/Meta/Finv/Quote/DeDup/Export/Job` |
| 11 | 文件与上传 | 三级叶子 | 四级叶子 | `dedup-export-file` | `/Meta/Finv/Quote/DeDup/Export/File` |
| 12 | 归档统计与预算 | 三级叶子 | 四级叶子 | `dedup-export-stats` | `/Meta/Finv/Quote/DeDup/Export/Stats` |
| 13 | 归档主数据与规则 | 三级叶子 | 四级叶子 | `dedup-export-config` | `/Meta/Finv/Quote/DeDup/Export/Config` |
| 14 | 归档日志与补丁 | 三级叶子 | 四级叶子 | `dedup-export-log` | `/Meta/Finv/Quote/DeDup/Export/Log` |
| 15 | 历史行情 | 一级分组 | 二级分组 | `history-quote` | — |
| 16 | 历史行情查询 | 二级叶子 | 三级叶子 | `quote-query` | `/Meta/Finv/Quote/History/HistoryQuoteQuery` |
| 17 | 元数据管理 | 一级分组 | 二级分组 | `metadata` | — |
| 18 | 业务元数据维护 | 二级分组 | 三级分组 | `meta-maintenance` | — |
| 19 | 交易所信息维护 | 三级叶子 | 四级叶子 | `meta-exchange` | `/meta/exchange` |
| 20 | 交易所下设市场信息维护 | 三级叶子 | 四级叶子 | `meta-market` | `/meta/market` |
| 21 | 规范证券信息维护 | 三级叶子 | 四级叶子 | `meta-security` | `/meta/security` |
| 22 | 历史行情数据导入 | 三级叶子 | 四级叶子 | `meta-import` | `/meta/import` |
| 23 | 配置管理 | 一级叶子 | 二级叶子 | `config-manage` | `/Meta/Finv/Quant/Config` |
| 24 | 环境管理 | 二级叶子 | **四级叶子**（挂 `quant-mvp-config`） | `environment` | `/Meta/Finv/Quant/Environment` |
| 25 | 模板管理 | 二级叶子 | **四级叶子**（挂 `quant-mvp-config`） | `template` | `/Meta/Finv/Quant/Template` |
| 26 | 账户管理 | 二级叶子 | **四级叶子**（挂 `quant-mvp-config`） | `account` | `/Meta/Finv/Quant/Account` |
| 27 | 资金管理 | 二级叶子 | **四级叶子**（挂 `quant-mvp-config`） | `fund` | `/Meta/Finv/Quant/Fund` |
| 28 | 持仓管理 | 二级叶子 | **四级叶子**（挂 `quant-mvp-config`） | `position` | `/Meta/Finv/Quant/Position` |
| 29 | ~~量化策略验证~~ | 已撤除（需求 10847） | — | `quant-backtest` | — |
| 30 | 黄金期货合约回测验证 | 二级叶子 | **四级叶子**（挂 `quant-backtest-menu`） | `backtest-gold-futures` | `/Meta/Finv/Quant/Backtest/GoldFutures` |
| 31 | 策略管理 | 一级叶子 | **四级叶子**（挂 `quant-strategy`） | `strategy` | `/Meta/Finv/Quant/Strategy` |
| 32 | 回测分析 | 一级叶子 | **四级叶子**（挂 `quant-backtest-menu`） | `backtest-analysis` | `/Meta/Finv/Quant/Backtest/Analysis` |
| 33 | 仿真数据验证 | 一级叶子 | **四级叶子**（挂 `quant-verify`） | `simulation-data` | `/Meta/Finv/Quant/Simulation/Data` |
| 34 | 模拟盘验证 | 一级叶子 | **四级叶子**（挂 `quant-verify`） | `simulation-paper` | `/Meta/Finv/Quant/Simulation/Paper` |
| 35 | 实盘仿真验证 | 一级叶子 | **四级叶子**（挂 `quant-verify`） | `simulation-live-sim` | `/Meta/Finv/Quant/Simulation/LiveSim` |
| 36 | 实盘交易 | 一级叶子 | **四级叶子**（挂 `quant-live`） | `live-trading` | `/Meta/Finv/Quant/LiveTrading` |

### 11.2 内容管理（内部 key `cms`，本轮新增占位）

| # | 菜单标题 | 层级 | key | 路由 | 视图组件 | 说明 |
|---|---|---|---|---|---|---|
| 1 | 内容管理 | 一级系统 | `cms` | `/Meta/CMS` | — | 顶栏切换器入口（与 CMS 一级菜单同名） |
| 2 | 内容管理控制台 | 二级叶子 | `cms-home` | `/Meta/CMS` | `Web/src/views/Common/PlaceholderView.vue` | 占位页，标记「规划中」，业务后续整合 |
