# 五日分钟行情采集配置

> 菜单路径：行情 → 行情采集 → 五日分钟行情采集配置
> 路由：`/Meta/Finv/Quote/RealtimeCollect/FiveDayFt`（路由名 `quote-collect-five-day-ft`，需求 11344）
> 源码：`Web/src/views/Meta/Finv/Quote/RealtimeCollect/FiveDayFtView.vue`
> 接口：[`API/Cdeu/FiveDayFtPage.md`](../../../API/Cdeu/FiveDayFtPage.md)
> 关联需求：11344（查询分页）、11345（新增采集证券）、11346（编辑）
> 数据源：Supabase `public.finv_quote_secu_ft`（**本地库无副本**，经服务端 PostgREST 只读代理）

## 1. 定位

把 Supabase 上的五日分钟行情采集配置表分页展示出来，用于排障与核对：
「哪些证券配了五日分钟采集、开关是开还是关、富途侧编码对不对」。

菜单位置：紧随「USC 映射规则」，位于「行情采集系统配置」**之前**
（先看采集配置表，再看采集链路的系统配置）。

本页支持 **查询分页**、**新增采集证券**（需求 11345）与**编辑单行**（需求 11346）；
不提供删除——删除在 Supabase 侧进行，避免误删线上采集配置。

新增后 **采集开关默认关闭**（`flag_enable=0`），确认无误后再手动改为「启用」。

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
| 新增 | 「重置」右侧；弹对话框输入 usc → 自动联查本地 4 张表补全 32 列 → 人工核对/手填 → 提交 |
| 编辑 | 表格最后一列「编辑」按钮；弹**全屏**对话框，值取自当前行，改动过的单元格标黄并标「已改」；**只提交改动列** |
| 数据表 | 列由服务端下发（21 列中文表头）；数值列右对齐、时间列本地化、`flag_enable` 显示「启用/禁用」并配色、`instrument` 按字典显示中文 |
| 排序 | 独立排序控件（排序列下拉 + 升/降序按钮），走服务端排序，不在前端本地排序 |
| 分页 | 单一分页：总条数、当前页 / 总页数、每页条数（15/20/50/100/200，默认 **15**）、首页/上页/下页/末页、跳页 |
| 空态 | 「无数据（该表当前为空，或筛选条件未命中）」 |

## 4. 列口径（32 列）

| 列 | 中文表头 | 展示口径 |
|---|---|---|
| `usc` | 统一编号 | 原文 |
| `name` | 证券名称 | 原文 |
| `region` | 区域 | 原文 |
| `market` | 市场 | 原文（细分为 NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX…） |
| `market_label` | 行情市场 | 原文 |
| `symbol` | 证券全码 | 原文（`ftmm_symbol`，如 `HK.00700`） |
| `currency` | 计价货币 | 原文 |
| `timezone` | 时区 | 原文 |
| `provider` | 提供方 | 原文 |
| `type_secu` | 证券分类 | 原文 |
| `type_secu_sub` | 证券子类 | 原文 |
| `stock_id` | 富途ID | 右对齐 |
| `market_type` | 市场类型 | 右对齐 |
| `market_code` | 市场代码 | 右对齐 |
| `instrument_type` | 品种类型 | 右对齐 |
| `sub_instrument_type` | 品种子类型 | 右对齐 |
| `req_section` | 订阅段 | 右对齐 |
| `weight_priority` | 采集权重 | 右对齐 |
| `weight_frequency` | 采集频率 | 右对齐 |
| `ts_latest_data` | 最新数据时间戳 | 右对齐 |
| `count_last_fetch` | 累计采集次数 | 右对齐 |
| `count_fail` | 累计失败次数 | 右对齐 |
| `count_stale` | 连续陈旧次数 | 右对齐 |
| `dt_update` | 数据更新时间 | 本地时间 |
| `dt_last_check` | 最近检查时间 | 本地时间 |
| `dt_last_fetch` | 最近采集时间 | 本地时间 |
| `gmt_update` | 更新时间 | 本地时间 |
| `gmt_create` | 创建时间 | 本地时间 |
| `futu_symbol` | 富途代码 | 原文（裸码，如 `00700`） |
| `flag_enable` | 采集开关 | `1`→启用（绿）/ `0`→禁用（灰） |
| `instrument` | 品种 | 字典：`stock`→股票 / `etf`→ETF / `index`→指数 / `forex`→外汇 / `crypto`→加密货币 / `futures`→期货 |
| `remark` | 备注 | 原文 |

