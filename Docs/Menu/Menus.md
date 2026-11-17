# Web 前端业务菜单总览（Menus）

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/Menus.md`
> 适用范围：`Web/` 前端侧边导航（`Web/src/App.vue` 中 `menuItems` 定义）中的全部业务菜单索引。
> 菜单文档规范见 [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md)；接口文档见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)。

## 菜单层级结构

```
仪表盘（/dashboard）
历史行情（分组）
  └─ 历史行情查询（/quote/query）
元数据管理（分组）
  └─ 业务元数据维护（二级分组）
      ├─ 交易所信息维护（/meta/exchange）
      ├─ 交易所下设市场信息维护（/meta/market）
      ├─ 规范证券信息维护（/meta/security）
      └─ 历史行情数据导入（/meta/import）
```

> 每个叶子菜单对应独立 URL 路由路径（见 `Web/src/router.ts`）；侧边导航支持两级分组展开。

## 菜单清单

| # | 菜单名称 | 层级 | key | 图标 | 路由 | 对应视图组件 | 使用的后端接口 | 菜单文档 |
|---|----------|------|-----|------|------|--------------|----------------|----------|
| 1 | 仪表盘 | 一级叶子 | `dashboard` | `mdi-view-dashboard` | `/dashboard` | `Web/src/views/DashboardView.vue` | `GET /API/V1/health/live`（存活探针） | — |
| 2 | 历史行情 | 一级分组 | `history-quote` | `mdi-chart-box` | — | — | — | — |
| 3 | 历史行情查询 | 二级叶子 | `quote-query` | `mdi-chart-line` | `/quote/query` | `Web/src/views/QuoteQueryView.vue` | `GET /API/V1/Quote/Query`、`GET /API/V1/Meta/FinvQuant/Metadata/Security/Options` | [HistoryQuote/HistoryQuoteQuery.md](HistoryQuote/HistoryQuoteQuery.md) |
| 4 | 元数据管理 | 一级分组 | `metadata` | `mdi-database-cog` | — | — | — | — |
| 5 | 业务元数据维护 | 二级分组 | `meta-maintenance` | `mdi-database-search` | — | — | — | — |
| 6 | 交易所信息维护 | 三级叶子 | `meta-exchange` | `mdi-office-building` | `/meta/exchange` | `Web/src/views/MetaExchangeView.vue` | `GET /API/V1/Meta/FinvQuant/Metadata/Exchange/List`、`POST /API/V1/Meta/FinvQuant/Metadata/Exchange/Save`、`POST /API/V1/Meta/FinvQuant/Metadata/Exchange/Toggle` | [Meta/MetaExchange.md](Meta/MetaExchange.md) |
| 7 | 交易所下设市场信息维护 | 三级叶子 | `meta-market` | `mdi-chart-areaspline` | `/meta/market` | `Web/src/views/MetaMarketView.vue` | `GET /API/V1/Meta/FinvQuant/Metadata/Market/List`、`POST /API/V1/Meta/FinvQuant/Metadata/Market/Save`、`POST /API/V1/Meta/FinvQuant/Metadata/Market/Toggle` | [Meta/MetaMarket.md](Meta/MetaMarket.md) |
| 8 | 规范证券信息维护 | 三级叶子 | `meta-security` | `mdi-tag-multiple` | `/meta/security` | `Web/src/views/MetaSecurityView.vue` | `GET /API/V1/Meta/FinvQuant/Metadata/Security/List`、`POST /API/V1/Meta/FinvQuant/Metadata/Security/Save`、`POST /API/V1/Meta/FinvQuant/Metadata/Security/Toggle`、`GET /API/V1/Meta/FinvQuant/Metadata/Security/Options`、`GET /API/V1/Meta/FinvQuant/Metadata/Security/Lookup` | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) |
| 9 | 历史行情数据导入 | 三级叶子 | `meta-import` | `mdi-database-import` | `/meta/import` | `Web/src/views/QuoteImportView.vue` | `POST /API/V1/Quote/Import/Upload`、`GET /API/V1/Meta/FinvQuant/Metadata/Security/Options`、`GET /API/V1/Meta/FinvQuant/Metadata/Security/Lookup` | [HistoryQuote/HistoryQuoteImport.md](HistoryQuote/HistoryQuoteImport.md) |

## 说明

- 菜单定义与视图组件一一对应，全部在 `Web/src/App.vue` 的 `menuItems` 中登记；新增菜单时须同步更新本索引。
- 图标统一使用 Material Design Icons（`@mdi/font`）；**注意：`mdi-chart-candlestick` 在 @mdi/font 7.4.47 中不存在**，K 线类菜单使用 `mdi-chart-line` / `mdi-chart-box`。
- 三个字典维护菜单（交易所/市场/证券）的 List 分页：**启用的（flag_enable='1'）优先展示，禁用的排后面**，同状态按 code 升序（见 `internal/meta/service.go`）。
- 「菜单文档」列指向对应业务菜单介绍文档（`Docs/Menu/xxx/XXXX.md`，见 [MenuSpec.md](../DevSpec/MenuSpec.md)）；暂无文档的菜单以 `—` 标注，待补充。
- 「使用的后端接口」列与对应 API 接口文档的「已使用位置登记」互为索引（见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 6 节）。
- 视图组件名称与菜单 key 的对应关系：

| 菜单 key | 视图组件 |
|----------|----------|
| `dashboard` | `DashboardView.vue` |
| `quote-query` | `QuoteQueryView.vue` |
| `meta-exchange` | `MetaExchangeView.vue` |
| `meta-market` | `MetaMarketView.vue` |
| `meta-security` | `MetaSecurityView.vue` |
| `meta-import` | `QuoteImportView.vue` |

## 相关文档

- [Docs/DevSpec/MenuSpec.md](../DevSpec/MenuSpec.md) — 前端菜单开发规范
- [Docs/API/README.md](../API/README.md) — 服务端 API 文档入口
- [Docs/API/APIs.md](../API/APIs.md) — 服务端 API 总览（接口清单索引）
