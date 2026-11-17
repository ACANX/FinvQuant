# 服务端 API 总览（APIs）

> 所属：FinvQuant 服务端 · 存放：`Docs/API/APIs.md`
> 适用范围：服务端（Go + Gin）`/API/V1` 路由组下的全部接口清单索引。
> 基路径：`/API/V1`（统一大写，见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)）；服务端端口 **16001**。
> 接口文档规范见 [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md)；菜单使用方索引见 [Docs/Menu/Menus.md](../Menu/Menus.md)。

## API 清单

| # | 方法 | 路径 | 说明 | 详细文档 | 已使用位置（业务菜单） |
|---|------|------|------|----------|------------------------|
| 1 | GET | `/API/V1/health/live` | 存活探针：进程存活即 200，返回 `status` / `server` | — | 仪表盘（`DashboardView.vue`） |
| 2 | GET | `/API/V1/health/ready` | 就绪探针：PG 与 Redis 均可达才 200，否则 503 | — | — |
| 3 | GET | `/API/V1/version` | 服务端版本信息：`name` / `version` / `go_version` / `commit` | — | — |
| 4 | POST | `/API/V1/Quote/Import/Upload` | 上传 MVSV 历史行情文件并导入 PG（字段级/整行覆盖） | [HistoryQuote/ImportsUpload.md](HistoryQuote/ImportsUpload.md) | 历史行情数据导入（`QuoteImportView.vue`） |
| 5 | GET | `/API/V1/Quote/Query` | 按证券代码+日期查询分钟级 K 线（周期 Min，1日/5日回溯，分页；可选回传证券名称 secu_name） | [HistoryQuote/HistoryQuote.md](HistoryQuote/HistoryQuote.md) | 历史行情查询（[Menus.md](../Menu/Menus.md#3) 之历史行情查询菜单，文档见 [HistoryQuote/HistoryQuoteQuery.md](../Menu/HistoryQuote/HistoryQuoteQuery.md)） |
| 6 | GET | `/API/V1/Meta/FinvQuant/Metadata/Exchange/List` | 分页查询交易所字典 finv_exchange | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（[Menus.md](../Menu/Menus.md#6)，文档见 [Meta/MetaExchange.md](../Menu/Meta/MetaExchange.md)） |
| 7 | POST | `/API/V1/Meta/FinvQuant/Metadata/Exchange/Save` | 新增/修改交易所字典 | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（同上） |
| 8 | POST | `/API/V1/Meta/FinvQuant/Metadata/Exchange/Toggle` | 禁用/启用交易所 | [Meta/MetaExchange.md](Meta/MetaExchange.md) | 交易所信息维护（同上） |
| 9 | GET | `/API/V1/Meta/FinvQuant/Metadata/Market/List` | 分页查询交易市场字典 finv_market | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（[Menus.md](../Menu/Menus.md#7)，文档见 [Meta/MetaMarket.md](../Menu/Meta/MetaMarket.md)） |
| 10 | POST | `/API/V1/Meta/FinvQuant/Metadata/Market/Save` | 新增/修改交易市场字典 | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（同上） |
| 11 | POST | `/API/V1/Meta/FinvQuant/Metadata/Market/Toggle` | 禁用/启用交易市场 | [Meta/MetaMarket.md](Meta/MetaMarket.md) | 交易所下设市场信息维护（同上） |
| 12 | GET | `/API/V1/Meta/FinvQuant/Metadata/Security/List` | 分页查询证券代码字典 finv_security | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（[Menus.md](../Menu/Menus.md#8)，文档见 [Meta/MetaSecurity.md](../Menu/Meta/MetaSecurity.md)） |
| 13 | POST | `/API/V1/Meta/FinvQuant/Metadata/Security/Save` | 新增/修改证券代码字典 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上） |
| 14 | POST | `/API/V1/Meta/FinvQuant/Metadata/Security/Toggle` | 禁用/启用证券代码 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上） |
| 15 | GET | `/API/V1/Meta/FinvQuant/Metadata/Security/Options` | 证券下拉选项（usc+security_name_cn，仅启用状态） | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 规范证券信息维护（同上）、历史行情查询（[HistoryQuote/HistoryQuoteQuery.md](../Menu/HistoryQuote/HistoryQuoteQuery.md)） |
| 16 | GET | `/API/V1/Meta/FinvQuant/Metadata/Security/Lookup` | 按代码（usc 或源证券代码）查询证券详情，供导入双策略带出/匹配 | [Meta/MetaSecurity.md](Meta/MetaSecurity.md) | 历史行情数据导入（[Menus.md](../Menu/Menus.md#9)，视图 `QuoteImportView.vue`） |

## 说明

- 接口清单以 `internal/api/router.go` 中 `/API/V1` 路由组注册为准；新增接口须同步更新本索引及 [Docs/API/README.md](README.md) 端点索引。
- 「详细文档」列指向对应接口文档（`Docs/API/xxx/XXXX.md`，见 [ApiSpec.md](../DevSpec/ApiSpec.md) 第 6 节）；暂无独立文档的接口以 `—` 标注。
- 「已使用位置（业务菜单）」列与菜单文档中的接口引用互为索引（见 [MenuSpec.md](../DevSpec/MenuSpec.md) 第 3 节）；接口新增使用方或下线时须同步更新。
- 通用探针/信息接口（health、version）不强制要求独立文档，但契约变更须同步本索引。

## 相关文档

- [Docs/API/README.md](README.md) — API 文档入口（响应信封、错误码、端点索引）
- [Docs/DevSpec/ApiSpec.md](../DevSpec/ApiSpec.md) — API 开发规范
- [Docs/Menu/Menus.md](../Menu/Menus.md) — Web 前端业务菜单总览