空值（`null` / 空串）统一显示 `—`。

## 4.1 新增对话框（需求 11345）

| 步骤 | 行为 |
|---|---|
| ① 输入 usc | 回车或点「补全元数据」→ 调 `GET .../FiveDayFt/Lookup` |
| ② 展示补全结果 | 对话框默认**全屏**（可点标题栏切换）；32 列逐行：「列 / 值 / 来源」三列，值输入框撑满剩余宽度，来源只显示短标签（`富途采集` / `证券主数据` / `键表` / `指标表` / `市场映射` / `默认值` / `留空` / `输入`），缺失列标黄并提示「补全不到，请手填」 |
| ③ 提示条 | 无富途采集记录 / 细分市场需核对 / 已有指标数据 / Supabase 已存在该 usc / 不在证券主数据（红色，禁止提交） |
| ④ 采集开关 | 固定预置「停用[0]（默认，先停采）」，可手动改「启用[1]」 |
| ⑤ 提交 | `POST .../FiveDayFt/Create` → 成功后刷新列表与下拉候选 |

补全口径（用 465 行真实数据校准）：`symbol` 取 `futu.finv_futu_quote_collect.ftmm_symbol`
（缺失再回退 `secu_key` / `indicator`）；`name`/`region`/`currency`/`timezone`/`type_secu*`
取 `public.finv_security`；富途编码类取 `futu.finv_futu_quote_collect`；
`provider` 默认 `FTMM`；`weight_priority` 默认 50、`weight_frequency` 默认 10；
采集统计列留空、时间列由服务端写 `now()`。

## 4.2 编辑对话框（需求 11346）

| 项 | 说明 |
|---|---|
| 打开方式 | 表格行尾「编辑」按钮 |
| 对话框尺寸 | 默认**非全屏**（最大宽度 1280px、高度 88vh），标题栏可一键切换全屏 / 退出全屏 |
| 可改列 | 22 列（32 列去掉 `usc`、5 个时间列、4 个采集统计非时间列） |
| 不可改列 | `usc`（主键）、`gmt_create`/`gmt_update`/`dt_update`（服务端维护）、`dt_last_check`/`dt_last_fetch`/`ts_latest_data`/`count_*`（采集链路维护） |
| 提交内容 | **只提交改动过的列**；清空某列表示置 `null` |
| 回显 | 改动过的单元格黄底 + 右侧标「已改」，未改为「未改」 |
| 保存后 | 自动刷新列表（`gmt_update` 由服务端刷新） |
| 采集开关 | 改成「启用[1]」即进入采集范围，对话框顶部有提醒 |

**写入限制（实测）**：Supabase 的 RLS 对 UPDATE 是**静默过滤**（`200` + 0 行 + `*/*`），
所以页面会提示 403「记录存在但更新影响 0 行：被 RLS 静默过滤」，需在 Supabase 侧执行：

```sql
create policy "finv_quote_secu_ft_update"
  on public.finv_quote_secu_ft
  for update to anon, authenticated
  using (true) with check (true);
```

**「市场」列要注意**：现网 465 行里有 205 行是细分市场（NASDAQ/NYSE/ARCA/USOTC/CER/COMP/HKFE/COMEX/CME/CBOT/NYMEX），
本地 4 张表没有这个粒度，对话框只能给「行情市场」作为建议值，会提示核对。

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
