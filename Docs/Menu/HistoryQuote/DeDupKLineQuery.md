# 去重分钟行情查询（DeDupKLineQuery）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/HistoryQuote/DeDupKLineQuery.md`
> 对应视图：`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue`
> 接口契约：见 [Docs/API/KLine/KLinePage.md](../../API/KLine/KLinePage.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 数据采集 → 行情数据去重 → 去重分钟行情查询
- **菜单 key**：`dedup-kline-query`
- **菜单图标**：`mdi-table-search`
- **URL 路由**：`/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Query`
- **对应视图组件**：`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineQueryView.vue`

## 2. 业务功能

查询 `dedup.finv_quote_secu_kline_min` 中**按 `(usc, ts)` 去重后**的分钟行情，支持：

| 功能 | 说明 |
|------|------|
| 证券筛选 | `usc` 下拉（`finv_security` 字典，展示 `usc:security_name_cn`），允许手动输入；留空查询全部证券 |
| 日期范围 | 开始/结束日期（含），前端转换为 `yyyymmdd` 传 `dateFrom`/`dateTo` |
| 分页 | 服务端 `Page` 接口，`limit` 50/100/200/500/1000，`offset` 由页码换算 |
| 脱敏展示 | 表格展示 `usc/date/time/o/h/l/c/v/t/cp/cr/pv/ext`；数值千分位、日期与时间格式化、涨跌幅加 `%` |

> 列表不输出 `remark` 与 `last_close`（与 `/Page` 接口契约一致）。

## 3. 默认查询条件

- 证券代码：默认 **`GCMain`**；
- 日期范围：默认**最近一个月**——以库内该证券最新交易日期为结束日、往前回推 1 个月（如当前库最新交易日为 2026-07-21，则默认 `2026-06-21` ~ `2026-07-21`）；取不到数据日期时回退本机当天；
- 「重置」恢复上述默认条件。

## 4. 请求与响应

- 请求：`GET /API/V1/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Page?usc=&dateFrom=&dateTo=&limit=&offset=`
- 响应：`data.cnt`（总数）、`data.list`（紧凑字段数组）。
- 错误：`body.code !== 0` 时展示 `message`；网络异常提示“网络错误：无法连接服务端”。

## 5. 交互约定

- 点击「查询」：页码回到第 1 页并重新请求；若已在第 1 页则直接刷新。
- 点击「重置」：清空证券与日期条件后回到第 1 页查询。
- 修改分页页码/每页条数：自动请求。
- 开始日期晚于结束日期：前端直接提示，不发起请求。
- 时间字段 `0` 正常展示为 `00:00:00`，仅 `null` 展示 `-`。
