# 五日分钟行情采集配置

> 菜单路径：行情 → 行情采集 → 五日分钟行情采集配置
> 路由：`/Meta/Finv/Quote/RealtimeCollect/FiveDayFt`（路由名 `quote-collect-five-day-ft`，需求 11344）
> 源码：`Web/src/views/Meta/Finv/Quote/RealtimeCollect/FiveDayFtView.vue`
> 接口：[`API/Cdeu/FiveDayFtPage.md`](../../../API/Cdeu/FiveDayFtPage.md)
> 数据源：Supabase `public.finv_quote_secu_ft`（**本地库无副本**，经服务端 PostgREST 只读代理）

## 1. 定位

把 Supabase 上的五日分钟行情采集配置表分页展示出来，用于排障与核对：
「哪些证券配了五日分钟采集、开关是开还是关、富途侧编码对不对」。

菜单位置：紧随「USC 映射规则」，位于「行情采集系统配置」**之前**
（先看采集配置表，再看采集链路的系统配置）。

本页**只读**：该表是采集链路的配置源，改动在 Supabase 侧进行，服务端不提供写接口。

## 2. 连接与鉴权

| 项 | 说明 |
|---|---|
| 连接来源 | `sys.sys_config` 20013 `{PROJECT_ID}\|{APIKEY}`（可在「行情采集系统配置」页维护） |
| 通道 | PostgREST `GET https://{PROJECT_ID}.supabase.co/rest/v1/finv_quote_secu_ft` |
| 复用 | `internal/meta/finv/quote/secukey` 的 `FetchQuery`（鉴权头 / 重试 / 错误映射与证券键表同步一致） |
| 安全 | APIKEY 只在进程内使用，**不下发前端、不写日志**；前端只看到页面数据 |

## 3. 页面行为

| 区域 | 说明 |
|---|---|
| 顶部 chip | 总条数、上游耗时（`elapsed_ms`） |
| 说明卡片 | 数据源与「只读」声明 |
| 筛选区 | 关键字（统一编号 / 代码 / 富途代码 / 名称）+ 行情市场、品种、证券分类、提供方四个下拉 + 采集开关（全部 / 启用 / 禁用） |
| 查询 / 重置 | 查询回到第 1 页；重置清空全部条件并恢复默认排序（`usc` 升序） |
| 数据表 | 列由服务端下发（21 列中文表头）；数值列右对齐、时间列本地化、`flag_enable` 显示「启用/禁用」并配色、`instrument` 按字典显示中文 |
| 排序 | 独立排序控件（排序列下拉 + 升/降序按钮），走服务端排序，不在前端本地排序 |
| 分页 | 单一分页：总条数、当前页 / 总页数、每页条数（15/20/50/100/200，默认 **15**）、首页/上页/下页/末页、跳页 |
| 空态 | 「无数据（该表当前为空，或筛选条件未命中）」 |

## 4. 列口径（21 列）

| 列 | 中文表头 | 展示口径 |
|---|---|---|
| `usc` | 统一编号 | 原文 |
| `symbol` | 证券代码 | 原文 |
| `futu_symbol` | 富途代码 | 原文 |
| `name` | 证券名称 | 原文 |
| `stock_id` | 富途ID | 右对齐 |
| `market` | 市场 | 原文 |
| `market_label` | 行情市场 | 原文 |
| `market_type` | 市场类型 | 右对齐 |
| `market_code` | 市场代码 | 右对齐 |
| `instrument` | 品种 | 字典：`stock`→股票 / `etfs`→ETF / `index`→指数 / `currency`→外汇 / `crypto`→加密货币 / `futures`→期货 / `sectors`→板块 |
| `instrument_type` | 品种类型 | 右对齐 |
| `sub_instrument_type` | 品种子类型 | 右对齐 |
| `type_secu` | 证券分类 | 原文 |
| `type_secu_sub` | 证券子类 | 原文 |
| `req_section` | 订阅段 | 右对齐 |
| `provider` | 提供方 | 原文 |
| `flag_enable` | 采集开关 | `1`→启用（绿）/ `0`→禁用（灰） |
| `remark` | 备注 | 原文 |
| `gmt_create` | 创建时间 | 本地时间 |
| `gmt_update` | 更新时间 | 本地时间 |
| `dt_update` | 数据更新 | 本地时间 |

空值（`null` / 空串）统一显示 `—`。

## 5. 当前数据量（2026-11-19 实测）

`GET .../FiveDayFt/Page` 返回 `total=0`、`list=[]`、`columns=21`、`elapsed_ms=839`；
直接对 PostgREST 请求得到 `Content-Range: */0`。

同一把 APIKEY 能读到 `public.finv_quote_secu_key` 的 73,212 行，因此**不是鉴权问题**：

| 现象 | 结论 |
|---|---|
| Supabase 控制台里也看不到数据 | 表本来就是空的，等采集侧写入，页面空态属正常 |
| 控制台能看到数据、本页为 0 行 | 该表开了 RLS 且策略没向该 key 开放 `select`，需在 Supabase 侧补策略 |

## 6. 故障排查

| 现象 | 原因 | 处理 |
|---|---|---|
| 「Supabase 连接不可用」 | 20013 未配置或格式错 | 到「行情采集系统配置」页补 `{PROJECT_ID}\|{APIKEY}` |
| 「Supabase 鉴权失败（HTTP 401/403）」 | APIKEY 失效 | 换 key 后保存 |
| 「Supabase 源表不存在或不可读（HTTP 404）」 | 表被改名/删了 | 确认 Supabase 侧 `public.finv_quote_secu_ft` 是否存在 |
| 翻到后面几页列表为空 | 该页超出总行数 | 正常（服务端把 416 当空页），点「首页」或改每页条数 |
| 下拉没有选项 | 表为空 | 正常，表有数据后自动出现 |
