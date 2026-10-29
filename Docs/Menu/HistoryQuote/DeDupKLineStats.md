# 去重分钟行情统计（DeDupKLineStats）— 菜单业务文档

> 所属：FinvQuant 前端菜单 · 存放：`Docs/Menu/HistoryQuote/DeDupKLineStats.md`
> 对应视图：`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue`
> 接口契约：见 [Docs/API/KLine/KLineStats.md](../../API/Quote/Cdeu/Min/KLineStats.md)

## 1. 菜单入口

- **菜单位置**：左侧导航栏 → 数据采集 → 行情数据去重 → 去重分钟行情统计
- **菜单 key**：`dedup-kline-stats`
- **菜单图标**：`mdi-chart-box-outline`
- **URL 路由**：`/Meta/Finv/Quote/DeDup/Secu/KLine/Min/Stats`
- **对应视图组件**：`Web/src/views/Meta/Finv/Quote/DeDup/DeDupKLineStatsView.vue`

## 2. 业务功能

对 `dedup.finv_quote_secu_kline_min` 做**简单统计分析与可视化**：

| 模块 | 内容 |
|------|------|
| KPI 卡片 | 记录总数、证券数、交易日数、成交量合计、成交额合计 |
| 汇总信息 | 日期范围、收盘价区间/均值、当前过滤条件 |
| 字段完整性 | `date/open/high/low/close/last_close/change_price/change_ratio/volume/turnover/provider` 空值行数（0 为正常） |
| 每日趋势 | 四个独立图表：每日记录数（柱）、每日成交量（柱）、每日成交额（柱）、每日收盘价均值（折线）；四者共用时间轴刻度与绘图区网格，横向对齐 |
| 证券分布 | 记录数 Top 20 证券（横向柱状图） |

> 布局约定：「每日记录数」「每日成交量」「每日成交额」「每日收盘价均值」「字段完整性（空值行数）」各占独立一行；四个每日图表绘图区左右边界一致、时间轴对齐；图表坐标轴与页面数字默认不使用千分位。

## 3. 默认查询条件

- 证券代码：默认 **`GCMain`**；
- 日期范围：默认**最近一个月**——以库内该证券最新交易日期为结束日、往前回推 1 个月（如当前库最新交易日为 2026-07-21，则默认 `2026-06-21` ~ `2026-07-21`）；取不到数据日期时回退本机当天；
- 「重置」恢复上述默认条件。

## 4. 请求与响应

- 请求：`GET /API/V1/Meta/Finv/Quote/Cdeu/Min/Stats?usc=&dateFrom=&dateTo=`
- 响应：`data.summary`（汇总 + `null_counts`）、`data.daily`（按日期升序）、`data.top_usc`（Top 20）。
- 三组结果在同一只读事务内查询，口径一致。

## 5. 交互约定

- 证券下拉可清空表示“全部证券”；日期范围可只填一端。
- 点击「统计」请求最新数据；点击「重置」清空条件后重新统计。
- 图表在容器尺寸变化（导航栏折叠/窗口缩放）时自动 `resize`；无数据时显示空态提示。
- 开始日期晚于结束日期：前端直接提示，不发起请求。
- 时间字段 `0` 正常展示为 `00:00:00`，仅 `null` 展示 `-`。
