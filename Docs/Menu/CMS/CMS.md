# 内容管理控制台（CMSHome）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/CMS/CMSHome.md`
> 对应视图：`Web/src/views/Common/PlaceholderView.vue`
> 导航方案：[Docs/Refactor/2610-012-WebMenuFourLevelRefactor.md](../../Refactor/2610-012-WebMenuFourLevelRefactor.md)

## 1. 菜单入口

- **菜单位置**：顶栏系统切换器 → 内容管理 → 左侧导航「内容管理控制台」
- **所属系统**：内容管理（内部 key `cms`，路径前缀 `/Meta/CMS`）
- **菜单 key**：`cms-home`
- **菜单图标**：`mdi-view-dashboard-outline`
- **URL 路由**：`/Meta/CMS`
- **对应视图组件**：`Web/src/views/Common/PlaceholderView.vue`

## 2. 业务功能

本菜单为**系统级占位**，本轮不实现任何业务功能，用途：

| 功能 | 说明 |
|------|------|
| 系统入口占位 | 让「内容管理」一级系统在顶栏切换器中可见、可切换，避免空系统被误判为加载失败 |
| 状态说明 | 页面展示「规划中」标记与后续接入计划，明确系统已接入、业务菜单待整合 |
| 导航闭环 | 切换到内容管理后左侧有一项可点击菜单，面包屑可正常生成「内容管理 / 内容管理控制台」 |

## 3. 使用方法

1. 点击顶栏系统切换器（默认显示「金融投资」）；
2. 在下拉列表中选择「内容管理」；
3. 左侧菜单切换为内容管理系统的菜单树（当前仅一项「内容管理控制台」）；
4. 点击「内容管理控制台」查看占位说明（规划中）。

> 切回「金融投资」时，左侧菜单与展开状态恢复为上次的金融投资菜单。

## 4. 处理逻辑

- **路由**：`/Meta/CMS` 在 `Web/src/router.ts` 中登记为 `name: 'cms-home'`，组件为 `PlaceholderView`，`meta.title` 为「内容管理控制台」；
- **系统归属**：`Web/src/config/menuConfig.ts` 中 `cms` 系统的 `match` 为 `['/Meta/CMS']`，路由前缀命中即归属该系统；
- **切换系统**：`SystemSwitcher` 触发 `select` → `App.vue` 的 `onSelectSystem` 依据 `localStorage['finvquant.nav.lastPathBySystem.v1']` 决定目标路由，无记录时落到系统落地路由 `/Meta/CMS`；
- **占位页内容**：`PlaceholderView.vue` 的 `plans` 中按 `route.name`（`cms-home`）取配置，展示图标、说明与规划功能清单；
- **无后端接口**：本菜单不调用任何 API。

## 5. 注意事项

- 本菜单**不提供业务功能**，后续整合 CMS 侧内容管理菜单时，只在 `menuConfig.ts` 的 `cms.children` 中追加节点，无需改动导航组件与路由骨架；
- 占位页复用 `PlaceholderView.vue`，与其他规划中菜单共用组件但各自独立文档（见 [MenuSpec.md](../../DevSpec/MenuSpec.md) §2.2）；
- 若需下线该系统入口，删除 `menuSystems` 中的 `cms` 项即可（左侧菜单与切换器同步消失，无需改其它代码）。
