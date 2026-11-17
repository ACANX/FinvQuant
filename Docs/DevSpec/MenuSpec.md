# 前端菜单开发规范（MenuSpec）

> 所属：FinvQuant 开发规范 · 存放：`Docs/DevSpec/`
> 适用范围：前端（`Web/`）视图菜单的新增、注册与业务文档。
> 文档规范见 [`DocSpec.md`](DocSpec.md)；API 接口文档见 [`ApiSpec.md`](ApiSpec.md)。

## 1. 菜单新增

- 前端视图组件存放于 `Web/src/views/`，每个菜单对应一个视图组件（如 `QuoteQueryView.vue`）。
- 新增菜单时须在 `Web/src/App.vue` 的 `menuItems` 中登记：菜单 `key`（英文驼峰）、`title`（简体中文标题）、`icon`（mdi 图标）。
- 视图组件与 `menuItems` 中的菜单项一一对应，禁止出现未登记入口的孤立视图。

## 2. 菜单文档

- **新添加的前端视图菜单，必须补充对应的业务菜单介绍文档**，介绍该菜单的业务功能、处理逻辑、使用方法、注意事项等。
- 文档位置通常在 `Docs/Menu/xxx/XXXX.md`（按业务模块建目录，文档名与菜单/视图名对应）。
- 参考示例：[`Docs/Menu/HistoryQuote/HistoryQuoteQuery.md`](../Menu/HistoryQuote/HistoryQuoteQuery.md)（对应视图 `Web/src/views/QuoteQueryView.vue`）。
- 菜单文档随视图代码在同一变更中提交，保持「代码 + 文档」同步交付。
- 新增菜单后须在 [`Menus.md`](../Menu/Menus.md) 菜单总索引中登记（菜单名称 / key / 图标 / 视图组件 / 使用的后端接口 / 菜单文档）。

## 3. 文档内容要求

业务菜单介绍文档应至少覆盖以下内容：

| 章节 | 说明 |
|------|------|
| 菜单入口 | 菜单位置、名称、key、图标、对应视图组件 |
| 业务功能概述 | 该菜单提供哪些能力（功能清单） |
| 使用方法 | 用户操作流程与步骤说明 |
| 处理逻辑 | 前端交互逻辑与后端处理链路（含请求参数、响应处理） |
| 注意事项 | 常见问题、限制与排查提示 |

- **业务菜单中使用到的后端 API 接口，须在业务菜单对应的文档中添加对应的 API 接口或接口文档引用**：逐一列出菜单调用的接口路径（如 `GET /API/V1/Quote/Query`）并链接到对应接口文档（`Docs/API/xxx/XXXX.md`），便于定位后端契约与排查问题。
- 菜单文档中的接口引用与 API 文档中的「已使用位置登记」（见 [ApiSpec.md](ApiSpec.md) 第 6 节）形成双向索引，接口文档登记使用方、菜单文档登记所用接口，须保持一致。
